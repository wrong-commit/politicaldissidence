package searching

// Engine identifies a URL search backend used by the UI guess-domain flow.
type Engine string

const (
	EngineBing       Engine = "bing"
	EngineDuckDuckGo Engine = "duckduckgo"
)

// Normalize returns a known engine; unknown/empty values become Bing.
func (e Engine) Normalize() Engine {
	switch e {
	case EngineDuckDuckGo:
		return EngineDuckDuckGo
	default:
		return EngineBing
	}
}

// Label is a human-readable engine name for logs and titles.
func (e Engine) Label() string {
	switch e.Normalize() {
	case EngineDuckDuckGo:
		return "DuckDuckGo"
	default:
		return "Bing"
	}
}

// Short is a compact label for modal titles (e.g. Bing, DDG).
func (e Engine) Short() string {
	switch e.Normalize() {
	case EngineDuckDuckGo:
		return "DDG"
	default:
		return "Bing"
	}
}

// SupportsPaging reports whether ← / → page fetches are available.
func (e Engine) SupportsPaging() bool {
	switch e.Normalize() {
	case EngineBing, EngineDuckDuckGo:
		return true
	default:
		return false
	}
}

// Next cycles Bing → DuckDuckGo → Bing.
func (e Engine) Next() Engine {
	if e.Normalize() == EngineBing {
		return EngineDuckDuckGo
	}
	return EngineBing
}
