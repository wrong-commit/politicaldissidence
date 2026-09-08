package panel

import (
	"strings"
	"testing"

	"politicaldissidence/searching"
)

func TestURLListItemIndex(t *testing.T) {
	if got := URLListItemIndex(0); got != -1 {
		t.Fatalf("status line: got %d want -1", got)
	}
	if got := URLListItemIndex(1); got != -1 {
		t.Fatalf("search term line: got %d want -1", got)
	}
	if got := URLListItemIndex(2); got != 0 {
		t.Fatalf("item0 host: got %d want 0", got)
	}
	if got := URLListItemIndex(3); got != 0 {
		t.Fatalf("item0 path: got %d want 0", got)
	}
	if got := URLListItemIndex(4); got != 0 {
		t.Fatalf("item0 blank: got %d want 0", got)
	}
	if got := URLListItemIndex(5); got != 1 {
		t.Fatalf("item1 host: got %d want 1", got)
	}
	if got := URLListCursorY(0); got != 2 {
		t.Fatalf("cursor0: got %d want 2", got)
	}
	if got := URLListCursorY(2); got != 8 {
		t.Fatalf("cursor2: got %d want 8", got)
	}
}

func TestSplitHostPath(t *testing.T) {
	host, path, ok := splitHostPath("https://foobar.com.au/website/sub/directory/opath")
	if !ok || host != "foobar.com.au" || path != "/website/sub/directory/opath" {
		t.Fatalf("got host=%q path=%q ok=%v", host, path, ok)
	}
	host, path, ok = splitHostPath("https://example.com")
	if !ok || host != "example.com" || path != "/" {
		t.Fatalf("no path: host=%q path=%q ok=%v", host, path, ok)
	}
	host, path, ok = splitHostPath("http://user:pass@ex.com:8080/x?q=1")
	if !ok || host != "ex.com" || path != "/x?q=1" {
		t.Fatalf("creds/port: host=%q path=%q ok=%v", host, path, ok)
	}
}

func TestDrawListUrlPanel(t *testing.T) {
	links := []searching.Link{
		{"https://foobar.com.au/website/sub/directory/opath", "desc"},
		{"https://already.example/path", "d2"},
		{"", "bad"},
	}
	text, _, _, display, err := DrawListUrlPanel(nil, links, []string{"Already.Example"}, "jane doe mp")
	if err == nil {
		t.Fatal("expected error for empty link")
	}
	if len(display) != 2 {
		t.Fatalf("display len=%d want 2", len(display))
	}
	if !strings.Contains(text, urlListStatusText) {
		t.Fatalf("missing status:\n%s", text)
	}
	if !strings.Contains(text, "Search: jane doe mp") {
		t.Fatalf("missing search term:\n%s", text)
	}
	if !strings.Contains(text, "1. foobar.com.au") {
		t.Fatalf("missing host line:\n%s", text)
	}
	if !strings.Contains(text, "/website/sub/directory/opath") {
		t.Fatalf("missing path:\n%s", text)
	}
	if !strings.Contains(text, "\x1b[90m") {
		t.Fatalf("expected muted already-added domain:\n%s", text)
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	// status + term + 2 items × (host, path, blank) = 8 lines
	if len(lines) < 8 {
		t.Fatalf("want status + term + 2×3-line items, got %d:\n%s", len(lines), text)
	}
	if lines[0] != urlListStatusText {
		t.Fatalf("expected status first, got %q", lines[0])
	}
	if lines[1] != "Search: jane doe mp" {
		t.Fatalf("expected search term second, got %q", lines[1])
	}
	if lines[4] != "" {
		t.Fatalf("expected blank after first path, got %q", lines[4])
	}
}
