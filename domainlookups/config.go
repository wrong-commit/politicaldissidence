package domainlookups

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const DefaultConfigPath = "domain_lookups.json"

// Config maps TLD suffixes (no leading dot) to ordered registrar source ids.
type Config map[string][]string

// LoadFile reads and validates domain_lookups.json.
func LoadFile(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfig(raw)
}

// ParseConfig unmarshals TLD → source list JSON.
func ParseConfig(raw []byte) (Config, error) {
	var m map[string][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("bad JSON: %w", err)
	}
	out := make(Config, len(m))
	for k, srcs := range m {
		tld := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(k, ".")))
		if tld == "" {
			return nil, fmt.Errorf("empty TLD key")
		}
		cleaned := make([]string, 0, len(srcs))
		for _, s := range srcs {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "" {
				continue
			}
			cleaned = append(cleaned, s)
		}
		if len(cleaned) == 0 {
			return nil, fmt.Errorf("tld %q: sources must be non-empty", tld)
		}
		out[tld] = cleaned
	}
	return out, nil
}

// SourcesForHostname returns configured sources for the longest matching TLD suffix.
// Returns nil when no TLD matches.
func (c Config) SourcesForHostname(hostname string) []string {
	if c == nil {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return nil
	}

	bestLen := -1
	var best []string
	for tld, srcs := range c {
		if host == tld || strings.HasSuffix(host, "."+tld) {
			if len(tld) > bestLen {
				bestLen = len(tld)
				best = srcs
			}
		}
	}
	if best == nil {
		return nil
	}
	out := make([]string, len(best))
	copy(out, best)
	return out
}
