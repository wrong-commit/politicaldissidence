package registrarcheck

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIsImplemented(t *testing.T) {
	if !IsImplemented(SourceGoDaddy) {
		t.Fatal("godaddy should be implemented")
	}
	if IsImplemented(SourceNamecheap) {
		t.Fatal("namecheap should not be implemented")
	}
	if IsImplemented("unknown") {
		t.Fatal("unknown should not be implemented")
	}
}

func TestLookup_NamecheapNotImplemented(t *testing.T) {
	info, err := Lookup("example.com", SourceNamecheap)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("err=%v", err)
	}
	if info.Message != "not implemented" {
		t.Fatalf("message=%q", info.Message)
	}
	if FormatNotImplemented(SourceNamecheap, "example.com") != "ERROR registrar namecheap example.com: not implemented" {
		t.Fatal(FormatNotImplemented(SourceNamecheap, "example.com"))
	}
}

func TestLookupGoDaddy_Classify(t *testing.T) {
	creds := func() (string, string, bool) { return "k", "s", true }

	t.Run("available true", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if !strings.Contains(r.URL.RawQuery, "checkType=FULL") {
				t.Fatalf("query=%s", r.URL.RawQuery)
			}
			if r.Header.Get("Authorization") != "sso-key k:s" {
				t.Fatalf("auth=%q", r.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"available":true,"domain":"example.com.au"}`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("www.example.com.au", SourceGoDaddy, client, creds)
		if err != nil {
			t.Fatal(err)
		}
		if info.Hostname != "example.com.au" {
			t.Fatalf("hostname=%q", info.Hostname)
		}
		if info.Purchaseable != TriYes || info.WeirdResponse != TriNo {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("available false", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"available":false}`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err != nil {
			t.Fatal(err)
		}
		if info.Purchaseable != TriNo || info.WeirdResponse != TriNo {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err == nil {
			t.Fatal("expected err")
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriYes {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("missing available", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"domain":"x.com"}`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err == nil {
			t.Fatal("expected err")
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriYes {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("available not boolean", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"available":"yes"}`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err != nil {
			t.Fatal(err)
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriWeird {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("available with error object", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"available":true,"error":{"code":"X"}}`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err != nil {
			t.Fatal(err)
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriWeird {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("HTTP 401", func(t *testing.T) {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 401,
				Body:       io.NopCloser(strings.NewReader(`unauthorized`)),
				Header:     make(http.Header),
			}, nil
		})}
		info, err := LookupWith("example.com", SourceGoDaddy, client, creds)
		if err == nil {
			t.Fatal("expected err")
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriYes {
			t.Fatalf("%+v", info)
		}
	})

	t.Run("missing credentials", func(t *testing.T) {
		info, err := LookupWith("example.com", SourceGoDaddy, nil, func() (string, string, bool) {
			return "", "", false
		})
		if err == nil {
			t.Fatal("expected err")
		}
		if info.Purchaseable != TriWeird || info.WeirdResponse != TriYes {
			t.Fatalf("%+v", info)
		}
		if info.Message != "missing GoDaddy credentials" {
			t.Fatalf("message=%q", info.Message)
		}
	})
}
