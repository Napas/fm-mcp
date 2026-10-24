package savefile

import (
	"bytes"
	"encoding/binary"
	"math"
)

// A person's contracts sit in the undecoded ACTUAL_PERSON part
// (0x2f424a0) as a block:
//
//	date, date, u8, u8 n + n × u32, u32   (fields the marker is validated by)
//	u8 5                                  (marker)
//	5 × (u8 type + body)                  main, loan, B club, future, B club 2
//
// Types: 0 FULL, 1 LOAN, 2 TRIAL, 3 B_CLUB, 4 YOUTH_TRIAL, 5 BASIC, 0xff
// empty. Every type but BASIC ends with a CONTRACT core (0x3269674) and a
// BASIC_CONTRACT tail (0x302c3a0):
//
//	core  i32 weekly loyalty bonus, i32, i32, date end, date start,
//	      u8 playing time, u8 (undecoded), u8 original playing time, 3 × u8,
//	      u32 n + n × 29-byte items, u8, date signed,
//	      u8, i8 playing-time score, i8 recent score, u8 squad number, 3 × u8,
//	      i64, date, u8
//	tail  u32 person index, i64 team index, i32 weekly wage, u8, u16
//
// The bytes between a slot's type and its core are only decoded for FULL
// contracts, so cores are found first (by their tail carrying the record's
// own person index) and slots are matched to them.
const (
	contractMarker = 5
	contractSlots  = 5
	slotMain       = 0
	slotLoan       = 1
	slotFuture     = 3

	typeFull  = 0
	typeLoan  = 1
	typeBasic = 5
	typeEmpty = 0xff

	coreFixedSize              = 55
	coreItemSize               = 29
	maxCoreItems               = 10
	coreTailSize               = 19
	coreLoyaltyOff             = 0 // CONTRACT +0x38
	coreEndOff                 = 12
	coreStartOff               = 16
	corePlayingTimeOff         = 20 // CONTRACT +0x54
	coreFlagsOff               = 23 // CONTRACT +0x57
	coreOriginalPlayingTimeOff = 22 // CONTRACT +0x56
	coreItemsOff               = 26
	coreSignedOff              = 31 // plus 29 × items
	coreScoreOff               = 36 // plus 29 × items; CONTRACT +0x5b
	coreRecentScoreOff         = 37 // plus 29 × items; CONTRACT +0x5c
	coreSquadNumberOff         = 38 // plus 29 × items; CONTRACT +0x5d
	coreUnsetOff               = 50 // plus 29 × items; a date, 1900 when unset
	tailTeamOff                = 4
	tailWageOff                = 12

	// Offsets in a 29-byte ring item (CONTRACT +0x30).
	eventTypeOff   = 1
	eventIDOff     = 2
	eventStatusOff = 4
	eventDateOff   = 13
	eventClosedOff = 17
	eventU14Off    = 21

	fullHeaderSize = 28 // u8, i32 loyalty bonus, 2 × i32, date, i64, u8, u16
	fullItemSize   = 31
	// Offsets in a 31-byte competition bonus item.
	bonusCompetitionOff = 6
	bonusAmountOff      = 10
	bonusTypeOff        = 25
	bonusTargetOff      = 26
	fullAfterHasSize    = 18 // u32 0, u8, 13-byte struct
	maxMarkerRefs       = 64

	minContractYear = 1900
	maxContractYear = 2200
)

// Contract is a player's contract with a team.
type Contract struct {
	Full       bool  // the slot type is FULL
	Kind       uint8 // FULL header byte 0 (FULL +0xc3); 0 when not FULL
	TeamIndex  int64 // the team the contract is with
	WeeklyWage int32 // GBP per week; contracts in euros are converted
	Start      Date  // joined the contract's team
	End        Date  // zero for BASIC contracts
	Signed     Date  // when the current terms were signed; the contract screen shows it as Start
	// PlayingTime is the agreed squad status, shown in squad lists and as
	// "Agreed" on the contract screen. OriginalPlayingTime was agreed at
	// signing; the game does not show it. 0 for BASIC contracts.
	PlayingTime         PlayingTime
	OriginalPlayingTime PlayingTime
	// PlayingTimeScore (-100..100) rates his game time: it starts mid-band of
	// PlayingTime and moves after matches. RecentPlayingTimeScore (-100..100)
	// is read for cup goalkeepers only (meaning a hypothesis).
	PlayingTimeScore       int8
	RecentPlayingTimeScore int8
	SquadNumber            uint8 // 255 when none
	// CoreFlags is core byte 23 (CONTRACT +0x57); bits 0, 3 and 6 are read by
	// the transfer-interest rules.
	CoreFlags uint8
	// Events are the core's ring items (CONTRACT +0x30), up to 10.
	Events []ContractEvent

	// LoyaltyBonus is the total loyalty bonus in GBP (FULL_CONTRACT +0xa8),
	// paid as WeeklyLoyalty a week (CONTRACT +0x38) until the contract ends.
	LoyaltyBonus  int32 // FULL contracts only
	WeeklyLoyalty int32
	Clauses       []Clause           // FULL contracts only
	Bonuses       []CompetitionBonus // FULL contracts only
}

// ContractEvent is one 29-byte item of a contract's ring (CONTRACT +0x30).
type ContractEvent struct {
	Type   uint8  // item byte 1 (+0x19)
	ID     uint16 // byte 2 (+0x16)
	Status int8   // byte 4 (+0x18)
	Date   Date   // byte 13 (+0x0)
	Closed Date   // byte 17 (+0x4); 1900-01-01 = open
	U14    uint16 // byte 21 (+0x14)
}

// Open reports whether the event is still open (Closed is 1900-01-01).
func (e ContractEvent) Open() bool { return e.Closed.Year == 1900 && e.Closed.dayOfYear() == 1 }

// PlayingTime is a contract's squad status, FM's "Playing Time". Names come
// from the game's table (26.3.2, 0x39279bc via 0x5265ab0). 0 is never stored.
type PlayingTime uint8

// playingTimeNames holds the names the game shows. 15-21 are goalkeeper
// statuses, 10-13 youth ones. The game shows 14 as "<B team> Regular"; the
// team is not decoded.
var playingTimeNames = [...]string{
	1: "Star Player", 2: "Important Player", 3: "Regular Starter", 4: "Squad Player",
	5: "Impact Sub", 6: "Squad Player", 7: "Fringe Player", 8: "Fringe Player",
	9: "Emergency Backup", 10: "Breakthrough Prospect", 11: "Future Prospect", 12: "Hot Prospect",
	13: "Youngster", 14: "B Team Regular", 15: "First-Choice Goalkeeper", 16: "Cup Goalkeeper",
	17: "Domestic Cup Goalkeeper", 18: "Continental Cup Goalkeeper", 19: "Cup Goalkeeper", 20: "Backup",
	21: "Emergency Backup", 22: "Surplus to Requirements",
}

// Name is the name the game shows; false for 0 and unknown values.
func (p PlayingTime) Name() (string, bool) {
	n := lookup(playingTimeNames[:], uint32(p))
	return n, n != ""
}

// Clause is a contract clause or bonus. Clauses (Type < 32) carry
// Value = -1 and a percentage, year or game count in Extra, or a fee in
// Value. Bonuses (Type ≥ 32) carry an amount in Value and Extra = 0xffff.
type Clause struct {
	Type  uint8
	Value int32
	Extra uint16
}

// CompetitionBonus is a FULL contract's bonus tied to a competition, one
// 31-byte item of the header's list:
//
//	u8 1, u8 0, date (1900-01-01), i32 competition, i32 amount,
//	u16 0xffff, u8 1, i32, i32, u8 type, i32 target competition, u8 0
//
// Type 10 is paid for qualifying for the target competition.
type CompetitionBonus struct {
	Type        uint8
	Competition int32 // COMP index; mostly the contract team's league for type 10
	Target      int32 // COMP index; -1 when none
	Value       int32 // GBP
}

// competitionBonusNames maps competition bonus types to names; type 10 is
// verified in game (Bernardi), other types are not decoded.
var competitionBonusNames = map[uint8]string{10: "qualification bonus"}

// Name names the bonus, without its competition; false for undecoded types.
func (b CompetitionBonus) Name() (string, bool) {
	n, ok := competitionBonusNames[b.Type]
	return n, ok
}

func (b CompetitionBonus) Amount() (int64, bool) { return int64(b.Value), b.Value >= 0 }

// contracts holds a person's contract slots; nil means empty.
type contracts struct {
	main, loan, future *Contract
}

type core struct {
	start, tail int
	contract    Contract
}

// marker is a contracts marker and the transfer-id list before it.
type marker struct {
	at   int // the u8 5
	list int // the list's u8 count; the person fields are read from here
	// prevOK is set when the i64 at list+listPrevClubOff reads as a
	// previous club: -1 or 0..MaxInt32.
	prevOK bool
}

// Offsets from a marker's list count to the ACTUAL_PERSON fields before it
// (writer 0x2f424a0): u16 +0x102, i64 ref +0xd8, u16, u16, u8, u8,
// date +0xe8, date +0xec, u8 +0x10b, then the list u8 n + n × u32 (+0x68).
const (
	listPrevClubOff = -23 // i64, written by the ref writer 0x335f42c
	listDate1Off    = -9  // date +0xe8
	listJoinedOff   = -5  // date +0xec
)

// readContracts finds the contract block in b[from:to] for person and
// returns it with its marker.
func readContracts(b []byte, from, to int, person uint32) (contracts, marker, bool) {
	cores := findCores(b, from, to, person)
	limit := to
	if len(cores) > 0 {
		limit = cores[0].start
	}
	markers := findMarkers(b, from, limit)
	// The marker closest to the first core is the most likely one.
	for i := len(markers) - 1; i >= 0; i-- {
		if cs, ok := parseSlots(b, markers[i].at, cores, person); ok {
			return cs, markers[i], true
		}
	}
	return contracts{}, marker{}, false
}

// findMarkers returns the bytes 5 in b[from:to] that are followed by a slot
// type and preceded by an id list u8 n + n × u32 + u32 with two dates
// before it.
func findMarkers(b []byte, from, to int) []marker {
	var out []marker
	for p := from; p+1 < to; p++ {
		if b[p] != contractMarker || !validSlotType(b[p+1]) {
			continue
		}
		if m, ok := markerAt(b, from, p); ok {
			out = append(out, m)
		}
	}
	return out
}

// markerAt validates the list before the marker at p. Ids between 1900 and
// 2200 can make a shorter list look valid, since they read as a year
// (118 of 138 955 players on 2036-01-16). The previous-club ref tells them
// apart: the false reading shifts it by 4 bytes, out of the club range in
// every case seen. So the longest list whose ref reads as a club wins, and
// the shortest one when none does.
func markerAt(b []byte, from, p int) (marker, bool) {
	var found []marker
	for n := 0; n < maxMarkerRefs; n++ {
		q := p - 5 - 4*n
		if q+listDate1Off < from {
			break
		}
		if int(b[q]) == n && validDates(b, q+listJoinedOff, q+listDate1Off) {
			found = append(found, marker{at: p, list: q, prevOK: q+listPrevClubOff >= from && validPrevClub(prevClubAt(b, q))})
		}
	}
	for i := len(found) - 1; i >= 0; i-- {
		if found[i].prevOK {
			return found[i], true
		}
	}
	if len(found) == 0 {
		return marker{}, false
	}
	return found[0], true
}

func prevClubAt(b []byte, list int) int64 {
	return int64(binary.LittleEndian.Uint64(b[list+listPrevClubOff:])) //nolint:gosec // signed ref
}

func validPrevClub(v int64) bool { return v == -1 || v >= 0 && v <= math.MaxInt32 }

// readPersonFields reads the ACTUAL_PERSON fields before m: the joined date,
// the previous club and the transfer ids.
func readPersonFields(b []byte, m marker) (joined Date, previous int64, ids []uint32) {
	joined, previous = dateAt(b, m.list+listJoinedOff), -1
	if m.prevOK {
		previous = prevClubAt(b, m.list)
	}
	for i := range int(b[m.list]) {
		ids = append(ids, binary.LittleEndian.Uint32(b[m.list+1+4*i:]))
	}
	return joined, previous, ids
}

func validSlotType(t byte) bool { return t <= typeBasic || t == typeEmpty }

func parseSlots(b []byte, m int, cores []core, person uint32) (contracts, bool) {
	var slots [contractSlots]*Contract
	p := m + 1
	for slot := range slots {
		if p >= len(b) {
			return contracts{}, false
		}
		t := b[p]
		p++
		switch {
		case t == typeEmpty:
			continue
		case t == typeBasic:
			if p+coreTailSize > len(b) || binary.LittleEndian.Uint32(b[p:]) != person {
				return contracts{}, false
			}
			slots[slot] = &Contract{TeamIndex: tailTeam(b, p), WeeklyWage: tailWage(b, p)}
			p += coreTailSize
			continue
		case t > typeBasic:
			return contracts{}, false
		}
		next := firstCoreFrom(cores, p)
		if next == nil {
			return contracts{}, false
		}
		c := next.contract
		switch t {
		case typeFull:
			h, ok := readFullHeader(b, p)
			if !ok || (!h.has && h.after+fullAfterHasSize != next.start) {
				return contracts{}, false
			}
			c.Full, c.Kind = true, h.kind
			c.LoyaltyBonus, c.Clauses, c.Bonuses = h.loyaltyBonus, h.clauses, h.bonuses
		case typeLoan:
			// LOAN header not decoded; its core follows.
		default: // TRIAL, B_CLUB, YOUTH_TRIAL: the core follows the type byte
			if next.start != p {
				return contracts{}, false
			}
		}
		slots[slot] = &c
		p = next.tail + coreTailSize
	}
	return contracts{main: slots[slotMain], loan: slots[slotLoan], future: slots[slotFuture]}, true
}

func firstCoreFrom(cores []core, p int) *core {
	for i := range cores {
		if cores[i].start >= p {
			return &cores[i]
		}
	}
	return nil
}

func competitionBonus(item []byte) CompetitionBonus {
	i32 := func(off int) int32 { return int32(binary.LittleEndian.Uint32(item[off:])) } //nolint:gosec // signed field
	return CompetitionBonus{
		Type:        item[bonusTypeOff],
		Competition: i32(bonusCompetitionOff),
		Target:      i32(bonusTargetOff),
		Value:       i32(bonusAmountOff),
	}
}

// fullHeader is a FULL_CONTRACT header up to its has-flag.
type fullHeader struct {
	kind         uint8
	loyaltyBonus int32
	clauses      []Clause
	bonuses      []CompetitionBonus
	after        int // the position after the has-flag
	has          bool
}

// readFullHeader reads the header at h (0x3490034).
func readFullHeader(b []byte, h int) (fullHeader, bool) {
	var out fullHeader
	r := newReader(b)
	r.off = h
	out.kind = r.u8()
	out.loyaltyBonus = r.i32()
	r.take(fullHeaderSize - 5)
	n := int(r.u8())
	for range n {
		out.clauses = append(out.clauses, Clause{Value: r.i32(), Extra: r.u16(), Type: r.u8()})
		r.u8()
	}
	n = int(r.u8())
	for range n {
		if item := r.take(fullItemSize); item != nil {
			out.bonuses = append(out.bonuses, competitionBonus(item))
		}
	}
	out.has = r.u8() != 0
	out.after = r.off
	return out, r.err == nil
}

// findCores returns the contract cores in b[from:to] whose tail carries
// person, in record order.
func findCores(b []byte, from, to int, person uint32) []core {
	key := binary.LittleEndian.AppendUint32(nil, person)
	var out []core
	for q := from; q < to; {
		i := bytes.Index(b[q:to], key)
		if i < 0 {
			break
		}
		q += i
		if q+coreTailSize > to {
			break
		}
		if c, ok := coreEndingAt(b, from, q); ok {
			out = append(out, c)
			q += coreTailSize
			continue
		}
		q++
	}
	return out
}

// coreEndingAt decodes the core whose tail starts at q, trying each
// possible item count.
func coreEndingAt(b []byte, from, q int) (core, bool) {
	for k := 0; k <= maxCoreItems; k++ {
		c := q - coreFixedSize - coreItemSize*k
		if c < from {
			break
		}
		if binary.LittleEndian.Uint32(b[c+coreItemsOff:]) != uint32(k) { //nolint:gosec // k ≤ 10
			continue
		}
		if !validDates(b, c+coreEndOff, c+coreStartOff, c+coreSignedOff+coreItemSize*k, c+coreUnsetOff+coreItemSize*k) {
			continue
		}
		items := coreItemSize * k
		var events []ContractEvent
		for i := range k {
			events = append(events, contractEvent(b[c+coreItemsOff+4+coreItemSize*i:]))
		}
		return core{start: c, tail: q, contract: Contract{
			TeamIndex:              tailTeam(b, q),
			WeeklyWage:             tailWage(b, q),
			End:                    dateAt(b, c+coreEndOff),
			Start:                  dateAt(b, c+coreStartOff),
			Signed:                 dateAt(b, c+coreSignedOff+items),
			PlayingTime:            PlayingTime(b[c+corePlayingTimeOff]),
			OriginalPlayingTime:    PlayingTime(b[c+coreOriginalPlayingTimeOff]),
			PlayingTimeScore:       int8(b[c+coreScoreOff+items]),       //nolint:gosec // signed field
			RecentPlayingTimeScore: int8(b[c+coreRecentScoreOff+items]), //nolint:gosec // signed field
			SquadNumber:            b[c+coreSquadNumberOff+items],
			CoreFlags:              b[c+coreFlagsOff],
			Events:                 events,
			WeeklyLoyalty:          int32(binary.LittleEndian.Uint32(b[c+coreLoyaltyOff:])), //nolint:gosec // signed field
		}}, true
	}
	return core{}, false
}

func contractEvent(item []byte) ContractEvent {
	return ContractEvent{
		Type:   item[eventTypeOff],
		ID:     binary.LittleEndian.Uint16(item[eventIDOff:]),
		Status: int8(item[eventStatusOff]), //nolint:gosec // signed field
		Date:   dateAt(item, eventDateOff),
		Closed: dateAt(item, eventClosedOff),
		U14:    binary.LittleEndian.Uint16(item[eventU14Off:]),
	}
}

func tailTeam(b []byte, q int) int64 {
	return int64(binary.LittleEndian.Uint64(b[q+tailTeamOff:])) //nolint:gosec // signed field
}

func tailWage(b []byte, q int) int32 {
	return int32(binary.LittleEndian.Uint32(b[q+tailWageOff:])) //nolint:gosec // signed field
}

func validDates(b []byte, at ...int) bool {
	for _, p := range at {
		if p < 0 || p+4 > len(b) {
			return false
		}
		if !dateAt(b, p).plausible(minContractYear, maxContractYear) {
			return false
		}
	}
	return true
}

// clauseNames maps clause and bonus types to names. They are inferred from
// the game's negotiation-flag strings (switch at 0x1232ea4, flag = type + 8)
// and from which players hold each bonus, so treat them as guesses.
var clauseNames = map[uint8]string{
	0:  "minimum fee release clause",
	1:  "relegation release clause",
	2:  "non-promotion release clause",
	3:  "yearly wage rise",
	4:  "promotion wage rise",
	5:  "relegation wage drop",
	8:  "sell-on percentage",
	10: "goal bonus after goals",
	11: "one-year extension after league games",
	13: "wage rise after league games",
	14: "promotion wage rise",
	15: "relegation wage drop",
	16: "minimum fee release clause (foreign clubs)",
	17: "minimum fee release clause (higher division clubs)",
	18: "minimum fee release clause (domestic clubs)",
	22: "optional contract extension by club",
	25: "one-year extension if promoted",
	26: "one-year extension if relegation is avoided",
	31: "minimum fee release clause (continental clubs)",
	32: "appearance fee",
	33: "goal bonus",
	34: "clean sheet bonus",
	37: "international cap bonus",
	38: "unused substitute fee",
	39: "assist bonus",
}

// minimumFeeReleaseClauses are the clauses a bid of the fee triggers, some
// only for one kind of club. Relegation and non-promotion clauses also need
// the event.
var minimumFeeReleaseClauses = map[uint8]bool{0: true, 16: true, 17: true, 18: true, 31: true}

// MinimumFeeRelease reports whether c is a minimum fee release clause.
func (c Clause) MinimumFeeRelease() bool { return minimumFeeReleaseClauses[c.Type] }

// percentClauses carry a percentage in Extra.
var percentClauses = map[uint8]bool{3: true, 4: true, 5: true, 8: true, 14: true, 15: true}

// countClauses carry a number of goals, games or years in Extra. Other
// clauses (release clauses among them) can hold an Extra of unknown meaning.
var countClauses = map[uint8]bool{10: true, 11: true, 13: true, 22: true, 25: true, 26: true}

const noExtra = 0xffff

// Name names the clause or bonus; "unknown clause" for unmapped types.
func (c Clause) Name() string {
	if n, ok := clauseNames[c.Type]; ok {
		return n
	}
	return "unknown clause"
}

// Amount is the fee or bonus in GBP, when the clause carries one.
func (c Clause) Amount() (int64, bool) { return int64(c.Value), c.Value >= 0 }

// Percent is the percentage of a wage-rise, wage-drop or sell-on clause.
func (c Clause) Percent() (int, bool) {
	return int(c.Extra), c.Extra != noExtra && percentClauses[c.Type]
}

// Count is the number of years or games a clause refers to.
func (c Clause) Count() (int, bool) {
	return int(c.Extra), c.Extra != noExtra && countClauses[c.Type]
}
