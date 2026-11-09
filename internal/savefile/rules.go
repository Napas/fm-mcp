package savefile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	// nationRulesOffset is where a 27.0.0 men's national-team container
	// record holds its rules tree, a struct without its type byte (no
	// fields when the nation has no rules).
	nationRulesOffset = 180
	// compNationOffset is the u32 NATION index in a 27.0.0 COMP record.
	compNationOffset = 57
	// The COMP record's list of format trees starts at offset 122, 124, 178
	// or 180 (27.0.0); the search covers compFormatStart..compFormatEnd.
	compFormatStart = compNationOffset + 4
	compFormatEnd   = 256
	maxCompFormats  = 64
)

// SquadRule is a squad-selection rule of a rule group (an "sqsr" item).
type SquadRule struct {
	Type      uint16 // rule type; the names are in the game's table (see "27.0.0 competition rules")
	Value     uint32
	HasValue  bool     // false for fee rules, which hold an amount instead
	Scope     uint8    // "cmty": 255 or 1 seen; meaning unknown
	StartYear uint16   // 0 when the rule has no start year
	EndYear   uint16   // 0 when the rule has no end year
	Other     []string // tags of further conditions, not decoded, e.g. "GmTy"
}

// RuleGroup is a named group of competition rules in a nation's rules file
// (an "fxrl" item), e.g. Lithuania's "TOPLYGA".
type RuleGroup struct {
	Name  string
	Squad []SquadRule
}

// CompetitionRules ties a competition to its nation's rule group.
type CompetitionRules struct {
	Nation int // NATION index
	Group  int // index into the nation's RuleGroups
}

// rulesGate is the first database version the offsets were verified on;
// the game's own gate may be lower.
const rulesGate = dbVersion27_0

// readGameRules reads the rules (see readRules) and sets the nations' rule
// groups. It needs decoded nations (nationsErr nil) and a save from rulesGate.
func readGameRules(body []byte, version uint16, ns []Nation, nationsErr error, containers []int, comps []Record) (map[int]CompetitionRules, error) {
	switch {
	case version < rulesGate:
		return nil, fmt.Errorf("%w: competition rules are read from 27.0.0 saves only", ErrUnsupported)
	case nationsErr != nil:
		return nil, fmt.Errorf("competition rules need the nations: %w", nationsErr)
	}
	return readRules(body, containers, ns, comps)
}

// readRules sets the rule groups of the nations whose men's containers
// (bounds from readNationFactors) hold a rules tree, and returns the rule
// group of each competition that names one, by COMP index. The error lists
// the nations whose rules do not decode; the rest are still read.
func readRules(body []byte, containers []int, ns []Nation, comps []Record) (map[int]CompetitionRules, error) {
	if len(containers) != len(ns)+1 {
		return nil, fmt.Errorf("%w: national-team containers", ErrNotFound)
	}
	prefixes := make([]string, len(ns))
	var errs []error
	for i := range ns {
		var err error
		ns[i].RuleGroups, prefixes[i], err = nationRules(body[containers[i]:containers[i+1]])
		if err != nil {
			errs = append(errs, fmt.Errorf("nation %d rules: %w", ns[i].Index, err))
		}
	}
	out := make(map[int]CompetitionRules)
	for i, c := range comps {
		if r, ok := competitionRules(body[c.Start:c.End], c.UID, ns, prefixes); ok {
			out[i] = r
		}
	}
	return out, errors.Join(errs...)
}

// nationRules reads the rules tree of the container record rec and returns
// its rule groups and its file's prefix, e.g. "lit_" for "lit_rules".
func nationRules(rec []byte) ([]RuleGroup, string, error) {
	tree, _, err := readTreePayload(rec, nationRulesOffset, treeStruct)
	if err != nil || len(tree.fields) == 0 {
		return nil, "", err
	}
	file, _ := tree.field("file")
	prefix, ok := strings.CutSuffix(file.str, "rules")
	if !ok {
		return nil, "", fmt.Errorf("%w: rules file %q", ErrCorrupt, file.str)
	}
	list, _ := tree.field("fxrl")
	groups := make([]RuleGroup, 0, len(list.items))
	for _, it := range list.items {
		name, _ := it.field("name")
		g := RuleGroup{Name: name.str}
		rules, _ := it.field("sqsr")
		for _, r := range rules.items {
			g.Squad = append(g.Squad, squadRule(r))
		}
		groups = append(groups, g)
	}
	return groups, prefix, nil
}

func squadRule(v treeValue) SquadRule {
	var r SquadRule
	for _, f := range v.fields {
		n, _ := f.value.uint()
		switch f.tag {
		case "type":
			r.Type = uint16(n) //nolint:gosec // rule types are below 500
		case "valu":
			r.Value, r.HasValue = uint32(n), true //nolint:gosec // at most 4 bytes
		case "cmty":
			r.Scope = uint8(n) //nolint:gosec // a u8
		case "styr":
			r.StartYear = uint16(n) //nolint:gosec // a year
		case "enyr":
			r.EndYear = uint16(n) //nolint:gosec // a year
		default:
			r.Other = append(r.Other, f.tag)
		}
	}
	return r
}

// competitionRules reads the COMP record rec of competition uid. It holds
// its NATION index and a list of format trees, each naming the competition
// and a rule-group index ("fxri"). The rule group counts when every format
// names the same one, it exists, and the formats' files share the nation's
// rules prefix: women's competitions ("wmn_eng_…") point into the women's
// rules, which are not read.
func competitionRules(rec []byte, uid uint32, ns []Nation, prefixes []string) (CompetitionRules, bool) {
	if len(rec) < compFormatStart {
		return CompetitionRules{}, false
	}
	nation := int(binary.LittleEndian.Uint32(rec[compNationOffset:]))
	if nation >= len(ns) {
		return CompetitionRules{}, false
	}
	formats, ok := formatList(rec, uid)
	if !ok {
		return CompetitionRules{}, false
	}
	group := -1
	for _, f := range formats {
		file, _ := f.field("file")
		fx, _ := f.field("fxri")
		n, ok := fx.uint()
		if !ok || !strings.HasPrefix(file.str, prefixes[nation]) || group >= 0 && int(n) != group {
			return CompetitionRules{}, false
		}
		group = int(n)
	}
	// 255 means none, and is past the last group.
	if group >= len(ns[nation].RuleGroups) {
		return CompetitionRules{}, false
	}
	return CompetitionRules{Nation: nation, Group: group}, true
}

// formatList finds the COMP record's list of format trees: the first list
// in the search window that decodes and whose every item names competition
// uid. The tries share one node budget (largest real list: 10 934 values,
// 27.0.0); wrong tries that spend it hide a later list.
func formatList(rec []byte, uid uint32) ([]treeValue, bool) {
	nodes := maxTreeNodes
	for p := compFormatStart; p < min(compFormatEnd, len(rec)-6) && nodes > 0; p++ {
		n := binary.LittleEndian.Uint32(rec[p:])
		if n == 0 || n > maxCompFormats || rec[p+4] != treeMarker || rec[p+5] != treeStruct {
			continue
		}
		list, _, err := readTreeNodes(rec, p, treeList, &nodes)
		if err == nil && namesOnly(list.items, uid) {
			return list.items, true
		}
	}
	return nil, false
}

func namesOnly(formats []treeValue, uid uint32) bool {
	for _, f := range formats {
		c, _ := f.field("comp")
		if got, ok := c.ref(); !ok || got != uid {
			return false
		}
	}
	return true
}
