package panel

import (
	"strings"
	"testing"
	"time"

	"politicaldissidence/data"
)

func TestDrawWhoisPanel_Empty(t *testing.T) {
	if got := DrawWhoisPanel("example.com", "", nil); got != "No WHOIS lookup yet" {
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
	})
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
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestDrawWhoisPanel_OmitsBlankFields(t *testing.T) {
	got := DrawWhoisPanel("bare.example", "", &data.WhoisRecord{
		CheckedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC),
		Expiry:    "2028-01-01",
	})
	if strings.Contains(got, "Status:") || strings.Contains(got, "Registrar:") || strings.Contains(got, "MP:") {
		t.Fatalf("unexpected blank fields:\n%s", got)
	}
	if !strings.Contains(got, "bare.example") || !strings.Contains(got, "Expiry: 2028-01-01") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestDrawWhoisPanel_Failure(t *testing.T) {
	got := DrawWhoisPanel("broken.example", "Bad MP", &data.WhoisRecord{
		CheckedAt: time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		Error:     "Could not get WHOIS for <broken.example>",
	})
	if !strings.HasPrefix(got, "Checked: 26-09-08 12:00\n") {
		t.Fatalf("checked first:\n%s", got)
	}
	if !strings.Contains(got, "broken.example") {
		t.Fatalf("missing host:\n%s", got)
	}
	if !strings.Contains(got, "Error: Could not get WHOIS for <broken.example>") {
		t.Fatalf("missing error:\n%s", got)
	}
	if strings.Contains(got, "Expiry:") {
		t.Fatalf("should not show expiry on failure:\n%s", got)
	}
}
