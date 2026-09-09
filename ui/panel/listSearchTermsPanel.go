package panel

import (
	"fmt"
	"strings"
)

const searchTermsStatusText = "←/→: Term, enter: Search, Ctrl+C: Cancel"

// DrawSearchTermsPanel renders the search-term picker modal body.
func DrawSearchTermsPanel(displayIndex, label, rendered string) string {
	var b strings.Builder
	b.WriteString(searchTermsStatusText)
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("%s · %s\n", displayIndex, strings.TrimSpace(label)))
	b.WriteString("Search: ")
	b.WriteString(strings.TrimSpace(rendered))
	b.WriteByte('\n')
	return b.String()
}
