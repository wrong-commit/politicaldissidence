package searching

/**
 * Search a term across search engines to find webpages
 */
import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type UrlSearcher struct {
}

/**
 * Link stores [0] URL and [1] description
 */
type Link []string

// httpClient is shared so search requests cannot hang forever.
var httpClient = &http.Client{Timeout: 20 * time.Second}

// DebugLog, when set, receives debug lines (e.g. request URLs) for the UI console.
var DebugLog func(string)

func debugLog(format string, args ...interface{}) {
	if DebugLog == nil {
		return
	}
	DebugLog(fmt.Sprintf(format, args...))
}

func (u UrlSearcher) Search(term string) ([]Link, error) {
	found, err := duckduck{}.Go(term, 0)
	if err == nil && len(found) > 0 {
		return found, nil
	}

	// DuckDuckGo often serves a bot-challenge page to automated clients.
	// Fall back to Bing so non-UI callers still get selectable URLs.
	bingLinks, bingErr := bing{}.Go(term, 0)
	if bingErr == nil && len(bingLinks) > 0 {
		return bingLinks, nil
	}

	if err != nil {
		if bingErr != nil {
			return nil, fmt.Errorf("duckduckgo: %v; bing: %w", err, bingErr)
		}
		return nil, err
	}
	if bingErr != nil {
		return nil, bingErr
	}
	return nil, fmt.Errorf("no search results for %q", term)
}

// SearchPage fetches one page of results for the given engine (0-based page).
// Used by g / ← / → / engine / term toggles.
func (u UrlSearcher) SearchPage(term string, page int, engine Engine) ([]Link, error) {
	switch engine.Normalize() {
	case EngineDuckDuckGo:
		return duckduck{}.Go(term, page)
	default:
		return bing{}.Go(term, page)
	}
}

// assertRequest returns an error if the request is invalid
func assertRequest(resp *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("could not open request: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("empty response")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// get performs a GET to the provided url.
func get(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err := assertRequest(resp, err); err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return bodyToString(resp.Body), nil
}

func bodyToString(resp io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp)
	return buf.String()
}
