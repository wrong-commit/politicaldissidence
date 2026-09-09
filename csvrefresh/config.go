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

// Config is the on-disk CSV refresh configuration.
type Config struct {
	CSVSourceURL string        `json:"csvSourceURL"`
	CSVFilename  string        `json:"csvFilename"`
	Interval     string        `json:"interval"`
	Format       string        `json:"format"`
	Columns      *ColumnConfig `json:"columns,omitempty"`
	Level        string        `json:"level"`

	// Resolved at load time (not JSON).
	IntervalDuration time.Duration `json:"-"`
	ColumnMap        csv.ColumnMap `json:"-"`
	ResolvedLevel    string        `json:"-"`
	NormalizedFormat string        `json:"-"`
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
	c.CSVSourceURL = strings.TrimSpace(c.CSVSourceURL)
	if c.CSVSourceURL == "" {
		return fmt.Errorf("csvSourceURL is required")
	}
	if strings.TrimSpace(c.CSVFilename) == "" {
		c.CSVFilename = "allsenel.csv"
	}
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

	format := strings.ToLower(strings.TrimSpace(c.Format))
	if format == "" {
		format = FormatSenators
	}
	c.NormalizedFormat = format

	switch format {
	case FormatSenators:
		c.ColumnMap = csv.SenatorColumns
		c.ResolvedLevel = strings.TrimSpace(c.Level)
		if c.ResolvedLevel == "" {
			c.ResolvedLevel = data.Level.FedSenator
		}
	case FormatMembers:
		c.ColumnMap = csv.MemberColumns
		c.ResolvedLevel = strings.TrimSpace(c.Level)
		if c.ResolvedLevel == "" {
			c.ResolvedLevel = data.Level.FedRep
		}
	case FormatCustom:
		if c.Columns == nil {
			return fmt.Errorf("columns required when format=custom")
		}
		c.ColumnMap = csv.ColumnMap{
			Honorific:     c.Columns.Honorific,
			FirstName:     c.Columns.FirstName,
			Surname:       c.Columns.Surname,
			OtherName:     c.Columns.OtherName,
			PreferredName: c.Columns.PreferredName,
			Party:         c.Columns.Party,
			State:         c.Columns.State,
			Electorate:    c.Columns.Electorate,
		}
		if !columnMapUsable(c.ColumnMap) {
			return fmt.Errorf("columns must map at least one non-empty header")
		}
		c.ResolvedLevel = strings.TrimSpace(c.Level)
	default:
		return fmt.Errorf("unknown format %q (want senators, members, or custom)", c.Format)
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
