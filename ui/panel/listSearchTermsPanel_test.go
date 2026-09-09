package panel

import (
	"strings"
	"testing"
)

func TestDrawSearchTermsPanel(t *testing.T) {
	text := DrawSearchTermsPanel("T2/5", "Name + party", "Jane Doe IND")
	if !strings.Contains(text, searchTermsStatusText) {
		t.Fatalf("missing status:\n%s", text)
	}
	if !strings.Contains(text, "T2/5 · Name + party") {
		t.Fatalf("missing header:\n%s", text)
	}
	if !strings.Contains(text, "Search: Jane Doe IND") {
		t.Fatalf("missing search:\n%s", text)
	}
}
