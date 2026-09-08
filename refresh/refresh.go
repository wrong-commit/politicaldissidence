package refresh

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/whois"
)

const (
	// EnvSkipBackgroundWhois disables the automatic background refresh when set to "true".
	EnvSkipBackgroundWhois = "SKIP_BACKGROUND_WHOIS_LOOKUP"
)

// DefaultMaxAge is the throttle window for background WHOIS lookups.
const DefaultMaxAge = data.WhoisMaxAge

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

// Deps holds injectable dependencies for a refresh run.
type Deps struct {
	Now          func() time.Time
	UpdateExpiry func(d *data.Domain) (whois.Info, error)
	Save         func(mps []data.MP) error
	Log          Logger
	MaxAge       time.Duration
	// Delay waits this long before each WHOIS lookup (background rate limit).
	// Zero means no delay (U / tests).
	Delay time.Duration
	Sleep func(time.Duration)
	// Force skips the 10-day throttle (U).
	Force bool
	// OnLookup is called after every attempted WHOIS (not for skipped/fresh domains).
	OnLookup func(mpName string, info whois.Info, err error)
}

// Result summarizes a refresh run.
type Result struct {
	TotalDomains   int
	CheckedMPs     int
	UpdatedDomains int
	Skipped        bool // true when env skip prevented a background start
}

// BackgroundEnabled reports whether the background job should run.
// SKIP_BACKGROUND_WHOIS_LOOKUP=true (case-insensitive) disables it.
func BackgroundEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipBackgroundWhois))
	return !strings.EqualFold(v, "true")
}

func FormatStart(totalDomains int) string {
	return fmt.Sprintf("INFO checking all (%d) MP domains", totalDomains)
}

func FormatDebug(mpName, hostname string) string {
	return fmt.Sprintf("DEBUG checking %s MP domain %s", mpName, hostname)
}

func FormatEnd(checkedMPs, updatedDomains int) string {
	return fmt.Sprintf("INFO checked %d MPs and updated %d domains", checkedMPs, updatedDomains)
}

func FormatSkipped(hostname string) string {
	return fmt.Sprintf("DEBUG skipping domain %s (fresh)", hostname)
}

func FormatSkippedEnv() string {
	return fmt.Sprintf("INFO background WHOIS skipped (%s=true)", EnvSkipBackgroundWhois)
}

func FormatError(hostname string, reason string) string {
	return fmt.Sprintf("ERROR whois %s: %s", hostname, reason)
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
	if d.UpdateExpiry == nil {
		d.UpdateExpiry = func(dom *data.Domain) (whois.Info, error) {
			return dom.UpdateExpiryInfo()
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

// DefaultLookupDelay is the pause before each background WHOIS lookup.
const DefaultLookupDelay = time.Second

// Run refreshes WHOIS for domains in mps according to Deps.
// When Force is false, fresh domains (within MaxAge) are skipped.
// Lookups are paced by Delay (if > 0). Save, when set, runs once at the end
// if any domain was updated — not after each lookup.
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
			if !deps.Force && !dom.NeedsWhois(deps.Now(), deps.MaxAge) {
				deps.Log.Debug(FormatSkipped(dom.Hostname))
				continue
			}
			mpAttempted = true
			if deps.Delay > 0 {
				deps.Sleep(deps.Delay)
			}
			mpName := mps[i].Name()
			deps.Log.Debug(FormatDebug(mpName, dom.Hostname))
			info, err := deps.UpdateExpiry(dom)
			attempted++
			// Ensure alert is set even if a custom UpdateExpiry skipped Domain.UpdateExpiryInfo.
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

// Runner ensures at most one background refresh runs at a time.
type Runner struct {
	mu      sync.Mutex
	running bool
}

// TryRun starts a refresh if one is not already running and background is enabled.
// If SKIP_BACKGROUND_WHOIS_LOOKUP=true, logs and returns a skipped Result without running.
// If already running, returns ok=false.
func (r *Runner) TryRun(mps []data.MP, deps Deps) (Result, bool) {
	deps = deps.withDefaults()

	if !BackgroundEnabled() {
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
