package registrarrefresh

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/domainlookups"
	"politicaldissidence/registrarcheck"
)

const (
	// EnvSkipBackgroundRegistrar disables automatic background registrar refresh when "true".
	EnvSkipBackgroundRegistrar = "SKIP_BACKGROUND_REGISTRAR_LOOKUP"
)

// DefaultMaxAge is the throttle window for background registrar lookups.
const DefaultMaxAge = data.RegistrarMaxAge

// DefaultConfigPath is the domain_lookups.json path loaded each run.
const DefaultConfigPath = domainlookups.DefaultConfigPath

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

// Deps holds injectable dependencies for a registrar refresh run.
type Deps struct {
	Now                  func() time.Time
	LoadConfig           func() (domainlookups.Config, error)
	UpdateRegistrarSource func(d *data.Domain, source string) (registrarcheck.Info, error)
	Save                 func(mps []data.MP) error
	Log                  Logger
	MaxAge               time.Duration
	Delay                time.Duration
	Sleep                func(time.Duration)
	Force                bool
	OnLookup             func(mpName string, info registrarcheck.Info, err error)
}

// Result summarizes a registrar refresh run.
type Result struct {
	TotalDomains   int
	CheckedMPs     int
	UpdatedDomains int
	Skipped        bool
}

// BackgroundEnabled reports whether the background registrar job should run.
func BackgroundEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipBackgroundRegistrar))
	return !strings.EqualFold(v, "true")
}

func FormatStart(totalDomains int) string {
	return fmt.Sprintf("INFO checking all (%d) MP domains registrar", totalDomains)
}

func FormatDebug(mpName, hostname, source string) string {
	return fmt.Sprintf("DEBUG checking %s MP domain %s registrar %s", mpName, hostname, source)
}

func FormatEnd(checkedMPs, updatedDomains int) string {
	return fmt.Sprintf("INFO checked %d MPs and updated %d domains registrar", checkedMPs, updatedDomains)
}

func FormatSkippedEnv() string {
	return fmt.Sprintf("INFO background registrar skipped (%s=true)", EnvSkipBackgroundRegistrar)
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
	if d.LoadConfig == nil {
		d.LoadConfig = func() (domainlookups.Config, error) {
			return domainlookups.LoadFile(DefaultConfigPath)
		}
	}
	if d.UpdateRegistrarSource == nil {
		d.UpdateRegistrarSource = func(dom *data.Domain, source string) (registrarcheck.Info, error) {
			return dom.UpdateRegistrarSource(source)
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

// DefaultLookupDelay is the pause before each domain's registrar pass.
const DefaultLookupDelay = time.Second

// Run refreshes registrar lookups for domains in mps according to Deps.
func Run(mps []data.MP, deps Deps) Result {
	deps = deps.withDefaults()
	total := countDomains(mps)
	deps.Log.Info(FormatStart(total))

	cfg, err := deps.LoadConfig()
	if err != nil {
		deps.Log.Error(fmt.Sprintf("ERROR registrar config: %v", err))
		deps.Log.Info(FormatEnd(0, 0))
		return Result{TotalDomains: total}
	}

	checkedMPs := 0
	updated := 0
	attempted := 0

	for i := range mps {
		mpAttempted := false
		for j := range mps[i].Domains {
			dom := &mps[i].Domains[j]
			sources := cfg.SourcesForHostname(dom.Hostname)
			if len(sources) == 0 {
				continue
			}
			if !deps.Force && !dom.NeedsRegistrar(deps.Now(), deps.MaxAge, sources) {
				continue
			}
			mpAttempted = true
			if deps.Delay > 0 {
				deps.Sleep(deps.Delay)
			}
			mpName := mps[i].Name()
			domainUpdated := false
			for _, src := range sources {
				deps.Log.Debug(FormatDebug(mpName, dom.Hostname, src))
				info, err := deps.UpdateRegistrarSource(dom, src)
				attempted++
				if deps.OnLookup != nil {
					deps.OnLookup(mpName, info, err)
				}
				if errors.Is(err, registrarcheck.ErrNotImplemented) {
					deps.Log.Error(registrarcheck.FormatNotImplemented(src, dom.Hostname))
					continue
				}
				if err != nil {
					deps.Log.Error(registrarcheck.FormatLookupError(src, dom.Hostname, err.Error()))
					// Persisted weird state still counts as an update attempt with stored record.
					domainUpdated = true
					deps.Log.Info(registrarcheck.FormatLookupInfo(src, dom.Hostname, info.Purchaseable, info.WeirdResponse))
					continue
				}
				domainUpdated = true
				deps.Log.Info(registrarcheck.FormatLookupInfo(src, dom.Hostname, info.Purchaseable, info.WeirdResponse))
			}
			dom.RefreshAlert(deps.Now(), data.AlertSoonWindow)
			if domainUpdated {
				updated++
			}
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

// Runner ensures at most one background registrar refresh runs at a time.
type Runner struct {
	mu      sync.Mutex
	running bool
}

// TryRun starts a registrar refresh if one is not already running and background is enabled.
// Force=true bypasses the env skip.
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
