package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"runtime"
	"testing"
)

// payload drops a tagged-tree value's type byte: the records hold their
// trees without one.
func payload(v []byte) []byte { return v[1:] }

// rulesTree is a nation's rules tree with file "<prefix>rules" and the given
// rule groups.
func rulesTree(prefix string, groups ...[]byte) []byte {
	return tStruct(
		tField("fles", tString(prefix+"comps / "+prefix+"rules")),
		tField("fxrl", tList(groups...)),
		tField("file", tString(prefix+"rules")),
	)
}

func ruleGroup(name string, rules ...[]byte) []byte {
	return tStruct(tField("name", tString(name)), tField("sqsr", tList(rules...)), tField("tcpf", tU8(0)))
}

func tSquadRule(scope uint8, typ uint16, value []byte, extra ...[]byte) []byte {
	fields := [][]byte{tField("cmty", tU8(scope)), tField("type", tU16(typ))}
	if value != nil {
		fields = append(fields, tField("valu", value))
	}
	return tStruct(append(fields, extra...)...)
}

// containerWithRules encodes a 27.0.0 men's container record holding tree
// at nationRulesOffset.
func containerWithRules(b []byte, index, uid uint32, tree []byte) []byte {
	rec := make([]byte, nationRulesOffset)
	rec[0] = tagContainer
	binary.LittleEndian.PutUint16(rec[168:], 74)
	return dbRecord(append(append(b, rec...), payload(tree)...), index, uid)
}

var (
	lithuaniaRules = rulesTree("lit_",
		ruleGroup("TOPLYGA",
			tSquadRule(255, 9, tU8(30)),
			tSquadRule(1, 94, tU8(3), tField("styr", tU16(2026)), tField("enyr", tU16(2030)), tField("GmTy", tU8(1))),
			tSquadRule(1, 141, tU16(500)),
		),
		ruleGroup("Lithuanian Cup", tSquadRule(1, 94, tU8(3))),
		tStruct(tField("name", tString("No squad rules"))),
	)
	lithuaniaGroups = []RuleGroup{
		{Name: "TOPLYGA", Squad: []SquadRule{
			{Type: 9, Value: 30, HasValue: true, Scope: 255},
			{Type: 94, Value: 3, HasValue: true, Scope: 1, StartYear: 2026, EndYear: 2030, Other: []string{"GmTy"}},
			{Type: 141, Value: 500, HasValue: true, Scope: 1},
		}},
		{Name: "Lithuanian Cup", Squad: []SquadRule{{Type: 94, Value: 3, HasValue: true, Scope: 1}}},
		{Name: "No squad rules"},
	}
)

// rulesBody is a 27.0.0 container table for nations 0 (no rules), 1
// (Lithuania's) and 2 (a tree that does not decode), and the nations.
func rulesBody(t *testing.T) ([]byte, []int, []Nation) {
	t.Helper()
	var recs []byte
	recs = containerWithRules(recs, 0, 5, tStruct())
	recs = containerWithRules(recs, 1, 779, lithuaniaRules)
	recs = containerWithRules(recs, 2, 790, tStruct(tField("what", tScalar(0x42))))
	body := tableBytes(3, recs)
	ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 779}, {Index: 2, UID: 790}}
	bounds, err := readNationFactors(body, 3, ns, dbVersion27_0)
	if err != nil {
		t.Fatalf("readNationFactors: %v", err)
	}
	return body, bounds, ns
}

func TestReadRulesGivesNationsTheirRuleGroups(t *testing.T) {
	body, bounds, ns := rulesBody(t)

	_, err := readRules(body, bounds, ns, nil)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt for nation 2", err)
	}
	if ns[0].RuleGroups != nil || ns[2].RuleGroups != nil {
		t.Errorf("rule groups of nations 0 and 2 = %v, %v; want none", ns[0].RuleGroups, ns[2].RuleGroups)
	}
	if !reflect.DeepEqual(ns[1].RuleGroups, lithuaniaGroups) {
		t.Errorf("Lithuania's rule groups = %+v\nwant %+v", ns[1].RuleGroups, lithuaniaGroups)
	}
}

// compRecord encodes a 27.0.0 COMP record of nation whose format list sits
// at listAt, and returns it with its Record.
func compRecord(b []byte, index, uid, nation uint32, listAt int, formats ...[]byte) ([]byte, Record) {
	start := len(b)
	rec := make([]byte, listAt)
	binary.LittleEndian.PutUint32(rec[compNationOffset:], nation)
	rec = append(rec, payload(tList(formats...))...)
	rec = append(rec, make([]byte, 40)...) // the record's undecoded rest
	b = dbRecord(append(b, rec...), index, uid)
	return b, Record{Index: index, UID: uid, Start: start, End: len(b)}
}

func format(comp []byte, file string, fxri uint8) []byte {
	return tStruct(tField("ftye", tU8(1)), tField("comp", comp), tField("fxri", tU8(fxri)), tField("file", tString(file)))
}

func TestReadRulesTiesCompetitionsToRuleGroups(t *testing.T) {
	body, bounds, ns := rulesBody(t)
	var comps []Record
	add := func(index, uid, nation uint32, listAt int, formats ...[]byte) {
		var rec Record
		body, rec = compRecord(body, index, uid, nation, listAt, formats...)
		comps = append(comps, rec)
	}
	add(0, 2300046, 1, 180, format(tU32(2300046), "lit_a_lyga", 0), format(tU32(2300046), "lit_a_lyga_9_teams", 0))
	add(1, 2300047, 1, 122, format(tStruct(tField("id  ", tTag("comp")), tField("comp", tPair(2300047, 2300047))), "lit_cup", 1))
	add(2, 3, 1, 124, format(tU32(3), "wmn_lit_first", 0))                      // a women's competition
	add(3, 4, 1, 178, format(tU32(4), "lit_super_cup", 255))                    // no rule group
	add(4, 5, 1, 180, format(tU32(5), "lit_playoff", 3))                        // past the last group
	add(5, 6, 1, 180, format(tU32(6), "lit_a", 0), format(tU32(6), "lit_b", 1)) // variants disagree
	add(6, 7, 1, 180, format(tU32(99), "lit_a", 0))                             // another competition's list
	add(7, 8, 3, 180, format(tU32(8), "lit_a", 0))                              // no such nation
	add(8, 9, 0, 180, format(tU32(9), "egy_prem", 0))                           // a nation without rules
	add(9, 10, 1, 180)                                                          // no formats
	add(10, 11, 1, 180, format(tU32(11), "lit_b", 1),                           // a list that does not decode
		tStruct(tField("comp", tU32(11)), tField("fxri", tU8(1)), tField("file", tString("lit_b")), tField("what", tScalar(0x42))))
	add(11, 12, 1, compFormatEnd, format(tU32(12), "lit_a", 0))                              // past the search window
	add(12, 13, 1, 180, tStruct(tField("comp", tU32(13)), tField("file", tString("lit_a")))) // no rule index

	got, _ := readRules(body, bounds, ns, comps)

	want := map[int]CompetitionRules{0: {Nation: 1, Group: 0}, 1: {Nation: 1, Group: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("competition rules = %v, want %v", got, want)
	}
}

func TestReadRulesNeedsTheContainers(t *testing.T) {
	ns := []Nation{{Index: 0, UID: 5}}
	if got, err := readRules(nil, nil, ns, nil); got != nil || !errors.Is(err, ErrNotFound) {
		t.Errorf("readRules = %v, %v; want nil, ErrNotFound", got, err)
	}
}

func TestReadGameRulesNeeds27AndTheNations(t *testing.T) {
	body, bounds, ns := rulesBody(t)
	for _, tt := range []struct {
		name    string
		version uint16
		nations error
		want    error
	}{
		{"26.3", dbVersion26_3, nil, ErrUnsupported},
		{"no nations", dbVersion27_0, ErrCorrupt, ErrCorrupt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := readGameRules(body, tt.version, ns, tt.nations, bounds, nil); got != nil || !errors.Is(err, tt.want) {
				t.Errorf("readGameRules = %v, %v; want nil, %v", got, err, tt.want)
			}
		})
	}
	if _, err := readGameRules(body, dbVersion27_0, ns, nil, bounds, nil); !errors.Is(err, ErrCorrupt) || ns[1].RuleGroups == nil {
		t.Errorf("27.0.0: err = %v, Lithuania's groups %v; want nation 2's ErrCorrupt and Lithuania's groups", err, ns[1].RuleGroups)
	}
}

func TestNationRulesNeedARulesFile(t *testing.T) {
	rec := append(make([]byte, nationRulesOffset), payload(tStruct(tField("file", tString("lit_comps")), tField("fxrl", tList())))...)
	if groups, _, err := nationRules(rec); groups != nil || !errors.Is(err, ErrCorrupt) {
		t.Errorf("nationRules = %v, %v; want nil, ErrCorrupt", groups, err)
	}
}

// Lists starting at many offsets share one node budget: 14 list heads each
// decode the same 60 000 empty fields, then fail.
func TestFormatListSharesANodeBudget(t *testing.T) {
	const heads, fields = 14, 60000
	head := binary.LittleEndian.AppendUint32(nil, 1)
	head = binary.LittleEndian.AppendUint32(append(head, treeMarker, treeStruct), fields)
	rec := append(make([]byte, compFormatStart), bytes.Repeat(head, heads)...)
	rec = append(rec, bytes.Repeat([]byte{'a', 'b', 'c', 'd', treeMarker, 0x00}, fields)...)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, ok := formatList(rec, 1); ok {
		t.Error("formatList found a list")
	}
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 64<<20 {
		t.Errorf("formatList allocated %d bytes for a %d-byte record", got, len(rec))
	}
}
