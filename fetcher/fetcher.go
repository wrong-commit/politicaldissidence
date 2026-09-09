package fetcher

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

const defaultAphListingURL = "https://www.aph.gov.au/Senators_and_Members/Guidelines_for_Contacting_Senators_and_Members/Address_labels_and_CSV_files"

// GetFunc fetches a URL and returns the response body bytes.
type GetFunc func(rawURL string) ([]byte, error)

// DefaultGet performs an HTTP GET and requires a 2xx status.
func DefaultGet(rawURL string) ([]byte, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body for %s", rawURL)
	}
	return body, nil
}

/*
 * Get the URL to the Senate's CSV file from the APH site
 */
func FindSenatorsCsv() (string, error) {
	return FindCSVURL(defaultAphListingURL, "allsenph.csv", DefaultGet)
}

/*
 * Get the URL to the House of Reps' Members CSV file from the APH site
 */
func FindMembersCsv() (string, error) {
	return FindCSVURL(defaultAphListingURL, "FamilynameRepsCSV.csv", DefaultGet)
}

// FindCSVURL fetches pageURL, finds an <a href> containing filename, and returns an absolute URL.
func FindCSVURL(pageURL, filename string, get GetFunc) (string, error) {
	if get == nil {
		get = DefaultGet
	}
	body, err := get(pageURL)
	if err != nil {
		return "", err
	}
	href, err := FindAnchorHref(body, filename)
	if err != nil {
		return "", err
	}
	return ResolveCSVURL(pageURL, href)
}

// FindAnchorHref parses HTML and returns the first <a href> whose value contains filename.
func FindAnchorHref(htmlBody []byte, filename string) (string, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		return "", err
	}
	return findAnchorWithPartialHref(doc, filename)
}

// ResolveCSVURL joins a possibly relative href against pageURL.
func ResolveCSVURL(pageURL, href string) (string, error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", errors.New("empty href")
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func findAnchorWithPartialHref(doc *html.Node, filename string) (string, error) {
	var iterate func(*html.Node) (string, error)
	iterate = func(n *html.Node) (string, error) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for i := 0; i < len(n.Attr); i++ {
				if n.Attr[i].Key == "href" && strings.Contains(n.Attr[i].Val, filename) {
					return n.Attr[i].Val, nil
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if found, err := iterate(c); err == nil {
				return found, nil
			}
		}
		return "", fmt.Errorf("Could not find <a/> with filename <%s>", filename)
	}
	return iterate(doc)
}
