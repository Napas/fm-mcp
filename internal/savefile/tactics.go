package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Layout of tactics_man.dat, version 26 (docs/format.md).
const (
	tacticsVersion      = 26
	humanTacticsTag     = 0x0d
	teamEntryTag        = 9
	selectionTag        = 3
	selectionSize       = 26 // the XI by tactic slot, then the bench
	setPiecesVersion    = 0x42
	personRefTag        = 2 // before each person in the set-piece lists
	tacticTag           = 0x22
	tacticObjectVersion = 0x42
	tacticHeaderTag     = 4
	positionVersion     = 0x42
	tacticSlots         = 11
	tacticBlockSize     = 191 // u32, 11 × (u64 + 5 u8), u32 + 10 u8, 30 u8
	lastLineupSize      = 242
	lastLineupVersion   = 8
	lastLineupFormation = 14  // offset of the in-possession formation, then the out-of-possession one
	lastLineupPlayers   = 86  // offset of the 26 person indices in a record
	lastLineupShirts    = 190 // offset of the 26 shirt numbers

	formationSize    = 36 // u8 tag, u8 mentality, u8, 11 × (u8 separator, u16 slot code)
	formationTag     = 3
	formationSlotTag = 5
	// formationPhaseShift: slot-code bit 13 is 1 out of possession; bits
	// 14..15 are 0.
	formationPhaseShift = 13
	formationOOPBit     = 1 << formationPhaseShift

	minPositionSize = 27 // a position with no role options
	minTacticSize   = tacticSlots * (2*minPositionSize + 1)
	maxTacticList   = 1024 // sanity cap on every list; samples hold at most 22
	maxTacticTeams  = 64   // team entries per human; samples hold at most 4

	// Team entry (writer 0x47a1cc4) and human (0x16ef560) tails.
	teamRecordsVersion = 3
	teamRecords        = 26 // records after the tactics (0x473fa94)
	teamRecordSize     = 22 // 4 × i32, 6 × u8 (0x473f170)
	routinesVersion    = 0x3c
	routineGroups      = 8 // player categories in a set-piece routine
	listTag            = 1 // u8 1 before the human's tactic headers, named positions and plans, and in each plan
	planTag            = 0x10
	planItemTag        = 2
	minSelectionSize   = 1 + 4 + 4*selectionSize
	minRoutineSize     = 2*routineGroups + 1 + 1 + 4 + 6
	minHeaderItemSize  = 2 + 4 + 3 + 4 + 4 + 4 // bitset with no words
	minPlanSize        = 1 + 4 + 4 + 1 + 4 + 4 + 6
	minPlanItemSize    = 1 + 4 + 1 + 4 + 4 + 3 + 2 + 4 + 2
	minHumanSize       = 1 + 4 + 4 + 2 + 5 + 5 + 4 + 5 + 4 + 4 // header and empty lists
	// maxLineupCandidates caps the places tried as the last-lineup table:
	// places followed, past their records, by a tactic's start. Real saves
	// have at most 26 in the whole body.
	maxLineupCandidates = 64
	// maxTacticsSize caps the tactics_man.dat body; real saves hold 3.7 MB.
	maxTacticsSize = 32 << 20
)

// presetAnchor starts every tactic: tag and object version.
var presetAnchor = []byte{tacticTag, tacticObjectVersion, 0}

// Tactics is the human manager's first team entry and every team's
// last-match lineup.
type Tactics struct {
	Human uint32 // person index of the human manager
	// Team is the TEAM index of the human's first team entry (sorted by team
	// index; his club's first team in every save seen). -1 when none.
	Team int32
	// Selection holds person indices: 0..10 the XI by tactic slot, 11.. the
	// bench; -1 = empty. All -1 right after a match.
	Selection [selectionSize]int32
	Active    int32 // hypothesis: index of the active tactic; -1 with no tactic
	Tactics   []Tactic
	// TacticErr is why Team, Selection, Active and Tactics are empty: the
	// save has more than one human (ErrUnsupported). nil otherwise.
	TacticErr error
	// LastLineups maps a TEAM index to the team's lineup in its last match.
	LastLineups map[int32]LastLineup
	// BadFormations counts last lineups whose Formation is nil.
	BadFormations int
	// WalkErr is where the file stopped parsing structurally; the last
	// lineups were then found by scanning. nil when it parsed.
	WalkErr *WalkError
}

// WalkError is where the structural walk of tactics_man.dat stopped.
type WalkError struct {
	Part string // the part being read, in plain words
	Err  error
}

func (e *WalkError) Error() string { return "in " + e.Part + ": " + e.Err.Error() }

func (e *WalkError) Unwrap() error { return e.Err }

// Parts of tactics_man.dat named by WalkError.
const (
	walkPartHuman     = "a manager's header"
	walkPartTeamEntry = "a team's selection or tactics"
	walkPartTeamTail  = "a team's set pieces or records"
	walkPartHumanTail = "a manager's tactic lists"
	walkPartTables    = "the last-lineup table"
)

// Tactic is one saved tactic.
type Tactic struct {
	Name string
	// Mentality is the game's Mentality enum (4 Balanced, 6 Attacking; 6 verified in game).
	Mentality uint8
	Slots     [tacticSlots]TacticSlot
}

// TacticSlot is one of a tactic's 11 positions, in and out of possession.
type TacticSlot struct {
	InPossession, OutOfPossession SlotRole
}

// SlotRole is a slot's position and role in one phase of play.
type SlotRole struct {
	Position uint32 // position mask: one position bit, plus a side bit when doubled
	Role     uint64 // one role bit plus one duty bit
}

// LastLineup is a team's lineup in its last match.
type LastLineup struct {
	Opponent int32 // TEAM index of the last match's opponent; -1 = none
	// Players holds person indices like Tactics.Selection.
	Players [selectionSize]int32
	// Formation is the shape and roles in that match; nil when undecodable.
	Formation *Formation
	// ShirtNumbers are the shirt numbers worn, by place in Players; 255 =
	// none. Places without a person may still hold a number.
	ShirtNumbers [selectionSize]uint8
}

// Formation is a last-match formation. Slot i is the slot of Players[i].
// Roles carry no duty bit.
type Formation struct {
	// Mentality is the game's Mentality enum.
	Mentality uint8
	Slots     [tacticSlots]TacticSlot
}

// ReadTactics decodes the save's tactics_man.dat.
func ReadTactics(a *Archive) (Tactics, error) {
	if e, ok := a.index["tactics_man.dat"]; ok && e.Size > maxTacticsSize+FileHeaderSize {
		return Tactics{}, fmt.Errorf("%w: tactics_man.dat is %d bytes", ErrUnsupported, e.Size)
	}
	f, err := a.ReadFile("tactics_man.dat")
	if err != nil {
		return Tactics{}, err
	}
	if f.Header.Version != tacticsVersion {
		return Tactics{}, fmt.Errorf("%w: tactics_man.dat version %d", ErrUnsupported, f.Header.Version)
	}
	t, err := parseTactics(f.Body)
	if err != nil {
		return Tactics{}, fmt.Errorf("tactics_man.dat: %w", err)
	}
	return t, nil
}

// parseTactics walks the body. When the walk fails, it scans for the
// last-lineup table and keeps the first team entry if it was read up to its
// tactics. With more than one human, only the last lineups are kept.
func parseTactics(body []byte) (Tactics, error) {
	t := Tactics{Team: -1, Active: -1}
	for i := range t.Selection {
		t.Selection[i] = -1
	}
	w := tacticsWalker{r: newReader(body)}
	table, err := w.walk()
	switch {
	case w.humanCount > 1:
		t.TacticErr = fmt.Errorf("%w: %d humans in tactics_man.dat", ErrUnsupported, w.humanCount)
	case len(w.humans) > 0:
		t.Human = w.humans[0].human
		if teams := w.humans[0].teams; len(teams) > 0 {
			t.Team, t.Selection, t.Active, t.Tactics = teams[0].team, teams[0].selection, teams[0].active, teams[0].tactics
		}
	}
	if err != nil {
		walkErr := &WalkError{Part: w.part, Err: err}
		var scanErr error
		if table, scanErr = findLastLineups(body, w.firstEntryEnd); scanErr != nil {
			return Tactics{}, fmt.Errorf("%w; scan: %w", walkErr, scanErr)
		}
		t.WalkErr = walkErr
	}
	t.LastLineups = table.lineups
	for _, l := range table.lineups {
		if l.Formation == nil {
			t.BadFormations++
		}
	}
	return t, nil
}

// lineupTable is the last-lineup table and the preset count after it.
type lineupTable struct {
	at      int // body offset of the table's u16 count
	presets int
	lineups map[int32]LastLineup
}

// tacticsWalker reads tactics_man.dat structurally.
type tacticsWalker struct {
	r          *reader
	part       string // the part being read
	humanCount int
	humans     []humanEntry // each team entry is added once read up to its tactics
	// firstEntryEnd is the offset after the first team entry's tactics; 0
	// until read. A fallback scan starts there.
	firstEntryEnd int
}

// humanEntry is one human's team entries, sorted by team index.
type humanEntry struct {
	human uint32 // person index
	teams []teamEntry
}

// teamEntry is a team entry up to its tactics.
type teamEntry struct {
	team      int32 // TEAM index
	selection [selectionSize]int32
	active    int32
	tactics   []Tactic
}

// walk reads the humans, then the last-lineup table, presets and empty
// fourth list. On failure w.part names where it stopped.
func (w *tacticsWalker) walk() (lineupTable, error) {
	r := w.r
	w.part = walkPartHuman
	w.humanCount = int(r.u16())
	for range listCount(r, uint32(w.humanCount), minHumanSize, "humans") { //nolint:gosec // a u16
		if w.readHuman(); r.err != nil {
			return lineupTable{}, r.err
		}
	}
	if r.err != nil {
		return lineupTable{}, r.err
	}
	w.part = walkPartTables
	if table, ok := newPresetChain(r.b).tableAt(r.off); ok {
		return table, nil
	}
	return lineupTable{}, fmt.Errorf("%w: no last-lineup table, presets and empty fourth list at offset %d", ErrCorrupt, r.off)
}

// readHuman reads one HUMAN_TACTICS; its tail is skipped.
func (w *tacticsWalker) readHuman() {
	r := w.r
	w.part = walkPartHuman
	r.expectU8(humanTacticsTag, "human tactics tag")
	human := r.u32()
	teams := r.u32()
	if r.err != nil {
		return
	}
	if teams > maxTacticTeams {
		r.fail("%d team entries", teams)
		return
	}
	w.humans = append(w.humans, humanEntry{human: human})
	h := &w.humans[len(w.humans)-1]
	for range teams {
		w.part = walkPartTeamEntry
		e := readTeamEntryHead(r, r.i32())
		if r.err != nil {
			return
		}
		h.teams = append(h.teams, e)
		if w.firstEntryEnd == 0 {
			w.firstEntryEnd = r.off
		}
		w.part = walkPartTeamTail
		if skipTeamEntryTail(r); r.err != nil {
			return
		}
	}
	w.part = walkPartHumanTail
	skipHumanTail(r)
}

// readTeamEntryHead reads a team entry up to its tactics.
func readTeamEntryHead(r *reader, team int32) teamEntry {
	e := teamEntry{team: team}
	r.expectU8(teamEntryTag, "team entry tag")
	e.selection = readSelection(r)
	skipSetPieces(r)
	e.active = r.i32()
	n := listCount(r, uint32(r.u16()), minTacticSize, "tactics")
	e.tactics = make([]Tactic, 0, n)
	for range n {
		tac, err := readTactic(r)
		if err != nil {
			break
		}
		e.tactics = append(e.tactics, tac)
	}
	return e
}

// readSelection reads u8 3, an empty string and the 26 selected persons.
func readSelection(r *reader) [selectionSize]int32 {
	var s [selectionSize]int32
	r.expectU8(selectionTag, "selection tag")
	r.str() // empty in every sample
	for i := range s {
		s[i] = r.i32()
	}
	return s
}

// skipTeamEntryTail skips what follows a team entry's tactics.
func skipTeamEntryTail(r *reader) {
	r.expectU16(teamRecordsVersion, "team records version")
	if n := r.u32(); n != teamRecords {
		r.fail("%d team records", n)
	}
	r.take(teamRecords * teamRecordSize)
	r.take((4 + teamRecordSize) * listCount(r, uint32(r.u16()), 4+teamRecordSize, "keyed records"))
	r.i32() // 0 in every sample
	skipSetPieceRoutines(r)
}

// setPieceRoutines is the write order of SET_PIECE_ROUTINES (writer
// 0x4748a6c) with the bytes before each routine. C0–C4 hold 10 players
// (defending, inference), A0–A4 9 (attacking); the addresses are the
// routine writers.
var setPieceRoutines = [...]struct {
	vector bool
	prefix int
}{
	{false, 0}, // C0 0x475ad04
	{false, 0}, // C1 0x475d484
	{false, 0}, // C2 0x4764410
	{true, 0},  // vector of C0
	{true, 0},  // vector of C1
	{true, 0},  // vector of C2
	{false, 1}, // C3 0x475f8a8
	{false, 1}, // C4 0x4761e5c
	{true, 1},  // vector of C3 (0x4772d20)
	{true, 1},  // vector of C4 (0x4773194)
	{false, 1}, // A1 0x476931c
	{false, 0}, // A2 0x4766ef8
	{false, 1}, // A4 0x476e0c0
	{true, 1},  // vector of A1 (0x4773dfc)
	{true, 0},  // vector of A2 (0x477421c)
	{true, 1},  // vector of A4 (0x4774b30)
	{false, 2}, // A0 0x4770674
	{false, 2}, // A3 0x476b8d0
	{true, 2},  // vector of A0 (0x4773950)
	{true, 2},  // vector of A3 (0x4774684)
}

// skipSetPieceRoutines skips the set-piece routines.
func skipSetPieceRoutines(r *reader) {
	r.expectU16(routinesVersion, "set-piece routines version")
	r.take(4)
	for _, p := range setPieceRoutines {
		if p.vector {
			skipRoutines(r, p.prefix)
			continue
		}
		r.take(p.prefix)
		skipRoutine(r)
	}
}

// skipRoutines skips u32 n + n × (prefix bytes, routine).
func skipRoutines(r *reader, prefix int) {
	for range listCount(r, r.u32(), prefix+minRoutineSize, "routines") {
		r.take(prefix)
		skipRoutine(r)
	}
}

// skipRoutine skips 8 × (u8 category, u8 n, n bytes), u8 n + n bytes,
// u8 n + n × 3 bytes, a string, u8, u32, u8.
func skipRoutine(r *reader) {
	for range routineGroups {
		r.u8()
		r.take(int(r.u8()))
	}
	r.take(int(r.u8()))
	r.take(3 * int(r.u8()))
	r.str()
	r.take(1 + 4 + 1)
}

// skipHumanTail skips a human's lists after the team entries.
func skipHumanTail(r *reader) {
	for range listCount(r, uint32(r.u16()), minTacticSize, "human tactics") {
		if _, err := readTactic(r); err != nil {
			return
		}
	}
	r.expectU8(listTag, "tactic headers tag")
	for range listCount(r, r.u32(), minHeaderItemSize, "tactic headers") {
		r.expectU16(tacticObjectVersion, "tactic header version")
		r.u32() // key
		readTacticHeader(r)
	}
	r.expectU8(listTag, "named positions tag")
	for range listCount(r, r.u32(), 2+4+2*minPositionSize, "named positions") {
		r.expectU16(positionVersion, "named position version")
		r.str()
		readSlotRole(r)
		readSlotRole(r)
	}
	for range listCount(r, r.u32(), 8, "selection keys") {
		r.u32()
		for range listCount(r, r.u32(), minSelectionSize, "selections") {
			readSelection(r)
		}
	}
	r.expectU8(listTag, "plans tag")
	for range listCount(r, r.u32(), minPlanSize, "plans") {
		skipPlan(r)
	}
	r.take(8 * listCount(r, r.u32(), 8, "team values")) // u32 team, u32
	r.take(5 * listCount(r, r.u32(), 5, "team flags"))  // u32 team, u8
}

// skipPlan skips one plan (writer 0x483c2cc).
func skipPlan(r *reader) {
	r.expectU8(planTag, "plan tag")
	r.str()
	for range listCount(r, r.u32(), minPlanItemSize, "plan items") {
		r.expectU8(planItemTag, "plan item tag")
		r.u32()
		r.expectU8(listTag, "plan item value tag")
		r.i32()
		r.str()
		r.take(3)
		r.expectU16(tacticObjectVersion, "plan item version")
		skipBitset(r)
		r.take(2)
	}
	r.expectU8(listTag, "plan end tag")
	r.i32()
	r.str()
	r.take(6)
}

// listCount returns n if it is within maxTacticList and fits the remaining
// bytes at minSize per item; else it fails the reader and returns 0.
func listCount(r *reader, n uint32, minSize int, what string) int {
	if r.err != nil {
		return 0
	}
	if n > maxTacticList || int(n) > r.remaining()/minSize {
		r.fail("%d %s", n, what)
		return 0
	}
	return int(n)
}

// skipSetPieces skips the set-piece takers (not decoded).
func skipSetPieces(r *reader) {
	r.expectU16(setPiecesVersion, "set pieces version")
	skipPersonLists(r, 2)
	r.take(1 + 4 + 3) // u8, i32 person, 3 × u8
	skipPersonLists(r, 3+7)
	r.take(1)
	skipPersonLists(r, 8)
	r.take(2 * int(r.u8())) // (u8, u8) pairs
}

// skipPersonLists skips lists of u32 n + n × (u8 2, i32 person index).
func skipPersonLists(r *reader, lists int) {
	for range lists {
		n := listCount(r, r.u32(), 5, "set-piece persons")
		for range n {
			r.expectU8(personRefTag, "person ref tag")
			r.u32()
		}
	}
}

// readTactic reads one tactic, including the optional maps after its slots.
func readTactic(r *reader) (Tactic, error) {
	var t Tactic
	r.expectU8(tacticTag, "tactic tag")
	r.expectU16(tacticObjectVersion, "tactic version")
	r.take(5) // build stamp, u8 kind (2 human, 1 preset)
	t.Name = r.str()
	for range 3 {
		r.str() // empty in every sample
	}
	t.Mentality = readTacticHeader(r)
	for i := range t.Slots {
		t.Slots[i].InPossession = readSlotRole(r)
		t.Slots[i].OutOfPossession = readSlotRole(r)
		if n := r.u8(); r.err == nil && int(n) != i {
			r.fail("slot %d numbered %d", i, n)
		}
	}
	if r.u8() != 0 {
		skipPositionMap(r)
		skipPositionMap(r)
		n := listCount(r, r.u32(), 8, "map keys")
		for range n {
			r.u32()
			r.take(6 * listCount(r, r.u32(), 6, "map items")) // u8, u8, u32
		}
		r.take(tacticBlockSize) // all zero in every sample
	}
	return t, r.err
}

// readTacticHeader reads u8 4, u8, the mentality, a bitset, the style
// ("Custom" in the human's tactic, empty in presets) and u32 "NULL"
// (0x47ac118). It returns the mentality.
func readTacticHeader(r *reader) uint8 {
	r.expectU8(tacticHeaderTag, "tactic header tag")
	r.u8()
	mentality := r.u8()
	skipBitset(r)
	r.str()
	r.u32()
	return mentality
}

// readSlotRole reads a POSITION: the mask, the role options with their player
// instructions (skipped), and the selected role.
func readSlotRole(r *reader) SlotRole {
	var p SlotRole
	r.expectU16(positionVersion, "position version")
	r.u8()
	p.Position = r.u32()
	r.take(2 + 2)
	n := listCount(r, r.u32(), 16, "role options")
	for range n {
		r.u64() // role
		skipBitset(r)
		r.u32()
	}
	r.take(12 * listCount(r, r.u32(), 12, "option pairs")) // u32, u64
	p.Role = r.u64()
	return p
}

// skipBitset skips u8, u8, u8 n, n × u32, u8.
func skipBitset(r *reader) {
	r.take(2)
	r.take(4 * int(r.u8()))
	r.take(1)
}

// skipPositionMap skips u32 n + n × (u32 key, u32 m + m × POSITION).
func skipPositionMap(r *reader) {
	n := listCount(r, r.u32(), 8, "map keys")
	for range n {
		r.u32()
		for range listCount(r, r.u32(), minPositionSize, "map positions") {
			readSlotRole(r)
		}
	}
}

// findLastLineups scans for the last-lineup table at or after from: a u16
// count of 242-byte records sorted by team, the u16-counted presets, then
// u16 0 at the end of the file.
func findLastLineups(b []byte, from int) (lineupTable, error) {
	presets := newPresetChain(b)
	tries := 0
	for p := from; p+2 <= len(b); p++ {
		n := int(binary.LittleEndian.Uint16(b[p:]))
		end := p + 2 + n*lastLineupSize
		if end+2+len(presetAnchor) > len(b) || !bytes.Equal(b[end+2:end+2+len(presetAnchor)], presetAnchor) {
			continue
		}
		if tries++; tries > maxLineupCandidates {
			return lineupTable{}, fmt.Errorf("%w: over %d last-lineup table candidates", ErrCorrupt, maxLineupCandidates)
		}
		if table, ok := presets.tableAt(p); ok {
			return table, nil
		}
	}
	return lineupTable{}, fmt.Errorf("%w: last-lineup table not found", ErrCorrupt)
}

// validLastLineups reports whether b holds n records with the right version
// and zero bytes, sorted by team.
func validLastLineups(b []byte, n int) bool {
	prev := int32(-1)
	for i := range n {
		r := newReader(b[i*lastLineupSize : (i+1)*lastLineupSize])
		version, z1, team, z2 := r.u32(), r.u8(), r.i32(), r.u8()
		if version != lastLineupVersion || z1 != 0 || z2 != 0 || team <= prev {
			return false
		}
		prev = team
	}
	return true
}

// readLastLineups decodes n records that validLastLineups accepted.
func readLastLineups(b []byte, n int) map[int32]LastLineup {
	out := make(map[int32]LastLineup, n)
	for i := range n {
		rec := b[i*lastLineupSize : (i+1)*lastLineupSize]
		r := newReader(rec)
		r.take(5) // version, u8 0
		team := r.i32()
		r.u8()
		l := LastLineup{Opponent: r.i32(), Formation: decodeFormation(rec[lastLineupFormation:lastLineupPlayers])}
		r.off = lastLineupPlayers
		for j := range l.Players {
			l.Players[j] = r.i32()
		}
		copy(l.ShirtNumbers[:], rec[lastLineupShirts:])
		out[team] = l
	}
	return out
}

// decodeFormation decodes both formations of a last-lineup record (writer
// 0x490f744); nil unless both decode with the same mentality.
func decodeFormation(b []byte) *Formation {
	if len(b) < 2*formationSize {
		return nil
	}
	f := &Formation{Mentality: b[1]}
	for phase, roles := range []map[int8]uint64{lineupInPossessionRoles, lineupOutOfPossessionRoles} {
		p := b[phase*formationSize : (phase+1)*formationSize]
		if p[0] != formationTag || p[1] != f.Mentality {
			return nil
		}
		for i := range f.Slots {
			c := p[3+3*i:]
			code := binary.LittleEndian.Uint16(c[1:])
			if c[0] != formationSlotTag || code>>formationPhaseShift != uint16(phase) {
				return nil
			}
			pos, ok := lineupCells[int8(code<<2)>>2] //nolint:gosec // signed bits 0..5
			role, ok2 := roles[int8(code>>5)>>1]     //nolint:gosec // signed bits 6..12
			if !ok || !ok2 {
				return nil
			}
			s := SlotRole{Position: pos, Role: role}
			if phase == 0 {
				f.Slots[i].InPossession = s
			} else {
				f.Slots[i].OutOfPossession = s
			}
		}
	}
	return f
}

// presetChain reads the last-lineup table and the preset list after it. It
// caches where each tactic ends, so candidates whose lists overlap parse each
// tactic once.
type presetChain struct {
	b   []byte
	end map[int]int // tactic start → its end; -1 when no tactic parses there
}

func newPresetChain(b []byte) *presetChain {
	return &presetChain{b: b, end: map[int]int{}}
}

// tableAt reads b[at:] as u16 n + n last-lineup records sorted by team, the
// presets and u16 0 at the end of b.
func (c *presetChain) tableAt(at int) (lineupTable, bool) {
	if at+2 > len(c.b) {
		return lineupTable{}, false
	}
	n := int(binary.LittleEndian.Uint16(c.b[at:]))
	end := at + 2 + n*lastLineupSize
	if end > len(c.b) || !validLastLineups(c.b[at+2:end], n) {
		return lineupTable{}, false
	}
	count, ok := c.read(end)
	if !ok {
		return lineupTable{}, false
	}
	return lineupTable{at: at, presets: count, lineups: readLastLineups(c.b[at+2:end], n)}, true
}

// read reports whether b[at:] is u16 n, n tactics and u16 0, and returns n.
func (c *presetChain) read(at int) (int, bool) {
	if at+2 > len(c.b) {
		return 0, false
	}
	n := int(binary.LittleEndian.Uint16(c.b[at:]))
	p := at + 2
	for range n {
		if p = c.tacticEnd(p); p < 0 {
			return 0, false
		}
	}
	if p+2 != len(c.b) || binary.LittleEndian.Uint16(c.b[p:]) != 0 {
		return 0, false
	}
	return n, true
}

func (c *presetChain) tacticEnd(p int) int {
	if e, ok := c.end[p]; ok {
		return e
	}
	r := newReader(c.b[p:])
	e := -1
	if _, err := readTactic(r); err == nil {
		e = p + r.off
	}
	c.end[p] = e
	return e
}
