package data

import (
	"fmt"
	"politicaldissidence/dnscheck"
	"politicaldissidence/whois"
	"strings"
	"time"
)

// WhoisMaxAge is the background refresh throttle window.
const WhoisMaxAge = 10 * 24 * time.Hour

// DnsMaxAge is the background DNS refresh throttle window.
const DnsMaxAge = WhoisMaxAge

// AlertSoonWindow is how far ahead an expiry counts as "soon" for sniping alerts.
const AlertSoonWindow = 90 * 24 * time.Hour

// Alert reason tokens (CLI report / tests); not persisted on Domain.
const (
	AlertReasonExpired  = "expired"
	AlertReasonSoon     = "soon"
	AlertReasonDNSEmpty = "dns-empty"
)

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
	/* Alert is true when a human should review this domain for sniping */
	Alert bool `json:"alert"`
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
		d.RefreshAlert(checked, AlertSoonWindow)
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
	d.RefreshAlert(checked, AlertSoonWindow)
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
		d.RefreshAlert(checked, AlertSoonWindow)
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
	d.RefreshAlert(checked, AlertSoonWindow)
	return info, nil
}

// expiryLayouts are tried in order when parsing Domain.Expiry for alerts.
// Calendar dates without a zone are interpreted in local time.
var expiryLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02-Jan-2006",
	"2006.01.02",
}

// ParseExpiryDate parses a WHOIS expiry string. ok is false when empty or unparseable.
func ParseExpiryDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range expiryLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	// RFC3339 in UTC when zone present but ParseInLocation failed on Local-only forms.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func startOfLocalDay(t time.Time) time.Time {
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// AlertReasons returns sniping reasons for d at time at with the given soon window.
// Missing/unparseable expiry contributes nothing; DNS errors are not dns-empty.
func AlertReasons(d Domain, at time.Time, soonWindow time.Duration) []string {
	if soonWindow <= 0 {
		soonWindow = AlertSoonWindow
	}
	var reasons []string
	if exp, ok := ParseExpiryDate(d.Expiry); ok {
		today := startOfLocalDay(at)
		expDay := startOfLocalDay(exp)
		if expDay.Before(today) {
			reasons = append(reasons, AlertReasonExpired)
		} else if !expDay.After(today.Add(soonWindow)) {
			reasons = append(reasons, AlertReasonSoon)
		}
	}
	if d.DNS != nil && d.DNS.Empty {
		reasons = append(reasons, AlertReasonDNSEmpty)
	}
	return reasons
}

// ComputeAlert reports whether d should be flagged for sniping review.
func ComputeAlert(d Domain, at time.Time, soonWindow time.Duration) bool {
	return len(AlertReasons(d, at, soonWindow)) > 0
}

// RefreshAlert sets Alert from current Expiry and DNS using the shared classifier.
func (d *Domain) RefreshAlert(at time.Time, soonWindow time.Duration) {
	d.Alert = ComputeAlert(*d, at, soonWindow)
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
