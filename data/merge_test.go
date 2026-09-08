package data

import (
	"testing"
	"time"
)

func TestMergeMPs_UniquePreserved(t *testing.T) {
	checked := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	a := []MP{{
		FirstName: "Ada", Surname: "Lovelace", Party: "ALP",
		Domains: []Domain{{
			Hostname:    "ada.example",
			Expiry:      "2027-01-01",
			LastChecked: checked,
			Whois:       &WhoisRecord{Registrar: "R"},
			DNS:         &DnsRecord{Outcome: "ok"},
			Https:       &HttpsRecord{Status: "enabled", HTTPStatus: 200},
		}},
	}}
	b := []MP{{FirstName: "Grace", Surname: "Hopper", Party: "LP"}}

	got := MergeMPs(a, b)
	if len(got.MPs) != 2 {
		t.Fatalf("len=%d", len(got.MPs))
	}
	if len(got.MergedNames) != 0 {
		t.Fatalf("MergedNames=%v", got.MergedNames)
	}
	if got.MPs[0].Party != "ALP" || got.MPs[0].Domains[0].Whois == nil || got.MPs[0].Domains[0].Https.HTTPStatus != 200 {
		t.Fatalf("unique member checks should be preserved: %+v", got.MPs[0].Domains[0])
	}
	if got.MPs[1].Name() != "Grace Hopper" {
		t.Fatalf("second = %q", got.MPs[1].Name())
	}
}

func TestMergeMPs_SameNameLaterPartyWins(t *testing.T) {
	a := []MP{{
		Honorific: "Ms", FirstName: "Jane", Surname: "Doe",
		Party: "ALP", Electorate: "Old", State: "NSW", Level: "old-level",
		Domains: []Domain{{Hostname: "a.example", Whois: &WhoisRecord{Registrar: "KeepMe"}}},
	}}
	b := []MP{{
		Honorific: "Hon", FirstName: "Jane", Surname: "Doe",
		Party: "LP", Electorate: "New", State: "VIC", Level: "new-level",
		PreferredName: "Jay",
		Domains:       []Domain{{Hostname: "b.example", DNS: &DnsRecord{Empty: true}}},
	}}

	got := MergeMPs(a, b)
	if len(got.MPs) != 1 {
		t.Fatalf("len=%d want 1", len(got.MPs))
	}
	if len(got.MergedNames) != 1 || got.MergedNames[0] != "Jane Doe" {
		t.Fatalf("MergedNames=%v", got.MergedNames)
	}
	mp := got.MPs[0]
	if mp.Party != "LP" || mp.Electorate != "New" || mp.State != "VIC" || mp.Level != "new-level" {
		t.Fatalf("party details: %+v", mp)
	}
	if mp.Honorific != "Hon" || mp.PreferredName != "Jay" {
		t.Fatalf("bio: %+v", mp)
	}
	if len(mp.Domains) != 2 {
		t.Fatalf("domains=%d", len(mp.Domains))
	}
	for _, d := range mp.Domains {
		if d.Whois != nil || d.DNS != nil || d.Https != nil || !d.LastChecked.IsZero() || d.Expiry != "" || d.Expired || d.Alert {
			t.Fatalf("checks should be cleared: %+v", d)
		}
	}
	if mp.Domains[0].Hostname != "a.example" || mp.Domains[1].Hostname != "b.example" {
		t.Fatalf("host order: %#v", mp.Domains)
	}
}

func TestMergeMPs_CaseInsensitiveNameAndHostname(t *testing.T) {
	a := []MP{{
		FirstName: "jane", Surname: "DOE",
		Domains: []Domain{{Hostname: "Example.COM", Whois: &WhoisRecord{}}},
	}}
	b := []MP{{
		FirstName: "Jane", Surname: "Doe", Party: "GRN",
		Domains: []Domain{{Hostname: "example.com"}, {Hostname: "other.net"}},
	}}

	got := MergeMPs(a, b)
	if len(got.MPs) != 1 {
		t.Fatalf("len=%d", len(got.MPs))
	}
	if got.MPs[0].Party != "GRN" {
		t.Fatalf("party=%q", got.MPs[0].Party)
	}
	if len(got.MPs[0].Domains) != 2 {
		t.Fatalf("domains=%#v", got.MPs[0].Domains)
	}
	if got.MPs[0].Domains[0].Hostname != "Example.COM" {
		t.Fatalf("keep first hostname spelling: %q", got.MPs[0].Domains[0].Hostname)
	}
}

func TestMergeMPs_IntraFileDedupe(t *testing.T) {
	a := []MP{
		{FirstName: "Sam", Surname: "Smith", Party: "ALP", Domains: []Domain{{Hostname: "one.com"}}},
		{FirstName: "Sam", Surname: "Smith", Party: "LP", Domains: []Domain{{Hostname: "two.com"}}},
	}
	got := MergeMPs(a, nil)
	if len(got.MPs) != 1 || got.MPs[0].Party != "LP" || len(got.MPs[0].Domains) != 2 {
		t.Fatalf("got=%+v", got)
	}
}
