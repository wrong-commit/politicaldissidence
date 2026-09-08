package searching

import (
	"strings"
	"testing"
)

func TestDuckDuckGoOffset(t *testing.T) {
	cases := []struct {
		page    int
		wantS   int
		wantOK  bool
	}{
		{0, 0, false},
		{-1, 0, false},
		{1, 10, true},
		{2, 25, true},
		{3, 40, true},
	}
	for _, tc := range cases {
		got, ok := DuckDuckGoOffset(tc.page)
		if ok != tc.wantOK || got != tc.wantS {
			t.Fatalf("DuckDuckGoOffset(%d)=(%d,%v) want (%d,%v)", tc.page, got, ok, tc.wantS, tc.wantOK)
		}
	}
}

func TestDuckDuckGoForm(t *testing.T) {
	p0 := duckDuckGoForm("golang", 0, "")
	if p0.Get("q") != "golang" || p0.Get("b") != "" || p0.Get("s") != "" {
		t.Fatalf("page0 form: %v", p0)
	}

	p1 := duckDuckGoForm("golang", 1, "vqd-token")
	if p1.Get("s") != "10" || p1.Get("dc") != "11" {
		t.Fatalf("page1 s/dc: %v", p1)
	}
	if p1.Get("vqd") != "vqd-token" || p1.Get("api") != "d.js" || p1.Get("v") != "l" {
		t.Fatalf("page1 paging fields: %v", p1)
	}
	if _, ok := p1["b"]; ok {
		t.Fatalf("page1 should omit b, got %v", p1)
	}

	p2 := duckDuckGoForm("golang", 2, "tok")
	if p2.Get("s") != "25" || p2.Get("dc") != "26" {
		t.Fatalf("page2 s/dc: %v", p2)
	}
}

func TestExtractVQD(t *testing.T) {
	html := `<html><body><form><input type="hidden" name="vqd" value="abc123"><input name="s" value="10"></form></body></html>`
	if got := extractVQD(html); got != "abc123" {
		t.Fatalf("extractVQD=%q want abc123", got)
	}
	if got := extractVQD("<html></html>"); got != "" {
		t.Fatalf("empty doc extractVQD=%q", got)
	}
	if !strings.Contains(html, "vqd") {
		t.Fatal("sanity")
	}
}
