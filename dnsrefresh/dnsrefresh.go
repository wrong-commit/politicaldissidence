package dnsrefresh

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/dnscheck"
)

const (
	// EnvSkipBackgroundDNS disables the automatic background DNS refresh when set to "true".
	EnvSkipBackgroundDNS = "SKIP_BACKGROUND_DNS_LOOKUP"
)

// DefaultMaxAge is the throttle window for background DNS lookups.
const DefaultMaxAge = data.DnsMaxAge

// Logger receives progress messages for the console / log panel.
type Logger interface {
	Info(msg string)
	Debug(msg string)
	Error(msg string)
}

// LogFn adapts plain functions to Logger.
type LogFn struct {
	OnInfo  func(string)
	OnDebug func(string)
	OnError func(string)
}

func (l LogFn) Info(msg string) {
	if l.OnInfo != nil {
		l.OnInfo(msg)
	}
}
func (l LogFn) Debug(msg string) {
	if l.OnDebug != nil {
		l.OnDebug(msg)
	}
}
func (l LogFn) Error(msg string) {
	if l.OnError != nil {
		l.OnError(msg)
	}
}

// Deps holds injectable dependencies for a DNS refresh run.
type Deps struct {
	Now       func() time.Time
	UpdateDns func(d *data.Domain) (dnscheck.Info, error)
	Save      func(mps []data.MP) error
	Log       Logger
	MaxAge    time.Duration
	Delay     time.Duration
	Sleep     func(time.Duration)
	Force     bool
	OnLookup  func(mpName string, info dnscheck.Info, err error)
}

// Result summarizes a DNS refresh run.
type Result struct {
	TotalDomains   int
	CheckedMPs     int
	UpdatedDomains int
	Skipped        bool
}

// BackgroundEnabled reports whether the background DNS job should run.
func BackgroundEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipBackgroundDNS))
	return !strings.EqualFold(v, "true")
}

func FormatStart(totalDomains int) string {
	return fmt.Sprintf("INFO checking DNS for all (%d) MP domains", totalDomains)
}

func FormatDebug(mpName, hostname string) string {
	return fmt.Sprintf("DEBUG dns %s MP domain %s", mpName, hostname)
}

func FormatEnd(checkedMPs, updatedDomains int) string {
	return fmt.Sprintf("INFO dns checked %d MPs and updated %d domains", checkedMPs, updatedDomains)
}

func FormatSkipped(hostname string) string {
	return fmt.Sprintf("DEBUG skipping DNS domain %s (fresh)", hostname)
}

func FormatSkippedEnv() string {
	return fmt.Sprintf("INFO background DNS skipped (%s=true)", EnvSkipBackgroundDNS)
}

func FormatError(hostname string, reason string) string {
	return fmt.Sprintf("ERROR dns %s: %s", hostname, reason)
}

func countDomains(mps []data.MP) int {
	n := 0
	for _, mp := range mps {
		n += len(mp.Domains)
	}
	return n
}

func (d Deps) withDefaults() Deps {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.UpdateDns == nil {
		d.UpdateDns = func(dom *data.Domain) (dnscheck.Info, error) {
			return dom.UpdateDns()
		}
	}
	if d.MaxAge == 0 {
		d.MaxAge = DefaultMaxAge
	}
	if d.Sleep == nil {
		d.Sleep = time.Sleep
	}
	if d.Log == nil {
		d.Log = LogFn{}
	}
	return d
}

// DefaultLookupDelay is the pause before each background DNS lookup.
const DefaultLookupDelay = time.Second

// Run refreshes DNS for domains in mps according to Deps.
func Run(mps []data.MP, deps Deps) Result {
	deps = deps.withDefaults()
	total := countDomains(mps)
	deps.Log.Info(FormatStart(total))

	checkedMPs := 0
	updated := 0
	attempted := 0

	for i := range mps {
		mpAttempted := false
		for j := range mps[i].Domains {
			dom := &mps[i].Domains[j]
			if !deps.Force && !dom.NeedsDns(deps.Now(), deps.MaxAge) {
				// deps.Log.Debug(FormatSkipped(dom.Hostname))
				continue
			}
			mpAttempted = true
			if deps.Delay > 0 {
				deps.Sleep(deps.Delay)
			}
			mpName := mps[i].Name()
			deps.Log.Debug(FormatDebug(mpName, dom.Hostname))
			info, err := deps.UpdateDns(dom)
			attempted++
			dom.RefreshAlert(deps.Now(), data.AlertSoonWindow)
			if deps.OnLookup != nil {
				deps.OnLookup(mpName, info, err)
			}
			if err != nil {
				deps.Log.Error(FormatError(dom.Hostname, err.Error()))
				continue
			}
			updated++
		}
		if mpAttempted {
			checkedMPs++
		}
	}

	if attempted > 0 && deps.Save != nil {
		_ = deps.Save(mps)
	}

	deps.Log.Info(FormatEnd(checkedMPs, updated))
	return Result{
		TotalDomains:   total,
		CheckedMPs:     checkedMPs,
		UpdatedDomains: updated,
	}
}

// Runner ensures at most one background DNS refresh runs at a time.
type Runner struct {
	mu      sync.Mutex
	running bool
}

// TryRun starts a DNS refresh if one is not already running and background is enabled.
// Force=true (manual recheck) bypasses the env skip.
func (r *Runner) TryRun(mps []data.MP, deps Deps) (Result, bool) {
	deps = deps.withDefaults()

	if !deps.Force && !BackgroundEnabled() {
		deps.Log.Info(FormatSkippedEnv())
		return Result{Skipped: true}, true
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return Result{}, false
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	return Run(mps, deps), true
}
