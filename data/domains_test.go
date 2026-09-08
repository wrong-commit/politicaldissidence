package data

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
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
