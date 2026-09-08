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
	// Prefer DDG then Bing (bot challenges are common on automated DDG).
	links, _, err := u.SearchPage(term, 0, EngineDuckDuckGo)
	return links, err
}

// SearchPage fetches one page of results for the given engine (0-based page).
// Used by g / ← / → / engine / term toggles.
// When DuckDuckGo fails or returns no links, falls back to Bing for the same page.
// used is the engine that produced the returned links (Bing when fallback succeeds).
func (u UrlSearcher) SearchPage(term string, page int, engine Engine) (links []Link, used Engine, err error) {
	engine = engine.Normalize()
	switch engine {
	case EngineDuckDuckGo:
		found, ddgErr := duckduck{}.Go(term, page)
		var bingLinks []Link
		var bingErr error
		if ddgErr != nil || len(found) == 0 {
			bingLinks, bingErr = bing{}.Go(term, page)
		}
		return coalesceDDGThenBing(found, ddgErr, bingLinks, bingErr)
	default:
		links, err := bing{}.Go(term, page)
		return links, EngineBing, err
	}
}

// coalesceDDGThenBing picks DDG results when usable, otherwise Bing.
func coalesceDDGThenBing(ddgLinks []Link, ddgErr error, bingLinks []Link, bingErr error) ([]Link, Engine, error) {
	if ddgErr == nil && len(ddgLinks) > 0 {
		return ddgLinks, EngineDuckDuckGo, nil
	}
	if bingErr == nil && len(bingLinks) > 0 {
		return bingLinks, EngineBing, nil
	}
	if ddgErr != nil {
		if bingErr != nil {
			return nil, EngineDuckDuckGo, fmt.Errorf("duckduckgo: %v; bing: %w", ddgErr, bingErr)
		}
		return nil, EngineDuckDuckGo, ddgErr
	}
	if bingErr != nil {
		return nil, EngineDuckDuckGo, bingErr
	}
	return nil, EngineDuckDuckGo, fmt.Errorf("no search results")
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
