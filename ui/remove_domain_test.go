package ui

import (
	"politicaldissidence/data"
	"testing"
)

func TestRemoveDomainAt(t *testing.T) {
	domains := []data.Domain{
		{Hostname: "a.com"},
		{Hostname: "b.com"},
		{Hostname: "c.com"},
	}

	out, newIdx, removed, ok := removeDomainAt(domains, 1)
	if !ok {
		t.Fatal("expected ok")
	}
	if removed.Hostname != "b.com" {
		t.Fatalf("removed=%q", removed.Hostname)
	}
	if newIdx != 1 {
		t.Fatalf("newIdx=%d want 1 (now c.com)", newIdx)
	}
	if len(out) != 2 || out[0].Hostname != "a.com" || out[1].Hostname != "c.com" {
		t.Fatalf("out=%v", out)
	}
	// Original slice must not be mutated via shared capacity.
	if domains[1].Hostname != "b.com" {
		t.Fatalf("original corrupted: %v", domains)
	}
}

func TestRemoveDomainAtLast(t *testing.T) {
	domains := []data.Domain{{Hostname: "a.com"}, {Hostname: "b.com"}}
	out, newIdx, removed, ok := removeDomainAt(domains, 1)
	if !ok || removed.Hostname != "b.com" {
		t.Fatalf("ok=%v removed=%q", ok, removed.Hostname)
	}
	if newIdx != 0 || len(out) != 1 || out[0].Hostname != "a.com" {
		t.Fatalf("out=%v newIdx=%d", out, newIdx)
	}
}

func TestRemoveDomainAtOnly(t *testing.T) {
	domains := []data.Domain{{Hostname: "solo.com"}}
	out, newIdx, removed, ok := removeDomainAt(domains, 0)
	if !ok || removed.Hostname != "solo.com" {
		t.Fatalf("ok=%v removed=%q", ok, removed.Hostname)
	}
	if len(out) != 0 || newIdx != 0 {
		t.Fatalf("out=%v newIdx=%d", out, newIdx)
	}
}

func TestRemoveDomainAtInvalid(t *testing.T) {
	domains := []data.Domain{{Hostname: "a.com"}}
	_, _, _, ok := removeDomainAt(domains, -1)
	if ok {
		t.Fatal("expected !ok for -1")
	}
	_, _, _, ok = removeDomainAt(domains, 1)
	if ok {
		t.Fatal("expected !ok for out of range")
	}
}

func TestRemoveDomainMutatesAllAndFilter(t *testing.T) {
	ui := &UI{state: &State{filter: "have domains", currentIndex: 0}}
	mps := []data.MP{
		{FirstName: "Has", Surname: "One", Domains: []data.Domain{{Hostname: "only.com"}}},
		{FirstName: "Has", Surname: "Two", Domains: []data.Domain{{Hostname: "keep.com"}}},
	}
	ui.state.all = &mps
	ui.applyFilter("have domains")
	ui.state.domainState = &DomainState{&mps[0].Domains, 0}

	newDomains, newIdx, removed, ok := removeDomainAt(ui.mpAt(0).Domains, 0)
	if !ok || removed.Hostname != "only.com" {
		t.Fatalf("ok=%v removed=%q", ok, removed.Hostname)
	}
	ui.mpAt(0).Domains = newDomains
	ui.applyFilter(ui.state.filter)

	if len((*ui.state.all)[0].Domains) != 0 {
		t.Fatalf("all[0] domains=%v", (*ui.state.all)[0].Domains)
	}
	if len(*ui.state.visible) != 1 || ui.mpAt(0).Surname != "Two" {
		t.Fatalf("filter should drop emptied MP; visible=%v", *ui.state.visible)
	}
	_ = newIdx
}
