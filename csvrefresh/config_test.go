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

func TestParseConfig_SenatorsDefaults(t *testing.T) {
	raw := []byte(`{
		"csvSourceURL": "https://example.org/page",
		"csvFilename": "allsenel.csv",
		"format": "senators"
	}`)
	c, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.NormalizedFormat != FormatSenators {
		t.Fatalf("format=%q", c.NormalizedFormat)
	}
	if c.ColumnMap != csv.SenatorColumns {
		t.Fatalf("columns=%+v", c.ColumnMap)
	}
	if c.ResolvedLevel != data.Level.FedSenator {
		t.Fatalf("level=%q", c.ResolvedLevel)
	}
	if c.IntervalDuration != time.Hour {
		t.Fatalf("interval=%v", c.IntervalDuration)
	}
}

func TestParseConfig_Members(t *testing.T) {
	raw := []byte(`{
		"csvSourceURL": "https://example.org/page",
		"csvFilename": "FamilynameRepsCSV.csv",
		"format": "members"
	}`)
	c, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.ColumnMap != csv.MemberColumns {
		t.Fatalf("columns=%+v", c.ColumnMap)
	}
	if c.ResolvedLevel != data.Level.FedRep {
		t.Fatalf("level=%q", c.ResolvedLevel)
	}
}

func TestParseConfig_Custom(t *testing.T) {
	raw := []byte(`{
		"csvSourceURL": "https://example.org/page",
		"csvFilename": "roster.csv",
		"format": "custom",
		"level": "State Senator",
		"columns": {
			"honorific": "Title",
			"firstName": "First Name",
			"surname": "Surname",
			"otherName": "",
			"preferredName": "",
			"party": "Party",
			"state": "State",
			"electorate": "District"
		}
	}`)
	c, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.ColumnMap.FirstName != "First Name" || c.ColumnMap.Electorate != "District" {
		t.Fatalf("map=%+v", c.ColumnMap)
	}
	if c.ResolvedLevel != "State Senator" {
		t.Fatalf("level=%q", c.ResolvedLevel)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty url", `{"csvSourceURL":""}`},
		{"bad json", `{`},
		{"unknown format", `{"csvSourceURL":"https://x","format":"nope"}`},
		{"custom missing columns", `{"csvSourceURL":"https://x","format":"custom"}`},
		{"custom empty columns", `{"csvSourceURL":"https://x","format":"custom","columns":{}}`},
		{"bad interval", `{"csvSourceURL":"https://x","interval":"nope"}`},
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
	body := `{"csvSourceURL":"https://example.org/p","format":"senators"}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.CSVSourceURL != "https://example.org/p" {
		t.Fatalf("url=%q", c.CSVSourceURL)
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
		"csvSourceURL": "https://example.org/page",
		"format": "senators",
		"columns": {"firstName": "Nope"}
	}`)
	c, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.ColumnMap.FirstName != csv.SenatorColumns.FirstName {
		t.Fatalf("should ignore custom columns: %+v", c.ColumnMap)
	}
}

func TestFormatMessages(t *testing.T) {
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
