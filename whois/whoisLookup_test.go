package whois

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return string(b)
}

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"strips www", "www.example.com", "example.com"},
		{"bare host unchanged", "example.com", "example.com"},
		{"www mid-label unchanged", "wwwexample.com", "wwwexample.com"},
		{"empty stays empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clean(tt.in); got != tt.want {
				t.Fatalf("clean(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetExpiryWith_Success(t *testing.T) {
	raw := testdata(t, "example_com.txt")
	got, err := GetExpiryWith("example.com", func(hostname string) (string, error) {
		return raw, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty expiry")
	}
	if strings.Contains(got, "Could not get WHOIS") || strings.Contains(got, "Could not parse") {
		t.Fatalf("expiry looks like error message: %q", got)
	}
	if !strings.Contains(got, "2027") {
		t.Fatalf("expected expiry containing 2027, got %q", got)
	}
}

func TestGetExpiryWith_FetchError(t *testing.T) {
	fetchErr := errors.New("network down")
	msg, err := GetExpiryWith("example.com.au", func(hostname string) (string, error) {
		return "", fetchErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fetchErr) && err.Error() != fetchErr.Error() {
		// fetcher error is returned as-is
		if err != fetchErr {
			t.Fatalf("want fetch error, got %v", err)
		}
	}
	if !strings.Contains(msg, "Could not get WHOIS for <example.com.au>") {
		t.Fatalf("message = %q", msg)
	}
}

func TestGetExpiryWith_ParseError(t *testing.T) {
	raw := testdata(t, "garbage.txt")
	msg, err := GetExpiryWith("broken.example", func(hostname string) (string, error) {
		return raw, nil
	})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(msg, "Could not parse <broken.example>") {
		t.Fatalf("message = %q", msg)
	}
}

func TestGetExpiryWith_CleansBeforeFetch(t *testing.T) {
	var fetched string
	raw := testdata(t, "example_com.txt")
	_, err := GetExpiryWith("www.example.com", func(hostname string) (string, error) {
		fetched = hostname
		return raw, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched != "example.com" {
		t.Fatalf("fetcher got %q, want example.com", fetched)
	}
}

func TestGetExpiryWith_EmptyExpiry(t *testing.T) {
	// Documented: parse succeeds without expiration → empty string, nil error.
	raw := testdata(t, "example_com_no_expiry.txt")
	got, err := GetExpiryWith("example.com", func(hostname string) (string, error) {
		return raw, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v (got %q)", err, got)
	}
	if got != "" {
		t.Fatalf("want empty expiry, got %q", got)
	}
}
