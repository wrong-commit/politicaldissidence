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

// Go will search DuckDuckGo's lite page and return top results
func (duckduck) Go(term string) ([]Link, error) {
	doc, err := postDuckDuckGo(term)
	if err != nil {
		return nil, err
	}
	if isDuckDuckGoChallenge(doc) {
		return nil, fmt.Errorf("duckduckgo blocked the request (bot challenge)")
	}
	// parse body as HTML for inspection
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

// postDuckDuckGo submits a search to DuckDuckGo lite.
func postDuckDuckGo(term string) (string, error) {
	endpoint := "https://lite.duckduckgo.com/lite/"
	values := url.Values{}
	values.Set("q", term)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PoliticalDissidence/1.0)")

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
