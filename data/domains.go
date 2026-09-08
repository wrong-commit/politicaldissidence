package data

import (
	"fmt"
	"politicaldissidence/dnscheck"
	"politicaldissidence/whois"
	"time"
)

// WhoisMaxAge is the background refresh throttle window.
const WhoisMaxAge = 10 * 24 * time.Hour

// DnsMaxAge is the background DNS refresh throttle window.
const DnsMaxAge = WhoisMaxAge

// lookupInfo is the WHOIS lookup function; overridden in tests.
var lookupInfo = whois.Lookup

// lookupDns is the DNS lookup function; overridden in tests.
var lookupDns = dnscheck.Lookup

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

// DnsRecord is the latest DNS emptiness check for a domain (only one kept).
// Persisted under Domain.DNS in mp_data.json.
type DnsRecord struct {
	CheckedAt time.Time `json:"checkedAt,omitempty"`
	Empty     bool      `json:"empty"`
	Outcome   string    `json:"outcome,omitempty"`
	A         []string  `json:"a,omitempty"`
	NS        []string  `json:"ns,omitempty"`
	MX        []string  `json:"mx,omitempty"`
	TXT       []string  `json:"txt,omitempty"`
	Error     string    `json:"error,omitempty"`
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
	/* Latest DNS emptiness check; omitempty when never looked up */
	DNS *DnsRecord `json:"dns,omitempty"`
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

// NeedsDns reports whether a DNS lookup should run for the background job.
// Never-checked domains and domains whose dns.checkedAt is at least maxAge ago need DNS.
func (d Domain) NeedsDns(at time.Time, maxAge time.Duration) bool {
	if d.DNS == nil || d.DNS.CheckedAt.IsZero() {
		return true
	}
	return !d.DNS.CheckedAt.After(at.Add(-maxAge))
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

// UpdateDns runs a DNS emptiness check and stores the latest DnsRecord.
func (d *Domain) UpdateDns() (dnscheck.Info, error) {
	return d.updateDns(lookupDns)
}

func (d *Domain) updateDns(lookup func(string) (dnscheck.Info, error)) (info dnscheck.Info, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in UpdateDns: %v", r)
		}
	}()
	info, err = lookup(d.Hostname)
	checked := now()
	if err != nil {
		msg := info.Message
		if msg == "" {
			msg = err.Error()
		}
		d.DNS = &DnsRecord{
			CheckedAt: checked,
			Empty:     false,
			Outcome:   string(dnscheck.OutcomeError),
			Error:     msg,
		}
		return info, err
	}
	d.DNS = &DnsRecord{
		CheckedAt: checked,
		Empty:     info.Empty,
		Outcome:   string(info.Outcome),
		A:         append([]string(nil), info.A...),
		NS:        append([]string(nil), info.NS...),
		MX:        append([]string(nil), info.MX...),
		TXT:       append([]string(nil), info.TXT...),
	}
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
