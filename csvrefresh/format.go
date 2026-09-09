package csvrefresh

import "fmt"

func FormatTickerSkipped() string {
	return fmt.Sprintf("INFO background CSV refresh ticker skipped (%s)", EnvSkipBackgroundCSVRefresh)
}

func FormatStart() string {
	return "INFO CSV refresh: fetching listing page"
}

func FormatDownloadURL(url string) string {
	return fmt.Sprintf("DEBUG CSV refresh: download URL %s", url)
}

func FormatDownloadOK(filename string, n int) string {
	return fmt.Sprintf("INFO CSV refresh: downloaded %s (%d bytes)", filename, n)
}

func FormatError(reason string) string {
	return fmt.Sprintf("ERROR CSV refresh: %s", reason)
}

func FormatParseError(reason string) string {
	return fmt.Sprintf("ERROR CSV refresh: parse: %s", reason)
}

func FormatParsed(n int, format string) string {
	return fmt.Sprintf("INFO CSV refresh: parsed %d members from CSV (format=%s)", n, format)
}

func FormatAdded(name string) string {
	return fmt.Sprintf("INFO CSV refresh: added name=%q", name)
}

func FormatMerged(name string) string {
	return fmt.Sprintf("INFO CSV refresh: merged name=%q", name)
}

func FormatEndUnchanged() string {
	return "INFO CSV refresh: no new or merged members"
}

func FormatEndChanged(added, merged int) string {
	return fmt.Sprintf("INFO CSV refresh: added %d, merged %d (unsaved — Ctrl+S to persist)", added, merged)
}

func FormatAlreadyRunning() string {
	return "INFO CSV refresh: already running"
}

func FormatConfigError(reason string) string {
	return fmt.Sprintf("ERROR CSV refresh: config %s", reason)
}
