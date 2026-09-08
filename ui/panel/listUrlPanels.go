package panel

import (
	"errors"
	"fmt"
	"politicaldissidence/searching"
	"strings"
	"unicode"

	"github.com/jroimartin/gocui"
)

// URL list layout: one status line, then three lines per result (host + path + blank).
const (
	URLListStatusLines  = 1
	URLListLinesPerItem = 3
)

const urlListStatusText = "enter: Add Domain, c: Copy Link"

// URLListItemIndex maps a view cursor Y to a selectable result index.
// Returns -1 if the cursor is on the status bar or otherwise invalid.
func URLListItemIndex(cursorY int) int {
	if cursorY < URLListStatusLines {
		return -1
	}
	return (cursorY - URLListStatusLines) / URLListLinesPerItem
}

// URLListCursorY returns the cursor Y for the host line of itemIndex (0-based).
func URLListCursorY(itemIndex int) int {
	if itemIndex < 0 {
		return URLListStatusLines
	}
	return URLListStatusLines + itemIndex*URLListLinesPerItem
}

// DrawListUrlPanel returns the modal buffer, preferred content size, and the
// display-ordered links (invalid URLs omitted). existingHosts marks already-added
// domains with gray foreground (case-insensitive).
func DrawListUrlPanel(g *gocui.Gui, links []searching.Link, existingHosts []string) (string, int, int, []searching.Link, error) {
	_ = g
	existing := hostSet(existingHosts)
	var (
		display []searching.Link
		errStr  string
		width   int
		b       strings.Builder
	)

	status := urlListStatusText + "\n"
	b.WriteString(status)
	if len(status) > width {
		width = len(status)
	}

	for _, link := range links {
		if len(link) == 0 || strings.TrimSpace(link[0]) == "" {
			errStr = fmt.Sprintf("empty link\n%s", errStr)
			continue
		}
		host, path, ok := splitHostPath(link[0])
		if !ok || host == "" {
			errStr = fmt.Sprintf("invalid link <%s>\n%s", link[0], errStr)
			continue
		}
		display = append(display, link)
		idx := len(display) // 1-based for display
		hostLine := fmt.Sprintf(" %d. %s ", idx, host)
		pathLine := "        " + path // indent with spaces (tabs render as junk in gocui)
		if existing[strings.ToLower(host)] {
			hostLine = mute(hostLine)
			pathLine = mute(pathLine)
		}
		b.WriteString(hostLine + "\n")
		b.WriteString(pathLine + "\n")
		b.WriteString("\n") // blank spacer after path
		if visibleLen(hostLine) > width {
			width = visibleLen(hostLine)
		}
		if visibleLen(pathLine) > width {
			width = visibleLen(pathLine)
		}
	}

	text := b.String()
	height := strings.Count(text, "\n")
	var err error
	if errStr != "" {
		err = errors.New(errStr)
	}
	return text, width, height, display, err
}

func hostSet(hosts []string) map[string]bool {
	m := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			m[h] = true
		}
	}
	return m
}

// splitHostPath extracts hostname and path(+query/fragment) from a URL-ish string.
// Path is "/" when the URL has no path segment.
func splitHostPath(raw string) (host, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	hostPart := raw
	if parts := strings.SplitN(hostPart, "//", 2); len(parts) == 2 {
		hostPart = parts[1]
	}
	path = "/"
	if i := strings.Index(hostPart, "/"); i > -1 {
		path = hostPart[i:]
		hostPart = hostPart[:i]
	}
	if at := strings.LastIndex(hostPart, "@"); at > -1 {
		hostPart = hostPart[at+1:]
	}
	// strip port for display selection (keep path as-is)
	if colon := strings.Index(hostPart, ":"); colon > -1 {
		hostPart = hostPart[:colon]
	}
	hostPart = strings.TrimSpace(hostPart)
	if hostPart == "" {
		return "", "", false
	}
	if path == "" {
		path = "/"
	}
	return hostPart, path, true
}

// HostFromURL returns the display/add hostname for a link URL.
func HostFromURL(raw string) (string, bool) {
	host, _, ok := splitHostPath(raw)
	return host, ok
}

func mute(s string) string {
	return "\x1b[90m" + s + "\x1b[0m"
}

// visibleLen is len ignoring ANSI CSI sequences used for mute styling.
func visibleLen(s string) int {
	n := 0
	inESC := false
	for _, r := range s {
		if inESC {
			if unicode.IsLetter(r) {
				inESC = false
			}
			continue
		}
		if r == '\x1b' {
			inESC = true
			continue
		}
		n++
	}
	return n
}
