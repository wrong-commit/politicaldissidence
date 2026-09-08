package whois

import (
	"fmt"
	"strings"

	likewhois "github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// Fetcher retrieves raw WHOIS text for a hostname (already cleaned by GetExpiry).
type Fetcher func(hostname string) (raw string, err error)

// defaultFetcher performs a live WHOIS query.
func defaultFetcher(hostname string) (string, error) {
	return likewhois.NewClient().Whois(hostname)
}

// GetExpiry returns the WHOIS expiry for hostname using the live network fetcher.
func GetExpiry(hostname string) (string, error) {
	return GetExpiryWith(hostname, defaultFetcher)
}

// GetExpiryWith returns the WHOIS expiry using the provided fetcher (for tests).
// On fetch/parse failure the string result is an error message and err is non-nil.
// On success with a missing expiration date, returns "" and nil (documented behavior).
func GetExpiryWith(hostname string, fetch Fetcher) (string, error) {
	if fetch == nil {
		fetch = defaultFetcher
	}
	raw, err := fetch(clean(hostname))
	if err != nil {
		return fmt.Sprintf("Could not get WHOIS for <%s>", hostname), err
	}
	result, err := whoisparser.Parse(raw)
	if err != nil {
		return fmt.Sprintf("Could not parse <%s>", hostname), err
	}
	return result.Domain.ExpirationDate, nil
}

func clean(h string) string {
	if strings.HasPrefix(h, "www.") {
		h = strings.Replace(h, "www.", "", 1)
	}
	return h
}
