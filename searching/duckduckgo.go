package searching

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// TODO: avoid resubmitting form to avoid flagging requests
type duckduck struct {
}

// DuckDuckGo first continuation offset and step match DDG-lite / SearXNG:
// page 1 (0-based) → s=10, then +15 per page.
const (
	duckDuckGoFirstOffset = 10
	duckDuckGoOffsetStep  = 15
)

// DuckDuckGoOffset returns the lite form `s` value for a 0-based page.
// Page 0 has no offset (ok=false).
func DuckDuckGoOffset(page int) (s int, ok bool) {
	if page <= 0 {
		return 0, false
	}
	return duckDuckGoFirstOffset + (page-1)*duckDuckGoOffsetStep, true
}

// Go searches DuckDuckGo lite and returns result links for the given 0-based page.
// Pages after 0 require a vqd from an intro (page 0) response.
func (duckduck) Go(term string, page int) ([]Link, error) {
	if page < 0 {
		page = 0
	}

	vqd := ""
	if page > 0 {
		intro, err := postDuckDuckGo(term, 0, "")
		if err != nil {
			return nil, err
		}
		if isDuckDuckGoChallenge(intro) {
			return nil, fmt.Errorf("duckduckgo blocked the request (bot challenge)")
		}
		vqd = extractVQD(intro)
		if vqd == "" {
			return nil, fmt.Errorf("duckduckgo: missing vqd for paging")
		}
	}

	doc, err := postDuckDuckGo(term, page, vqd)
	if err != nil {
		return nil, err
	}
	if isDuckDuckGoChallenge(doc) {
		return nil, fmt.Errorf("duckduckgo blocked the request (bot challenge)")
	}
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	return parseSearchResults(node)
}

func isDuckDuckGoChallenge(doc string) bool {
	return strings.Contains(doc, "anomaly.js") ||
		strings.Contains(doc, "challenge-form") ||
		strings.Contains(doc, "anomaly-modal")
}

// hasClass returns true if the provided node has a class. only basic check of class name
func hasClass(n *html.Node, className string) bool {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "class" && attr.Val == className {
				return true
			}
		}
	}
	return false
}

// parseSearchResults iterates the HTML to extract search results
func parseSearchResults(node *html.Node) ([]Link, error) {

	var isSponsoredLink func(*html.Node) bool
	isSponsoredLink = func(n *html.Node) bool {
		for parent := n; parent != nil; parent = parent.Parent {
			if parent.Data == "tr" && hasClass(parent, "result-sponsored") {
				return true
			}
		}
		return false
	}

	var iterate func(*html.Node, []Link) ([]Link, error)
	iterate = func(n *html.Node, found []Link) ([]Link, error) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if hasClass(n, "result-link") {
				// add href attribute and first child element as links
				if href := findHref(n); href != "" {
					description := "<unknown>"
					// assume first child is text
					if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
						description = n.FirstChild.Data
					}
					l := Link{href, description}
					found = append(found, l)
				}
			}
			// TODO: bug in html parser does not accept class attribute on TRs. boo ! use tokenizer API  https://zetcode.com/golang/net-html/
			if isSponsoredLink(n) {
				return nil, nil
			}
		}
		// tree iterator
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			foundOther, _ := iterate(c, found)
			if len(foundOther) > len(found) {
				found = foundOther
			}
			if found == nil {
				break
			}
		}
		return found, nil
	}

	return iterate(node, make([]Link, 0))
}

func findHref(n *html.Node) string {
	for _, a := range n.Attr {
		if a.Key == "href" {
			return a.Val
		}
	}
	return ""
}

func extractVQD(doc string) string {
	node, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return ""
	}
	return findNamedInputValue(node, "vqd")
}

func findNamedInputValue(n *html.Node, name string) string {
	if n.Type == html.ElementNode && n.Data == "input" {
		var inputName, inputVal string
		for _, a := range n.Attr {
			switch a.Key {
			case "name":
				inputName = a.Val
			case "value":
				inputVal = a.Val
			}
		}
		if inputName == name && inputVal != "" {
			return inputVal
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if v := findNamedInputValue(c, name); v != "" {
			return v
		}
	}
	return ""
}

// postDuckDuckGo submits a search to DuckDuckGo lite.
// page is 0-based; vqd is required when page > 0.
func postDuckDuckGo(term string, page int, vqd string) (string, error) {
	endpoint := "https://lite.duckduckgo.com/lite/"
	values := duckDuckGoForm(term, page, vqd)
	// debugLog("DEBUG searching duckduckgo POST %s %s", endpoint, values.Encode())

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if page > 0 {
		req.Header.Set("Referer", "https://lite.duckduckgo.com/")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-User", "?1")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("duckduckgo request failed: %w", err)
	}
	defer resp.Body.Close()

	// 202 is used for the anomaly/challenge interstitial; treat only 200 as success.
	if resp.StatusCode != http.StatusOK {
		body := bodyToString(resp.Body)
		if isDuckDuckGoChallenge(body) {
			return "", fmt.Errorf("duckduckgo blocked the request (status %d)", resp.StatusCode)
		}
		return "", fmt.Errorf("unexpected status %d %s", resp.StatusCode, resp.Status)
	}
	return bodyToString(resp.Body), nil
}

// duckDuckGoForm builds the lite POST body for a 0-based page.
func duckDuckGoForm(term string, page int, vqd string) url.Values {
	values := url.Values{}
	values.Set("q", term)
	if page <= 0 {
		values.Set("b", "")
		return values
	}
	s, _ := DuckDuckGoOffset(page)
	values.Set("s", fmt.Sprintf("%d", s))
	values.Set("dc", fmt.Sprintf("%d", s+1))
	values.Set("nextParams", "")
	values.Set("v", "l")
	values.Set("o", "json")
	values.Set("api", "d.js")
	if vqd != "" {
		values.Set("vqd", vqd)
	}
	return values
}
