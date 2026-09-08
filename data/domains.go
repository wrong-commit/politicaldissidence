package data

import (
	"fmt"
	"politicaldissidence/whois"
	"time"
)

// WhoisMaxAge is the background refresh throttle window.
const WhoisMaxAge = 10 * 24 * time.Hour

// lookupInfo is the WHOIS lookup function; overridden in tests.
var lookupInfo = whois.Lookup

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
	info, err := d.UpdateExpiryInfo()
	if err != nil {
		return info.Message, err
	}
	return info.ExpirationDate, nil
}

// UpdateExpiryInfo runs WHOIS and updates Expiry / LastChecked on success.
// The returned Info is for session display only and is never persisted on Domain.
func (d *Domain) UpdateExpiryInfo() (whois.Info, error) {
	return d.updateExpiryInfo(lookupInfo)
}

func (d *Domain) updateExpiryInfo(lookup func(string) (whois.Info, error)) (info whois.Info, err error) {
	// catch panic inside whois when domain is invalid
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in UpdateExpiry: %v", r)
		}
	}()
	info, err = lookup(d.Hostname)
	if err != nil {
		return info, err
	}
	d.Expiry = info.ExpirationDate
	d.LastChecked = now()
	// TODO: check if date is after current date
	return info, nil
}

// updateExpiry is kept for tests that inject a string-only lookup.
func (d *Domain) updateExpiry(lookup func(string) (string, error)) (expiry string, err error) {
	info, err := d.updateExpiryInfo(func(hostname string) (whois.Info, error) {
		s, e := lookup(hostname)
		if e != nil {
			return whois.Info{Hostname: hostname, Message: s}, e
		}
		return whois.Info{Hostname: hostname, ExpirationDate: s}, nil
	})
	if err != nil {
		return info.Message, err
	}
	return info.ExpirationDate, nil
}
