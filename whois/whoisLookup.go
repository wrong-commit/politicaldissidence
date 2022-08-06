package whois

import (
	"fmt"
	"strings"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// GetExpiry returns the WhoIS expiry for the URL
func GetExpiry(hostname string) (string, error) {
	raw, err := whois.NewClient().Whois(clean(hostname))
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
