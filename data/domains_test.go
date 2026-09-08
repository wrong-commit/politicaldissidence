package data

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"politicaldissidence/dnscheck"
	"politicaldissidence/httpscheck"
)

func TestDomain_UpdateExpiry_Success(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com", Expiry: "old", LastChecked: time.Time{}}
	got, err := d.updateExpiry(func(hostname string) (string, error) {
		return "2027-06-15T00:00:00Z", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2027-06-15T00:00:00Z" {
		t.Fatalf("got expiry %q", got)
	}
	if d.Expiry != "2027-06-15T00:00:00Z" {
		t.Fatalf("domain expiry %q", d.Expiry)
	}
	if !d.LastChecked.Equal(fixed) {
		t.Fatalf("LastChecked = %v, want %v", d.LastChecked, fixed)
	}
	if d.Whois == nil || d.Whois.Expiry != "2027-06-15T00:00:00Z" {
		t.Fatalf("Whois = %+v", d.Whois)
	}
	if !d.Whois.CheckedAt.Equal(fixed) {
		t.Fatalf("Whois.CheckedAt = %v", d.Whois.CheckedAt)
	}
}

func TestDomain_UpdateExpiry_FailureLeavesState(t *testing.T) {
	prev := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com", Expiry: "keep-me", LastChecked: prev}
	lookupErr := errors.New("whois failed")
	msg, err := d.updateExpiry(func(hostname string) (string, error) {
		return "Could not get WHOIS for <example.com>", lookupErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if d.Expiry != "keep-me" {
		t.Fatalf("Expiry changed to %q", d.Expiry)
	}
	if !d.LastChecked.Equal(prev) {
		t.Fatalf("LastChecked changed to %v", d.LastChecked)
	}
	if !strings.Contains(msg, "Could not get WHOIS") {
		t.Fatalf("msg = %q", msg)
	}
	if d.Whois == nil || d.Whois.Error == "" || !d.Whois.CheckedAt.Equal(fixed) {
		t.Fatalf("Whois should record failure: %+v", d.Whois)
	}
}

func TestDomain_UpdateExpiry_NeverCheckedBecomesChecked(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com"}
	if !d.LastChecked.IsZero() {
		t.Fatal("expected zero LastChecked")
	}
	if _, err := d.updateExpiry(func(string) (string, error) { return "2028-01-01", nil }); err != nil {
		t.Fatal(err)
	}
	if d.LastChecked.IsZero() {
		t.Fatal("expected LastChecked set")
	}
}

func TestDomain_JSONRoundTrip(t *testing.T) {
	checked := time.Date(2026, 2, 1, 15, 4, 5, 0, time.UTC)
	orig := Domain{
		Hostname:    "example.com.au",
		Expiry:      "2027-01-01",
		Expired:     false,
		LastChecked: checked,
		Whois: &WhoisRecord{
			CheckedAt: checked,
			Expiry:    "2027-01-01",
			Registrar: "Example Registrar",
			Status:    []string{"ok"},
		},
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"whois"`) || !strings.Contains(string(b), `"registrar"`) {
		t.Fatalf("expected whois in JSON: %s", b)
	}
	var got Domain
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Hostname != orig.Hostname || got.Expiry != orig.Expiry || got.Expired != orig.Expired {
		t.Fatalf("got %+v", got)
	}
	if !got.LastChecked.Equal(orig.LastChecked) {
		t.Fatalf("LastChecked %v != %v", got.LastChecked, orig.LastChecked)
	}
	if got.Whois == nil || got.Whois.Registrar != "Example Registrar" || !got.Whois.CheckedAt.Equal(checked) {
		t.Fatalf("Whois %+v", got.Whois)
	}

	var missing Domain
	if err := json.Unmarshal([]byte(`{"hostname":"x.com","expiry":"","expired":false}`), &missing); err != nil {
		t.Fatal(err)
	}
	if !missing.LastChecked.IsZero() {
		t.Fatalf("missing lastChecked should be zero, got %v", missing.LastChecked)
	}
	if missing.Whois != nil {
		t.Fatalf("missing whois should be nil, got %+v", missing.Whois)
	}
}

func TestDomain_UpdateExpiry_PanicRecovery(t *testing.T) {
	d := Domain{Hostname: "panic.example", Expiry: "old"}
	_, err := d.updateExpiry(func(string) (string, error) {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected recovered error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if d.Expiry != "old" {
		t.Fatalf("Expiry should be unchanged, got %q", d.Expiry)
	}
}

func TestDomain_NeedsWhois(t *testing.T) {
	at := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	maxAge := WhoisMaxAge

	tests := []struct {
		name        string
		lastChecked time.Time
		want        bool
	}{
		{"never checked", time.Time{}, true},
		{"fresh 0 days", at, false},
		{"fresh 9 days", at.Add(-9 * 24 * time.Hour), false},
		{"just under 10 days", at.Add(-10*24*time.Hour + time.Second), false},
		{"exactly 10 days", at.Add(-10 * 24 * time.Hour), true},
		{"far past", at.Add(-90 * 24 * time.Hour), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Domain{LastChecked: tt.lastChecked}
			if got := d.NeedsWhois(at, maxAge); got != tt.want {
				t.Fatalf("NeedsWhois = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDomain_UpdateDns_Success(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com"}
	info, err := d.updateDns(func(hostname string) (dnscheck.Info, error) {
		return dnscheck.Info{
			Hostname: hostname,
			Outcome:  dnscheck.OutcomeEmpty,
			Empty:    true,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !info.Empty || d.DNS == nil || !d.DNS.Empty || d.DNS.Outcome != "empty" {
		t.Fatalf("DNS=%+v info=%+v", d.DNS, info)
	}
	if !d.DNS.CheckedAt.Equal(fixed) {
		t.Fatalf("CheckedAt=%v", d.DNS.CheckedAt)
	}
}

func TestDomain_UpdateDns_Failure(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com"}
	lookupErr := errors.New("i/o timeout")
	_, err := d.updateDns(func(hostname string) (dnscheck.Info, error) {
		return dnscheck.Info{
			Hostname: hostname,
			Outcome:  dnscheck.OutcomeError,
			Message:  "Could not get DNS for <example.com>: i/o timeout",
		}, lookupErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if d.DNS == nil || d.DNS.Empty || d.DNS.Error == "" || d.DNS.Outcome != "error" {
		t.Fatalf("DNS=%+v", d.DNS)
	}
	if !d.DNS.CheckedAt.Equal(fixed) {
		t.Fatalf("CheckedAt=%v", d.DNS.CheckedAt)
	}
}

func TestDomain_DNS_JSONRoundTrip(t *testing.T) {
	checked := time.Date(2026, 2, 1, 15, 4, 5, 0, time.UTC)
	orig := Domain{
		Hostname: "example.com.au",
		Expiry:   "2027-01-01",
		DNS: &DnsRecord{
			CheckedAt: checked,
			Empty:     true,
			Outcome:   "empty",
			A:         []string{},
		},
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"dns"`) || !strings.Contains(string(b), `"empty":true`) {
		t.Fatalf("expected dns in JSON: %s", b)
	}
	var got Domain
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.DNS == nil || !got.DNS.Empty || got.DNS.Outcome != "empty" || !got.DNS.CheckedAt.Equal(checked) {
		t.Fatalf("DNS=%+v", got.DNS)
	}

	var missing Domain
	if err := json.Unmarshal([]byte(`{"hostname":"x.com","expiry":"","expired":false}`), &missing); err != nil {
		t.Fatal(err)
	}
	if missing.DNS != nil {
		t.Fatalf("missing dns should be nil, got %+v", missing.DNS)
	}
}

func TestDomain_NeedsDns(t *testing.T) {
	at := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	maxAge := DnsMaxAge

	tests := []struct {
		name string
		dns  *DnsRecord
		want bool
	}{
		{"never checked", nil, true},
		{"zero checkedAt", &DnsRecord{}, true},
		{"fresh", &DnsRecord{CheckedAt: at}, false},
		{"fresh 9 days", &DnsRecord{CheckedAt: at.Add(-9 * 24 * time.Hour)}, false},
		{"exactly 10 days", &DnsRecord{CheckedAt: at.Add(-10 * 24 * time.Hour)}, true},
		{"far past", &DnsRecord{CheckedAt: at.Add(-90 * 24 * time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Domain{DNS: tt.dns}
			if got := d.NeedsDns(at, maxAge); got != tt.want {
				t.Fatalf("NeedsDns = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlertReasons(t *testing.T) {
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.Local)
	soon := AlertSoonWindow
	far := at.Add(120 * 24 * time.Hour).Format("2006-01-02")
	near := at.Add(30 * 24 * time.Hour).Format("2006-01-02")
	yesterday := at.Add(-24 * time.Hour).Format("2006-01-02")

	updatedFresh := at.AddDate(0, -6, 0).Format("2006-01-02")
	updatedExact := at.AddDate(0, -AlertWhoisUpdatedMonths, 0).Format("2006-01-02")
	updatedStale := at.AddDate(0, -AlertWhoisUpdatedMonths, -1).Format("2006-01-02")

	tests := []struct {
		name string
		d    Domain
		want []string
	}{
		{"A1 expired", Domain{Expiry: yesterday}, []string{AlertReasonExpired}},
		{"A2 soon", Domain{Expiry: near}, []string{AlertReasonSoon}},
		{"A3 far ok", Domain{Expiry: far, DNS: &DnsRecord{Outcome: "ok"}}, nil},
		{"A4 dns empty", Domain{Expiry: far, DNS: &DnsRecord{Empty: true, Outcome: "empty"}}, []string{AlertReasonDNSEmpty}},
		{"A5 empty expiry", Domain{Expiry: "", DNS: &DnsRecord{Outcome: "ok"}}, nil},
		{"A5b unparseable", Domain{Expiry: "not-a-date", DNS: &DnsRecord{Outcome: "ok"}}, nil},
		{"A6 soon+empty", Domain{Expiry: near, DNS: &DnsRecord{Empty: true}}, []string{AlertReasonSoon, AlertReasonDNSEmpty}},
		{"A7 dns error", Domain{Expiry: far, DNS: &DnsRecord{Empty: false, Outcome: "error", Error: "timeout"}}, nil},
		{"A8 updated fresh", Domain{Expiry: far, Whois: &WhoisRecord{Updated: updatedFresh}}, nil},
		{"A8b updated exact 12m", Domain{Expiry: far, Whois: &WhoisRecord{Updated: updatedExact}}, nil},
		{"A8c updated stale", Domain{Expiry: far, Whois: &WhoisRecord{Updated: updatedStale}}, []string{AlertReasonUpdatedStale}},
		{"A8d updated unparseable", Domain{Expiry: far, Whois: &WhoisRecord{Updated: "not-a-date"}}, nil},
		{"A8e soon+updated-stale", Domain{Expiry: near, Whois: &WhoisRecord{Updated: updatedStale}}, []string{AlertReasonSoon, AlertReasonUpdatedStale}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlertReasons(tt.d, at, soon)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRefreshAlert_Clears(t *testing.T) {
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.Local)
	far := at.Add(120 * 24 * time.Hour).Format("2006-01-02")
	d := Domain{Expiry: far, Alert: true, DNS: &DnsRecord{Outcome: "ok"}}
	d.RefreshAlert(at, AlertSoonWindow)
	if d.Alert {
		t.Fatal("expected Alert cleared")
	}
}

func TestRefreshAlert_JSONRoundTrip(t *testing.T) {
	orig := Domain{Hostname: "x.com", Expiry: "2027-01-01", Alert: true}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"alert":true`) {
		t.Fatalf("json=%s", b)
	}
	var got Domain
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Alert {
		t.Fatal("Alert not preserved")
	}
	var missing Domain
	if err := json.Unmarshal([]byte(`{"hostname":"x.com","expiry":"","expired":false}`), &missing); err != nil {
		t.Fatal(err)
	}
	if missing.Alert {
		t.Fatal("missing alert should be false")
	}
}

func TestRefreshAlert_AfterWhoisOnly(t *testing.T) {
	at := time.Date(2026, 9, 8, 12, 0, 0, 0, time.Local)
	prevNow := now
	now = func() time.Time { return at }
	defer func() { now = prevNow }()

	near := at.Add(30 * 24 * time.Hour).Format("2006-01-02")
	d := Domain{Hostname: "soon.example"}
	_, err := d.updateExpiry(func(string) (string, error) { return near, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !d.Alert {
		t.Fatal("expected Alert after soon WHOIS")
	}
}

func TestRefreshAlert_AfterDnsOnly(t *testing.T) {
	at := time.Date(2026, 9, 8, 12, 0, 0, 0, time.Local)
	prevNow := now
	now = func() time.Time { return at }
	defer func() { now = prevNow }()

	far := at.Add(200 * 24 * time.Hour).Format("2006-01-02")
	d := Domain{Hostname: "empty.example", Expiry: far}
	_, err := d.updateDns(func(hostname string) (dnscheck.Info, error) {
		return dnscheck.Info{Hostname: hostname, Outcome: dnscheck.OutcomeEmpty, Empty: true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Alert {
		t.Fatal("expected Alert after empty DNS")
	}
}

func TestDomain_UpdateHttps_Success(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	notAfter := fixed.Add(30 * 24 * time.Hour)
	d := Domain{Hostname: "example.com"}
	info, err := d.updateHttps(func(hostname string) (httpscheck.Info, error) {
		return httpscheck.Info{
			Apex:       httpscheck.HostInfo{Hostname: "example.com", Status: httpscheck.StatusEnabled, NotAfter: notAfter, HTTPStatus: 200},
			WWW:        httpscheck.HostInfo{Hostname: "www.example.com", Status: httpscheck.StatusMissing, Message: "refused", HTTPStatus: 404},
			Status:     httpscheck.StatusEnabled,
			NotAfter:   notAfter,
			HTTPStatus: 404,
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != httpscheck.StatusEnabled || d.Https == nil || d.Https.Status != "enabled" {
		t.Fatalf("Https=%+v info=%+v", d.Https, info)
	}
	if !d.Https.CheckedAt.Equal(fixed) || !d.Https.NotAfter.Equal(notAfter) {
		t.Fatalf("Https=%+v", d.Https)
	}
	if d.Https.HTTPStatus != 404 {
		t.Fatalf("HTTPStatus=%d", d.Https.HTTPStatus)
	}
	if d.Https.Apex == nil || d.Https.Apex.Hostname != "example.com" || d.Https.WWW == nil {
		t.Fatalf("host records=%+v", d.Https)
	}
	if d.Https.Apex.HTTPStatus != 200 || d.Https.WWW.HTTPStatus != 404 {
		t.Fatalf("host HTTP=%+v", d.Https)
	}
}

func TestDomain_UpdateHttps_Missing(t *testing.T) {
	fixed := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	prevNow := now
	now = func() time.Time { return fixed }
	defer func() { now = prevNow }()

	d := Domain{Hostname: "example.com"}
	_, err := d.updateHttps(func(hostname string) (httpscheck.Info, error) {
		return httpscheck.Info{
			Apex:    httpscheck.HostInfo{Hostname: "example.com", Status: httpscheck.StatusMissing, Message: "timeout"},
			WWW:     httpscheck.HostInfo{Hostname: "www.example.com", Status: httpscheck.StatusMissing, Message: "refused"},
			Status:  httpscheck.StatusMissing,
			Message: "timeout; refused",
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Https == nil || d.Https.Status != "missing" || d.Https.Error == "" {
		t.Fatalf("Https=%+v", d.Https)
	}
	if !d.Https.NotAfter.IsZero() {
		t.Fatalf("should not invent NotAfter: %v", d.Https.NotAfter)
	}
}

func TestDomain_Https_JSONRoundTrip(t *testing.T) {
	checked := time.Date(2026, 2, 1, 15, 4, 5, 0, time.UTC)
	notAfter := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	orig := Domain{
		Hostname: "example.com.au",
		Expiry:   "2027-01-01",
		Https: &HttpsRecord{
			CheckedAt:  checked,
			Status:     "enabled",
			NotAfter:   notAfter,
			HTTPStatus: 404,
			Apex: &HttpsHostRecord{
				Hostname:   "example.com.au",
				Status:     "enabled",
				NotAfter:   notAfter,
				HTTPStatus: 200,
			},
			WWW: &HttpsHostRecord{
				Hostname:   "www.example.com.au",
				Status:     "missing",
				Error:      "refused",
				HTTPStatus: 404,
			},
		},
	}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"https"`) || !strings.Contains(string(b), `"enabled"`) {
		t.Fatalf("expected https in JSON: %s", b)
	}
	if !strings.Contains(string(b), `"httpStatus":404`) {
		t.Fatalf("expected httpStatus in JSON: %s", b)
	}
	var got Domain
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Https == nil || got.Https.Status != "enabled" || !got.Https.CheckedAt.Equal(checked) {
		t.Fatalf("Https=%+v", got.Https)
	}
	if got.Https.HTTPStatus != 404 {
		t.Fatalf("HTTPStatus=%d", got.Https.HTTPStatus)
	}
	if got.Https.Apex == nil || got.Https.WWW == nil || got.Https.WWW.Error != "refused" {
		t.Fatalf("hosts=%+v", got.Https)
	}

	var missing Domain
	if err := json.Unmarshal([]byte(`{"hostname":"x.com","expiry":"","expired":false}`), &missing); err != nil {
		t.Fatal(err)
	}
	if missing.Https != nil {
		t.Fatalf("missing https should be nil, got %+v", missing.Https)
	}
}

func TestDomain_NeedsHttps(t *testing.T) {
	at := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	maxAge := HttpsMaxAge

	tests := []struct {
		name  string
		https *HttpsRecord
		want  bool
	}{
		{"never checked", nil, true},
		{"zero checkedAt", &HttpsRecord{}, true},
		{"fresh", &HttpsRecord{CheckedAt: at}, false},
		{"fresh 9 days", &HttpsRecord{CheckedAt: at.Add(-9 * 24 * time.Hour)}, false},
		{"exactly 10 days", &HttpsRecord{CheckedAt: at.Add(-10 * 24 * time.Hour)}, true},
		{"far past", &HttpsRecord{CheckedAt: at.Add(-90 * 24 * time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Domain{Https: tt.https}
			if got := d.NeedsHttps(at, maxAge); got != tt.want {
				t.Fatalf("NeedsHttps = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlertReasons_HTTPS(t *testing.T) {
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.Local)
	soon := AlertSoonWindow
	far := at.Add(120 * 24 * time.Hour).Format("2006-01-02")
	near := at.Add(30 * 24 * time.Hour).Format("2006-01-02")

	tests := []struct {
		name string
		d    Domain
		want []string
	}{
		{"A1 https-expired", Domain{Expiry: far, Https: &HttpsRecord{Status: "expired"}}, []string{AlertReasonHTTPSExpired}},
		{"A2 https-missing", Domain{Expiry: far, Https: &HttpsRecord{Status: "missing"}}, []string{AlertReasonHTTPSMissing}},
		{"A2b https-soon", Domain{Expiry: far, Https: &HttpsRecord{Status: "soon"}}, []string{AlertReasonHTTPSSoon}},
		{"A3 https-enabled", Domain{Expiry: far, Https: &HttpsRecord{Status: "enabled"}}, nil},
		{"A4 https-nil", Domain{Expiry: far}, nil},
		{"A5 http-404", Domain{Expiry: far, Https: &HttpsRecord{Status: "enabled", HTTPStatus: 404}}, []string{AlertReasonHTTP404}},
		{"A5b http-500", Domain{Expiry: far, Https: &HttpsRecord{Status: "enabled", HTTPStatus: 500}}, []string{AlertReasonHTTP500}},
		{"A5c http-200", Domain{Expiry: far, Https: &HttpsRecord{Status: "enabled", HTTPStatus: 200}}, nil},
		{"A6 soon+https-expired", Domain{Expiry: near, Https: &HttpsRecord{Status: "expired"}}, []string{AlertReasonSoon, AlertReasonHTTPSExpired}},
		{"A7 missing+http-404", Domain{Expiry: far, Https: &HttpsRecord{Status: "missing", HTTPStatus: 404}}, []string{AlertReasonHTTPSMissing, AlertReasonHTTP404}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AlertReasons(tt.d, at, soon)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRefreshAlert_AfterHttpsClears(t *testing.T) {
	at := time.Date(2026, 9, 8, 12, 0, 0, 0, time.Local)
	prevNow := now
	now = func() time.Time { return at }
	defer func() { now = prevNow }()

	far := at.Add(200 * 24 * time.Hour).Format("2006-01-02")
	d := Domain{
		Hostname: "fix.example",
		Expiry:   far,
		Alert:    true,
		Https:    &HttpsRecord{Status: "missing"},
	}
	d.RefreshAlert(at, AlertSoonWindow)
	if !d.Alert {
		t.Fatal("expected Alert while https-missing")
	}
	_, err := d.updateHttps(func(hostname string) (httpscheck.Info, error) {
		return httpscheck.Info{
			Status:   httpscheck.StatusEnabled,
			NotAfter: at.Add(30 * 24 * time.Hour),
			Apex:     httpscheck.HostInfo{Hostname: "fix.example", Status: httpscheck.StatusEnabled, NotAfter: at.Add(30 * 24 * time.Hour)},
			WWW:      httpscheck.HostInfo{Hostname: "www.fix.example", Status: httpscheck.StatusEnabled, NotAfter: at.Add(30 * 24 * time.Hour)},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Alert {
		t.Fatal("expected Alert cleared after https enabled")
	}
}
