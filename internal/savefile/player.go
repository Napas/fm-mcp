package savefile

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

const (
	tagPlayer      = 2
	tagPlayerCoach = 3
	// flagFemale is set in a person's trailer flags for women (verified on
	// every club player of the 26.2.0 test save against the club's women's flag).
	flagFemale = 0x1000
	// flagBit3: meaning unknown; the personality rule (0x36b906c) reads it
	// (docs/format.md, "Person table").
	flagBit3 = 0x8
	// flagOnlyOneClub and flagRetiring make the transfer-interest check
	// refuse every club (0x1285498, 0x1283a90; docs/format.md, "Person table").
	flagOnlyOneClub = 1 << 15
	flagRetiring    = 1 << 18

	// Offsets of the fixed player part from the record start, written by
	// ACTUAL_PLAYER's serializer (0x2f9c868 in game_plugin 26.3.2).
	offReputations      = 1   // 3 × u16
	offCurrentAbility   = 7   // i16
	offPotentialAbility = 9   // i16
	offNationalTeamRef  = 19  // i32
	offClub             = 23  // i64
	offPositions        = 31  // 15 × u8
	offAttributes       = 46  // 54 × u8
	offValue            = 100 // i32
	offJoinedClub       = 105 // date
	offSharpness        = 113 // u16
	offCondition        = 117 // u16
	offHeight           = 128 // i16
	playerPartSize      = 130 // fixed fields read here

	// The basic person part (names, birth date, nation) is found by scanning;
	// the person part before it is not decoded.
	basicFixedSize = 19                // three (u32 ref + u8 flag) + u32 full-name length
	basicTailSize  = 4 + 9 + 8 + 8 + 8 // date, unknown, nation, hidden attributes, birthplace
	traitsSize     = 8                 // u64 trait mask ending the person part
	minBirthYear   = 1901              // dummy name structs carry 1900
	maxBirthYear   = 2199

	// The relationship vector follows the birthplace (BASIC writer 0x2f85600,
	// vector writer 0x36e0d0c): u8 present, then u8 n (the game writes at
	// most 125) and n 16-byte items (0x37bff84).
	maxRelations     = 125
	relationItemSize = 16
	relationClub     = 1 // item +10, object kind: a CLUB table index
	relationFavoured = 3 // item +11, relation type: favoured club (getter 0x2f92e50)
)

// PositionNames labels Player.Positions. Verified in game_plugin 26.3.2:
// the string-keyed loader at 0x48c6228 and the side-flag table at
// 0x51f9658 (SW only by elimination).
var PositionNames = [15]string{"GK", "SW", "DL", "DC", "DR", "DM", "ML", "MC", "MR", "AML", "AMC", "AMR", "ST", "WBL", "WBR"}

// GKPosition is the index of the goalkeeper rating in Player.Positions.
const GKPosition = 0

// AttributeNames labels Player.Attributes. Verified in game_plugin 26.3.2
// (the string-keyed loader at 0x48c6228, the display table at 0x520380c and
// the foot tie-break in 0x48b97c4). "OffTheBall" is "Movement" in the engine.
var AttributeNames = [54]string{
	"Crossing", "Dribbling", "Finishing", "Heading", "LongShots", "Marking", "OffTheBall", "Passing", "Penalties", "Tackling",
	"Vision", "Handling", "AerialReach", "CommandOfArea", "Communication", "Kicking", "Throwing", "Anticipation", "Decisions", "OneOnOnes",
	"Positioning", "Reflexes", "FirstTouch", "Technique", "LeftFoot", "RightFoot", "Flair", "Corners", "Teamwork", "WorkRate",
	"LongThrows", "Eccentricity", "RushingOut", "TendencyToPunch", "Acceleration", "FreeKicks", "Strength", "Stamina", "Pace", "JumpingReach",
	"Leadership", "Dirtiness", "Balance", "Bravery", "Consistency", "Aggression", "Agility", "ImportantMatches", "InjuryProneness", "Versatility",
	"NaturalFitness", "Determination", "Composure", "Concentration",
}

// HiddenAttributeNames labels Player.Hidden. Verified in game_plugin 26.3.2:
// the string-keyed loader 0x2f89dac stores them at person +0x40…+0x47 in
// this order, and the BASIC writer 0x2f85600 writes those 8 bytes.
var HiddenAttributeNames = [8]string{"Adaptability", "Ambition", "Loyalty", "Pressure", "Professionalism", "Sportsmanship", "Temperament", "Controversy"}

// TraitNames labels the bits of Player.Traits (FM's player traits), from the
// formatter 0x370e010 and the English UI text. Bits 48 and 53 have no label.
var TraitNames = [64]string{
	"Runs With Ball Down Left", "Runs With Ball Down Right", "Runs With Ball Through Centre", "Gets Into Opposition Area",
	"Moves Into Channels", "Gets Forward Whenever Possible", "Plays Short Simple Passes", "Tries Killer Balls Often",
	"Shoots From Distance", "Shoots With Power", "Places Shots", "Curls Ball",
	"Likes To Round Keeper", "Likes To Try To Beat Offside Trap", "Uses Outside Of Foot", "Marks Opponent Tightly",
	"Winds Up Opponents", "Argues With Officials", "Plays With Back To Goal", "Comes Deep To Get Ball",
	"Plays One-Twos", "Likes To Lob Keeper", "Dictates Tempo", "Attempts Overhead Kicks",
	"Looks For Pass Rather Than Attempting To Score", "Plays No Through Balls", "Stops Play", "Knocks Ball Past Opponent",
	"Moves Ball To Right Foot Before Dribble Attempt", "Moves Ball To Left Foot Before Dribble Attempt", "Dwells On Ball", "Arrives Late In Opponents' Area",
	"Tries To Play Way Out Of Trouble", "Stays Back At All Times", "Avoids Using Weaker Foot", "Tries Tricks",
	"Tries Long Range Free Kicks", "Dives Into Tackles", "Does Not Dive Into Tackles", "Cuts Inside From Both Wings",
	"Hugs Line", "Gets Crowd Going", "Tries First Time Shots", "Tries Long Range Passes",
	"Likes Ball Played Into Feet", "Hits Free Kicks With Power", "Likes To Beat Opponent Repeatedly", "Likes To Switch Ball To Wide Areas",
	"", "Play Football For as Long as Possible", "Possesses Long Flat Throw", "Runs With Ball Often",
	"Runs With Ball Rarely", "", "Does not Move Into Channels", "Uses Long Throw To Start Counter Attacks",
	"Refrains From Taking Long Shots", "Cuts Inside From Left Wing", "Cuts Inside From Right Wing", "Crosses Early",
	"Brings Ball Out of Defence", "Demands to Take Free Kicks", "Demands to Take Penalties", "Plays Ball With Feet",
}

// ClubAffinity is a person's affinity for a club (relation type 3 to a
// club). The game lists values of 50 or more as favoured clubs; a negative
// Value is a disliked club (hypothesis).
type ClubAffinity struct {
	Club  int64 // CLUB table index
	Value int8  // -50..100 in the saves seen; the personality rule's Devoted needs > 99
	// Reason is item +8: 1 supporter, 2 as a player; others unlabelled.
	Reason uint8
}

// ReasonName names Reason as the profile's "As a supporter" and "As a
// player" (in-game samples); false for other values.
func (c ClubAffinity) ReasonName() (string, bool) {
	switch c.Reason {
	case 1:
		return "supporter", true
	case 2:
		return "player", true
	}
	return "", false
}

// Player is a decoded player record from game_db.dat. Fields marked "guess"
// are inferred from values, not from code.
type Player struct {
	Index       uint32
	UID         uint32
	PlayerCoach bool
	Female      bool
	FlagBit3    bool // trailer flags bit 3 (see flagBit3)
	// Retiring (trailer flags bit 18) and OnlyOneClub (bit 15) make the
	// game's transfer-interest check refuse every club (0x1283a90, 0x1285498).
	Retiring    bool
	OnlyOneClub bool

	FirstName  string
	LastName   string
	CommonName string // set for players known by one name
	FullName   string // often empty
	BirthDate  Date

	NationIndex     int64 // index into the nation table; names are not stored in the save
	BirthplaceIndex int64 // guess: city index
	TeamIndex       int64 // index into the team table; -1 for none
	NationalTeamRef int32 // team index of the national squad he is called up to; -1 for none

	// Reputations, 0..10000; verified by code: the game's editor field codes
	// "hr", "cr" and "wr" name them (setters at 0x2f2c434…).
	HomeReputation    uint16
	CurrentReputation uint16
	WorldReputation   uint16
	CurrentAbility    int16 // 1..200
	PotentialAbility  int16 // 1..200

	Positions  [15]uint8 // 1..20, labelled by PositionNames
	Attributes [54]uint8 // 1..100; shown in game as clamp((v+2)/5, 1, 20)
	Hidden     [8]uint8  // 1..20, labelled by HiddenAttributeNames
	Traits     uint64    // bit i set: the player has TraitNames[i]

	Value      int32  // guess (strong): GBP; the game caps it at 300 000 000
	JoinedClub Date   // date joined the current club; 1900 when unset
	Sharpness  uint16 // guess: match sharpness 0..10000
	Condition  uint16 // guess (strong): condition 0..10000
	Height     int16  // cm

	// ACTUAL_PERSON fields before the contracts marker; unset when the
	// marker is not found.
	JoinedClubPerson Date     // +0xec: joined the current club; 1900 when unset, set for loanees too
	PreviousClub     int64    // +0xd8: CLUB index of the club he left; -1 for none
	TransferIDs      []uint32 // +0x68: TRANSFER ids linked to him; nil when none

	Contract       *Contract // nil for free agents
	Loan           *Contract // set while on loan; TeamIndex is the loan team
	FutureContract *Contract // a pre-agreed contract that starts later
	Injuries       []Injury
	ClubAffinities []ClubAffinity // in save order; nil when the vector is absent or does not decode
}

func parsePlayer(body []byte, rec Record, nt NameTables) (Player, error) {
	b := body[rec.Start:rec.End]
	if len(b) < playerPartSize+personTrailerSize {
		return Player{}, fmt.Errorf("%w: player %d is %d bytes", ErrCorrupt, rec.Index, len(b))
	}
	p := Player{
		Index:             rec.Index,
		UID:               rec.UID,
		PlayerCoach:       rec.Tag == tagPlayerCoach,
		Female:            rec.Flags&flagFemale != 0,
		FlagBit3:          rec.Flags&flagBit3 != 0,
		Retiring:          rec.Flags&flagRetiring != 0,
		OnlyOneClub:       rec.Flags&flagOnlyOneClub != 0,
		HomeReputation:    binary.LittleEndian.Uint16(b[offReputations:]),
		CurrentReputation: binary.LittleEndian.Uint16(b[offReputations+2:]),
		WorldReputation:   binary.LittleEndian.Uint16(b[offReputations+4:]),
		CurrentAbility:    int16(binary.LittleEndian.Uint16(b[offCurrentAbility:])),   //nolint:gosec // signed field
		PotentialAbility:  int16(binary.LittleEndian.Uint16(b[offPotentialAbility:])), //nolint:gosec // signed field
		NationalTeamRef:   int32(binary.LittleEndian.Uint32(b[offNationalTeamRef:])),  //nolint:gosec // signed field
		TeamIndex:         int64(binary.LittleEndian.Uint64(b[offClub:])),             //nolint:gosec // signed field
	}
	p.Value = int32(binary.LittleEndian.Uint32(b[offValue:])) //nolint:gosec // signed field
	p.JoinedClub = dateAt(b, offJoinedClub)
	p.Sharpness = binary.LittleEndian.Uint16(b[offSharpness:])
	p.Condition = binary.LittleEndian.Uint16(b[offCondition:])
	p.Height = int16(binary.LittleEndian.Uint16(b[offHeight:])) //nolint:gosec // signed field
	copy(p.Positions[:], b[offPositions:])
	copy(p.Attributes[:], b[offAttributes:])

	person := b[:len(b)-personTrailerSize]
	at, ok := findBasicPart(person, nt)
	if !ok {
		return Player{}, fmt.Errorf("%w: names and birth date of player %d", ErrNotFound, rec.Index)
	}
	injuriesEnd := at
	p.PreviousClub = -1
	if cs, m, ok := readContracts(b, playerPartSize, at, rec.Index); ok {
		p.Contract, p.Loan, p.FutureContract = cs.main, cs.loan, cs.future
		p.JoinedClubPerson, p.PreviousClub, p.TransferIDs = readPersonFields(b, m)
		injuriesEnd = m.at
	}
	p.Injuries = findInjuries(b, offValue, injuriesEnd)
	// The ACTUAL_PERSON writer (0x2f424a0) ends with the trait mask.
	if at >= playerPartSize+traitsSize {
		p.Traits = binary.LittleEndian.Uint64(b[at-traitsSize:])
	}
	r := newReader(person)
	r.off = at
	p.FirstName = nt.first(r.u32())
	r.u8()
	p.LastName = nt.last(r.u32())
	r.u8()
	p.CommonName = nt.common(r.u32())
	r.u8()
	p.FullName = r.str()
	p.BirthDate = r.date()
	r.take(9)
	p.NationIndex = r.i64()
	copy(p.Hidden[:], r.take(8))
	p.BirthplaceIndex = r.i64()
	p.ClubAffinities = clubAffinities(person[r.off:])
	return p, r.err
}

// clubAffinities reads the relationship vector at the start of b and keeps
// the club-affinity items. The vector is optional data, so a bad one gives
// nil rather than failing the player; the rest of the vector is not read.
func clubAffinities(b []byte) []ClubAffinity {
	if len(b) < 2 || b[0] != 1 {
		return nil
	}
	n := int(b[1])
	if n > maxRelations || 2+n*relationItemSize > len(b) {
		return nil
	}
	var out []ClubAffinity
	for i := range n {
		it := b[2+i*relationItemSize:]
		if it[10] != relationClub || it[11] != relationFavoured {
			continue
		}
		out = append(out, ClubAffinity{
			Club:   int64(binary.LittleEndian.Uint64(it)), //nolint:gosec // signed ref
			Value:  int8(it[12]),                          //nolint:gosec // signed field
			Reason: it[8],
		})
	}
	return out
}

// findBasicPart returns the offset in b of the last plausible basic person
// part: three name references (each a u32 within its table or unset,
// followed by a zero flag byte), a short UTF-8 full name and a birth date.
// The undecoded person part before it holds a dummy copy dated 1900, so the
// last match wins.
func findBasicPart(b []byte, nt NameTables) (int, bool) {
	// Common-name refs of generated players can exceed the common-name
	// table (record 83559 in the test save; up to 646431 in a 26.3.2 save),
	// so they are bounded by the largest name table.
	maxCommon := max(len(nt.First), len(nt.Last), len(nt.Common))
	for p := len(b) - basicFixedSize - basicTailSize; p >= 1; p-- {
		first := binary.LittleEndian.Uint32(b[p:])
		last := binary.LittleEndian.Uint32(b[p+5:])
		common := binary.LittleEndian.Uint32(b[p+10:])
		if b[p+4] != 0 || b[p+9] != 0 || b[p+14] != 0 {
			continue
		}
		if !validNameRef(first, len(nt.First)) || !validNameRef(last, len(nt.Last)) || !validNameRef(common, maxCommon) {
			continue
		}
		if first == noName && last == noName && common == noName {
			continue
		}
		n := int(binary.LittleEndian.Uint32(b[p+15:]))
		q := p + basicFixedSize + n
		if n > maxNameLen || q+basicTailSize > len(b) || !utf8.Valid(b[p+basicFixedSize:q]) {
			continue
		}
		doy := binary.LittleEndian.Uint16(b[q:])
		year := binary.LittleEndian.Uint16(b[q+2:])
		if doy >= 1 && doy <= 366 && year >= minBirthYear && year <= maxBirthYear {
			return p, true
		}
	}
	return 0, false
}

// parsePlayers decodes the player and player-coach records. Records that do
// not decode are skipped and counted; it fails only when most do.
func parsePlayers(body []byte, recs []Record, nt NameTables) ([]Player, int, error) {
	var players []Player
	var skipped int
	var firstErr error
	for _, rec := range recs {
		if rec.Tag != tagPlayer && rec.Tag != tagPlayerCoach {
			continue
		}
		p, err := parsePlayer(body, rec, nt)
		if err != nil {
			skipped++
			firstErr = cmp.Or(firstErr, err)
			continue
		}
		players = append(players, p)
	}
	if skipped > len(players) {
		return nil, 0, fmt.Errorf("%d of %d players failed to decode: %w", skipped, skipped+len(players), firstErr)
	}
	return players, skipped, nil
}
