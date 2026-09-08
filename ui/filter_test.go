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
