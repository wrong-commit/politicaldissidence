package searching

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type bing struct{}

// BingPageSize is the approximate number of organic results per Bing page.
const BingPageSize = 10

// BingFirst returns the Bing `first` query value for a 0-based page index.
func BingFirst(page int) int {
	if page < 0 {
		page = 0
	}
	return page*BingPageSize + 1
}

// bingSearchURL builds a Bing HTML SERP URL.
// Required paging knobs: q, first (1, 11, 21, …). count=10 matches ~one page.
// FORM=PERE matches browser “page N” pagination (page >= 1); first page omits it.
// Session chrome (cvid, FPIG, sp, pq, …) is intentionally omitted.
func bingSearchURL(term string, page int) string {
	v := url.Values{}
	v.Set("q", term)
	v.Set("count", fmt.Sprintf("%d", BingPageSize))
	v.Set("first", fmt.Sprintf("%d", BingFirst(page)))
	// if page > 0 {
		// v.Set("FORM", "PERE")
	// }
	return "https://www.bing.com/search?" + v.Encode()
}

// Go searches Bing and returns result links for the given 0-based page.
func (bing) Go(term string, page int) ([]Link, error) {
	endpoint := bingSearchURL(term, page)
	// debugLog("DEBUG searching bing %s", endpoint)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err := assertRequest(resp, err); err != nil {
		return nil, fmt.Errorf("bing request failed: %w", err)
	}
	defer resp.Body.Close()

	node, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseBingResults(node)
}

func parseBingResults(node *html.Node) ([]Link, error) {
	found := make([]Link, 0)
	seen := make(map[string]bool)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h2" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != html.ElementNode || c.Data != "a" {
					continue
				}
				href := findHref(c)
				if href == "" {
					continue
				}
				realURL := unwrapBingURL(href)
				if realURL == "" || seen[realURL] {
					continue
				}
				seen[realURL] = true
				description := strings.TrimSpace(textContent(c))
				if description == "" {
					description = realURL
				}
				found = append(found, Link{realURL, description})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)

	if len(found) == 0 {
		return nil, fmt.Errorf("no bing results parsed")
	}
	return found, nil
}

// unwrapBingURL turns Bing redirect links into the destination URL when possible.
func unwrapBingURL(href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		if u, err := url.Parse(href); err == nil {
			if raw := u.Query().Get("u"); strings.HasPrefix(raw, "a1") {
				payload := raw[2:]
				decoded, err := base64.StdEncoding.DecodeString(payload)
				if err != nil {
					decoded, err = base64.RawURLEncoding.DecodeString(payload)
				}
				if err == nil {
					out := string(decoded)
					if strings.HasPrefix(out, "http://") || strings.HasPrefix(out, "https://") {
						return out
					}
				}
			}
			if u.Host == "www.bing.com" || u.Host == "bing.com" {
				return ""
			}
		}
		return href
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textContent(c))
	}
	return b.String()
}
