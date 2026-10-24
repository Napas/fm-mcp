package savefile

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
)

type basicPart struct {
	first, last uint32
	common      int32
	full        string
	dob         Date
	nation      int64
	hidden      [8]byte
	birthplace  int64
	traits      uint64 // the person part's last 8 bytes, just before this part
	relations   []byte // written after the birthplace; when nil, the record's 0, 0 filler is read as the present flag
}

func (b basicPart) encode() []byte {
	out := binary.LittleEndian.AppendUint32(nil, b.first)
	out = append(out, 0)
	out = binary.LittleEndian.AppendUint32(out, b.last)
	out = append(out, 0)
	out = binary.LittleEndian.AppendUint32(out, uint32(b.common)) //nolint:gosec // two's complement intended
	out = append(out, 0)
	out = appendString(out, b.full)
	out = appendDate(out, b.dob)
	out = append(out, make([]byte, 9)...)
	out = binary.LittleEndian.AppendUint64(out, uint64(b.nation)) //nolint:gosec // two's complement intended
	out = append(out, b.hidden[:]...)
	out = binary.LittleEndian.AppendUint64(out, uint64(b.birthplace)) //nolint:gosec // two's complement intended
	return append(out, b.relations...)
}

// relationItem is one 16-byte item of the BASIC part's relationship vector.
func relationItem(ref int64, reason, kind, relation uint8, value int8) []byte {
	out := binary.LittleEndian.AppendUint64(nil, uint64(ref)) //nolint:gosec // two's complement intended
	out = append(out, reason, 0, kind, relation, byte(value)) //nolint:gosec // two's complement intended
	return append(out, 78, 0, 0xff)
}

// relations is the relationship vector: present flag, u8 count, items.
func relations(items ...[]byte) []byte {
	out := []byte{1, byte(len(items))} //nolint:gosec // test vectors hold few items
	for _, it := range items {
		out = append(out, it...)
	}
	return out
}

// playerRecord builds a tag-2 record with a dummy 1900 name struct before the
// real basic part.
func playerRecord(index, uid uint32, basic basicPart) []byte {
	return playerRecordWith(index, uid, basic, nil)
}

// playerRecordWith builds a player record with personPart (e.g. contract
// bytes) placed in the undecoded person part.
func playerRecordWith(index, uid uint32, basic basicPart, personPart []byte) []byte {
	b := []byte{2}
	for _, v := range []uint16{7500, 7550, 6500, 151, 163} {
		b = binary.LittleEndian.AppendUint16(b, v)
	}
	b = append(b, make([]byte, 8)...)
	b = binary.LittleEndian.AppendUint32(b, 0xffffffff)
	b = binary.LittleEndian.AppendUint64(b, 547)
	for i := range 15 {
		b = append(b, byte(1+i))
	}
	for i := range 54 {
		b = append(b, byte(20+i))
	}
	b = binary.LittleEndian.AppendUint32(b, 51500000) // +100 value
	b = append(b, 12)                                 // +104
	b = append(b, 0xb4, 0x00, 0xe7, 0x07)             // +105 joined 2023 day 180
	b = append(b, 0x01, 0x00, 0x6c, 0x07)             // +109
	b = binary.LittleEndian.AppendUint16(b, 5400)     // +113 sharpness
	b = binary.LittleEndian.AppendUint16(b, 0)        // +115
	b = binary.LittleEndian.AppendUint16(b, 9300)     // +117 condition
	b = binary.LittleEndian.AppendUint16(b, 255)      // +119
	b = append(b, make([]byte, 6)...)                 // +121
	b = append(b, 2)                                  // +127
	b = binary.LittleEndian.AppendUint16(b, 175)      // +128 height
	b = append(b, 3)                                  // +130
	b = append(b, personPart...)
	dummy := basicPart{first: 0, last: 0, common: -1, dob: Date{Raw: 1, Year: 1900}, nation: 3}
	b = append(b, dummy.encode()...)
	b = append(b, 0x01)
	b = binary.LittleEndian.AppendUint64(b, basic.traits)
	b = append(b, basic.encode()...)
	b = append(b, 0, 0)
	return append(b, encodePersonTrailer(0x400400000, index, uid)...)
}

var testNames = NameTables{
	First:  []string{"Oldřich", "James"},
	Last:   []string{"Rua", "Maddison"},
	Common: []string{"Quinn"},
}

func TestParsePlayerReadsFixedPartAndLastBasicPart(t *testing.T) {
	maddison := basicPart{
		first: 1, last: 1, common: -1, full: "James Daniel Maddison",
		dob: Date{Raw: 328, Year: 1996}, nation: 139,
		hidden: [8]byte{12, 17, 12, 14, 15, 8, 13, 12}, birthplace: 1040,
		traits: 0x0100000000000002,
	}
	rec := playerRecord(8804, 28093567, maddison)
	body := append([]byte{0xff}, rec...)
	pr := Record{Tag: 2, Index: 8804, UID: 28093567, Start: 1, End: len(body)}

	p, err := parsePlayer(body, pr, testNames)
	if err != nil {
		t.Fatalf("parsePlayer: %v", err)
	}

	if p.Index != 8804 || p.UID != 28093567 || p.PlayerCoach {
		t.Errorf("identity = %d/%d coach=%v", p.Index, p.UID, p.PlayerCoach)
	}
	if p.FirstName != "James" || p.LastName != "Maddison" || p.CommonName != "" || p.FullName != "James Daniel Maddison" {
		t.Errorf("names = %q %q %q %q", p.FirstName, p.LastName, p.CommonName, p.FullName)
	}
	if p.BirthDate != (Date{Raw: 328, Year: 1996}) || p.NationIndex != 139 || p.BirthplaceIndex != 1040 {
		t.Errorf("dob/nation/birthplace = %+v %d %d", p.BirthDate, p.NationIndex, p.BirthplaceIndex)
	}
	if p.Hidden != [8]uint8{12, 17, 12, 14, 15, 8, 13, 12} || p.Traits != 0x0100000000000002 {
		t.Errorf("Hidden = %v, Traits = %#x", p.Hidden, p.Traits)
	}
	if p.HomeReputation != 7500 || p.CurrentReputation != 7550 || p.WorldReputation != 6500 {
		t.Errorf("reputations = %d %d %d", p.HomeReputation, p.CurrentReputation, p.WorldReputation)
	}
	if p.CurrentAbility != 151 || p.PotentialAbility != 163 || p.TeamIndex != 547 || p.NationalTeamRef != -1 {
		t.Errorf("CA/PA/club/national = %d %d %d %d", p.CurrentAbility, p.PotentialAbility, p.TeamIndex, p.NationalTeamRef)
	}
	if p.Positions[0] != 1 || p.Positions[14] != 15 || p.Attributes[0] != 20 || p.Attributes[53] != 73 {
		t.Errorf("positions/attributes = %v %v", p.Positions, p.Attributes)
	}
	if p.Value != 51500000 || p.Height != 175 || p.Condition != 9300 || p.Sharpness != 5400 || p.JoinedClub != (Date{Raw: 180, Year: 2023}) {
		t.Errorf("value/height/condition/sharpness/joined = %d %d %d %d %+v", p.Value, p.Height, p.Condition, p.Sharpness, p.JoinedClub)
	}
}

func TestParsePlayerResolvesCommonName(t *testing.T) {
	quinn := basicPart{first: noName, last: noName, common: 0, dob: Date{Raw: 40, Year: 2001}, nation: 139}
	rec := playerRecord(1, 7, quinn)
	pr := Record{Tag: 2, Index: 1, UID: 7, Start: 0, End: len(rec)}

	p, err := parsePlayer(rec, pr, testNames)
	if err != nil {
		t.Fatalf("parsePlayer: %v", err)
	}

	if p.CommonName != "Quinn" || p.FirstName != "" || p.LastName != "" {
		t.Errorf("names = %q %q %q", p.FirstName, p.LastName, p.CommonName)
	}
}

func TestParsePlayerReportsMissingBasicPart(t *testing.T) {
	rec := append([]byte{2}, make([]byte, 140)...)
	rec = append(rec, encodePersonTrailer(0x400400000, 0, 9)...)
	pr := Record{Tag: 2, Index: 0, UID: 9, Start: 0, End: len(rec)}

	_, err := parsePlayer(rec, pr, testNames)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestParsePlayerToleratesStaleCommonNameRef(t *testing.T) {
	// Generated players can carry a common-name ref beyond the common- and
	// first-name tables (646431 in a 26.3.2 save); it stays unresolved.
	names := NameTables{First: []string{"A", "B"}, Last: []string{"X", "Y", "Z"}, Common: []string{"Q"}}
	newgen := basicPart{first: 1, last: 2, common: 2, dob: Date{Raw: 109, Year: 2017}, nation: 139}
	rec := playerRecord(98699, 2000455995, newgen)
	pr := Record{Tag: 2, Index: 98699, UID: 2000455995, Start: 0, End: len(rec)}

	p, err := parsePlayer(rec, pr, names)
	if err != nil {
		t.Fatalf("parsePlayer: %v", err)
	}

	if p.FirstName != "B" || p.LastName != "Z" || p.CommonName != "" {
		t.Errorf("names = %q %q %q", p.FirstName, p.LastName, p.CommonName)
	}
}

func TestParsePlayerReadsGenderFromTrailerFlags(t *testing.T) {
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 2004}, nation: 139}
	rec := playerRecord(3, 9, basic)
	for _, tt := range []struct {
		flags  uint64
		female bool
	}{{0x400400000, false}, {0x400401000, true}} {
		pr := Record{Tag: 2, Index: 3, UID: 9, Flags: tt.flags, Start: 0, End: len(rec)}
		p, err := parsePlayer(rec, pr, testNames)
		if err != nil {
			t.Fatal(err)
		}
		if p.Female != tt.female {
			t.Errorf("flags %#x: Female = %v, want %v", tt.flags, p.Female, tt.female)
		}
	}
}

func TestParsePlayerReadsFlagBit3FromTrailerFlags(t *testing.T) {
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 2004}, nation: 139}
	rec := playerRecord(3, 9, basic)
	for _, tt := range []struct {
		flags uint64
		bit3  bool
	}{{0x400400000, false}, {0x400400008, true}, {0x400401000, false}} {
		pr := Record{Tag: 2, Index: 3, UID: 9, Flags: tt.flags, Start: 0, End: len(rec)}
		p, err := parsePlayer(rec, pr, testNames)
		if err != nil {
			t.Fatal(err)
		}
		if p.FlagBit3 != tt.bit3 {
			t.Errorf("flags %#x: FlagBit3 = %v, want %v", tt.flags, p.FlagBit3, tt.bit3)
		}
	}
}

func parseWithRelations(t *testing.T, rel []byte) Player {
	t.Helper()
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 2004}, nation: 139, birthplace: 1040, relations: rel}
	rec := playerRecord(3, 9, basic)
	p, err := parsePlayer(rec, Record{Tag: 2, Index: 3, UID: 9, Start: 0, End: len(rec)}, testNames)
	if err != nil {
		t.Fatalf("parsePlayer: %v", err)
	}
	return p
}

func TestParsePlayerReadsClubAffinitiesFromTheRelationshipVector(t *testing.T) {
	p := parseWithRelations(t, relations(
		relationItem(1115, 1, relationClub, relationFavoured, 100), // supporter
		relationItem(6, 0, relationClub, 72, 1),                    // another club relation
		relationItem(190, 2, relationClub, relationFavoured, 100),  // as a player
		relationItem(158530, 6, 3, relationFavoured, 90),           // a person, not a club
		relationItem(464, 2, relationClub, relationFavoured, -50),  // disliked
	))

	want := []ClubAffinity{{Club: 1115, Value: 100, Reason: 1}, {Club: 190, Value: 100, Reason: 2}, {Club: 464, Value: -50, Reason: 2}}
	if !reflect.DeepEqual(p.ClubAffinities, want) {
		t.Errorf("ClubAffinities = %+v, want %+v", p.ClubAffinities, want)
	}
	if p.BirthplaceIndex != 1040 {
		t.Errorf("BirthplaceIndex = %d", p.BirthplaceIndex)
	}
}

// The vector is optional data: a bad one leaves no club affinities but keeps
// the player.
func TestParsePlayerSkipsMissingOrBadRelationshipVectors(t *testing.T) {
	item := relationItem(1115, 1, relationClub, relationFavoured, 100)
	tooMany := append([]byte{1, maxRelations + 1}, make([]byte, relationItemSize*(maxRelations+1))...)
	for name, rel := range map[string][]byte{
		"absent":          nil, // the present flag is 0
		"unknown flag":    append([]byte{2, 1}, item...),
		"truncated items": append([]byte{1, 5}, item...),
		"too many items":  tooMany,
	} {
		if p := parseWithRelations(t, rel); p.ClubAffinities != nil {
			t.Errorf("%s: ClubAffinities = %+v, want none", name, p.ClubAffinities)
		}
	}
}

func TestParsePlayerReadsTheLargestRelationshipVector(t *testing.T) {
	items := make([][]byte, maxRelations)
	for i := range items {
		items[i] = relationItem(int64(i), 2, relationClub, relationFavoured, 50)
	}
	p := parseWithRelations(t, relations(items...))
	if len(p.ClubAffinities) != maxRelations || p.ClubAffinities[maxRelations-1].Club != maxRelations-1 {
		t.Errorf("got %d affinities, want %d", len(p.ClubAffinities), maxRelations)
	}
}

func TestClubAffinityReasonName(t *testing.T) {
	for reason, want := range map[uint8]string{1: "supporter", 2: "player", 0: "", 4: ""} {
		got, ok := ClubAffinity{Reason: reason}.ReasonName()
		if got != want || ok != (want != "") {
			t.Errorf("reason %d: %q, %v; want %q", reason, got, ok, want)
		}
	}
}

func TestTraitNamesLeaveBits48And53Unnamed(t *testing.T) {
	named := 0
	for bit, name := range TraitNames {
		if name != "" {
			named++
		}
		if (bit == 48 || bit == 53) != (name == "") {
			t.Errorf("bit %d = %q", bit, name)
		}
	}
	if named != 62 || TraitNames[0] != "Runs With Ball Down Left" || TraitNames[15] != "Marks Opponent Tightly" ||
		TraitNames[34] != "Avoids Using Weaker Foot" || TraitNames[63] != "Plays Ball With Feet" {
		t.Errorf("%d names; 0 %q, 15 %q, 34 %q, 63 %q", named, TraitNames[0], TraitNames[15], TraitNames[34], TraitNames[63])
	}
}

func TestHiddenAttributeNamesFollowTheLoader(t *testing.T) {
	want := [8]string{"Adaptability", "Ambition", "Loyalty", "Pressure", "Professionalism", "Sportsmanship", "Temperament", "Controversy"}
	if HiddenAttributeNames != want {
		t.Errorf("HiddenAttributeNames = %v", HiddenAttributeNames)
	}
}

type testContract struct {
	person     uint32
	team       int64
	wage       int32
	start, end Date
	signed     Date
	// Core bytes 20-22: playing time, previous playing time (hypothesis)
	// and original playing time.
	playingTime [3]byte
	// Core bytes 36-38 after the items: playing-time score, recent score
	// and squad number.
	score, recentScore int8
	squadNumber        uint8
	itemCount          int
	items              [][]byte // raw 29-byte ring items; set instead of itemCount
	kind               byte     // FULL header byte 0
	coreFlags          byte     // core byte 23
	weeklyLoyalty      int32    // core offset 0
	loyaltyBonus       int32    // FULL header offset 1
}

// encode writes a CONTRACT core and its BASIC_CONTRACT tail.
func (c testContract) encode(b []byte) []byte {
	b = le32(b, c.weeklyLoyalty)
	b = le32(b, -1)
	b = le32(b, -1)
	b = appendDate(b, c.end)
	b = appendDate(b, c.start)
	b = append(b, c.playingTime[:]...)
	b = append(b, c.coreFlags, 0, 0)
	if c.items != nil {
		c.itemCount = len(c.items)
	}
	b = binary.LittleEndian.AppendUint32(b, uint32(c.itemCount)) //nolint:gosec // small test data
	b = append(b, make([]byte, 29*c.itemCount)...)
	for i, it := range c.items {
		copy(b[len(b)-29*(len(c.items)-i):], it)
	}
	b = append(b, 0)
	b = appendDate(b, c.signed)
	b = append(b, 0, byte(c.score), byte(c.recentScore), c.squadNumber, 0, 0, 0) //nolint:gosec // two's complement intended
	b = append(b, make([]byte, 8)...)
	b = appendDate(b, Date{Raw: 1, Year: 1900})
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint32(b, c.person)
	b = binary.LittleEndian.AppendUint64(b, uint64(c.team)) //nolint:gosec // two's complement intended
	b = le32(b, c.wage)
	b = append(b, 1)
	return binary.LittleEndian.AppendUint16(b, 11)
}

// personFields are the ACTUAL_PERSON fields written before the contracts
// marker.
type personFields struct {
	previousClub int64
	date1        Date // +0xe8
	joined       Date // +0xec
	u8           byte // +0x10b
	transferIDs  []uint32
}

// block encodes the fields from the ACTUAL_PERSON part's first u16 to the
// marker, the constant 5, then five slots.
func (f personFields) block(slots ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint16(nil, 0x1000)              // u16 +0x102
	b = binary.LittleEndian.AppendUint64(b, uint64(f.previousClub)) //nolint:gosec // two's complement intended
	b = append(b, 0, 0, 0, 0, 8, 3)                                 // u16, u16, u8, u8
	b = appendDate(appendDate(b, f.date1), f.joined)                // +0xe8, +0xec
	b = append(b, f.u8, byte(len(f.transferIDs)))                   //nolint:gosec // small test data
	for _, id := range f.transferIDs {
		b = binary.LittleEndian.AppendUint32(b, id)
	}
	b = binary.LittleEndian.AppendUint32(b, 0) // u32 +0xb0
	b = append(b, 5)
	for _, s := range slots {
		b = append(b, s...)
	}
	return b
}

// contractBlock encodes the person part's contract block with default
// person fields.
func contractBlock(slots ...[]byte) []byte {
	return personFields{previousClub: -1, date1: Date{Raw: 180, Year: 2023}, joined: Date{Raw: 1, Year: 1900}, u8: 9, transferIDs: []uint32{42}}.block(slots...)
}

func emptySlot() []byte { return []byte{0xff} }

// fullSlot encodes a FULL_CONTRACT slot: type 0, header with clauses, core.
func fullSlot(c testContract, clauses ...Clause) []byte {
	return fullSlotWithBonuses(c, nil, clauses...)
}

// fullSlotWithBonuses is fullSlot with raw 31-byte competition bonus items.
func fullSlotWithBonuses(c testContract, bonuses [][]byte, clauses ...Clause) []byte {
	b := []byte{0, c.kind} // type, contract type
	b = le32(b, c.loyaltyBonus)
	b = append(b, make([]byte, 8)...)     // two i32
	b = append(b, 0x01, 0x00, 0x6c, 0x07) // date
	b = binary.LittleEndian.AppendUint64(b, ^uint64(0))
	b = append(b, 0, 0, 0)            // u8, u16
	b = append(b, byte(len(clauses))) //nolint:gosec // small test data
	for _, cl := range clauses {
		b = le32(b, cl.Value)
		b = binary.LittleEndian.AppendUint16(b, cl.Extra)
		b = append(b, cl.Type, 0)
	}
	b = append(b, byte(len(bonuses))) //nolint:gosec // small test data
	for _, item := range bonuses {
		b = append(b, item...)
	}
	b = append(b, 0)                                     // has-flag 0
	b = append(b, 0, 0, 0, 0, 0xff)                      // u32 0, u8
	b = append(b, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0) // 13-byte struct
	return c.encode(b)
}

// loanSlot encodes a LOAN_CONTRACT slot; its header is not decoded.
func loanSlot(c testContract) []byte {
	return c.encode([]byte{1, 0xde, 0xad, 0xbe, 0xef})
}

func parseWithBlock(t *testing.T, block []byte) Player {
	t.Helper()
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 1998}, nation: 143}
	person := binary.LittleEndian.AppendUint32([]byte{5, 0, 0}, 77) // stray copy of the person index
	rec := playerRecordWith(77, 9, basic, append(person, block...))
	pr := Record{Tag: 2, Index: 77, UID: 9, Start: 0, End: len(rec)}
	p, err := parsePlayer(rec, pr, testNames)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParsePlayerReadsContractSlots(t *testing.T) {
	main := testContract{person: 77, team: 663, wage: 149164, start: Date{Raw: 240, Year: 2023}, end: Date{Raw: 181, Year: 2028}, signed: Date{Raw: 240, Year: 2023}, playingTime: [3]byte{3, 2, 2}, score: 88, recentScore: -5, squadNumber: 19, itemCount: 2, weeklyLoyalty: 2299, loyaltyBonus: 482888}
	loan := testContract{person: 77, team: 547, wage: 149164, start: Date{Raw: 240, Year: 2025}, end: Date{Raw: 181, Year: 2026}, signed: Date{Raw: 240, Year: 2025}, playingTime: [3]byte{4, 0, 7}, score: -15, recentScore: 3, squadNumber: 28, weeklyLoyalty: 15}
	release := Clause{Type: 0, Value: 60_000_000, Extra: 0xffff}
	sellOn := Clause{Type: 8, Value: -1, Extra: 15}

	p := parseWithBlock(t, contractBlock(fullSlot(main, release, sellOn), loanSlot(loan), emptySlot(), emptySlot(), emptySlot()))

	want := Contract{
		Full: true, TeamIndex: 663, WeeklyWage: 149164, Start: main.start, End: main.end, Signed: main.signed, PlayingTime: 3, OriginalPlayingTime: 2,
		PlayingTimeScore: 88, RecentPlayingTimeScore: -5, SquadNumber: 19,
		LoyaltyBonus: 482888, WeeklyLoyalty: 2299, Clauses: []Clause{release, sellOn},
		Events: []ContractEvent{{}, {}},
	}
	if p.Contract == nil || !reflect.DeepEqual(*p.Contract, want) {
		t.Errorf("Contract = %+v, want %+v", p.Contract, want)
	}
	if p.Loan == nil || p.Loan.TeamIndex != 547 || p.Loan.End != loan.end || p.Loan.PlayingTime != 4 || p.Loan.OriginalPlayingTime != 7 ||
		p.Loan.PlayingTimeScore != -15 || p.Loan.RecentPlayingTimeScore != 3 || p.Loan.SquadNumber != 28 || p.Loan.WeeklyLoyalty != 15 || p.Loan.LoyaltyBonus != 0 {
		t.Errorf("Loan = %+v, want to team 547 until %+v, playing time 4, original 7, score -15, recent 3, number 28, weekly loyalty 15, no total", p.Loan, loan.end)
	}
	if p.FutureContract != nil {
		t.Errorf("FutureContract = %+v, want none", p.FutureContract)
	}
}

func TestParsePlayerReadsCompetitionBonuses(t *testing.T) {
	main := testContract{person: 77, team: 652, wage: 21605, start: Date{Raw: 12, Year: 2035}, end: Date{Raw: 181, Year: 2039}, signed: Date{Raw: 12, Year: 2035}, playingTime: [3]byte{3, 0, 3}}
	// Carlo Bernardi's item (26.3.2): Ligue 1 (11), £60 964 for qualifying
	// for the UEFA Champions League (637).
	bernardi, _ := hex.DecodeString("010001006c070b00000024ee0000ffff01ffffffffffffffff0a7d02000000")
	other, _ := hex.DecodeString("010001006c0784020000fc2e0000ffff01ffffffffffffffff06ffffffff00")

	p := parseWithBlock(t, contractBlock(fullSlotWithBonuses(main, [][]byte{bernardi, other}), emptySlot(), emptySlot(), emptySlot(), emptySlot()))

	want := []CompetitionBonus{
		{Type: 10, Competition: 11, Target: 637, Value: 60964},
		{Type: 6, Competition: 644, Target: -1, Value: 12028},
	}
	if p.Contract == nil || !reflect.DeepEqual(p.Contract.Bonuses, want) {
		t.Errorf("Contract = %+v, want bonuses %+v", p.Contract, want)
	}
}

func TestParsePlayerWithOnlyFutureContract(t *testing.T) {
	future := testContract{person: 77, team: 900, wage: 20000, start: Date{Raw: 182, Year: 2036}, end: Date{Raw: 181, Year: 2039}, signed: Date{Raw: 300, Year: 2035}}

	p := parseWithBlock(t, contractBlock(emptySlot(), emptySlot(), emptySlot(), fullSlot(future), emptySlot()))

	if p.Contract != nil || p.Loan != nil {
		t.Errorf("contract %+v, loan %+v; want none", p.Contract, p.Loan)
	}
	if p.FutureContract == nil || p.FutureContract.TeamIndex != 900 || p.FutureContract.WeeklyWage != 20000 {
		t.Errorf("FutureContract = %+v", p.FutureContract)
	}
}

func TestParsePlayerWithoutContract(t *testing.T) {
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 1998}, nation: 143}
	rec := playerRecord(5, 9, basic)
	pr := Record{Tag: 2, Index: 5, UID: 9, Start: 0, End: len(rec)}

	p, err := parsePlayer(rec, pr, testNames)
	if err != nil {
		t.Fatal(err)
	}

	if p.Contract != nil || p.Loan != nil || p.FutureContract != nil {
		t.Errorf("free agent has contracts %+v %+v %+v", p.Contract, p.Loan, p.FutureContract)
	}
	if p.PreviousClub != -1 || p.TransferIDs != nil || p.JoinedClubPerson != (Date{}) {
		t.Errorf("free agent without a contract block: previous club %d, transfer ids %v, joined %+v", p.PreviousClub, p.TransferIDs, p.JoinedClubPerson)
	}
}

func TestParsePlayerPersonFields(t *testing.T) {
	main := testContract{person: 77, team: 652, wage: 21605, start: Date{Raw: 228, Year: 2035}, end: Date{Raw: 181, Year: 2039}, signed: Date{Raw: 228, Year: 2035}, playingTime: [3]byte{3, 0, 3}}
	slots := [][]byte{fullSlot(main), emptySlot(), emptySlot(), emptySlot(), emptySlot()}
	unset := Date{Raw: 1, Year: 1900}
	// Real bytes (uid 2002114050, 2036-01-16) from 40 bytes before a false
	// n = 1 list to the marker: the list is empty and the previous club is
	// -1, and the false reading's previous club would not be a club index.
	falseLonger, _ := hex.DecodeString("ff0000035b0019000c01000a800b1f0a2a1e2b10c0ffffffffffffffff1400" + "03000a0801006c070301e7070a00596e000005")
	for _, tt := range []struct {
		name              string
		block             []byte
		flags             uint64
		joined            Date
		previous          int64
		ids               []uint32
		retiring, onlyOne bool
	}{
		{
			name:  "two transfers, retiring, one club",
			block: personFields{previousClub: 474, date1: unset, joined: Date{Raw: 228, Year: 2035}, u8: 10, transferIDs: []uint32{10183, 10810}}.block(slots...),
			flags: 0x430400208 | 1<<15 | 1<<18, joined: Date{Raw: 228, Year: 2035}, previous: 474, ids: []uint32{10183, 10810},
			retiring: true, onlyOne: true,
		},
		{
			name:  "no transfers, no previous club",
			block: personFields{previousClub: -1, date1: unset, joined: Date{Raw: 246, Year: 2035}, u8: 10}.block(slots...),
			flags: 0x430400208, joined: Date{Raw: 246, Year: 2035}, previous: -1,
		},
		{
			// Id 2003 makes n = 0 look valid too: its "date" reads 2003.
			name:  "a transfer id that reads as a year",
			block: personFields{previousClub: 652, date1: unset, joined: Date{Raw: 234, Year: 2035}, u8: 10, transferIDs: []uint32{2003}}.block(slots...),
			flags: 0x430400208, joined: Date{Raw: 234, Year: 2035}, previous: 652, ids: []uint32{2003},
		},
		{
			name:  "a longer list that does not hold",
			block: append(falseLonger, bytes.Join(slots, nil)...),
			flags: 0x430400208, joined: Date{Raw: 259, Year: 2023}, previous: -1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 1998}, nation: 143}
			rec := playerRecordWith(77, 9, basic, tt.block)
			p, err := parsePlayer(rec, Record{Tag: 2, Index: 77, UID: 9, Flags: tt.flags, Start: 0, End: len(rec)}, testNames)
			if err != nil {
				t.Fatal(err)
			}
			if p.Contract == nil || p.Contract.TeamIndex != 652 {
				t.Fatalf("Contract = %+v, want one with team 652", p.Contract)
			}
			if p.JoinedClubPerson != tt.joined {
				t.Errorf("JoinedClubPerson = %+v, want %+v", p.JoinedClubPerson, tt.joined)
			}
			if p.PreviousClub != tt.previous {
				t.Errorf("PreviousClub = %d, want %d", p.PreviousClub, tt.previous)
			}
			if !reflect.DeepEqual(p.TransferIDs, tt.ids) {
				t.Errorf("TransferIDs = %v, want %v", p.TransferIDs, tt.ids)
			}
			if p.Retiring != tt.retiring || p.OnlyOneClub != tt.onlyOne {
				t.Errorf("Retiring %v, OnlyOneClub %v; want %v, %v", p.Retiring, p.OnlyOneClub, tt.retiring, tt.onlyOne)
			}
		})
	}
}

// injuryList encodes the player part's list of current injuries.
func injuryList(injuries ...Injury) []byte {
	b := []byte{byte(len(injuries))} //nolint:gosec // small test data
	for _, in := range injuries {
		b = append(b, injuryItem(in)...)
	}
	return b
}

// injuryItem encodes one current injury with its match records.
func injuryItem(in Injury, matches ...[]byte) []byte {
	b := []byte{2}
	b = binary.LittleEndian.AppendUint16(b, in.ID)
	b = append(b, byte(len(matches))) //nolint:gosec // small test data
	for _, m := range matches {
		b = append(b, m...)
	}
	b = appendDate(b, in.Start)
	b = binary.LittleEndian.AppendUint16(b, in.DaysMin)
	b = binary.LittleEndian.AppendUint16(b, in.DaysMax)
	b = append(b, 0, 1, 0)
	b = le32(b, -1)
	b = append(b, 0)
	b = le32(b, 547)
	b = append(b, 0)
	b = le32(b, -1)
	return append(b, 0, 0, 0, 0xff, 0xff)
}

// veigaMatch is the match record of Danilo Veiga's bruised ankle (26.3.2
// save, 2035-12-02).
var veigaMatch = []byte{
	0x01, 0x50, 0x5d, 0xf3, 0x07, 0x20, 0x01, 0x00, 0x00, 0x02, 0x00, 0x8c, 0x02,
	0x00, 0x00, 0x00, 0x9e, 0x02, 0x00, 0x00, 0x03, 0x01, 0x6e, 0x0d, 0x03, 0x00,
}

func TestParsePlayerReadsInjuriesWithMatchRecords(t *testing.T) {
	knock := Injury{ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}
	acl := Injury{ID: 27, Start: Date{Raw: 188, Year: 2025}, DaysMin: 267, DaysMax: 297}
	block := append([]byte{1, 2}, injuryItem(knock, veigaMatch)...)
	block = append(block, injuryItem(acl)...)

	p := parseWithBlock(t, block)

	if !reflect.DeepEqual(p.Injuries, []Injury{knock, acl}) {
		t.Errorf("Injuries = %+v, want [%+v %+v]", p.Injuries, knock, acl)
	}
}

func TestParsePlayerReadsInjuryWithBothRefsSet(t *testing.T) {
	// Person 46920 of a 26.3.2 save: a moderate calf muscle strain from
	// 2035-10-09, with a set i32 after the u16 and 0 (not -1) in the i32
	// after the team.
	item := []byte{
		0x02, 0x5d, 0x00, 0x01,
		0x01, 0x1a, 0x4b, 0xf3, 0x07, 0x51, 0x02, 0x00, 0x00, 0x01, 0x00, 0x07, 0x0b,
		0x00, 0x00, 0x00, 0x19, 0x01, 0x00, 0x00, 0x01, 0x02, 0x6c, 0xec, 0x02, 0x00,
		0x1a, 0x59, 0xf3, 0x07, 0x18, 0x00, 0x3c, 0x00, 0x04, 0x01, 0x00, 0x99, 0xf8, 0x02, 0x00,
		0x00, 0x19, 0x02, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff,
	}

	p := parseWithBlock(t, append([]byte{1}, item...))

	want := []Injury{{ID: 93, Start: Date{Raw: 0x591a, Year: 2035}, DaysMin: 24, DaysMax: 60}}
	if !reflect.DeepEqual(p.Injuries, want) {
		t.Errorf("Injuries = %+v, want %+v", p.Injuries, want)
	}
}

func TestParsePlayerRejectsBadMatchRecords(t *testing.T) {
	knock := Injury{ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}
	bad := append([]byte{7}, veigaMatch[1:]...) // match records start with 1

	p := parseWithBlock(t, append([]byte{1}, injuryItem(knock, bad)...))

	if len(p.Injuries) != 0 {
		t.Errorf("Injuries = %+v, want none", p.Injuries)
	}
}

func TestParsePlayerRejectsTooManyMatchRecords(t *testing.T) {
	knock := Injury{ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}
	matches := make([][]byte, maxMatchRecords+1)
	for i := range matches {
		matches[i] = veigaMatch
	}

	p := parseWithBlock(t, append([]byte{1}, injuryItem(knock, matches...)...))

	if len(p.Injuries) != 0 {
		t.Errorf("Injuries = %+v, want none", p.Injuries)
	}
}

func TestFindInjuriesStopsAtTruncatedMatchRecord(t *testing.T) {
	knock := Injury{ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}
	b := append([]byte{1}, injuryItem(knock, veigaMatch)...)
	record := 1 + injuryHeadSize
	for to := record + 1; to < record+matchRecordSize; to++ {
		if got := findInjuries(b[:to], 0, to); got != nil {
			t.Errorf("region ending %d bytes into the match record: Injuries = %+v, want none", to-record, got)
		}
	}
}

func TestParsePlayerReadsInjuries(t *testing.T) {
	acl := Injury{ID: 27, Start: Date{Raw: 188, Year: 2025}, DaysMin: 267, DaysMax: 297}
	main := testContract{person: 77, team: 547, wage: 147000, start: Date{Raw: 179, Year: 2023}, end: Date{Raw: 181, Year: 2028}, signed: Date{Raw: 179, Year: 2023}}
	block := append([]byte{0x11, 0x02}, injuryList(acl)...) // a stray 2 before the list
	block = append(block, contractBlock(fullSlot(main), emptySlot(), emptySlot(), emptySlot(), emptySlot())...)

	p := parseWithBlock(t, block)

	if len(p.Injuries) != 1 || p.Injuries[0] != acl {
		t.Errorf("Injuries = %+v, want [%+v]", p.Injuries, acl)
	}
	if p.Contract == nil || p.Contract.WeeklyWage != 147000 {
		t.Errorf("Contract = %+v", p.Contract)
	}
}

func TestParsePlayersSkipsBadRecords(t *testing.T) {
	good := playerRecord(0, 7, basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 1998}, nation: 139})
	bad := append([]byte{2}, make([]byte, 140)...)
	bad = append(bad, encodePersonTrailer(0x400400000, 1, 8)...)
	staff := append([]byte{1, 0, 0}, encodePersonTrailer(0x400400000, 2, 9)...)
	body := append(append(append([]byte{}, good...), bad...), staff...)
	recs := []Record{
		{Tag: 2, Index: 0, UID: 7, Start: 0, End: len(good)},
		{Tag: 2, Index: 1, UID: 8, Start: len(good), End: len(good) + len(bad)},
		{Tag: 1, Index: 2, UID: 9, Start: len(good) + len(bad), End: len(body)},
	}

	players, skipped, err := parsePlayers(body, recs, testNames)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 || players[0].UID != 7 || skipped != 1 {
		t.Errorf("got %d players (skipped %d), want 1 (skipped 1)", len(players), skipped)
	}

	if _, _, err := parsePlayers(body, recs[1:2], testNames); err == nil {
		t.Error("parsePlayers succeeded when every player failed")
	}
}

func TestGKPositionNamesTheGoalkeeperRating(t *testing.T) {
	if PositionNames[GKPosition] != "GK" {
		t.Errorf("PositionNames[GKPosition] = %q", PositionNames[GKPosition])
	}
}

// ringItem encodes a 29-byte contract ring item.
func ringItem(typ byte, id uint16, status int8, date, closed Date, u14 uint16) []byte {
	b := []byte{7, typ}
	b = binary.LittleEndian.AppendUint16(b, id)
	b = append(b, byte(status)) //nolint:gosec // two's complement intended
	b = append(b, make([]byte, 8)...)
	b = appendDate(appendDate(b, date), closed)
	b = binary.LittleEndian.AppendUint16(b, u14)
	return append(b, make([]byte, 29-len(b))...)
}

func TestCoreEvents(t *testing.T) {
	unset := Date{Raw: 1, Year: 1900}
	open := ringItem(12, 650, 3, Date{Raw: 300, Year: 2035}, unset, 0x1234)
	closed := ringItem(37, 9, -2, Date{Raw: 20, Year: 2033}, Date{Raw: 100, Year: 2033}, 7)
	main := testContract{person: 77, team: 663, wage: 1, start: Date{Raw: 240, Year: 2023}, end: Date{Raw: 181, Year: 2028}, signed: Date{Raw: 240, Year: 2023}, playingTime: [3]byte{3, 2, 2}, items: [][]byte{open, closed}, kind: 2, coreFlags: 0x09}

	p := parseWithBlock(t, contractBlock(fullSlot(main), emptySlot(), emptySlot(), emptySlot(), emptySlot()))

	c := p.Contract
	if c == nil || !c.Full || c.Kind != 2 || c.CoreFlags != 0x09 || len(c.Events) != 2 {
		t.Fatalf("Contract = %+v, want full, kind 2, flags 0x09, 2 events", c)
	}
	want := []ContractEvent{
		{Type: 12, ID: 650, Status: 3, Date: Date{Raw: 300, Year: 2035}, Closed: unset, U14: 0x1234},
		{Type: 37, ID: 9, Status: -2, Date: Date{Raw: 20, Year: 2033}, Closed: Date{Raw: 100, Year: 2033}, U14: 7},
	}
	if !reflect.DeepEqual(c.Events, want) {
		t.Errorf("Events = %+v, want %+v", c.Events, want)
	}
	if !c.Events[0].Open() || c.Events[1].Open() {
		t.Errorf("Open = %v, %v; want true, false", c.Events[0].Open(), c.Events[1].Open())
	}
}
