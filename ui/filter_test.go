package ui

import (
	"politicaldissidence/data"
	"testing"
)

func TestApplyFilterSharesAll(t *testing.T) {
	ui := &UI{state: &State{filter: "all"}}
	mps := []data.MP{
		{FirstName: "Has", Surname: "Domain", Domains: []data.Domain{{Hostname: "a.com"}}},
		{FirstName: "No", Surname: "Domain"},
	}
	ui.state.all = &mps
	ui.applyFilter("all")
	if ui.state.visible != ui.state.all {
		t.Fatal("filter all should alias visible to all")
	}
	ui.mpAt(1).Domains = append(ui.mpAt(1).Domains, data.Domain{Hostname: "new.com"})
	if len((*ui.state.all)[1].Domains) != 1 {
		t.Fatalf("mutation via mpAt should update all, got %d", len((*ui.state.all)[1].Domains))
	}
}

func TestApplyFilterMutationsReachAll(t *testing.T) {
	ui := &UI{state: &State{filter: "all"}}
	mps := []data.MP{
		{FirstName: "Has", Surname: "Domain", Domains: []data.Domain{{Hostname: "a.com"}}},
		{FirstName: "No", Surname: "Domain"},
	}
	ui.state.all = &mps
	ui.applyFilter("no domains")
	if len(*ui.state.visible) != 1 {
		t.Fatalf("visible=%d", len(*ui.state.visible))
	}
	mp := ui.mpAt(0)
	mp.Domains = append(mp.Domains, data.Domain{Hostname: "saved.com"})
	if len((*ui.state.all)[1].Domains) != 1 || (*ui.state.all)[1].Domains[0].Hostname != "saved.com" {
		t.Fatalf("filtered edit did not land in all: %#v", (*ui.state.all)[1].Domains)
	}
}

func TestApplyFilterHaveAlerts(t *testing.T) {
	ui := &UI{state: &State{filter: "all"}}
	mps := []data.MP{
		{FirstName: "Alert", Surname: "One", Domains: []data.Domain{{Hostname: "a.com", Alert: true}}},
		{FirstName: "Quiet", Surname: "Two", Domains: []data.Domain{{Hostname: "b.com", Alert: false}}},
		{FirstName: "No", Surname: "Domain"},
		{FirstName: "Alert", Surname: "Mixed", Domains: []data.Domain{
			{Hostname: "ok.com", Alert: false},
			{Hostname: "bad.com", Alert: true},
		}},
	}
	ui.state.all = &mps
	ui.applyFilter("have alerts")
	if len(*ui.state.visible) != 2 {
		t.Fatalf("visible=%d want 2", len(*ui.state.visible))
	}
	if ui.mpAt(0).Surname != "One" || ui.mpAt(1).Surname != "Mixed" {
		t.Fatalf("got %s / %s", ui.mpAt(0).Surname, ui.mpAt(1).Surname)
	}
}

func TestNextFilterIncludesHaveAlerts(t *testing.T) {
	ui := &UI{state: &State{filter: "all"}}
	got := []string{ui.state.filter}
	for i := 0; i < 4; i++ {
		ui.state.filter = ui.nextFilter()
		got = append(got, ui.state.filter)
	}
	want := []string{"all", "have domains", "no domains", "have alerts", "all"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cycle %v want %v", got, want)
		}
	}
}
