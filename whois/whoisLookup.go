package whois

import (
	"fmt"
	"strings"

	likewhois "github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// Fetcher retrieves raw WHOIS text for a hostname (already cleaned by Lookup).
type Fetcher func(hostname string) (raw string, err error)

// defaultFetcher performs a live WHOIS query.
func defaultFetcher(hostname string) (string, error) {
	return likewhois.NewClient().Whois(hostname)
}

// Info holds parsed WHOIS fields for display (and expiry for Domain updates).
// Message is set on fetch/parse failure; empty on success.
type Info struct {
	Hostname       string
	Status         []string
	CreatedDate    string
	UpdatedDate    string
	ExpirationDate string
	Registrar      string
	NameServers    []string
	Message        string
}

// Lookup returns WHOIS info for hostname using the live network fetcher.
func Lookup(hostname string) (Info, error) {
	return LookupWith(hostname, defaultFetcher)
}

// LookupWith returns WHOIS info using the provided fetcher (for tests).
// On fetch/parse failure Message is an error string and err is non-nil.
// On success with a missing expiration date, ExpirationDate is "" and err is nil.
func LookupWith(hostname string, fetch Fetcher) (Info, error) {
	info := Info{Hostname: hostname}
	if fetch == nil {
		fetch = defaultFetcher
	}
	raw, err := fetch(clean(hostname))
	if err != nil {
		info.Message = fmt.Sprintf("Could not get WHOIS for <%s>", hostname)
		return info, err
	}
	result, err := whoisparser.Parse(raw)
	if err != nil {
		info.Message = fmt.Sprintf("Could not parse <%s>", hostname)
		return info, err
	}
	if result.Domain != nil {
		info.Status = append([]string(nil), result.Domain.Status...)
		info.CreatedDate = result.Domain.CreatedDate
		info.UpdatedDate = result.Domain.UpdatedDate
		info.ExpirationDate = result.Domain.ExpirationDate
		info.NameServers = append([]string(nil), result.Domain.NameServers...)
	}
	if result.Registrar != nil {
		info.Registrar = result.Registrar.Name
		if info.Registrar == "" {
			info.Registrar = result.Registrar.Organization
		}
	}
	return info, nil
}

// GetExpiry returns the WHOIS expiry for hostname using the live network fetcher.
func GetExpiry(hostname string) (string, error) {
	return GetExpiryWith(hostname, defaultFetcher)
}

// GetExpiryWith returns the WHOIS expiry using the provided fetcher (for tests).
// On fetch/parse failure the string result is an error message and err is non-nil.
// On success with a missing expiration date, returns "" and nil (documented behavior).
func GetExpiryWith(hostname string, fetch Fetcher) (string, error) {
	info, err := LookupWith(hostname, fetch)
	if err != nil {
		return info.Message, err
	}
	return info.ExpirationDate, nil
}

func clean(h string) string {
	if strings.HasPrefix(h, "www.") {
		h = strings.Replace(h, "www.", "", 1)
	}
	return h
}
