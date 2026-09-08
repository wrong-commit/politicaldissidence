package panel

import (
	"strings"
	"testing"
	"time"

	"politicaldissidence/data"
)

func TestDrawListDomainPanel_LastChecked(t *testing.T) {
	checked := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	domains := []data.Domain{
		{Hostname: "a.example", Expiry: "2027-01-01", LastChecked: checked, DNS: &data.DnsRecord{Outcome: "ok"}},
		{Hostname: "b.example", Expiry: "", LastChecked: time.Time{}},
		{Hostname: "c.example", Expiry: "2026-01-01", Expired: true, LastChecked: checked, DNS: &data.DnsRecord{Empty: true, Outcome: "empty"}},
		{Hostname: "d.example", Expiry: "2027-01-01", LastChecked: checked, DNS: &data.DnsRecord{Error: "timeout", Outcome: "error"}},
		{Hostname: "e.example", Expiry: "2027-01-01", Alert: true, LastChecked: checked, DNS: &data.DnsRecord{Outcome: "ok"}},
		{Hostname: "f.example", Expiry: "2025-01-01", Expired: true, Alert: true, LastChecked: checked, DNS: &data.DnsRecord{Outcome: "ok"}},
	}
	got := DrawListDomainPanel(nil, &domains)

	if !strings.Contains(got, "a.example 2027-01-01  checked 26-03-01\n") {
		t.Fatalf("missing ok row:\n%s", got)
	}
	if !strings.Contains(got, "b.example <?>  checked never\n") {
		t.Fatalf("missing never row:\n%s", got)
	}
	if !strings.Contains(got, "c.example 2026-01-01[!]  checked 26-03-01\n") {
		t.Fatalf("missing empty row:\n%s", got)
	}
	if !strings.Contains(got, "d.example 2027-01-01  checked 26-03-01\n") {
		t.Fatalf("missing err row:\n%s", got)
	}
	if !strings.Contains(got, "e.example 2027-01-01[x]  checked 26-03-01  alert: true\n") {
		t.Fatalf("missing alert [x] row:\n%s", got)
	}
	if !strings.Contains(got, "f.example 2025-01-01[!][x]  checked 26-03-01  alert: true\n") {
		t.Fatalf("missing [!][x] row:\n%s", got)
	}
	if strings.Contains(got, "dns:") {
		t.Fatalf("unexpected dns marker:\n%s", got)
	}
}

func TestAlertMarker(t *testing.T) {
	if got := AlertMarker(true); got != "alert: true" {
		t.Fatalf("true: got %q", got)
	}
	if got := AlertMarker(false); got != "" {
		t.Fatalf("false: got %q", got)
	}
}
