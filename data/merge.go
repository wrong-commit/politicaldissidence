package data

import "strings"

// MergeResult is the output of MergeMPs.
type MergeResult struct {
	MPs         []MP
	MergedNames []string // display names that collapsed at least once (order of first merge)
	AddedNames  []string // names from b that were not in a (order of first appearance in b)
}

// MergeMPs merges two MP slices (a then b). Later records win for party/bio fields
// when names match; domains are unioned. Name-merged members get domain checks cleared.
func MergeMPs(a, b []MP) MergeResult {
	var out []MP
	indexByName := make(map[string]int)
	mergedOnce := make(map[string]struct{})
	var mergedNames []string
	var addedNames []string

	fromA := make(map[string]struct{}, len(a))
	for _, mp := range a {
		fromA[nameKey(mp)] = struct{}{}
	}

	appendOrMerge := func(mp MP, fromB bool) {
		key := nameKey(mp)
		if i, ok := indexByName[key]; ok {
			out[i] = mergeMP(out[i], mp)
			if _, seen := mergedOnce[key]; !seen {
				mergedOnce[key] = struct{}{}
				mergedNames = append(mergedNames, out[i].Name())
			}
			return
		}
		indexByName[key] = len(out)
		out = append(out, cloneMP(mp))
		if fromB {
			if _, existed := fromA[key]; !existed {
				addedNames = append(addedNames, out[len(out)-1].Name())
			}
		}
	}

	for _, mp := range a {
		appendOrMerge(mp, false)
	}
	for _, mp := range b {
		appendOrMerge(mp, true)
	}
	return MergeResult{MPs: out, MergedNames: mergedNames, AddedNames: addedNames}
}

func nameKey(mp MP) string {
	return strings.ToLower(strings.TrimSpace(mp.Name()))
}

func cloneMP(mp MP) MP {
	cp := mp
	if mp.Domains != nil {
		cp.Domains = append([]Domain(nil), mp.Domains...)
	}
	return cp
}

// mergeMP folds later into existing: later party/bio wins; domains unioned with checks cleared.
func mergeMP(existing, later MP) MP {
	merged := existing
	merged.Honorific = later.Honorific
	merged.FirstName = later.FirstName
	merged.Surname = later.Surname
	merged.OtherName = later.OtherName
	merged.PreferredName = later.PreferredName
	merged.Electorate = later.Electorate
	merged.Party = later.Party
	merged.State = later.State
	merged.Level = later.Level
	merged.Domains = mergeDomainsClearChecks(existing.Domains, later.Domains)
	return merged
}

func mergeDomainsClearChecks(a, b []Domain) []Domain {
	seen := make(map[string]struct{})
	var out []Domain
	add := func(list []Domain) {
		for _, d := range list {
			host := strings.TrimSpace(d.Hostname)
			if host == "" {
				continue
			}
			key := strings.ToLower(host)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, Domain{Hostname: host})
		}
	}
	add(a)
	add(b)
	if out == nil {
		return []Domain{}
	}
	return out
}
