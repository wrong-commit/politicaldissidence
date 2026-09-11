package data

import (
	"errors"
	"fmt"
	"politicaldissidence/dnscheck"
	"politicaldissidence/httpscheck"
	"politicaldissidence/registrarcheck"
	"politicaldissidence/whois"
	"strings"
	"time"
)

// WhoisMaxAge is the background refresh throttle window.
const WhoisMaxAge = 10 * 24 * time.Hour

// DnsMaxAge is the background DNS refresh throttle window.
const DnsMaxAge = WhoisMaxAge

// HttpsMaxAge is the background HTTPS refresh throttle window.
const HttpsMaxAge = WhoisMaxAge

// RegistrarMaxAge is the background registrar refresh throttle window.
const RegistrarMaxAge = WhoisMaxAge

// AlertSoonWindow is how far ahead a WHOIS expiry counts as "soon" for sniping alerts.
const AlertSoonWindow = 90 * 24 * time.Hour

// AlertWhoisUpdatedMonths is how old Whois.Updated may be before alerting.
// Age is measured in calendar months via time.Time.AddDate.
const AlertWhoisUpdatedMonths = 12

// HttpsSoonWindow is the default window for HTTPS status "soon" (cert NotAfter).
// Let's Encrypt typically renews around 30 days before expiry, so 15 days is a
// useful neglect signal (renewal should already have happened).
// Override per call with Domain.UpdateHttpsWindow.
const HttpsSoonWindow = 15 * 24 * time.Hour

// Alert reason tokens (CLI report / tests); not persisted on Domain.
const (
	AlertReasonExpired       = "expired"
	AlertReasonSoon          = "soon"
	AlertReasonUpdatedStale  = "updated-stale"
	AlertReasonDNSEmpty      = "dns-empty"
	AlertReasonHTTPSExpired  = "https-expired"
	AlertReasonHTTPSSoon     = "https-soon"
	AlertReasonHTTPSMissing  = "https-missing"
	AlertReasonHTTP404                 = "http-404"
	AlertReasonHTTP500                 = "http-500"
	AlertReasonRegistrarWeird          = "registrar-weird"
	AlertReasonRegistrarPurchaseable   = "registrar-purchaseable"
)

// lookupRegistrar is the registrar lookup function; overridden in tests.
var lookupRegistrar = registrarcheck.Lookup

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

// HttpsHostRecord is the latest TLS probe result for one hostname form.
type HttpsHostRecord struct {
	Hostname   string    `json:"hostname,omitempty"`
	Status     string    `json:"status,omitempty"`
	NotAfter   time.Time `json:"notAfter,omitempty"`
	Error      string    `json:"error,omitempty"`
	HTTPStatus int       `json:"httpStatus,omitempty"` // GET / status; 0 when fetch failed
}

// HttpsRecord is the latest dual-host HTTPS certificate check (only one kept).
// Persisted under Domain.Https in mp_data.json.
type HttpsRecord struct {
	CheckedAt  time.Time        `json:"checkedAt,omitempty"`
	Status     string           `json:"status,omitempty"` // aggregated: enabled | expired | missing
	NotAfter   time.Time        `json:"notAfter,omitempty"`
	Error      string           `json:"error,omitempty"` // optional aggregate message
	HTTPStatus int              `json:"httpStatus,omitempty"` // aggregated page status; 0 when unknown
	Apex       *HttpsHostRecord `json:"apex,omitempty"`
	WWW        *HttpsHostRecord `json:"www,omitempty"`
}

// RegistrarLookupRecord is the latest registrar availability check for one source.
type RegistrarLookupRecord struct {
	CheckedAt     time.Time `json:"checkedAt,omitempty"`
	Purchaseable  string    `json:"purchaseable"`  // yes | no | weird
	WeirdResponse string    `json:"weirdResponse"` // yes | no | weird
	Error         string    `json:"error,omitempty"`
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
	/* Latest HTTPS certificate check; omitempty when never looked up */
	Https *HttpsRecord `json:"https,omitempty"`
	/* Latest registrar availability by source id; omitempty when never looked up */
	RegistrarLookups map[string]*RegistrarLookupRecord `json:"registrarLookups,omitempty"`
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

// NeedsHttps reports whether an HTTPS lookup should run for the background job.
// Never-checked domains and domains whose https.checkedAt is at least maxAge ago need HTTPS.
func (d Domain) NeedsHttps(at time.Time, maxAge time.Duration) bool {
	if d.Https == nil || d.Https.CheckedAt.IsZero() {
		return true
	}
	return !d.Https.CheckedAt.After(at.Add(-maxAge))
}

// NeedsRegistrar reports whether any implemented configured source is due.
// Unimplemented source ids are ignored for freshness.
func (d Domain) NeedsRegistrar(at time.Time, maxAge time.Duration, sources []string) bool {
	for _, src := range sources {
		if !registrarcheck.IsImplemented(src) {
			continue
		}
		rec := d.RegistrarLookups[src]
		if rec == nil || rec.CheckedAt.IsZero() {
			return true
		}
		if !rec.CheckedAt.After(at.Add(-maxAge)) {
			return true
		}
	}
	return false
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

// UpdateHttps runs a dual-host HTTPS certificate check using HttpsSoonWindow.
func (d *Domain) UpdateHttps() (httpscheck.Info, error) {
	return d.UpdateHttpsWindow(HttpsSoonWindow)
}

// UpdateHttpsWindow is like UpdateHttps but uses the given soon window for StatusSoon.
// soonWindow <= 0 falls back to HttpsSoonWindow.
func (d *Domain) UpdateHttpsWindow(soonWindow time.Duration) (httpscheck.Info, error) {
	if soonWindow <= 0 {
		soonWindow = HttpsSoonWindow
	}
	return d.updateHttps(func(hostname string) (httpscheck.Info, error) {
		return httpscheck.LookupWith(hostname, nil, nil, 0, now, soonWindow)
	})
}

func (d *Domain) updateHttps(lookup func(string) (httpscheck.Info, error)) (info httpscheck.Info, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in UpdateHttps: %v", r)
		}
	}()
	info, err = lookup(d.Hostname)
	checked := now()
	if err != nil {
		msg := info.Message
		if msg == "" {
			msg = err.Error()
		}
		d.Https = &HttpsRecord{
			CheckedAt:  checked,
			Status:     string(httpscheck.StatusMissing),
			Error:      msg,
			HTTPStatus: info.HTTPStatus,
			Apex:       hostRecordFromInfo(info.Apex),
			WWW:        hostRecordFromInfo(info.WWW),
		}
		d.RefreshAlert(checked, AlertSoonWindow)
		return info, err
	}
	d.Https = &HttpsRecord{
		CheckedAt:  checked,
		Status:     string(info.Status),
		NotAfter:   info.NotAfter,
		Error:      info.Message,
		HTTPStatus: info.HTTPStatus,
		Apex:       hostRecordFromInfo(info.Apex),
		WWW:        hostRecordFromInfo(info.WWW),
	}
	d.RefreshAlert(checked, AlertSoonWindow)
	return info, nil
}

func hostRecordFromInfo(h httpscheck.HostInfo) *HttpsHostRecord {
	rec := &HttpsHostRecord{
		Hostname:   h.Hostname,
		Status:     string(h.Status),
		NotAfter:   h.NotAfter,
		Error:      h.Message,
		HTTPStatus: h.HTTPStatus,
	}
	return rec
}

// UpdateRegistrarSource runs one registrar source lookup and persists the result.
// ErrNotImplemented is returned without writing RegistrarLookups.
func (d *Domain) UpdateRegistrarSource(source string) (registrarcheck.Info, error) {
	return d.updateRegistrarSource(source, lookupRegistrar)
}

func (d *Domain) updateRegistrarSource(source string, lookup func(hostname, source string) (registrarcheck.Info, error)) (info registrarcheck.Info, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in UpdateRegistrarSource: %v", r)
		}
	}()
	info, err = lookup(d.Hostname, source)
	if err != nil && errors.Is(err, registrarcheck.ErrNotImplemented) {
		return info, err
	}
	checked := now()
	if d.RegistrarLookups == nil {
		d.RegistrarLookups = make(map[string]*RegistrarLookupRecord)
	}
	rec := &RegistrarLookupRecord{
		CheckedAt:     checked,
		Purchaseable:  info.Purchaseable,
		WeirdResponse: info.WeirdResponse,
	}
	if info.Purchaseable == "" {
		rec.Purchaseable = registrarcheck.TriWeird
	}
	if info.WeirdResponse == "" {
		rec.WeirdResponse = registrarcheck.TriYes
	}
	if info.Message != "" {
		rec.Error = info.Message
	}
	d.RegistrarLookups[source] = rec
	d.RefreshAlert(checked, AlertSoonWindow)
	return info, err
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
// Missing/unparseable expiry or Whois.Updated contributes nothing; DNS errors are not dns-empty.
func AlertReasons(d Domain, at time.Time, soonWindow time.Duration) []string {
	if soonWindow <= 0 {
		soonWindow = AlertSoonWindow
	}
	var reasons []string
	today := startOfLocalDay(at)
	if exp, ok := ParseExpiryDate(d.Expiry); ok {
		expDay := startOfLocalDay(exp)
		if expDay.Before(today) {
			reasons = append(reasons, AlertReasonExpired)
		} else if !expDay.After(today.Add(soonWindow)) {
			reasons = append(reasons, AlertReasonSoon)
		}
	}
	if d.Whois != nil {
		if updated, ok := ParseExpiryDate(d.Whois.Updated); ok {
			updatedDay := startOfLocalDay(updated)
			cutoff := startOfLocalDay(at.AddDate(0, -AlertWhoisUpdatedMonths, 0))
			if updatedDay.Before(cutoff) {
				reasons = append(reasons, AlertReasonUpdatedStale)
			}
		}
	}
	if d.DNS != nil && d.DNS.Empty {
		reasons = append(reasons, AlertReasonDNSEmpty)
	}
	if d.Https != nil {
		switch d.Https.Status {
		case string(httpscheck.StatusExpired):
			reasons = append(reasons, AlertReasonHTTPSExpired)
		case string(httpscheck.StatusSoon):
			reasons = append(reasons, AlertReasonHTTPSSoon)
		case string(httpscheck.StatusMissing):
			reasons = append(reasons, AlertReasonHTTPSMissing)
		}
		switch d.Https.HTTPStatus {
		case 404:
			reasons = append(reasons, AlertReasonHTTP404)
		case 500:
			reasons = append(reasons, AlertReasonHTTP500)
		}
	}
	for _, rec := range d.RegistrarLookups {
		if rec == nil {
			continue
		}
		if rec.Purchaseable == registrarcheck.TriWeird ||
			rec.WeirdResponse == registrarcheck.TriYes ||
			rec.WeirdResponse == registrarcheck.TriWeird {
			reasons = append(reasons, AlertReasonRegistrarWeird)
			break
		}
	}
	for _, rec := range d.RegistrarLookups {
		if rec == nil {
			continue
		}
		if rec.Purchaseable == registrarcheck.TriYes {
			reasons = append(reasons, AlertReasonRegistrarPurchaseable)
			break
		}
	}
	return reasons
}

// ComputeAlert reports whether d should be flagged for sniping review.
func ComputeAlert(d Domain, at time.Time, soonWindow time.Duration) bool {
	return len(AlertReasons(d, at, soonWindow)) > 0
}

// RefreshAlert sets Alert from current Expiry, Whois.Updated, DNS, HTTPS, and registrar using the shared classifier.
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
