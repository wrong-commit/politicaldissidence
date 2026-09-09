package searchterms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"politicaldissidence/data"
)

const DefaultConfigPath = "search_terms.json"

// rawTerm is one entry in search_terms.json.
type rawTerm struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Template string `json:"template"`
}

type rawConfig struct {
	Terms []rawTerm `json:"terms"`
}

// TemplateData is the root passed to term templates (no Domains).
type TemplateData struct {
	Honorific     string
	FirstName     string
	Surname       string
	OtherName     string
	PreferredName string
	Electorate    string
	Party         string
	State         string
	Level         string
	Name          string
	NameWithHonorific string
}

// FromMP builds TemplateData from an MP.
func FromMP(mp data.MP) TemplateData {
	return TemplateData{
		Honorific:         mp.Honorific,
		FirstName:         mp.FirstName,
		Surname:           mp.Surname,
		OtherName:         mp.OtherName,
		PreferredName:     mp.PreferredName,
		Electorate:        mp.Electorate,
		Party:             mp.Party,
		State:             mp.State,
		Level:             mp.Level,
		Name:              mp.Name(),
		NameWithHonorific: mp.NameWithHonorific(),
	}
}

// Term is a compiled search-term template.
type Term struct {
	ID    string
	Label string
	tmpl  *template.Template
}

// Config is the loaded (or built-in) term list.
type Config struct {
	Terms []Term
	// FromFile is true when loaded from disk successfully.
	FromFile bool
}

// Builtin returns the two hard-coded terms matching MP.SearchTerm1 / SearchTerm2.
func Builtin() *Config {
	c, err := ParseConfig([]byte(`{
  "terms": [
    {
      "id": "t1",
      "label": "T1",
      "template": "{{.NameWithHonorific}} member for {{.Electorate}} {{.Party}} "
    },
    {
      "id": "t2",
      "label": "T2",
      "template": "{{.Name}} member for {{.Electorate}} {{.Party}} "
    }
  ]
}`))
	if err != nil {
		panic("searchterms: builtin config: " + err.Error())
	}
	c.FromFile = false
	return c
}

// LoadFile reads and validates a config JSON file.
func LoadFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := ParseConfig(raw)
	if err != nil {
		return nil, err
	}
	c.FromFile = true
	return c, nil
}

// ParseConfig unmarshals and validates config JSON bytes.
func ParseConfig(raw []byte) (*Config, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("bad JSON: %w", err)
	}
	if len(rc.Terms) == 0 {
		return nil, fmt.Errorf("terms must contain at least one entry")
	}

	seen := make(map[string]bool, len(rc.Terms))
	out := make([]Term, 0, len(rc.Terms))
	for i, rt := range rc.Terms {
		n := i + 1
		id := strings.TrimSpace(rt.ID)
		if id == "" {
			id = fmt.Sprintf("T%d", n)
		}
		if seen[id] {
			return nil, fmt.Errorf("terms[%d]: duplicate id %q", i, id)
		}
		seen[id] = true

		label := strings.TrimSpace(rt.Label)
		if label == "" {
			label = id
		}

		tmplStr := strings.TrimSpace(rt.Template)
		if tmplStr == "" {
			return nil, fmt.Errorf("terms[%d]: template is required", i)
		}
		tmpl, err := template.New(id).Parse(rt.Template)
		if err != nil {
			return nil, fmt.Errorf("terms[%d]: template: %w", i, err)
		}
		out = append(out, Term{ID: id, Label: label, tmpl: tmpl})
	}
	return &Config{Terms: out}, nil
}

// Len returns the number of terms.
func (c *Config) Len() int {
	if c == nil {
		return 0
	}
	return len(c.Terms)
}

// ClampIndex returns i forced into [0, Len()-1]. changed is true when clamped.
func (c *Config) ClampIndex(i int) (clamped int, changed bool) {
	n := c.Len()
	if n == 0 {
		return 0, i != 0
	}
	if i < 0 {
		return 0, true
	}
	if i >= n {
		return n - 1, true
	}
	return i, false
}

// WrapIndex moves i by delta with wrap-around. ok is false when Len() <= 1
// (caller should no-op without re-fetching).
func (c *Config) WrapIndex(i, delta int) (next int, ok bool) {
	n := c.Len()
	if n <= 1 {
		return 0, false
	}
	i, _ = c.ClampIndex(i)
	next = ((i+delta)%n + n) % n
	return next, true
}

// Render executes terms[i] against mp. Index is clamped.
func (c *Config) Render(i int, mp data.MP) (string, error) {
	if c == nil || c.Len() == 0 {
		return "", fmt.Errorf("no search terms")
	}
	i, _ = c.ClampIndex(i)
	var buf bytes.Buffer
	if err := c.Terms[i].tmpl.Execute(&buf, FromMP(mp)); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// DisplayIndex returns 1-based "Tn/N" for UI titles.
func (c *Config) DisplayIndex(i int) string {
	n := c.Len()
	if n == 0 {
		return "T?/0"
	}
	i, _ = c.ClampIndex(i)
	return fmt.Sprintf("T%d/%d", i+1, n)
}

// LabelAt returns the term label at index i (clamped).
func (c *Config) LabelAt(i int) string {
	if c == nil || c.Len() == 0 {
		return ""
	}
	i, _ = c.ClampIndex(i)
	return c.Terms[i].Label
}
