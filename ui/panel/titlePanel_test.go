package panel

import (
	"strings"
	"testing"
)

func TestTitleASCIIDims(t *testing.T) {
	w, h := TitleASCIIDims("ab\ncdef\nx")
	if w != 4 || h != 3 {
		t.Fatalf("TitleASCIIDims = %d,%d; want 4,3", w, h)
	}
	w, h = TitleASCIIDims("")
	if w != 0 || h != 0 {
		t.Fatalf("empty dims = %d,%d; want 0,0", w, h)
	}
}

func TestDrawTitleASCIICenters(t *testing.T) {
	art := "HI"
	got := DrawTitleASCII(art, 6, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("height %d; want 5\n%q", len(lines), got)
	}
	// Vertical center: padTop = (5-1)/2 = 2 → line index 2 holds art
	if lines[2] != "  HI" {
		t.Fatalf("centered line = %q; want %q", lines[2], "  HI")
	}
	for i, line := range lines {
		if i == 2 {
			continue
		}
		if line != "" {
			t.Fatalf("line %d = %q; want empty", i, line)
		}
	}
}

func TestDrawTitleASCIIEmpty(t *testing.T) {
	if got := DrawTitleASCII("", 10, 5); got != "" {
		t.Fatalf("empty art = %q; want empty", got)
	}
}

func TestDrawTitleASCIIClipsWide(t *testing.T) {
	got := DrawTitleASCII("ABCDEFGH", 4, 1)
	if got != "ABCD" {
		t.Fatalf("clip = %q; want ABCD", got)
	}
}
