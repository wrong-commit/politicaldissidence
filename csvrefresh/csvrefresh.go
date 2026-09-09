package csvrefresh

import (
	"bytes"
	"sync"

	"politicaldissidence/csv"
	"politicaldissidence/data"
	"politicaldissidence/fetcher"
)

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

// Deps holds injectable dependencies for a CSV refresh run. There is no Save.
type Deps struct {
	Log    Logger
	Config *Config
	// Current returns the in-memory MP list (A).
	Current func() []data.MP
	// Get fetches a URL body (listing page and CSV).
	Get fetcher.GetFunc
	// FindURL resolves the CSV download URL from listing HTML (defaults to fetcher helpers).
	FindURL func(pageURL, filename string, body []byte) (string, error)
	// Parse parses CSV bytes with the given column map and level.
	Parse func(body []byte, cols csv.ColumnMap, level string) ([]data.MP, error)
	// Merge merges A and B (defaults to data.MergeMPs).
	Merge func(a, b []data.MP) data.MergeResult
	// Apply replaces in-memory MPs after a successful merge. Must not write disk.
	Apply func(merged []data.MP)
}

// Result summarizes a CSV refresh run.
type Result struct {
	AddedCount  int
	MergedCount int
	ParsedCount int
	Skipped     bool // true when config missing or already running handled by TryRun
	Applied     bool
}

// Runner ensures only one CSV refresh runs at a time.
type Runner struct {
	mu      sync.Mutex
	running bool
}

// TryRun starts a refresh if one is not already running.
// Env skip is NOT checked here (ticker arming only). Missing config → ERROR + skipped.
func (r *Runner) TryRun(deps Deps) (Result, bool) {
	deps = deps.withDefaults()
	if deps.Config == nil {
		deps.Log.Error(FormatConfigError("missing or invalid"))
		return Result{Skipped: true}, true
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		deps.Log.Info(FormatAlreadyRunning())
		return Result{}, false
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	return Run(deps), true
}

func (d Deps) withDefaults() Deps {
	if d.Log == nil {
		d.Log = LogFn{}
	}
	if d.Get == nil {
		d.Get = fetcher.DefaultGet
	}
	if d.FindURL == nil {
		d.FindURL = func(pageURL, filename string, body []byte) (string, error) {
			href, err := fetcher.FindAnchorHref(body, filename)
			if err != nil {
				return "", err
			}
			return fetcher.ResolveCSVURL(pageURL, href)
		}
	}
	if d.Parse == nil {
		d.Parse = func(body []byte, cols csv.ColumnMap, level string) ([]data.MP, error) {
			return csv.ParseReader(bytes.NewReader(body), cols, level)
		}
	}
	if d.Merge == nil {
		d.Merge = data.MergeMPs
	}
	if d.Current == nil {
		d.Current = func() []data.MP { return nil }
	}
	return d
}

// Run executes fetch → parse → merge → apply. Never writes disk.
func Run(deps Deps) Result {
	deps = deps.withDefaults()
	cfg := deps.Config
	if cfg == nil {
		deps.Log.Error(FormatConfigError("missing or invalid"))
		return Result{Skipped: true}
	}

	deps.Log.Info(FormatStart())

	pageBody, err := deps.Get(cfg.CSVSourceURL)
	if err != nil {
		deps.Log.Error(FormatURLError(cfg.CSVSourceURL, err.Error()))
		return Result{}
	}

	csvURL, err := deps.FindURL(cfg.CSVSourceURL, cfg.CSVFilename, pageBody)
	if err != nil {
		deps.Log.Error(FormatURLError(cfg.CSVSourceURL, err.Error()))
		return Result{}
	}
	deps.Log.Debug(FormatDownloadURL(csvURL))

	csvBody, err := deps.Get(csvURL)
	if err != nil {
		deps.Log.Error(FormatURLError(csvURL, err.Error()))
		return Result{}
	}
	deps.Log.Info(FormatDownloadOK(cfg.CSVFilename, len(csvBody)))

	parsed, err := deps.Parse(csvBody, cfg.ColumnMap, cfg.ResolvedLevel)
	if err != nil {
		deps.Log.Error(FormatParseError(err.Error()))
		return Result{}
	}
	deps.Log.Info(FormatParsed(len(parsed), cfg.NormalizedFormat))

	a := deps.Current()
	merged := deps.Merge(a, parsed)

	for _, name := range merged.AddedNames {
		deps.Log.Info(FormatAdded(name))
	}
	for _, name := range merged.MergedNames {
		deps.Log.Info(FormatMerged(name))
	}

	res := Result{
		AddedCount:  len(merged.AddedNames),
		MergedCount: len(merged.MergedNames),
		ParsedCount: len(parsed),
	}

	if res.AddedCount == 0 && res.MergedCount == 0 {
		deps.Log.Info(FormatEndUnchanged())
		// Still apply so memory matches merge output (stable / deduped).
		if deps.Apply != nil {
			deps.Apply(merged.MPs)
			res.Applied = true
		}
		return res
	}

	deps.Log.Info(FormatEndChanged(res.AddedCount, res.MergedCount))
	if deps.Apply != nil {
		deps.Apply(merged.MPs)
		res.Applied = true
	}
	return res
}
