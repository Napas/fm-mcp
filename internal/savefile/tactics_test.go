package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Synthetic tactics_man.dat bodies (layout in docs/format.md).

func appendPosition(b []byte, p SlotRole, options int) []byte {
	b = binary.LittleEndian.AppendUint16(b, 0x42)
	b = append(b, 2)
	b = binary.LittleEndian.AppendUint32(b, p.Position)
	b = append(b, 0xff, 0, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, uint32(options)) //nolint:gosec // test sizes
	for i := range options {
		b = binary.LittleEndian.AppendUint64(b, p.Role|uint64(i))
		b = append(b, 1, 1, 2)                          // bitset head, 2 words
		b = binary.LittleEndian.AppendUint64(b, 0x0f0f) // the two words
		b = append(b, 0)
		b = binary.LittleEndian.AppendUint32(b, 7)
	}
	b = binary.LittleEndian.AppendUint32(b, 1) // one (u32, u64) pair
	b = binary.LittleEndian.AppendUint32(b, 3)
	b = binary.LittleEndian.AppendUint64(b, 9)
	return binary.LittleEndian.AppendUint64(b, p.Role)
}

// appendTactic encodes t; extra adds the optional maps and block.
func appendTactic(b []byte, t Tactic, extra bool) []byte {
	b = append(b, tacticTag)
	b = binary.LittleEndian.AppendUint16(b, tacticObjectVersion)
	b = append(b, 0x1a, 3, 0, 1, 2)
	b = appendString(b, t.Name)
	for range 3 {
		b = appendString(b, "")
	}
	b = append(b, tacticHeaderTag, 2, t.Mentality, 1, 1, 3)
	for range 3 {
		b = binary.LittleEndian.AppendUint32(b, 0xabcd)
	}
	b = append(b, 0)
	b = appendString(b, "Custom")
	b = append(b, "LLUN"...)
	for i, s := range t.Slots {
		b = appendPosition(b, s.InPossession, 2)
		b = appendPosition(b, s.OutOfPossession, 1)
		b = append(b, byte(i))
	}
	if !extra {
		return append(b, 0)
	}
	b = append(b, 1)
	for range 2 { // two position maps of one key each
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = binary.LittleEndian.AppendUint32(b, 5)
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = appendPosition(b, SlotRole{Position: 0x10, Role: 0x200002}, 1)
	}
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 5)
	b = binary.LittleEndian.AppendUint32(b, 2)
	b = append(b, 1, 2, 3, 0, 0, 0, 4, 5, 6, 0, 0, 0)
	b = binary.LittleEndian.AppendUint32(b, 1)
	return append(b, make([]byte, tacticBlockSize-4)...)
}

func appendPersonList(b []byte, persons ...int32) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(persons))) //nolint:gosec // test sizes
	for _, p := range persons {
		b = append(b, 2)
		b = binary.LittleEndian.AppendUint32(b, uint32(p)) //nolint:gosec // test values
	}
	return b
}

func appendSetPieces(b []byte) []byte {
	b = binary.LittleEndian.AppendUint16(b, setPiecesVersion)
	b = appendPersonList(b, 55473, 40427)
	b = appendPersonList(b)
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 55473)
	b = append(b, 1, 2, 3)
	for range 3 {
		b = appendPersonList(b, 77416)
	}
	for range 7 {
		b = appendPersonList(b)
	}
	b = append(b, 0)
	for range 8 {
		b = appendPersonList(b, 147261, 163497)
	}
	return append(b, 1, 4, 5)
}

func appendSelection(b []byte, sel [selectionSize]int32) []byte {
	b = append(b, selectionTag)
	b = appendString(b, "")
	for _, p := range sel {
		b = binary.LittleEndian.AppendUint32(b, uint32(p)) //nolint:gosec // test values
	}
	return b
}

// appendTeamEntry encodes a team entry keyed by team: its head (selection,
// set pieces, tactics) and a tail with keyed records and routine vectors of
// the given lengths.
func appendTeamEntry(b []byte, team int32, sel [selectionSize]int32, active int32, keyed, routines int, tactics ...Tactic) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(team)) //nolint:gosec // test values
	b = append(b, teamEntryTag)
	b = appendSelection(b, sel)
	b = appendSetPieces(b)
	b = binary.LittleEndian.AppendUint32(b, uint32(active))       //nolint:gosec // test values
	b = binary.LittleEndian.AppendUint16(b, uint16(len(tactics))) //nolint:gosec // test sizes
	for _, t := range tactics {
		b = appendTactic(b, t, true)
	}
	return appendTeamEntryTail(b, keyed, routines)
}

// appendRecord22 encodes a 22-byte record: four zero i32 sliders and six
// flag bytes, the last set, as every sample holds it.
func appendRecord22(b []byte) []byte {
	return append(append(b, make([]byte, 16)...), 0, 0, 0, 0, 0, 1)
}

func appendTeamEntryTail(b []byte, keyed, routines int) []byte {
	b = binary.LittleEndian.AppendUint16(b, teamRecordsVersion)
	b = binary.LittleEndian.AppendUint32(b, teamRecords)
	for range teamRecords {
		b = appendRecord22(b)
	}
	b = binary.LittleEndian.AppendUint16(b, uint16(keyed)) //nolint:gosec // test sizes
	for i := range keyed {
		b = binary.LittleEndian.AppendUint32(b, uint32(1000+i)) //nolint:gosec // test values
		b = appendRecord22(b)
	}
	b = binary.LittleEndian.AppendUint32(b, 0)
	return appendSetPieceRoutines(b, routines)
}

// appendRoutine encodes one set-piece routine: a player in each of the 8
// categories, a priority order, one instruction triple.
func appendRoutine(b []byte) []byte {
	for g := range routineGroups {
		b = append(b, byte(g), 1, byte(g+2))
	}
	b = append(b, 3, 0, 2, 1)
	b = append(b, 1, 4, 5, 6)
	b = appendString(b, "")
	b = append(b, 1)
	b = append(b, "LLUN"...)
	return append(b, 48)
}

func appendRoutines(b []byte, prefix, n int) []byte {
	b = appendCount(b, n)
	for range n {
		b = append(b, bytes.Repeat([]byte{1}, prefix)...)
		b = appendRoutine(b)
	}
	return b
}

// appendSetPieceRoutines encodes the routine block with every vector of
// length n.
func appendSetPieceRoutines(b []byte, n int) []byte {
	b = binary.LittleEndian.AppendUint16(b, routinesVersion)
	b = append(b, 0, 0, 0, 0)
	for _, p := range setPieceRoutines {
		if p.vector {
			b = appendRoutines(b, p.prefix, n)
		} else {
			b = appendRoutine(append(b, bytes.Repeat([]byte{1}, p.prefix)...))
		}
	}
	return b
}

func appendBitset(b []byte) []byte {
	b = append(b, 6, 2, 3)
	for range 3 {
		b = binary.LittleEndian.AppendUint32(b, 0x40004)
	}
	return append(b, 0)
}

// appendCount appends a u32 list count.
func appendCount(b []byte, n int) []byte {
	return binary.LittleEndian.AppendUint32(b, uint32(n)) //nolint:gosec // test sizes
}

// appendHumanTail encodes the lists after a human's team entries, each with
// n items.
func appendHumanTail(b []byte, n int) []byte {
	b = binary.LittleEndian.AppendUint16(b, uint16(n)) //nolint:gosec // test sizes
	for range n {
		b = appendTactic(b, zazTactic(), false)
	}
	b = append(b, 1)
	b = appendCount(b, n)
	for range n {
		b = binary.LittleEndian.AppendUint16(b, tacticObjectVersion)
		b = binary.LittleEndian.AppendUint32(b, 0)
		b = appendBitset(append(b, tacticHeaderTag, 2, 1))
		b = append(appendString(b, "Custom"), "LLUN"...)
	}
	b = append(b, 1)
	b = appendCount(b, n)
	for range n {
		b = binary.LittleEndian.AppendUint16(b, positionVersion)
		b = appendString(b, "Wide")
		b = appendPosition(b, SlotRole{Position: 0x800, Role: 0x8800000}, 1)
		b = appendPosition(b, SlotRole{Position: 0x800, Role: 0x4000400000}, 0)
	}
	b = appendCount(b, n)
	for range n {
		b = binary.LittleEndian.AppendUint32(b, 652)
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = appendSelection(b, zazSelection())
	}
	b = append(b, 1)
	b = appendCount(b, n)
	for range n {
		b = appendString(append(b, planTag), "")
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = append(b, planItemTag)
		b = binary.LittleEndian.AppendUint32(b, 0x200001)
		b = binary.LittleEndian.AppendUint32(append(b, 1), 5)
		b = append(appendString(b, ""), 1, 2, 3)
		b = binary.LittleEndian.AppendUint16(b, tacticObjectVersion)
		b = append(appendBitset(b), 0, 0)
		b = binary.LittleEndian.AppendUint32(append(b, 1), 0)
		b = append(appendString(b, ""), 0, 0, 0, 0, 0, 7)
	}
	b = appendCount(b, n)
	for range n {
		b = binary.LittleEndian.AppendUint32(b, 652)
		b = binary.LittleEndian.AppendUint32(b, 0xffffffff)
	}
	b = appendCount(b, n)
	for range n {
		b = append(binary.LittleEndian.AppendUint32(b, 599), 0)
	}
	return b
}

// appendFormationCodes encodes one formation from raw slot codes.
func appendFormationCodes(b []byte, mentality uint8, codes [tacticSlots]uint16) []byte {
	b = append(b, 3, mentality, 2)
	for _, c := range codes {
		b = append(b, 5)
		b = binary.LittleEndian.AppendUint16(b, c)
	}
	return b
}

// slotCode packs a cell (bits 0..5), a role index (bits 6..12) and the
// out-of-possession bit.
func slotCode(cell, role int8, oop bool) uint16 {
	c := uint16(uint8(cell))&0x3f | uint16(uint8(role))&0x7f<<6 //nolint:gosec // two's complement bit fields
	if oop {
		c |= formationOOPBit
	}
	return c
}

// formationCodes encodes one phase of slots as slot codes with the
// decoding tables reversed. It panics on a value without a code.
func formationCodes(slots [tacticSlots]TacticSlot, oop bool) [tacticSlots]uint16 {
	roles := lineupInPossessionRoles
	if oop {
		roles = lineupOutOfPossessionRoles
	}
	var out [tacticSlots]uint16
	for i, s := range slots {
		r := s.InPossession
		if oop {
			r = s.OutOfPossession
		}
		cell, ok := reverseLookup(lineupCells, r.Position)
		role, ok2 := reverseLookup(roles, r.Role)
		if !ok || !ok2 {
			panic(fmt.Sprintf("slot %d: %+v has no code", i, r))
		}
		out[i] = slotCode(cell, role, oop)
	}
	return out
}

func reverseLookup[V comparable](m map[int8]V, v V) (int8, bool) {
	for k, x := range m {
		if x == v {
			return k, true
		}
	}
	return 0, false
}

// appendLastLineup encodes l; a nil formation is written as zero bytes,
// which do not decode.
func appendLastLineup(b []byte, team int32, l LastLineup) []byte {
	b = binary.LittleEndian.AppendUint32(b, 8)
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint32(b, uint32(team)) //nolint:gosec // test values
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint32(b, uint32(l.Opponent)) //nolint:gosec // test values
	if f := l.Formation; f != nil {
		b = appendFormationCodes(b, f.Mentality, formationCodes(f.Slots, false))
		b = appendFormationCodes(b, f.Mentality, formationCodes(f.Slots, true))
	} else {
		b = append(b, make([]byte, 2*formationSize)...)
	}
	for _, p := range l.Players {
		b = binary.LittleEndian.AppendUint32(b, uint32(p)) //nolint:gosec // test values
	}
	b = append(b, l.ShirtNumbers[:]...)
	return append(b, bytes.Repeat([]byte{0xee}, 26)...)
}

// appendHuman encodes a human whose first team entry is team with the given
// selection, active index and tactics, followed by team+1 holding a copy of
// the tactic, a keyed record and routine vectors of two.
func appendHuman(b []byte, human uint32, team int32, sel [selectionSize]int32, active int32, tactics []Tactic) []byte {
	b = append(b, humanTacticsTag)
	b = binary.LittleEndian.AppendUint32(b, human)
	b = binary.LittleEndian.AppendUint32(b, 2)
	b = appendTeamEntry(b, team, sel, active, 0, 1, tactics...)
	b = appendTeamEntry(b, team+1, emptySelection(), 0, 1, 2, Tactic{Name: "copy"})
	return appendHumanTail(b, 1)
}

// appendTablesAfterHumans encodes the last-lineup table, the presets and an
// empty fourth list.
func appendTablesAfterHumans(b []byte, lineups map[int32]LastLineup, teams []int32, presets ...Tactic) []byte {
	b = binary.LittleEndian.AppendUint16(b, uint16(len(teams))) //nolint:gosec // test sizes
	for _, t := range teams {
		b = appendLastLineup(b, t, lineups[t])
	}
	b = binary.LittleEndian.AppendUint16(b, uint16(len(presets))) //nolint:gosec // test sizes
	for _, p := range presets {
		b = appendTactic(b, p, false)
	}
	return binary.LittleEndian.AppendUint16(b, 0)
}

// tacticsBody encodes a one-human tactics_man.dat body (see appendHuman),
// the last-lineup table, the presets and an empty fourth list.
func tacticsBody(human uint32, team int32, sel [selectionSize]int32, active int32, tactics []Tactic, lineups map[int32]LastLineup, teams []int32, presets ...Tactic) []byte {
	b := binary.LittleEndian.AppendUint16(nil, 1)
	b = appendHuman(b, human, team, sel, active, tactics)
	return appendTablesAfterHumans(b, lineups, teams, presets...)
}

// copyEntry is the second team entry appendHuman writes.
func copyEntry(team int32) teamEntry {
	return teamEntry{team: team + 1, selection: emptySelection(), active: 0, tactics: []Tactic{{Name: "copy"}}}
}

func emptySelection() [selectionSize]int32 {
	var s [selectionSize]int32
	for i := range s {
		s[i] = -1
	}
	return s
}

// zazTactic is FC Metz's tactic in the KŽ save of 2035-12-09.
func zazTactic() Tactic {
	ipPos := [11]uint32{1, 0x20, 0x200010, 0x100010, 0x40, 0x200400, 0x10, 0x800, 0x1000, 0x100400, 0x4000}
	oopPos := [11]uint32{1, 4, 0x200010, 0x100010, 8, 0x200080, 0x10, 0x800, 0x1000, 0x100080, 0x2000}
	ipRole := [11]uint64{0x401000, 0x4000800000, 0x40000000800000, 0x40000000800000, 0x4000800000, 0x100000000800000, 0x1200000, 0x8800000, 0x8800000, 0x100000000800000, 0x80800000}
	oopRole := [11]uint64{0x400001, 0x200200, 0x200008, 0x200008, 0x200200, 0x208000, 0x200008, 0x4000400000, 0x4000400000, 0x208000, 0x40400000}
	t := Tactic{Name: "ZaZ - Autumn 3.39 ST", Mentality: 6}
	for i := range t.Slots {
		t.Slots[i] = TacticSlot{
			InPossession:    SlotRole{Position: ipPos[i], Role: ipRole[i]},
			OutOfPossession: SlotRole{Position: oopPos[i], Role: oopRole[i]},
		}
	}
	return t
}

func zazSelection() [selectionSize]int32 {
	s := emptySelection()
	copy(s[:], []int32{183247, 40427, 55473, 253146, 32027, 182575, 216422, 77416, 163497, 172728, 147261,
		285144, 183822, 167367, 63053, 146891, 47469, 167660, 102147, 221000})
	return s
}

// zazShirtNumbers are the shirt numbers of FC Metz's last lineup in the KŽ
// save of 2035-12-09, by lineup place: Sami 88 … Ly 1, then the bench.
func zazShirtNumbers() [selectionSize]uint8 {
	return [selectionSize]uint8{88, 22, 5, 18, 32, 8, 43, 17, 16, 6, 1, 77, 70, 25, 15, 4, 3, 13, 28, 27, 3, 11, 22, 5, 41, 24}
}

// zazFormation is FC Metz's last-match formation in the KŽ saves: the
// tactic's positions and roles without their duty bits.
func zazFormation() *Formation {
	f := &Formation{Mentality: 6}
	for i, s := range zazTactic().Slots {
		s.InPossession.Role &^= dutyMask
		s.OutOfPossession.Role &^= dutyMask
		f.Slots[i] = s
	}
	return f
}

// Raw slot codes of FC Metz's last-lineup record (KŽ saves, 2035-12-02 and
// 12-09).
var (
	zazInPossessionCodes    = [tacticSlots]uint16{6961, 56, 1076, 1078, 60, 1214, 7541, 7554, 7558, 1152, 7817}
	zazOutOfPossessionCodes = [tacticSlots]uint16{14449, 15027, 14644, 14646, 15031, 15417, 14645, 8194, 8198, 15419, 15940}
)

func zazFormationBytes() []byte {
	b := appendFormationCodes(nil, 6, zazInPossessionCodes)
	return appendFormationCodes(b, 6, zazOutOfPossessionCodes)
}

func TestDecodeFormation(t *testing.T) {
	got := decodeFormation(zazFormationBytes())
	if got == nil {
		t.Fatal("Metz's formation does not decode")
	}
	if want := zazFormation(); *got != *want {
		t.Errorf("got %+v\nwant %+v", *got, *want)
	}
}

func TestDecodeFormationRoundTrip(t *testing.T) {
	// Every cell and role index of the tables decodes to its value.
	for cell, pos := range lineupCells {
		for idx, role := range lineupInPossessionRoles {
			var codes [tacticSlots]uint16
			for i := range codes {
				codes[i] = slotCode(cell, idx, false)
			}
			b := appendFormationCodes(nil, 4, codes)
			b = appendFormationCodes(b, 4, zazOutOfPossessionCodes)
			f := decodeFormation(b)
			if f == nil || f.Slots[3].InPossession != (SlotRole{Position: pos, Role: role}) {
				t.Fatalf("cell %d, role %d: %+v; want %#x, %#x", cell, idx, f, pos, role)
			}
		}
	}
	for idx, role := range lineupOutOfPossessionRoles {
		codes := zazOutOfPossessionCodes
		codes[5] = slotCode(-7, idx, true) // DM (right)
		b := appendFormationCodes(nil, 4, zazInPossessionCodes)
		f := decodeFormation(appendFormationCodes(b, 4, codes))
		if f == nil || f.Slots[5].OutOfPossession.Role != role {
			t.Fatalf("out-of-possession role %d: %+v; want %#x", idx, f, role)
		}
	}
}

func TestDecodeFormationRejectsBadBytes(t *testing.T) {
	valid := zazFormationBytes()
	patched := func(off int, v ...byte) []byte {
		b := bytes.Clone(valid)
		copy(b[off:], v)
		return b
	}
	code := func(phase, slot int, c uint16) []byte {
		return patched(phase*formationSize+3+3*slot+1, binary.LittleEndian.AppendUint16(nil, c)...)
	}
	tests := []struct {
		name string
		b    []byte
	}{
		{"in-possession tag", patched(0, 4)},
		{"out-of-possession tag", patched(formationSize, 2)},
		{"slot separator", patched(3+3*4, 6)},
		{"mentality differs between phases", patched(formationSize+1, 5)},
		{"in-possession slot with the out-of-possession bit", code(0, 1, 56|formationOOPBit)},
		{"out-of-possession slot without its bit", code(1, 1, 15027&^formationOOPBit)},
		{"bit 14 set", code(0, 1, 56|1<<14)},
		{"bit 15 set", code(1, 1, 15027|1<<15)},
		{"unknown cell -14", code(0, 0, 6961&^0x3f|0x32)},
		{"unknown cell -16 (empty mask)", code(0, 0, 6961&^0x3f|0x30)},
		{"unknown in-possession role -19", code(0, 1, 56|uint16(0x6d)<<6)},
		{"unknown out-of-possession role 8", code(1, 1, 15027&^(0x7f<<6)|8<<6)},
		{"truncated", valid[:2*formationSize-1]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if f := decodeFormation(tt.b); f != nil {
				t.Errorf("decoded %+v, want a failure", f)
			}
		})
	}
}

func TestFormationMentalityName(t *testing.T) {
	if name, ok := (Formation{Mentality: 6}).MentalityName(); !ok || name != "Attacking" {
		t.Errorf("mentality 6 = %q, %v; want Attacking", name, ok)
	}
	if name, ok := (Formation{Mentality: 8}).MentalityName(); ok {
		t.Errorf("mentality 8 = %q, want none", name)
	}
}

func TestParseTacticsDropsUndecodedFormations(t *testing.T) {
	lineups := map[int32]LastLineup{
		652: {Opponent: 1545, Players: zazSelection(), Formation: zazFormation()},
		670: {Opponent: 652, Players: zazSelection()}, // formation bytes all zero
	}
	got, err := parseTactics(tacticsBody(158530, 652, zazSelection(), 0, []Tactic{zazTactic()}, lineups, []int32{652, 670}, zazTactic()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.LastLineups, lineups) || got.BadFormations != 1 {
		t.Errorf("last lineups %+v, %d bad formations; want %+v, 1", got.LastLineups, got.BadFormations, lineups)
	}
}

func TestParseTactics(t *testing.T) {
	preset := zazTactic()
	preset.Name, preset.Mentality = "4-4-2", 4
	last := LastLineup{Opponent: 1545, Players: zazSelection(), Formation: zazFormation(), ShirtNumbers: zazShirtNumbers()}
	other := LastLineup{Opponent: 652, Players: emptySelection(), Formation: zazFormation()}
	lineups := map[int32]LastLineup{652: last, 670: other}

	tests := []struct {
		name string
		body []byte
		want Tactics
	}{
		{
			name: "selected XI and one tactic",
			body: tacticsBody(158530, 652, zazSelection(), 0, []Tactic{zazTactic()}, lineups, []int32{652, 670}, preset),
			want: Tactics{
				Human: 158530, Team: 652, Selection: zazSelection(), Active: 0, Tactics: []Tactic{zazTactic()},
				LastLineups: lineups,
			},
		},
		{
			name: "no tactic, empty selection, no last lineups",
			body: tacticsBody(88351, 547, emptySelection(), -1, nil, nil, nil, preset),
			want: Tactics{
				Human: 88351, Team: 547, Selection: emptySelection(), Active: -1, Tactics: []Tactic{},
				LastLineups: map[int32]LastLineup{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTactics(tt.body)
			if err != nil {
				t.Fatalf("parseTactics: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestParseTacticsNoHumans(t *testing.T) {
	body := binary.LittleEndian.AppendUint16(nil, 0)
	body = binary.LittleEndian.AppendUint16(body, 0) // no last lineups
	body = binary.LittleEndian.AppendUint16(body, 1)
	body = appendTactic(body, zazTactic(), false)
	body = binary.LittleEndian.AppendUint16(body, 0)

	got, err := parseTactics(body)
	if err != nil {
		t.Fatal(err)
	}
	want := Tactics{Team: -1, Selection: emptySelection(), Active: -1, LastLineups: map[int32]LastLineup{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// validTacticsBody is a one-human body with one last lineup and one preset.
func validTacticsBody() []byte {
	return tacticsBody(158530, 652, zazSelection(), 0, []Tactic{zazTactic()},
		map[int32]LastLineup{652: {Opponent: 1, Players: emptySelection(), Formation: zazFormation()}}, []int32{652}, zazTactic())
}

func TestParseTacticsRejectsBadInput(t *testing.T) {
	valid := validTacticsBody()
	if _, err := parseTactics(valid); err != nil {
		t.Fatalf("valid body: %v", err)
	}
	fourth := bytes.Clone(valid)
	fourth[len(fourth)-2] = 1

	tests := []struct {
		name string
		body []byte
		want error
	}{
		{"no last-lineup table", valid[:len(valid)-2], ErrCorrupt},
		{"non-empty fourth list", fourth, ErrCorrupt},
		{"truncated", valid[:200], ErrTruncated},
		{"empty", nil, ErrTruncated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseTactics(tt.body); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestParseTacticsFallsBackToScan breaks the walk in different places: the
// last lineups are then found by scanning. The first team entry is kept when
// it was read up to its tactics.
func TestParseTacticsFallsBackToScan(t *testing.T) {
	valid := validTacticsBody()
	want, err := parseTactics(valid)
	if err != nil {
		t.Fatalf("valid body: %v", err)
	}
	patched := func(off int, v ...byte) []byte {
		b := bytes.Clone(valid)
		copy(b[off:], v)
		return b
	}
	// Offsets in valid: 0 humans, 2 human tag, 15 team entry tag, 16
	// selection tag, 125 set pieces version.
	firstTactic := bytes.Index(valid, []byte{tacticTag, 0x42, 0, 0x1a})
	// Tactic head 8, four strings, header 3 + bitset 16 + style + "NULL".
	firstSlot := firstTactic + 8 + 4*4 + len("ZaZ - Autumn 3.39 ST") + 3 + 16 + 4 + len("Custom") + 4
	firstOptions := firstSlot + 9 + 2
	if valid[125] != setPiecesVersion || valid[firstSlot] != positionVersion || valid[firstOptions] != 2 {
		t.Fatalf("offsets do not match the encoding")
	}
	// The first and second team entries' records and routines, and the
	// human's plan.
	firstRecords := bytes.Index(valid, []byte{teamRecordsVersion, 0, teamRecords, 0, 0, 0})
	firstRoutines := bytes.Index(valid, []byte{routinesVersion, 0, 0, 0, 0, 0, 0, 1, 2})
	records := bytes.LastIndex(valid, []byte{teamRecordsVersion, 0, teamRecords, 0, 0, 0})
	routines := bytes.LastIndex(valid, []byte{routinesVersion, 0, 0, 0, 0, 0, 0, 1, 2})
	plan := bytes.Index(valid, []byte{planTag, 0, 0, 0, 0, 1, 0, 0, 0, planItemTag})
	keyed := records + 6 + teamRecords*teamRecordSize
	if firstRecords < 0 || firstRecords == records || firstRoutines == routines || plan < 0 || valid[keyed] != 1 {
		t.Fatalf("offsets do not match the encoding")
	}

	trailing := binary.LittleEndian.AppendUint16(nil, 1)
	trailing = appendHuman(trailing, 158530, 652, zazSelection(), 0, []Tactic{zazTactic()})
	trailing = appendTablesAfterHumans(append(trailing, 0xde, 0xad, 0xbe, 0xef), want.LastLineups, []int32{652}, zazTactic())

	tests := []struct {
		name      string
		body      []byte
		keepFirst bool
		part      string
	}{
		{"human tag", patched(2, 0x0e), false, walkPartHuman},
		{"team entry tag", patched(15, 8), false, walkPartTeamEntry},
		{"selection tag", patched(16, 4), false, walkPartTeamEntry},
		{"set pieces version", patched(125, 0x43), false, walkPartTeamEntry},
		{"tactic tag", patched(firstTactic, 0x23), false, walkPartTeamEntry},
		{"position version", patched(firstSlot, 0x41), false, walkPartTeamEntry},
		{"huge role option count", patched(firstOptions, 0xff, 0xff, 0xff, 0xff), false, walkPartTeamEntry},
		{"first entry's records version", patched(firstRecords, 4), true, walkPartTeamTail},
		{"first entry's routines version", patched(firstRoutines, 0x3d), true, walkPartTeamTail},
		{"team records version", patched(records, 4), true, walkPartTeamTail},
		{"team record count", patched(records+2, 25), true, walkPartTeamTail},
		{"huge keyed record count", patched(keyed, 0xff, 0xff), true, walkPartTeamTail},
		{"routines version", patched(routines, 0x3d), true, walkPartTeamTail},
		{"plan tag", patched(plan, 0x11), true, walkPartHumanTail},
		{"bytes after the human", trailing, true, walkPartTables},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTactics(tt.body)
			if err != nil {
				t.Fatalf("err = %v, want the scan to find the last lineups", err)
			}
			if got.WalkErr == nil || !errors.Is(got.WalkErr, ErrCorrupt) || got.WalkErr.Part != tt.part {
				t.Errorf("WalkErr = %v, want ErrCorrupt in %q", got.WalkErr, tt.part)
			}
			if !reflect.DeepEqual(got.LastLineups, want.LastLineups) {
				t.Errorf("last lineups %+v, want %+v", got.LastLineups, want.LastLineups)
			}
			first := teamEntry{team: -1, selection: emptySelection(), active: -1}
			if tt.keepFirst {
				first = teamEntry{want.Team, want.Selection, want.Active, want.Tactics}
			}
			if gotFirst := (teamEntry{got.Team, got.Selection, got.Active, got.Tactics}); !reflect.DeepEqual(gotFirst, first) {
				t.Errorf("first entry = %+v\nwant %+v", gotFirst, first)
			}
		})
	}
}

// TestParseTacticsScanStartsAfterTheFirstEntry: when the walk fails after
// the first team entry, the scan starts after that entry's tactics, so false
// candidates inside it (here more than the candidate cap, in a tactic name)
// are not tried.
func TestParseTacticsScanStartsAfterTheFirstEntry(t *testing.T) {
	decoys := zazTactic()
	decoys.Name = strings.Repeat("\x00\x00\x01\x00\x22\x42\x00", maxLineupCandidates+1)
	lineups := map[int32]LastLineup{652: {Opponent: 1, Players: emptySelection(), Formation: zazFormation()}}
	body := tacticsBody(158530, 652, zazSelection(), 0, []Tactic{decoys}, lineups, []int32{652}, zazTactic())
	body[bytes.LastIndex(body, []byte{routinesVersion, 0, 0, 0, 0, 0, 0, 1, 2})] = 0x3d // second entry
	if _, err := findLastLineups(body, 0); !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "candidates") {
		t.Fatalf("scan from 0: err = %v, want the candidate cap", err)
	}
	got, err := parseTactics(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.WalkErr == nil || got.Team != 652 || len(got.Tactics) != 1 || !reflect.DeepEqual(got.LastLineups, lineups) {
		t.Errorf("walk error %v, team %d, %d tactics, last lineups %+v", got.WalkErr, got.Team, len(got.Tactics), got.LastLineups)
	}
}

// twoHumansBody encodes two humans, 158 530 (team 652, one tactic) and
// 158 531 (team 900), and one last lineup.
func twoHumansBody() ([]byte, map[int32]LastLineup) {
	b := binary.LittleEndian.AppendUint16(nil, 2)
	b = appendHuman(b, 158530, 652, zazSelection(), 0, []Tactic{zazTactic()})
	b = appendHuman(b, 158531, 900, emptySelection(), -1, nil)
	lineups := map[int32]LastLineup{652: {Opponent: 900, Players: zazSelection(), Formation: zazFormation()}}
	return appendTablesAfterHumans(b, lineups, []int32{652}, zazTactic()), lineups
}

func TestTacticsWalkReadsEveryHuman(t *testing.T) {
	body, lineups := twoHumansBody()
	w := tacticsWalker{r: newReader(body)}
	table, err := w.walk()
	if err != nil {
		t.Fatal(err)
	}
	want := []humanEntry{
		{human: 158530, teams: []teamEntry{{team: 652, selection: zazSelection(), tactics: []Tactic{zazTactic()}}, copyEntry(652)}},
		{human: 158531, teams: []teamEntry{{team: 900, selection: emptySelection(), active: -1, tactics: []Tactic{}}, copyEntry(900)}},
	}
	if !reflect.DeepEqual(w.humans, want) || !reflect.DeepEqual(table.lineups, lineups) {
		t.Errorf("humans %+v, last lineups %+v\nwant %+v, %+v", w.humans, table.lineups, want, lineups)
	}
}

// TestParseTacticsTwoHumans: with more than one human the tactic is
// unsupported, but the walk still reaches the last lineups.
func TestParseTacticsTwoHumans(t *testing.T) {
	body, lineups := twoHumansBody()
	got, err := parseTactics(body)
	if err != nil {
		t.Fatal(err)
	}
	want := Tactics{Team: -1, Selection: emptySelection(), Active: -1, LastLineups: lineups}
	if !errors.Is(got.TacticErr, ErrUnsupported) || !strings.Contains(got.TacticErr.Error(), "2 humans") {
		t.Errorf("TacticErr = %v, want ErrUnsupported naming 2 humans", got.TacticErr)
	}
	got.TacticErr = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	// A second human count over a body that does not walk: the scan finds
	// the last lineups, the tactic stays unsupported.
	broken := validTacticsBody()
	broken[0] = 2
	got, err = parseTactics(broken)
	if err != nil || got.WalkErr == nil || !errors.Is(got.TacticErr, ErrUnsupported) || got.Team != -1 || len(got.LastLineups) != 1 {
		t.Errorf("broken: err %v, walk error %v, tactic error %v, team %d, %d last lineups", err, got.WalkErr, got.TacticErr, got.Team, len(got.LastLineups))
	}
}

// consumes reports whether skip reads b up to its last byte, which is extra.
func consumes(t *testing.T, b []byte, skip func(*reader)) {
	t.Helper()
	r := newReader(append(bytes.Clone(b), 0xaa))
	skip(r)
	if r.err != nil || r.remaining() != 1 {
		t.Errorf("err %v, %d bytes left; want nil, 1", r.err, r.remaining())
	}
}

// skipError returns the error skip leaves on b.
func skipError(b []byte, skip func(*reader)) error {
	r := newReader(b)
	skip(r)
	return r.err
}

func TestSkipBitset(t *testing.T) {
	consumes(t, appendBitset(nil), skipBitset)
}

func TestSkipRoutine(t *testing.T) {
	routine := appendRoutine(nil)
	consumes(t, routine, skipRoutine)
	if err := skipError(routine[:len(routine)-1], skipRoutine); !errors.Is(err, ErrTruncated) {
		t.Errorf("truncated routine: err = %v, want ErrTruncated", err)
	}
	for _, prefix := range []int{0, 1, 2} {
		consumes(t, appendRoutines(nil, prefix, 3), func(r *reader) { skipRoutines(r, prefix) })
	}
	huge := binary.LittleEndian.AppendUint32(nil, 1<<20)
	if err := skipError(append(huge, routine...), func(r *reader) { skipRoutines(r, 0) }); !errors.Is(err, ErrCorrupt) {
		t.Errorf("huge routine count: err = %v, want ErrCorrupt", err)
	}
}

func TestSkipSetPieceRoutines(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		consumes(t, appendSetPieceRoutines(nil, n), skipSetPieceRoutines)
	}
	b := appendSetPieceRoutines(nil, 1)
	if err := skipError(b[:len(b)-1], skipSetPieceRoutines); !errors.Is(err, ErrTruncated) {
		t.Errorf("truncated: err = %v, want ErrTruncated", err)
	}
	b[0] = 0x3d
	if err := skipError(b, skipSetPieceRoutines); !errors.Is(err, ErrCorrupt) {
		t.Errorf("version 0x3d: err = %v, want ErrCorrupt", err)
	}
}

func TestSkipTeamEntryTail(t *testing.T) {
	for _, keyed := range []int{0, 3} {
		consumes(t, appendTeamEntryTail(nil, keyed, 1), skipTeamEntryTail)
	}
	b := appendTeamEntryTail(nil, 0, 1)
	b[2] = 25 // record count
	if err := skipError(b, skipTeamEntryTail); !errors.Is(err, ErrCorrupt) {
		t.Errorf("25 records: err = %v, want ErrCorrupt", err)
	}
}

func TestReadTeamEntry(t *testing.T) {
	b := appendTeamEntry(nil, 652, zazSelection(), 0, 2, 2, zazTactic())
	var got teamEntry
	consumes(t, b[4:], func(r *reader) {
		got = readTeamEntryHead(r, 652)
		skipTeamEntryTail(r)
	})
	want := teamEntry{team: 652, selection: zazSelection(), tactics: []Tactic{zazTactic()}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestSkipHumanTail(t *testing.T) {
	for _, n := range []int{0, 2} {
		consumes(t, appendHumanTail(nil, n), skipHumanTail)
	}
	b := appendHumanTail(nil, 1)
	headers := 2 + len(appendTactic(nil, zazTactic(), false)) // the tactic headers' list tag
	b[headers] = 2
	if err := skipError(b, skipHumanTail); !errors.Is(err, ErrCorrupt) {
		t.Errorf("list tag 2: err = %v, want ErrCorrupt", err)
	}
}

func TestParseTacticsRejectsWrongSlotNumber(t *testing.T) {
	bad := appendTactic(nil, zazTactic(), false)
	bad[len(bad)-2] = 9 // slot 10's number, just before the flag byte
	if _, err := readTactic(newReader(bad)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestReadTacticsChecksVersion(t *testing.T) {
	body := tacticsBody(7, 1, emptySelection(), -1, nil, nil, nil, zazTactic())
	a := openSave(t, buildSave(t, testInner{path: "tactics_man.dat", data: append(innerFile("tad.", 25, ""), body...)}))
	if _, err := ReadTactics(a); !errors.Is(err, ErrUnsupported) {
		t.Errorf("version 25: err = %v, want ErrUnsupported", err)
	}
	a = openSave(t, buildSave(t, testInner{path: "tactics_man.dat", data: append(innerFile("tad.", tacticsVersion, ""), body...)}))
	got, err := ReadTactics(a)
	if err != nil || got.Human != 7 || got.Team != 1 {
		t.Errorf("version %d: got %+v, %v", tacticsVersion, got, err)
	}
}

func TestSlotRoleNames(t *testing.T) {
	tests := []struct {
		role       SlotRole
		inPos      bool
		code, side string
		name       string
	}{
		{SlotRole{Position: 1, Role: 0x401000}, true, "GK", "", "Ball-Playing Goalkeeper"},
		{SlotRole{Position: 0x200010, Role: 0x40000000800000}, true, "DC", "right", "Overlapping Centre-Back"},
		{SlotRole{Position: 0x100400, Role: 0x100000000800000}, true, "MC", "left", "Channel Midfielder"},
		{SlotRole{Position: 0x4000, Role: 0x80800000}, true, "ST", "", "Channel Forward"},
		{SlotRole{Position: 0x100080, Role: 0x208000}, false, "DM", "left", "Defensive Midfielder"},
		{SlotRole{Position: 0x2000, Role: 0x40400000}, false, "AMC", "", "Attacking Midfielder"},
		{SlotRole{Position: 0x10, Role: 0x2000008}, false, "DC", "", "Centre-Back"},              // stopper
		{SlotRole{Position: 0x10, Role: 0x4000008}, false, "DC", "", "Centre-Back"},              // cover
		{SlotRole{Position: 0x2000, Role: 0x400000200}, true, "AMC", "", "Attacking Midfielder"}, // float
	}
	for _, tt := range tests {
		if code, ok := tt.role.PositionCode(); !ok || code != tt.code {
			t.Errorf("%#x: PositionCode = %q, %v; want %q", tt.role.Position, code, ok, tt.code)
		}
		if side := tt.role.Side(); side != tt.side {
			t.Errorf("%#x: Side = %q, want %q", tt.role.Position, side, tt.side)
		}
		name, ok := tt.role.OutOfPossessionRoleName()
		if tt.inPos {
			name, ok = tt.role.InPossessionRoleName()
		}
		if !ok || name != tt.name {
			t.Errorf("%#x: role name = %q, %v; want %q", tt.role.Role, name, ok, tt.name)
		}
	}
}

func TestSlotRoleNamesRejectUnknown(t *testing.T) {
	for _, pos := range []uint32{
		0,                // no position
		0x11,             // two positions
		0x8000,           // unmapped position bit
		0x10 | 0x1000000, // unknown flag
	} {
		if code, ok := (SlotRole{Position: pos}).PositionCode(); ok {
			t.Errorf("position %#x: code %q, want a failure", pos, code)
		}
	}
	if side := (SlotRole{Position: 0x10 | 0x300000}).Side(); side != "" {
		t.Errorf("both sides: side %q, want none", side)
	}
	for _, role := range []uint64{
		0x200000,             // a duty without a role
		0x3 | 0x200000,       // two roles
		0x2 | 0x4 | 0x200000, // two roles
		1<<63 | 0x200000,     // unmapped role
	} {
		r := SlotRole{Position: 0x10, Role: role}
		if name, ok := r.InPossessionRoleName(); ok {
			t.Errorf("role %#x: in-possession name %q, want a failure", role, name)
		}
		if name, ok := r.OutOfPossessionRoleName(); ok {
			t.Errorf("role %#x: out-of-possession name %q, want a failure", role, name)
		}
	}
}

func TestPositionMasksFollowPositionNames(t *testing.T) {
	for i, mask := range PositionMasks {
		if code, ok := (SlotRole{Position: mask}).PositionCode(); !ok || code != PositionNames[i] {
			t.Errorf("PositionMasks[%d] = %#x is %q, %v; want %s", i, mask, code, ok, PositionNames[i])
		}
	}
}

func TestRoleBitDropsTheDuty(t *testing.T) {
	for role, want := range map[uint64]uint64{
		0x401000:         0x1000,           // Ball-Playing Goalkeeper, support
		0x40000000800000: 0x40000000000000, // Overlapping Centre-Back, attack
		0x400000200:      0x200,            // Attacking Midfielder, float
		0x2000008:        0x8,              // Centre-Back, stopper
		0x8000000:        0x8000000,        // no duty
	} {
		if got := (SlotRole{Role: role}).RoleBit(); got != want {
			t.Errorf("%#x: RoleBit = %#x, want %#x", role, got, want)
		}
	}
}

func TestInPossessionRoleName(t *testing.T) {
	if name, ok := InPossessionRoleName(0x40000000000000); !ok || name != "Overlapping Centre-Back" {
		t.Errorf("0x40000000000000: %q, %v", name, ok)
	}
	if name, ok := InPossessionRoleName(0x8800000); !ok || name != "Inside Forward" {
		t.Errorf("with a duty: %q, %v", name, ok)
	}
	if name, ok := InPossessionRoleName(1 << 63); ok {
		t.Errorf("unmapped bit: %q", name)
	}
}

func TestMentalityNames(t *testing.T) {
	want := []string{"", "Very Defensive", "Defensive", "Cautious", "Balanced", "Positive", "Attacking", "Very Attacking"}
	for v := range 256 {
		name, ok := Tactic{Mentality: uint8(v)}.MentalityName() //nolint:gosec // 0..255
		if v >= 1 && v < len(want) {
			if !ok || name != want[v] {
				t.Errorf("mentality %d = %q, %v; want %q", v, name, ok, want[v])
			}
		} else if ok {
			t.Errorf("mentality %d = %q, want none", v, name)
		}
	}
}

func TestParseTacticsHumanWithoutTeams(t *testing.T) {
	body := func(teams uint32) []byte {
		b := binary.LittleEndian.AppendUint16(nil, 1)
		b = append(b, humanTacticsTag)
		b = binary.LittleEndian.AppendUint32(b, 88351)
		b = binary.LittleEndian.AppendUint32(b, teams)
		b = appendHumanTail(b, 0)
		b = binary.LittleEndian.AppendUint16(b, 0) // no last lineups
		b = binary.LittleEndian.AppendUint16(b, 1)
		b = appendTactic(b, zazTactic(), false)
		return binary.LittleEndian.AppendUint16(b, 0)
	}
	got, err := parseTactics(body(0))
	if err != nil {
		t.Fatal(err)
	}
	want := Tactics{Human: 88351, Team: -1, Selection: emptySelection(), Active: -1, LastLineups: map[int32]LastLineup{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got, err := parseTactics(body(maxTacticTeams + 1)); err != nil || got.WalkErr == nil || !errors.Is(got.WalkErr, ErrCorrupt) {
		t.Errorf("%d teams: err = %v, walk error %v; want nil, ErrCorrupt", maxTacticTeams+1, err, got.WalkErr)
	}
}

// ladderBody holds a u16 n for every n in 1..rungs, each n × 242 bytes plus
// two before one preset anchor, so every n is a candidate last-lineup table
// whose records are invalid.
func ladderBody(rungs int) []byte {
	anchor := 2 + 4 + lastLineupSize*rungs
	b := make([]byte, anchor, anchor+64)
	for n := 1; n <= rungs; n++ {
		binary.LittleEndian.PutUint16(b[anchor-4-lastLineupSize*n:], uint16(n)) //nolint:gosec // n ≤ 65535
	}
	binary.LittleEndian.PutUint16(b[anchor-2:], 1) // one preset that does not parse
	b = append(b, presetAnchor...)
	return append(b, bytes.Repeat([]byte{0xee}, 61)...)
}

func TestFindLastLineupsLadderIsBounded(t *testing.T) {
	body := ladderBody(8000)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := parseTactics(body)
	runtime.ReadMemStats(&after)

	if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "candidates") {
		t.Errorf("err = %v, want the candidate cap", err)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 16<<20 {
		t.Errorf("allocated %d MB, want under 16", alloc>>20)
	}
}

// presetChainBody holds k tactics whose last bytes read as an empty
// last-lineup table and a preset count of 65535, then bytes that are no
// tactic: every tactic is a candidate whose presets run to the end.
func presetChainBody(k int) []byte {
	b := binary.LittleEndian.AppendUint16(nil, 0) // no humans
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint16(b, 0xffff)
	for range k {
		tac := appendTactic(nil, zazTactic(), true)
		copy(tac[len(tac)-4:], []byte{0, 0, 0xff, 0xff})
		b = append(b, tac...)
	}
	return append(b, 0xee, 0xee, 0xee)
}

func TestFindLastLineupsParsesEachPresetOnce(t *testing.T) {
	const k = 50
	body := presetChainBody(k)
	allocs := testing.AllocsPerRun(1, func() {
		if _, err := parseTactics(body); !errors.Is(err, ErrCorrupt) {
			t.Errorf("err = %v, want ErrCorrupt", err)
		}
	})
	// About two strings per parsed tactic, plus the cache.
	if allocs > 10*k {
		t.Errorf("%v allocations for %d tactics; each tactic should be parsed once", allocs, k)
	}
}

func TestFindLastLineupsCapsCandidates(t *testing.T) {
	_, err := parseTactics(presetChainBody(maxLineupCandidates + 1))
	if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "candidates") {
		t.Errorf("err = %v, want the candidate cap", err)
	}
}

func TestReadTacticsRejectsHugeFile(t *testing.T) {
	a := openSave(t, buildSave(t, testInner{
		path: "tactics_man.dat",
		data: innerFile("tad.", tacticsVersion, ""),
		size: maxTacticsSize + FileHeaderSize + 1,
	}))
	if _, err := ReadTactics(a); !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("err = %v, want a size error", err)
	}
}
