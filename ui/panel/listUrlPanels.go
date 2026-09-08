package panel

import (
	"errors"
	"fmt"
	"politicaldissidence/searching"
	"strings"

	"github.com/jroimartin/gocui"
)

// DrawListUrlPanel returns the text and dimensions of the panel
// returned ( BufferContent, BufferWidth, BufferHeight, error )
// error contains warnings to be logged
func DrawListUrlPanel(g *gocui.Gui, links []searching.Link) (string, int, int, error) {
	domains, err := domainFromUrl(links)
	// calculate content and content sizes
	newBufferText, newBufferWidth := renderList(domains)
	newBufferHeight := strings.Count(newBufferText, "\n")
	return newBufferText, newBufferWidth, newBufferHeight, err
}

// domainFromUrl returns a domain name from a url for all links, error contains all invalid urls
func domainFromUrl(links []searching.Link) ([]string, error) {
	domains := make([]string, 0)
	var errStr string
	for _, link := range links {
		if len(link) == 0 || strings.TrimSpace(link[0]) == "" {
			errStr = fmt.Sprintf("empty link\n%s", errStr)
			continue
		}
		host := link[0]

		// TODO: move this into Link itself
		// remove scheme when present
		if parts := strings.SplitN(host, "//", 2); len(parts) == 2 {
			host = parts[1]
		}
		// remove path after host:port
		if i := strings.Index(host, "/"); i > -1 {
			host = host[:i]
		}
		// strip credentials / port noise for display selection
		if at := strings.LastIndex(host, "@"); at > -1 {
			host = host[at+1:]
		}
		host = strings.TrimSpace(host)
		if host == "" {
			errStr = fmt.Sprintf("invalid link <%s>\n%s", link[0], errStr)
		} else {
			domains = append(domains, host)
		}
	}
	var err error
	if errStr != "" {
		err = errors.New(errStr)
	}
	return domains, err
}

// renderList takes domain names to return a formatted string like:
// `1. example.domain.com `
// `2. other.page.tld `
func renderList(domains []string) (string, int) {
	// calculate content and content sizes
	var width int = 0
	var buffer string = ""
	for i, domain := range domains {
		line := fmt.Sprintf(" %d. %s \n", i, domain)
		if len(line) > width {
			width = len(line)
		}
		buffer += line
	}
	return buffer, width
}
