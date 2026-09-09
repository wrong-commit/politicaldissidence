package csvrefresh

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"politicaldissidence/csv"
	"politicaldissidence/data"
)

const (
	// DefaultConfigPath is the config file next to the app binary / working directory.
	DefaultConfigPath = "csv_refresh.json"
	// EnvSkipBackgroundCSVRefresh disables the hourly ticker when set to "true".
	EnvSkipBackgroundCSVRefresh = "SKIP_BACKGROUND_CSV_REFRESH"

	FormatSenators = "senators"
	FormatMembers  = "members"
	FormatCustom   = "custom"
)

// ColumnConfig is the JSON shape for custom CSV header mapping.
type ColumnConfig struct {
	Honorific     string `json:"honorific"`
	FirstName     string `json:"firstName"`
	Surname       string `json:"surname"`
	OtherName     string `json:"otherName"`
	PreferredName string `json:"preferredName"`
	Party         string `json:"party"`
	State         string `json:"state"`
	Electorate    string `json:"electorate"`
}

// Entry is one CSV source in csv_refresh.json.
type Entry struct {
	CSVSourceURL string        `json:"csvSourceURL"`
	CSVFilename  string        `json:"csvFilename"`
	Format       string        `json:"format"`
	Columns      *ColumnConfig `json:"columns,omitempty"`
	Level        string        `json:"level"`

	// Resolved at load time (not JSON).
	ColumnMap        csv.ColumnMap `json:"-"`
	ResolvedLevel    string        `json:"-"`
	NormalizedFormat string        `json:"-"`
}

// Config is the on-disk CSV refresh file (interval + one or more entries).
type Config struct {
	Interval string  `json:"interval"`
	Entries  []Entry `json:"entries"`

	// Resolved at load time (not JSON).
	IntervalDuration time.Duration `json:"-"`
}

// LoadFile reads and validates a config JSON file.
func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(raw)
}

// ParseConfig unmarshals and validates config JSON bytes.
func ParseConfig(raw []byte) (*Config, error) {
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("bad JSON: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate fills defaults and resolved fields.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Interval) == "" {
		c.Interval = "1h"
	}
	d, err := time.ParseDuration(c.Interval)
	if err != nil {
		return fmt.Errorf("interval: %w", err)
	}
	if d <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	c.IntervalDuration = d

	if len(c.Entries) == 0 {
		return fmt.Errorf("entries must contain at least one CSV source")
	}
	for i := range c.Entries {
		if err := c.Entries[i].Validate(); err != nil {
			return fmt.Errorf("entries[%d]: %w", i, err)
		}
	}
	return nil
}

// Validate fills defaults and resolved fields for one entry.
func (e *Entry) Validate() error {
	e.CSVSourceURL = strings.TrimSpace(e.CSVSourceURL)
	if e.CSVSourceURL == "" {
		return fmt.Errorf("csvSourceURL is required")
	}
	if strings.TrimSpace(e.CSVFilename) == "" {
		e.CSVFilename = "allsenel.csv"
	}

	format := strings.ToLower(strings.TrimSpace(e.Format))
	if format == "" {
		format = FormatSenators
	}
	e.NormalizedFormat = format

	switch format {
	case FormatSenators:
		e.ColumnMap = csv.SenatorColumns
		e.ResolvedLevel = strings.TrimSpace(e.Level)
		if e.ResolvedLevel == "" {
			e.ResolvedLevel = data.Level.FedSenator
		}
	case FormatMembers:
		e.ColumnMap = csv.MemberColumns
		e.ResolvedLevel = strings.TrimSpace(e.Level)
		if e.ResolvedLevel == "" {
			e.ResolvedLevel = data.Level.FedRep
		}
	case FormatCustom:
		if e.Columns == nil {
			return fmt.Errorf("columns required when format=custom")
		}
		e.ColumnMap = csv.ColumnMap{
			Honorific:     e.Columns.Honorific,
			FirstName:     e.Columns.FirstName,
			Surname:       e.Columns.Surname,
			OtherName:     e.Columns.OtherName,
			PreferredName: e.Columns.PreferredName,
			Party:         e.Columns.Party,
			State:         e.Columns.State,
			Electorate:    e.Columns.Electorate,
		}
		if !columnMapUsable(e.ColumnMap) {
			return fmt.Errorf("columns must map at least one non-empty header")
		}
		e.ResolvedLevel = strings.TrimSpace(e.Level)
	default:
		return fmt.Errorf("unknown format %q (want senators, members, or custom)", e.Format)
	}
	return nil
}

func columnMapUsable(m csv.ColumnMap) bool {
	for _, h := range []string{
		m.Honorific, m.FirstName, m.Surname, m.OtherName,
		m.PreferredName, m.Party, m.State, m.Electorate,
	} {
		if strings.TrimSpace(h) != "" {
			return true
		}
	}
	return false
}

// TickerEnabled reports whether the hourly ticker should arm.
// SKIP_BACKGROUND_CSV_REFRESH=true disables it; Ctrl+L is unaffected.
func TickerEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvSkipBackgroundCSVRefresh))
	return !strings.EqualFold(v, "true")
}
