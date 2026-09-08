package dnsrefresh

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/dnscheck"
)

type memLog struct {
	mu   sync.Mutex
	info []string
	dbg  []string
	err  []string
}

func (m *memLog) Info(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = append(m.info, msg)
}
func (m *memLog) Debug(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dbg = append(m.dbg, msg)
}
func (m *memLog) Error(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = append(m.err, msg)
}

func sampleMP(nameFirst, surname, host string, dnsChecked time.Time) data.MP {
	var dns *data.DnsRecord
	if !dnsChecked.IsZero() {
		dns = &data.DnsRecord{CheckedAt: dnsChecked, Outcome: "ok"}
	}
	return data.MP{
		Honorific: "Ms",
		FirstName: nameFirst,
		Surname:   surname,
		Domains: []data.Domain{{
			Hostname: host,
			DNS:      dns,
		}},
	}
}

func TestFormatMessages(t *testing.T) {
	if got := FormatStart(42); got != "INFO checking DNS for all (42) MP domains" {
		t.Fatalf("start: %q", got)
	}
	if got := FormatDebug("Jane Doe", "example.com.au"); got != "DEBUG dns Jane Doe MP domain example.com.au" {
		t.Fatalf("debug: %q", got)
	}
	if got := FormatEnd(12, 18); got != "INFO dns checked 12 MPs and updated 18 domains" {
		t.Fatalf("end: %q", got)
	}
}

func TestRun_CountsAndSkip(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-2 * 24 * time.Hour)
	stale := now.Add(-20 * 24 * time.Hour)

	mps := []data.MP{
		sampleMP("Fresh", "One", "fresh.example", fresh),
		sampleMP("Stale", "Two", "stale.example", stale),
		sampleMP("Never", "Three", "never.example", time.Time{}),
	}
	mps[0].Domains = append(mps[0].Domains, data.Domain{
		Hostname: "also-fresh.example",
		DNS:      &data.DnsRecord{CheckedAt: fresh},
	})

	log := &memLog{}
	var saves int
	var hosts []string

	res := Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateDns: func(d *data.Domain) (dnscheck.Info, error) {
			hosts = append(hosts, d.Hostname)
			d.DNS = &data.DnsRecord{CheckedAt: now, Empty: true, Outcome: "empty"}
			return dnscheck.Info{Hostname: d.Hostname, Empty: true, Outcome: dnscheck.OutcomeEmpty}, nil
		},
		Save: func([]data.MP) error {
			saves++
			return nil
		},
		Log:    log,
		MaxAge: data.DnsMaxAge,
		Force:  false,
	})

	if res.TotalDomains != 4 {
		t.Fatalf("TotalDomains=%d", res.TotalDomains)
	}
	if len(hosts) != 2 {
		t.Fatalf("hosts %#v", hosts)
	}
	for _, h := range hosts {
		if strings.Contains(h, "fresh") {
			t.Fatalf("should not dns fresh domain %s", h)
		}
	}
	if saves != 1 {
		t.Fatalf("saves=%d", saves)
	}
	if res.UpdatedDomains != 2 || res.CheckedMPs != 2 {
		t.Fatalf("res=%+v", res)
	}
}

func TestRun_ContinueOnError(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	mps := []data.MP{
		sampleMP("Bad", "One", "bad.example", time.Time{}),
		sampleMP("Good", "Two", "good.example", time.Time{}),
	}
	log := &memLog{}
	var hosts []string
	res := Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateDns: func(d *data.Domain) (dnscheck.Info, error) {
			hosts = append(hosts, d.Hostname)
			if d.Hostname == "bad.example" {
				return dnscheck.Info{Hostname: d.Hostname, Message: "fail"}, errors.New("nope")
			}
			d.DNS = &data.DnsRecord{CheckedAt: now, Outcome: "ok"}
			return dnscheck.Info{Hostname: d.Hostname, Outcome: dnscheck.OutcomeOK}, nil
		},
		Log:   log,
		Force: true,
	})
	if len(hosts) != 2 {
		t.Fatalf("hosts %#v", hosts)
	}
	if res.UpdatedDomains != 1 {
		t.Fatalf("updated=%d", res.UpdatedDomains)
	}
	if len(log.err) == 0 {
		t.Fatal("expected ERROR log")
	}
}

func TestRun_LookupDelay(t *testing.T) {
	var sleeps int
	mps := []data.MP{sampleMP("A", "One", "a.example", time.Time{}), sampleMP("B", "Two", "b.example", time.Time{})}
	_ = Run(mps, Deps{
		UpdateDns: func(d *data.Domain) (dnscheck.Info, error) {
			return dnscheck.Info{Hostname: d.Hostname, Outcome: dnscheck.OutcomeOK}, nil
		},
		Delay: time.Second,
		Sleep: func(time.Duration) { sleeps++ },
		Force: true,
		Log:   &memLog{},
	})
	if sleeps != 2 {
		t.Fatalf("sleeps=%d", sleeps)
	}
}

func TestRun_Empty(t *testing.T) {
	log := &memLog{}
	res := Run(nil, Deps{Log: log, Force: true})
	if res.TotalDomains != 0 {
		t.Fatalf("%+v", res)
	}
	if len(log.info) < 2 {
		t.Fatalf("info=%v", log.info)
	}
}

func TestBackgroundEnabled(t *testing.T) {
	t.Setenv(EnvSkipBackgroundDNS, "true")
	if BackgroundEnabled() {
		t.Fatal("true should disable")
	}
	t.Setenv(EnvSkipBackgroundDNS, "")
	if !BackgroundEnabled() {
		t.Fatal("empty should enable")
	}
	t.Setenv(EnvSkipBackgroundDNS, "false")
	if !BackgroundEnabled() {
		t.Fatal("false should enable")
	}
	t.Setenv(EnvSkipBackgroundDNS, "TRUE")
	if BackgroundEnabled() {
		t.Fatal("TRUE should disable")
	}
}

func TestRunner_TryRun_EnvSkip(t *testing.T) {
	t.Setenv(EnvSkipBackgroundDNS, "true")
	log := &memLog{}
	var called bool
	r := &Runner{}
	res, ok := r.TryRun([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
		UpdateDns: func(d *data.Domain) (dnscheck.Info, error) {
			called = true
			return dnscheck.Info{}, nil
		},
		Log: log,
	})
	if !ok || !res.Skipped || called {
		t.Fatalf("ok=%v res=%+v called=%v", ok, res, called)
	}
	if len(log.info) == 0 || !strings.Contains(log.info[0], EnvSkipBackgroundDNS) {
		t.Fatalf("info=%v", log.info)
	}
}

func TestRunner_SingleFlight(t *testing.T) {
	t.Setenv(EnvSkipBackgroundDNS, "")
	r := &Runner{}
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = r.TryRun([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
			UpdateDns: func(d *data.Domain) (dnscheck.Info, error) {
				close(started)
				<-release
				return dnscheck.Info{Hostname: d.Hostname, Outcome: dnscheck.OutcomeOK}, nil
			},
			Force: true,
			Log:   &memLog{},
		})
	}()
	<-started
	_, ok := r.TryRun(nil, Deps{Force: true, Log: &memLog{}})
	if ok {
		t.Fatal("second run should be rejected")
	}
	close(release)
}
