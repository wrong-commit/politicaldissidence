package panel

import (
	"strings"
	"testing"
	"time"

	"politicaldissidence/data"
)

func TestDrawWhoisPanel_Empty(t *testing.T) {
	if got := DrawWhoisPanel("example.com", "", nil, nil, nil); got != "No WHOIS lookup yet" {
		t.Fatalf("nil: %q", got)
	}
}

func TestDrawWhoisPanel_Success(t *testing.T) {
	checked := time.Date(2026, 9, 8, 15, 4, 0, 0, time.UTC)
	got := DrawWhoisPanel("example.com.au", "Jane Doe", &data.WhoisRecord{
		CheckedAt:   checked,
		Status:      []string{"clientTransferProhibited"},
		Created:     "2019-01-02",
		Updated:     "2025-01-02",
		Expiry:      "2027-01-01",
		Registrar:   "Example Registrar Pty Ltd",
		NameServers: []string{"ns1.example.net", "ns2.example.net"},
	}, nil, nil)
	lines := strings.Split(got, "\n")
	if len(lines) == 0 || lines[0] != "Checked: 26-09-08 15:04" {
		t.Fatalf("checked should be first line, got:\n%s", got)
	}
	for _, want := range []string{
		"example.com.au",
		"MP: Jane Doe",
		"Status: clientTransferProhibited",
		"Created: 2019-01-02",
		"Expiry: 2027-01-01",
		"Registrar: Example Registrar Pty Ltd",
		"  ns1.example.net",
		"HTTPS Status: not checked yet",
		"HTTP Status: -",
		"Certificate Expiry: -",
		"DNS: not checked yet",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	httpsIdx := strings.Index(got, "HTTPS Status:")
	dnsIdx := strings.Index(got, "DNS:")
	if httpsIdx < 0 || dnsIdx < 0 || httpsIdx > dnsIdx {
		t.Fatalf("HTTPS should appear before DNS:\n%s", got)
	}
}

func TestDrawWhoisPanel_WithHttpsAndDns(t *testing.T) {
	whoisChecked := time.Date(2026, 9, 8, 15, 4, 0, 0, time.UTC)
	dnsChecked := time.Date(2026, 9, 8, 15, 5, 0, 0, time.UTC)
	certExpiry := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	got := DrawWhoisPanel("example.com.au", "Jane Doe", &data.WhoisRecord{
		CheckedAt: whoisChecked,
		Expiry:    "2027-01-01",
	}, &data.HttpsRecord{
		Status:     "enabled",
		NotAfter:   certExpiry,
		HTTPStatus: 200,
	}, &data.DnsRecord{
		CheckedAt: dnsChecked,
		Empty:     true,
		Outcome:   "empty",
	})
	if !strings.Contains(got, "HTTPS Status: enabled") {
		t.Fatalf("missing HTTPS status:\n%s", got)
	}
	if !strings.Contains(got, "HTTP Status: 200") {
		t.Fatalf("missing HTTP status:\n%s", got)
	}
	if !strings.Contains(got, "Certificate Expiry: 2026-09-09") {
		t.Fatalf("missing cert expiry:\n%s", got)
	}
	httpsIdx := strings.Index(got, "HTTPS Status:")
	httpIdx := strings.Index(got, "HTTP Status:")
	certIdx := strings.Index(got, "Certificate Expiry:")
	if httpsIdx < 0 || httpIdx < 0 || certIdx < 0 || !(httpsIdx < httpIdx && httpIdx < certIdx) {
		t.Fatalf("expected HTTPS Status then HTTP Status then Certificate Expiry:\n%s", got)
	}
	if !strings.Contains(got, "DNS (26-09-08 15:05): empty") {
		t.Fatalf("missing DNS header:\n%s", got)
	}
	dnsIdx := strings.Index(got, "DNS (")
	nsIdx := strings.Index(got, "Expiry: 2027-01-01")
	if nsIdx < 0 || dnsIdx < 0 || !(nsIdx < httpsIdx && httpsIdx < dnsIdx) {
		t.Fatalf("expected WHOIS then HTTPS then DNS:\n%s", got)
	}
}

func TestDrawWhoisPanel_HttpsStatuses(t *testing.T) {
	cases := []struct {
		status     string
		notAfter   time.Time
		httpStatus int
		wantHTTP   string
		wantExpiry string
	}{
		{"enabled", time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC), 200, "200", "2026-09-09"},
		{"soon", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), 404, "404", "2026-10-01"},
		{"expired", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), 500, "500", "2025-01-02"},
		{"missing", time.Time{}, 0, "-", "-"},
	}
	for _, tt := range cases {
		got := DrawWhoisPanel("x.example", "", &data.WhoisRecord{Expiry: "2028-01-01"}, &data.HttpsRecord{
			Status:     tt.status,
			NotAfter:   tt.notAfter,
			HTTPStatus: tt.httpStatus,
		}, nil)
		if !strings.Contains(got, "HTTPS Status: "+tt.status) {
			t.Fatalf("status %s missing in:\n%s", tt.status, got)
		}
		if !strings.Contains(got, "HTTP Status: "+tt.wantHTTP) {
			t.Fatalf("http for %s: want %s in:\n%s", tt.status, tt.wantHTTP, got)
		}
		if !strings.Contains(got, "Certificate Expiry: "+tt.wantExpiry) {
			t.Fatalf("expiry for %s: want %s in:\n%s", tt.status, tt.wantExpiry, got)
		}
	}
}

func TestDrawWhoisPanel_HttpsNotChecked(t *testing.T) {
	got := DrawWhoisPanel("bare.example", "", &data.WhoisRecord{
		CheckedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC),
		Expiry:    "2028-01-01",
	}, nil, nil)
	if !strings.Contains(got, "HTTPS Status: not checked yet") {
		t.Fatalf("got:\n%s", got)
	}
	if !strings.Contains(got, "HTTP Status: -") {
		t.Fatalf("missing HTTP placeholder:\n%s", got)
	}
	if strings.Contains(got, "HTTPS Status: enabled") {
		t.Fatalf("should not invent enabled:\n%s", got)
	}
}

func TestDrawWhoisPanel_OmitsBlankFields(t *testing.T) {
	got := DrawWhoisPanel("bare.example", "", &data.WhoisRecord{
		CheckedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC),
		Expiry:    "2028-01-01",
	}, nil, nil)
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "Registrar:") || strings.HasPrefix(line, "MP:") {
			t.Fatalf("unexpected blank field line %q in:\n%s", line, got)
		}
	}
	if !strings.Contains(got, "bare.example") || !strings.Contains(got, "Expiry: 2028-01-01") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestDrawWhoisPanel_Failure(t *testing.T) {
	got := DrawWhoisPanel("broken.example", "Bad MP", &data.WhoisRecord{
		CheckedAt: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		Error:     "Could not get WHOIS for <broken.example>",
	}, nil, nil)
	if !strings.HasPrefix(got, "Checked: 26-09-08 12:00\n") {
		t.Fatalf("checked first:\n%s", got)
	}
	if !strings.Contains(got, "broken.example") {
		t.Fatalf("missing host:\n%s", got)
	}
	if !strings.Contains(got, "Error: Could not get WHOIS for <broken.example>") {
		t.Fatalf("missing error:\n%s", got)
	}
	if strings.Contains(got, "Expiry:") && !strings.Contains(got, "Certificate Expiry:") {
		t.Fatalf("should not show registrar expiry on failure:\n%s", got)
	}
	// WHOIS error path should still not show registrar Expiry: line
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "Expiry:") {
			t.Fatalf("should not show expiry on failure:\n%s", got)
		}
	}
}

func TestDnsMarker(t *testing.T) {
	if got := DnsMarker(nil); got != "dns:?" {
		t.Fatalf("nil=%q", got)
	}
	if got := DnsMarker(&data.DnsRecord{Error: "timeout"}); got != "dns:err" {
		t.Fatalf("err=%q", got)
	}
	if got := DnsMarker(&data.DnsRecord{Empty: true, Outcome: "empty"}); got != "dns:empty" {
		t.Fatalf("empty=%q", got)
	}
	if got := DnsMarker(&data.DnsRecord{Outcome: "nxdomain", Empty: true}); got != "dns:empty" {
		t.Fatalf("nxdomain=%q", got)
	}
	if got := DnsMarker(&data.DnsRecord{Outcome: "ok", A: []string{"1.1.1.1"}}); got != "dns:ok" {
		t.Fatalf("ok=%q", got)
	}
}
