package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"iter"
)

// tableEnd starts the 9-byte marker that follows every game_db table
// (written by 0x33e9ea0): u32 0, u8 1, u32.
var tableEnd = []byte{0, 0, 0, 0, 1}

const (
	tableEndSize = 9

	noAgeLimit       = 100
	unsetAgeLimit    = 255
	teamTypeReserves = 1
	teamTypeB        = 3
	teamTypeSecond   = 16
	// minRecordSize is a lower bound on any club or team record (each ends
	// with a database record).
	minRecordSize = databaseRecordSize

	tagClub            = 0
	tagLeagueBodyClub  = 12
	tagTeam            = 10
	tagNationalTeam    = 11
	finTypeBase        = 0
	finTypeFull        = 1
	finTypeNone        = 2
	maxTeamPlayers     = 10_000
	nationalTeamGateA  = 0xe94 // data version from which national teams carry one more byte
	nationalTeamGateB  = 0xe93 // … and one more u32
	financeBaseSize    = 45
	financeSubItemSize = 230
)

// Club is a CLUB or LEAGUE_BODY_CLUB record (serializer 0x308b1a0). Men's
// and women's sides of the same club are separate records.
type Club struct {
	Index           uint32
	UID             uint32
	Name            string // as shown in game; may be a licensed-out fake name
	ShortName       string
	NationIndex     int32
	Women           bool
	Teams           []int32  // TEAM indices of this club's own teams
	AffiliatedTeams []int32  // first teams of other clubs, e.g. a B club
	Balance         int32    // bank balance in GBP (verified in game)
	Budgets         *Budgets // nil without full finances (CLUB_FINANCE)
}

// Budgets are a club's budgets from CLUB_FINANCE (0x3118c7c), in GBP.
type Budgets struct {
	Transfer   int32 // +0x7cc: the current transfer budget; may be negative
	WeeklyWage int32 // +0x810: the total wage budget per week
}

// Team is a TEAM or NATIONAL_TEAM record (serializers 0x394e254 and
// 0x35dc620). Player records point at teams, not clubs.
type Team struct {
	Index        uint32
	UID          uint32 // a first team shares its club's uid
	National     bool
	Container    int32 // CLUB index, or national-team container index (= nation index for men's teams)
	Reputation   uint16
	LastPosition uint8 // guess: last league position
	Division     int32 // COMP index
	Stadium      int32
	Manager      int32   // person index
	Continental  int32   // COMP index of the continental competition
	Players      []int32 // person indices: a national team's called-up squad; a club team's players and most of its loanees out
	Captain      int32   // guess
	ViceCaptain  int32   // guess
	TeamType     uint8   // guess: 0 first team, 1 reserves, 3 B team, 10 U21, 12 U18, 16 second team
	AgeLimit     uint8   // 100 or 255 = none, else U<n>
	Name         string
	ShortName    string
}

// readClubTable parses the CLUB table holding count records and returns it
// with the offset of its end marker.
func readClubTable(body []byte, count uint32) ([]Club, int, error) {
	return readTable(body, count, []byte{tagClub, tagLeagueBodyClub}, "CLUB", readClub)
}

// readTeamTable parses the TEAM table holding count records. dataVersion is
// the game_db header's database version.
func readTeamTable(body []byte, count uint32, dataVersion uint16) ([]Team, int, error) {
	return readTable(body, count, []byte{tagNationalTeam, tagTeam}, "TEAM", func(r *reader) (Team, uint32) {
		return readTeam(r, dataVersion)
	})
}

// readTable finds a table of count records by its head (u32 count, u8 0,
// first record tag) right after another table's end marker, and accepts it
// only when every record parses with index 0..count-1 and an end marker
// follows the last one.
func readTable[T any](body []byte, count uint32, firstTags []byte, name string, parse func(*reader) (T, uint32)) ([]T, int, error) {
	for _, tag := range firstTags {
		head := tableHead(count, tag)
		for p := range tableHeads(body, head) {
			// The first record starts at its tag, the head's last byte.
			if recs, end, ok := parseTable(body, p+len(head)-1, count, parse); ok {
				return recs, end, nil
			}
		}
	}
	return nil, 0, fmt.Errorf("%w: %s table with %d records", ErrNotFound, name, count)
}

// tableHead is the start of a table: u32 count, u8 0, the first record's tag.
func tableHead(count uint32, tag byte) []byte {
	return append(binary.LittleEndian.AppendUint32(nil, count), 0, tag)
}

// tableHeads yields the offsets of head in body that follow another table's
// end marker.
func tableHeads(body, head []byte) iter.Seq[int] {
	return func(yield func(int) bool) {
		for p := 0; ; p++ {
			i := bytes.Index(body[p:], head)
			if i < 0 {
				return
			}
			p += i
			if afterTableEnd(body, p) && !yield(p) {
				return
			}
		}
	}
}

// afterTableEnd reports whether offset p follows a table end marker.
func afterTableEnd(body []byte, p int) bool {
	return p >= tableEndSize && bytes.HasPrefix(body[p-tableEndSize:], tableEnd)
}

func parseTable[T any](body []byte, start int, count uint32, parse func(*reader) (T, uint32)) ([]T, int, bool) {
	r := newReader(body)
	r.off = start
	recs := make([]T, 0, capacity(count, len(body)-start, minRecordSize))
	for i := range count {
		rec, index := parse(r)
		if r.err != nil || index != i {
			return nil, 0, false
		}
		recs = append(recs, rec)
	}
	if !bytes.HasPrefix(body[r.off:], tableEnd) {
		return nil, 0, false
	}
	return recs, r.off, true
}

func readClub(r *reader) (Club, uint32) {
	var c Club
	switch tag := r.u8(); tag {
	case tagClub:
	case tagLeagueBodyClub:
		r.u32()
	default:
		r.fail("club tag %d", tag)
	}
	c.NationIndex = r.i32()
	r.take(4 * 4) // refs; the last is a city
	r.u32()
	r.take(2)
	c.Name = r.str()
	c.ShortName = r.str()
	r.take(1 + 2 + 4 + 1)
	r.refs8()
	switch fin := r.u8(); fin {
	case finTypeBase:
		c.Balance = readFinanceBase(r)
	case finTypeFull:
		c.Budgets, c.Balance = readFullFinance(r)
	case finTypeNone:
	default:
		r.fail("finance type %d", fin)
	}
	r.take(25 * int(r.u8()))
	r.take(12 * int(r.u8()))
	r.take(2 + 4)
	c.Women, c.Teams, c.AffiliatedTeams = readTeamContainer(r)
	c.Index, c.UID = readDatabaseRecord(r)
	return c, c.Index
}

// readFinanceBase reads CLUB_FINANCE_BASE (0x3116a18) and returns its first
// field, the bank balance.
func readFinanceBase(r *reader) int32 {
	balance := r.i32()
	r.take(financeBaseSize - 4)
	return balance
}

// readFullFinance reads CLUB_FINANCE (0x3118c7c), which only active clubs
// have, and returns its budgets and the bank balance.
func readFullFinance(r *reader) (*Budgets, int32) {
	b := &Budgets{}
	r.take(4) // +0x7d0, probably the season's starting transfer budget
	b.Transfer = r.i32()
	r.take(4 + 1 + 9*4 + 4 + 1 + 4 + 4 + 4) // next season's transfer budget (+0x7d4), then fields not decoded
	b.WeeklyWage = r.i32()
	for range 4 {
		r.take(1 + 4 + financeSubItemSize)
		r.take(8 * int(r.u8()))
		r.take(8 * int(r.u8()))
	}
	r.take(24 + 2 + 8 + 5 + 8 + 4 + 16)
	r.take(financeSubItemSize * int(r.u8()))
	r.take(49 * int(r.u32()))
	r.take(4 + 1 + 20 + 8 + 16 + 2 + 20)
	r.take(19 * int(r.u8()))
	r.take(22)
	return b, readFinanceBase(r)
}

// readTeamContainer reads the TEAM_CONTAINER base (0x39a2730) up to its
// database record.
func readTeamContainer(r *reader) (women bool, teams, affiliated []int32) {
	r.u8()
	r.u8()
	women = r.u8() == 1
	teams = r.refs8()
	affiliated = r.refs8()
	for range 3 {
		r.refs8()
	}
	r.take(16 * int(r.u8()))
	r.take(28 * int(r.u16()))
	return women, teams, affiliated
}

// readDatabaseRecord reads the index/uid block that ends every record.
func readDatabaseRecord(r *reader) (index, uid uint32) {
	index, uid = r.u32(), r.u32()
	r.u32()
	return index, uid
}

func readTeam(r *reader, dataVersion uint16) (Team, uint32) {
	var t Team
	switch tag := r.u8(); tag {
	case tagNationalTeam:
		t.National = true
		r.take(12 + 1 + 4 + 24 + 7)
		if dataVersion >= nationalTeamGateA {
			r.take(1)
		}
		if dataVersion >= nationalTeamGateB {
			r.take(4)
		}
		r.take(10 * int(r.u8()))
	case tagTeam:
	default:
		r.fail("team tag %d", tag)
	}
	r.u8()
	t.LastPosition = r.u8()
	t.Reputation = r.u16()
	t.Division = r.i32()
	r.take(8)
	t.Stadium = r.i32()
	t.Manager = r.i32()
	t.Continental = r.i32()
	r.take(1 + 4)
	n := int(r.u16())
	if n > maxTeamPlayers {
		r.fail("%d players in a team", n)
	}
	for range n {
		t.Players = append(t.Players, r.i32())
	}
	t.Captain = r.i32()
	t.ViceCaptain = r.i32()
	r.take(5 + 4 + 1)
	t.TeamType = r.u8()
	r.u8()
	r.u8()
	t.AgeLimit = r.u8()
	r.take(2)
	t.Container = r.i32()
	t.Name = r.str()
	r.u8()
	t.ShortName = r.str()
	r.u8()
	t.Index, t.UID = readDatabaseRecord(r)
	return t, t.Index
}

// Label names a non-first team: its age limit ("U21") or its type
// (guesses: 1 reserves, 3 B team, 16 second team). First teams get "".
func (t Team) Label() string {
	if t.AgeLimit != noAgeLimit && t.AgeLimit != unsetAgeLimit {
		return fmt.Sprintf("U%d", t.AgeLimit)
	}
	switch t.TeamType {
	case teamTypeReserves:
		return "Reserves"
	case teamTypeB:
		return "B"
	case teamTypeSecond:
		return "II"
	}
	return ""
}
