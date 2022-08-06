package searching

/**
 * Search a term across search engines to find webpages
 */
import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type UrlSearcher struct {
}

/**
 * Link stores [0] URL and [1] description
 */
type Link []string

func (u UrlSearcher) Search(term string) ([]Link, error) {
	found, err := duckduck{}.Go(term)
	return found, err
}

// assertRequest returns an error if the request is invalid
func assertRequest(resp *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("could not open request to GET %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status to %d %s: %w", resp.StatusCode, resp.Status, err)
	}
	return nil
}

// search performs a GET to the provided url. returned read need to be closed
func get(url string) (string, error) {
	resp, err := http.Get(url)
	if err := assertRequest(resp, err); err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return bodyToString(&resp.Body), nil
}

func bodyToString(resp *io.ReadCloser) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(*resp)
	return buf.String()
}
