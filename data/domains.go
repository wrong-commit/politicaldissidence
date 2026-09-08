package data

import (
	"fmt"
	"politicaldissidence/whois"
	"time"
)

// WhoisMaxAge is the background refresh throttle window.
const WhoisMaxAge = 10 * 24 * time.Hour

// lookupExpiry is the WHOIS expiry function; overridden in tests.
var lookupExpiry = whois.GetExpiry

// now returns the current time; overridden in tests.
var now = time.Now

type Domain struct {
	/* Domain Hostname */
	Hostname string `json:"hostname"`
	/* Empty string indicates domain expiry cannot be determined */
	Expiry  string `json:"expiry"`
	Expired bool   `json:"expired"`
	/* Zero time means never checked; omitempty keeps it out of JSON until set */
	LastChecked time.Time `json:"lastChecked,omitempty"`
}

// NeedsWhois reports whether a WHOIS lookup should run for the background job.
// Never-checked domains and domains whose LastChecked is at least maxAge ago need WHOIS.
// Fresh lookups (now - LastChecked < maxAge) are skipped.
func (d Domain) NeedsWhois(at time.Time, maxAge time.Duration) bool {
	if d.LastChecked.IsZero() {
		return true
	}
	return !d.LastChecked.After(at.Add(-maxAge))
}

// UpdateExpiry updates the expiry on a domain object via live WHOIS.
func (d *Domain) UpdateExpiry() (string, error) {
	return d.updateExpiry(lookupExpiry)
}

func (d *Domain) updateExpiry(lookup func(string) (string, error)) (expiry string, err error) {
	// catch panic inside whois when domain is invalid
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in UpdateExpiry: %v", r)
		}
	}()
	var expiryDate string
	expiryDate, err = lookup(d.Hostname)
	if err != nil {
		return expiryDate, err
	}
	d.Expiry = expiryDate
	d.LastChecked = now()
	// TODO: check if date is after current date
	return d.Expiry, nil
}
