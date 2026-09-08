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

// WhoisRecord is the latest WHOIS lookup for a domain (only one kept).
// Persisted under Domain.Whois in mp_data.json.
type WhoisRecord struct {
	CheckedAt   time.Time `json:"checkedAt,omitempty"`
	Status      []string  `json:"status,omitempty"`
	Created     string    `json:"created,omitempty"`
	Updated     string    `json:"updated,omitempty"`
	Expiry      string    `json:"expiry,omitempty"`
	Registrar   string    `json:"registrar,omitempty"`
	NameServers []string  `json:"nameServers,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type Domain struct {
	/* Domain Hostname */
	Hostname string `json:"hostname"`
	/* Empty string indicates domain expiry cannot be determined */
	Expiry  string `json:"expiry"`
	Expired bool   `json:"expired"`
	/* Zero time means never checked; omitempty keeps it out of JSON until set */
	LastChecked time.Time `json:"lastChecked,omitempty"`
	/* Latest WHOIS detail for the WHOIS panel; omitempty when never looked up */
	Whois *WhoisRecord `json:"whois,omitempty"`
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

// UpdateExpiryInfo runs WHOIS, updates Expiry / LastChecked on success,
// and always stores the latest WhoisRecord (success or failure).
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
	checked := now()
	if err != nil {
		msg := info.Message
		if msg == "" {
			msg = err.Error()
		}
		d.Whois = &WhoisRecord{
			CheckedAt: checked,
			Error:     msg,
		}
		return info, err
	}
	d.Expiry = info.ExpirationDate
	d.LastChecked = checked
	d.Whois = &WhoisRecord{
		CheckedAt:   checked,
		Status:      append([]string(nil), info.Status...),
		Created:     info.CreatedDate,
		Updated:     info.UpdatedDate,
		Expiry:      info.ExpirationDate,
		Registrar:   info.Registrar,
		NameServers: append([]string(nil), info.NameServers...),
	}
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
