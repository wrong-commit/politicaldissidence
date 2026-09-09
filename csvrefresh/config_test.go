package csvrefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"politicaldissidence/csv"
	"politicaldissidence/data"
)

func entryJSON(format string) string {
	switch format {
	case FormatMembers:
		return `{"csvSourceURL":"https://example.org/page","csvFilename":"FamilynameRepsCSV.csv","format":"members"}`
	case FormatCustom:
		return `{
			"csvSourceURL":"https://example.org/page",
			"csvFilename":"roster.csv",
			"format":"custom",
			"level":"State Senator",
			"columns":{
				"honorific":"Title",
				"firstName":"First Name",
				"surname":"Surname",
				"otherName":"",
				"preferredName":"",
				"party":"Party",
				"state":"State",
				"electorate":"District"
			}
		}`
	default:
		return `{"csvSourceURL":"https://example.org/page","csvFilename":"allsenel.csv","format":"senators"}`
	}
}

func wrapEntries(entries ...string) []byte {
	return []byte(`{"interval":"1h","entries":[` + strings.Join(entries, ",") + `]}`)
}

func TestParseConfig_SenatorsDefaults(t *testing.T) {
	c, err := ParseConfig(wrapEntries(entryJSON(FormatSenators)))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) != 1 {
		t.Fatalf("entries=%d", len(c.Entries))
	}
	e := c.Entries[0]
	if e.NormalizedFormat != FormatSenators {
		t.Fatalf("format=%q", e.NormalizedFormat)
	}
	if e.ColumnMap != csv.SenatorColumns {
		t.Fatalf("columns=%+v", e.ColumnMap)
	}
	if e.ResolvedLevel != data.Level.FedSenator {
		t.Fatalf("level=%q", e.ResolvedLevel)
	}
	if c.IntervalDuration != time.Hour {
		t.Fatalf("interval=%v", c.IntervalDuration)
	}
}

func TestParseConfig_Members(t *testing.T) {
	c, err := ParseConfig(wrapEntries(entryJSON(FormatMembers)))
	if err != nil {
		t.Fatal(err)
	}
	e := c.Entries[0]
	if e.ColumnMap != csv.MemberColumns {
		t.Fatalf("columns=%+v", e.ColumnMap)
	}
	if e.ResolvedLevel != data.Level.FedRep {
		t.Fatalf("level=%q", e.ResolvedLevel)
	}
}

func TestParseConfig_Custom(t *testing.T) {
	c, err := ParseConfig(wrapEntries(entryJSON(FormatCustom)))
	if err != nil {
		t.Fatal(err)
	}
	e := c.Entries[0]
	if e.ColumnMap.FirstName != "First Name" || e.ColumnMap.Electorate != "District" {
		t.Fatalf("map=%+v", e.ColumnMap)
	}
	if e.ResolvedLevel != "State Senator" {
		t.Fatalf("level=%q", e.ResolvedLevel)
	}
}

func TestParseConfig_MultipleEntries(t *testing.T) {
	c, err := ParseConfig(wrapEntries(entryJSON(FormatSenators), entryJSON(FormatMembers)))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) != 2 {
		t.Fatalf("entries=%d", len(c.Entries))
	}
	if c.Entries[0].NormalizedFormat != FormatSenators || c.Entries[1].NormalizedFormat != FormatMembers {
		t.Fatalf("formats=%q %q", c.Entries[0].NormalizedFormat, c.Entries[1].NormalizedFormat)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"bad json", `{`},
		{"empty entries", `{"entries":[]}`},
		{"missing entries", `{"interval":"1h"}`},
		{"empty url", `{"entries":[{"csvSourceURL":""}]}`},
		{"unknown format", `{"entries":[{"csvSourceURL":"https://x","format":"nope"}]}`},
		{"custom missing columns", `{"entries":[{"csvSourceURL":"https://x","format":"custom"}]}`},
		{"custom empty columns", `{"entries":[{"csvSourceURL":"https://x","format":"custom","columns":{}}]}`},
		{"bad interval", `{"interval":"nope","entries":[{"csvSourceURL":"https://x"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseConfig([]byte(tc.raw)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "csv_refresh.json")
	body := wrapEntries(`{"csvSourceURL":"https://example.org/p","format":"senators"}`)
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Entries[0].CSVSourceURL != "https://example.org/p" {
		t.Fatalf("url=%q", c.Entries[0].CSVSourceURL)
	}
}

func TestTickerEnabled(t *testing.T) {
	t.Setenv(EnvSkipBackgroundCSVRefresh, "")
	if !TickerEnabled() {
		t.Fatal("want enabled")
	}
	t.Setenv(EnvSkipBackgroundCSVRefresh, "true")
	if TickerEnabled() {
		t.Fatal("want disabled")
	}
	t.Setenv(EnvSkipBackgroundCSVRefresh, "TRUE")
	if TickerEnabled() {
		t.Fatal("want disabled case-insensitive")
	}
}

func TestParseConfig_IgnoresColumnsForSenators(t *testing.T) {
	raw := []byte(`{
		"entries": [{
			"csvSourceURL": "https://example.org/page",
			"format": "senators",
			"columns": {"firstName": "Nope"}
		}]
	}`)
	c, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Entries[0].ColumnMap.FirstName != csv.SenatorColumns.FirstName {
		t.Fatalf("should ignore custom columns: %+v", c.Entries[0].ColumnMap)
	}
}

func TestFormatMessages(t *testing.T) {
	if got := FormatRunStart(2); got != "INFO CSV refresh: starting (2 entries)" {
		t.Fatalf("%q", got)
	}
	if got := FormatEntryStart(1, 2, "allsenel.csv", "senators"); got != "INFO CSV refresh: entry 1/2 filename=allsenel.csv format=senators" {
		t.Fatalf("%q", got)
	}
	if got := FormatStart(); got != "INFO CSV refresh: fetching listing page" {
		t.Fatalf("%q", got)
	}
	if got := FormatDownloadOK("allsenel.csv", 12); got != "INFO CSV refresh: downloaded allsenel.csv (12 bytes)" {
		t.Fatalf("%q", got)
	}
	if got := FormatParsed(3, "senators"); got != "INFO CSV refresh: parsed 3 members from CSV (format=senators)" {
		t.Fatalf("%q", got)
	}
	if got := FormatAdded("Ada Lovelace"); !strings.Contains(got, `added name="Ada Lovelace"`) {
		t.Fatalf("%q", got)
	}
	if got := FormatEndUnchanged(); got != "INFO CSV refresh: no new or merged members" {
		t.Fatalf("%q", got)
	}
	if got := FormatEndChanged(1, 2); got != "INFO CSV refresh: added 1, merged 2 (unsaved — Ctrl+S to persist)" {
		t.Fatalf("%q", got)
	}
	if got := FormatTickerSkipped(); !strings.Contains(got, EnvSkipBackgroundCSVRefresh) {
		t.Fatalf("%q", got)
	}
}
