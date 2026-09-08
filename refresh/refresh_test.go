package refresh

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/whois"
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

func sampleMP(nameFirst, surname, host string, lastChecked time.Time) data.MP {
	return data.MP{
		Honorific: "Ms",
		FirstName: nameFirst,
		Surname:   surname,
		Domains: []data.Domain{{
			Hostname:    host,
			LastChecked: lastChecked,
		}},
	}
}

func TestFormatMessages(t *testing.T) {
	if got := FormatStart(42); got != "INFO checking all (42) MP domains" {
		t.Fatalf("start: %q", got)
	}
	if got := FormatDebug("Jane Doe", "example.com.au"); got != "DEBUG checking Jane Doe MP domain example.com.au" {
		t.Fatalf("debug: %q", got)
	}
	if got := FormatEnd(12, 18); got != "INFO checked 12 MPs and updated 18 domains" {
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
	// second domain on first MP to pad total count
	mps[0].Domains = append(mps[0].Domains, data.Domain{
		Hostname:    "also-fresh.example",
		LastChecked: fresh,
	})

	log := &memLog{}
	var saves int
	var whoisHosts []string

	res := Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			whoisHosts = append(whoisHosts, d.Hostname)
			d.Expiry = "2028-01-01"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Save: func(mps []data.MP) error {
			saves++
			return nil
		},
		Log:    log,
		MaxAge: data.WhoisMaxAge,
		Force:  false,
	})

	if res.TotalDomains != 4 {
		t.Fatalf("TotalDomains=%d want 4", res.TotalDomains)
	}
	if len(log.info) < 2 || log.info[0] != FormatStart(4) {
		t.Fatalf("start log: %#v", log.info)
	}
	if log.info[len(log.info)-1] != FormatEnd(2, 2) {
		t.Fatalf("end log: %#v want checked 2 updated 2", log.info)
	}
	if len(whoisHosts) != 2 {
		t.Fatalf("whois hosts %#v", whoisHosts)
	}
	for _, h := range whoisHosts {
		if strings.Contains(h, "fresh") {
			t.Fatalf("should not whois fresh domain %s", h)
		}
	}
	for _, line := range log.dbg {
		if strings.Contains(line, FormatDebug("Ms Fresh One", "fresh.example")) ||
			strings.Contains(line, FormatDebug("Ms Fresh One", "also-fresh.example")) {
			t.Fatalf("lookup DEBUG for skipped domain: %s", line)
		}
	}
	if saves != 1 {
		t.Fatalf("saves=%d want 1 (once at end)", saves)
	}
	if res.CheckedMPs != 2 || res.UpdatedDomains != 2 {
		t.Fatalf("result %+v", res)
	}
}

func TestRun_ContinueOnError(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	mps := []data.MP{
		sampleMP("Bad", "One", "bad.example", time.Time{}),
		sampleMP("Good", "Two", "good.example", time.Time{}),
	}
	log := &memLog{}
	res := Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			if d.Hostname == "bad.example" {
				return whois.Info{Hostname: d.Hostname, Message: "fail"}, errors.New("nope")
			}
			d.Expiry = "ok"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Save: func([]data.MP) error { return nil },
		Log:  log,
	})
	if res.CheckedMPs != 2 || res.UpdatedDomains != 1 {
		t.Fatalf("result %+v", res)
	}
	if len(log.err) != 1 || !strings.Contains(log.err[0], "bad.example") {
		t.Fatalf("errors %#v", log.err)
	}
	if log.info[len(log.info)-1] != FormatEnd(2, 1) {
		t.Fatalf("end %q", log.info[len(log.info)-1])
	}
}

func TestRun_Empty(t *testing.T) {
	log := &memLog{}
	res := Run(nil, Deps{Log: log})
	if res.TotalDomains != 0 || res.CheckedMPs != 0 || res.UpdatedDomains != 0 {
		t.Fatalf("%+v", res)
	}
	if len(log.info) != 2 {
		t.Fatalf("info %#v", log.info)
	}
	if log.info[0] != FormatStart(0) || log.info[1] != FormatEnd(0, 0) {
		t.Fatalf("info %#v", log.info)
	}
}

func TestRun_ForceCtrlU(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-1 * time.Hour)
	mp := sampleMP("Jane", "Doe", "example.com.au", fresh)
	log := &memLog{}
	calls := 0
	res := Run([]data.MP{mp}, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			calls++
			d.Expiry = "2029-01-01"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Save:  func([]data.MP) error { return nil },
		Log:   log,
		Force: true,
	})
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if log.info[0] != FormatStart(1) {
		t.Fatalf("start %q", log.info[0])
	}
	if len(log.dbg) != 1 || log.dbg[0] != FormatDebug("Jane Doe", "example.com.au") {
		t.Fatalf("debug %#v", log.dbg)
	}
	if log.info[1] != FormatEnd(1, 1) {
		t.Fatalf("end %q", log.info[1])
	}
	if res.UpdatedDomains != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestRun_ForceFailureUpdatedZero(t *testing.T) {
	mp := sampleMP("Jane", "Doe", "example.com.au", time.Time{})
	log := &memLog{}
	_ = Run([]data.MP{mp}, Deps{
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			return whois.Info{Hostname: d.Hostname, Message: "fail"}, errors.New("no")
		},
		Log:   log,
		Force: true,
	})
	if log.info[len(log.info)-1] != FormatEnd(1, 0) {
		t.Fatalf("end %q", log.info[len(log.info)-1])
	}
}

func TestRun_SaveOnceAtEnd(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	mps := []data.MP{
		sampleMP("A", "One", "a.example", time.Time{}),
		sampleMP("B", "Two", "b.example", time.Time{}),
	}
	var saves int
	var saved []data.MP
	_ = Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			d.Expiry = "2030-01-01"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Save: func(m []data.MP) error {
			saves++
			saved = append([]data.MP(nil), m...)
			if len(m) > 0 {
				saved[0].Domains = append([]data.Domain(nil), m[0].Domains...)
			}
			return nil
		},
		Log: &memLog{},
	})
	if saves != 1 {
		t.Fatalf("saves=%d want 1", saves)
	}
	if len(saved) != 2 {
		t.Fatalf("saved %#v", saved)
	}
	if saved[0].Domains[0].Expiry != "2030-01-01" {
		t.Fatalf("expiry %q", saved[0].Domains[0].Expiry)
	}
}

func TestRun_LookupDelay(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	mps := []data.MP{
		sampleMP("A", "One", "a.example", time.Time{}),
		sampleMP("B", "Two", "b.example", time.Time{}),
	}
	var sleeps []time.Duration
	_ = Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			d.Expiry = "ok"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Delay: time.Second,
		Sleep: func(d time.Duration) { sleeps = append(sleeps, d) },
		Log:   &memLog{},
	})
	if len(sleeps) != 2 {
		t.Fatalf("sleeps=%v want 2", sleeps)
	}
	for _, s := range sleeps {
		if s != time.Second {
			t.Fatalf("sleep %v want 1s", s)
		}
	}
}

func TestRun_ForceNoDelay(t *testing.T) {
	sleeps := 0
	_ = Run([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			d.Expiry = "ok"
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
		},
		Force: true,
		Delay: 0,
		Sleep: func(time.Duration) { sleeps++ },
		Log:   &memLog{},
	})
	if sleeps != 0 {
		t.Fatalf("Force path should not sleep, got %d", sleeps)
	}
}

func TestRunner_SingleFlight(t *testing.T) {
	r := &Runner{}
	log := &memLog{}
	started := make(chan struct{})
	release := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, ok := r.TryRun([]data.MP{sampleMP("A", "B", "slow.example", time.Time{})}, Deps{
			UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
				close(started)
				<-release
				d.Expiry = "ok"
				d.LastChecked = time.Now()
				return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry}, nil
			},
			Log: log,
		})
		if !ok {
			t.Error("first run should start")
		}
	}()

	<-started
	_, ok := r.TryRun(nil, Deps{Log: &memLog{}})
	if ok {
		t.Fatal("second overlapping run should be rejected")
	}
	close(release)
	wg.Wait()
}

func TestRunner_EnvSkip(t *testing.T) {
	t.Setenv(EnvSkipBackgroundWhois, "true")
	r := &Runner{}
	log := &memLog{}
	calls := 0
	res, ok := r.TryRun([]data.MP{sampleMP("A", "B", "x.example", time.Time{})}, Deps{
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			calls++
			return whois.Info{Hostname: d.Hostname}, nil
		},
		Save: func([]data.MP) error {
			t.Fatal("should not save")
			return nil
		},
		Log: log,
	})
	if !ok {
		t.Fatal("env skip should still return ok")
	}
	if !res.Skipped || calls != 0 {
		t.Fatalf("res=%+v calls=%d", res, calls)
	}
	if len(log.info) != 1 || log.info[0] != FormatSkippedEnv() {
		t.Fatalf("info %#v", log.info)
	}
}

func TestBackgroundEnabled(t *testing.T) {
	t.Setenv(EnvSkipBackgroundWhois, "")
	if !BackgroundEnabled() {
		t.Fatal("empty should enable")
	}
	t.Setenv(EnvSkipBackgroundWhois, "false")
	if !BackgroundEnabled() {
		t.Fatal("false should enable")
	}
	t.Setenv(EnvSkipBackgroundWhois, "TRUE")
	if BackgroundEnabled() {
		t.Fatal("TRUE should disable")
	}
}

func TestRun_OnLookupOnlyForAttempted(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-2 * 24 * time.Hour)
	stale := now.Add(-20 * 24 * time.Hour)

	mps := []data.MP{
		sampleMP("Fresh", "One", "fresh.example", fresh),
		sampleMP("Stale", "Two", "stale.example", stale),
	}

	var looked []string
	_ = Run(mps, Deps{
		Now: func() time.Time { return now },
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			d.Expiry = "2028-01-01"
			d.LastChecked = now
			return whois.Info{Hostname: d.Hostname, ExpirationDate: d.Expiry, Registrar: "R"}, nil
		},
		OnLookup: func(mpName string, info whois.Info, err error) {
			looked = append(looked, info.Hostname)
			if err != nil {
				t.Fatalf("unexpected err for %s: %v", info.Hostname, err)
			}
			if info.Registrar != "R" {
				t.Fatalf("info not passed through: %+v", info)
			}
			if !strings.Contains(mpName, "Stale") {
				t.Fatalf("mpName %q", mpName)
			}
		},
		Log: &memLog{},
	})
	if len(looked) != 1 || looked[0] != "stale.example" {
		t.Fatalf("OnLookup hosts %#v", looked)
	}
}

func TestRun_OnLookupOnFailure(t *testing.T) {
	var gotErr error
	var gotHost string
	_ = Run([]data.MP{sampleMP("Bad", "One", "bad.example", time.Time{})}, Deps{
		UpdateExpiry: func(d *data.Domain) (whois.Info, error) {
			return whois.Info{Hostname: d.Hostname, Message: "Could not get WHOIS"}, errors.New("nope")
		},
		OnLookup: func(mpName string, info whois.Info, err error) {
			gotHost = info.Hostname
			gotErr = err
		},
		Force: true,
		Log:   &memLog{},
	})
	if gotHost != "bad.example" || gotErr == nil {
		t.Fatalf("host=%q err=%v", gotHost, gotErr)
	}
}
