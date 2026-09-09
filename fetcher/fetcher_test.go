package fetcher

import (
	"errors"
	"testing"
)

func TestFindAnchorHref(t *testing.T) {
	html := `<html><body>
<a href="/ Senators /foo">nope</a>
<a href="/Downloads/allsenel.csv">Senators CSV</a>
</body></html>`
	got, err := FindAnchorHref([]byte(html), "allsenel.csv")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/Downloads/allsenel.csv" {
		t.Fatalf("href=%q", got)
	}
}

func TestFindAnchorHref_Missing(t *testing.T) {
	_, err := FindAnchorHref([]byte(`<html><a href="/other.csv">x</a></html>`), "allsenel.csv")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveCSVURL_Relative(t *testing.T) {
	got, err := ResolveCSVURL(
		"https://www.aph.gov.au/Senators_and_Members/page",
		"/Downloads/allsenel.csv",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.aph.gov.au/Downloads/allsenel.csv"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveCSVURL_Absolute(t *testing.T) {
	abs := "https://cdn.example/allsenel.csv"
	got, err := ResolveCSVURL("https://www.aph.gov.au/page", abs)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q", got)
	}
}

func TestFindCSVURL(t *testing.T) {
	page := "https://example.org/list"
	html := `<html><a href="/files/allsenel.csv">csv</a></html>`
	get := func(u string) ([]byte, error) {
		if u != page {
			t.Fatalf("get %q", u)
		}
		return []byte(html), nil
	}
	got, err := FindCSVURL(page, "allsenel.csv", get)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.org/files/allsenel.csv" {
		t.Fatalf("got %q", got)
	}
}

func TestFindCSVURL_GetError(t *testing.T) {
	_, err := FindCSVURL("https://example.org", "x.csv", func(string) ([]byte, error) {
		return nil, errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
