package httpsrefresh

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/httpscheck"
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

func sampleMP(nameFirst, surname, host string, httpsChecked time.Time) data.MP {
	var https *data.HttpsRecord
	if !httpsChecked.IsZero() {
		https = &data.HttpsRecord{CheckedAt: httpsChecked, Status: "enabled"}
	}
	return data.MP{
		Honorific: "Ms",
		FirstName: nameFirst,
		Surname:   surname,
		Domains: []data.Domain{{
			Hostname: host,
			Https:    https,
		}},
	}
}

func TestFormatMessages(t *testing.T) {
	if got := FormatStart(42); got != "INFO checking all (42) MP domains HTTPS" {
		t.Fatalf("start: %q", got)
	}
	if got := FormatDebug("Jane Doe", "example.com.au"); got != "DEBUG checking Jane Doe MP domain example.com.au HTTPS" {
		t.Fatalf("debug: %q", got)
	}
	if got := FormatEnd(12, 18); got != "INFO checked 12 MPs and updated 18 domains HTTPS" {
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
		Https:    &data.HttpsRecord{CheckedAt: fresh},
	})

	log := &memLog{}
	var saves int
	var hosts []string

	res := Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateHttps: func(d *data.Domain) (httpscheck.Info, error) {
			hosts = append(hosts, d.Hostname)
			d.Https = &data.HttpsRecord{CheckedAt: now, Status: "missing"}
			return httpscheck.Info{Status: httpscheck.StatusMissing}, nil
		},
		Save: func([]data.MP) error {
			saves++
			return nil
		},
		Log:    log,
		MaxAge: data.HttpsMaxAge,
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
			t.Fatalf("should not https fresh domain %s", h)
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
		UpdateHttps: func(d *data.Domain) (httpscheck.Info, error) {
			hosts = append(hosts, d.Hostname)
			if d.Hostname == "bad.example" {
				return httpscheck.Info{Message: "fail"}, errors.New("nope")
			}
			d.Https = &data.HttpsRecord{CheckedAt: now, Status: "enabled"}
			return httpscheck.Info{Status: httpscheck.StatusEnabled}, nil
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
		UpdateHttps: func(d *data.Domain) (httpscheck.Info, error) {
			return httpscheck.Info{Status: httpscheck.StatusEnabled}, nil
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

func TestBackgroundEnabled(t *testing.T) {
	t.Setenv(EnvSkipBackgroundHTTPS, "true")
	if BackgroundEnabled() {
		t.Fatal("true should disable")
	}
	t.Setenv(EnvSkipBackgroundHTTPS, "")
	if !BackgroundEnabled() {
		t.Fatal("empty should enable")
	}
	t.Setenv(EnvSkipBackgroundHTTPS, "TRUE")
	if BackgroundEnabled() {
		t.Fatal("TRUE should disable")
	}
}

func TestRunner_TryRun_EnvSkip(t *testing.T) {
	t.Setenv(EnvSkipBackgroundHTTPS, "true")
	log := &memLog{}
	var called bool
	r := &Runner{}
	res, ok := r.TryRun([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
		UpdateHttps: func(d *data.Domain) (httpscheck.Info, error) {
			called = true
			return httpscheck.Info{}, nil
		},
		Log: log,
	})
	if !ok || !res.Skipped || called {
		t.Fatalf("ok=%v res=%+v called=%v", ok, res, called)
	}
	if len(log.info) == 0 || !strings.Contains(log.info[0], EnvSkipBackgroundHTTPS) {
		t.Fatalf("info=%v", log.info)
	}
}

func TestRunner_SingleFlight(t *testing.T) {
	t.Setenv(EnvSkipBackgroundHTTPS, "")
	r := &Runner{}
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = r.TryRun([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
			UpdateHttps: func(d *data.Domain) (httpscheck.Info, error) {
				close(started)
				<-release
				return httpscheck.Info{Status: httpscheck.StatusEnabled}, nil
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
