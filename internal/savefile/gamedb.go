package savefile

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Date is the game's 4-byte date: a packed u16 and a u16 year.
type Date struct {
	Raw  uint16 // low 9 bits: 1-based day of year; upper 7 bits unknown
	Year uint16
}

func (r *reader) date() Date {
	return Date{Raw: r.u16(), Year: r.u16()}
}

func dateAt(b []byte, p int) Date {
	return Date{Raw: binary.LittleEndian.Uint16(b[p:]), Year: binary.LittleEndian.Uint16(b[p+2:])}
}

func (d Date) dayOfYear() uint16 { return d.Raw & 0x1ff }

// IsZero reports whether the date is unset (year 0).
func (d Date) IsZero() bool { return d.Year == 0 }

// plausible reports whether d has a day of year 1..366 and a year in
// minYear..maxYear. Scanners use it to validate candidate structures.
func (d Date) plausible(minYear, maxYear uint16) bool {
	doy := d.dayOfYear()
	return doy >= 1 && doy <= 366 && d.Year >= minYear && d.Year <= maxYear
}

// Time converts the date to midnight UTC. The day-of-year reading of Raw
// is a hypothesis that matches every date checked so far.
func (d Date) Time() time.Time {
	return time.Date(int(d.Year), time.January, int(d.dayOfYear()), 0, 0, 0, 0, time.UTC)
}

// GameDBHeader is the start of game_db.dat's body, written by the
// function at 0x32c8e44 in game_plugin 26.3.2.
type GameDBHeader struct {
	DatabaseVersion         uint16 // constant per build: 3727 in 26.2.0, 3733 in 26.3.2
	Const7                  uint16 // always 7
	Const0                  uint16 // always 0
	Unknown790              uint32 // UnknownNNN: meaning unknown; NNN is the writer's object offset
	OriginalDatabaseVersion uint16
	Unknown7C6              uint16
	Label                   string // "2620" in the test save
	Date7D8                 Date
	Unknown7DC              uint32
	Year                    uint16 // capped at 2025 by the writer
	Unknown7CA              uint16
	GameDate                Date // the in-game date
	Unknown830              uint32
	Flag80D                 uint8
	Flag80E                 uint8
	Unknown808              uint32
	Size                    int // header length; the next section starts here
}

// ParseGameDBHeader decodes the header at the start of game_db.dat's body.
func ParseGameDBHeader(body []byte) (GameDBHeader, error) {
	r := newReader(body)
	h := GameDBHeader{
		DatabaseVersion:         r.u16(),
		Const7:                  r.u16(),
		Const0:                  r.u16(),
		Unknown790:              r.u32(),
		OriginalDatabaseVersion: r.u16(),
		Unknown7C6:              r.u16(),
		Label:                   r.str(),
		Date7D8:                 r.date(),
		Unknown7DC:              r.u32(),
		Year:                    r.u16(),
		Unknown7CA:              r.u16(),
		GameDate:                r.date(),
		Unknown830:              r.u32(),
		Flag80D:                 r.u8(),
		Flag80E:                 r.u8(),
		Unknown808:              r.u32(),
	}
	if r.err != nil {
		return GameDBHeader{}, r.err
	}
	h.Size = r.off
	return h, nil
}

// verifiedDatabaseVersions are the game_db.dat database versions whose
// layouts were checked against real saves: 3727 (26.2.0) and 3733 (26.3.2).
var verifiedDatabaseVersions = map[uint16]bool{3727: true, 3733: true}

// VerifiedDatabaseVersion reports whether saves with this database version
// were checked. Other versions may still parse but can decode wrong values.
func VerifiedDatabaseVersion(v uint16) bool { return verifiedDatabaseVersions[v] }

// GameDB is the decoded part of game_db.dat.
type GameDB struct {
	Header          GameDBHeader
	Players         []Player    // every player and player-coach
	Clubs           []Club      // indexed by club index
	Teams           []Team      // indexed by team index; Player.TeamIndex is a team index
	CompetitionUIDs []uint32    // competition index → uid; names are in the language database
	NationCount     uint32      // nations in the save; names are in the language database
	InjuryCount     uint32      // injury types in the save; names are in the language database
	SkippedPlayers  int         // player records that did not decode
	Currencies      []Currency  // indexed by currency index
	CurrencyErr     error       // why Currencies is empty; the rest of the save still loads
	Continents      []Continent // indexed by continent index
	Nations         []Nation    // indexed by nation index
	NationsErr      error       // why Continents and Nations are empty; the rest of the save still loads
	// CompetitionKeys are the ACTUAL_COMP_ID records by key index; a
	// fixture's competition ref is a key index.
	CompetitionKeys    []CompetitionKey
	CompetitionKeysErr error    // why CompetitionKeys is empty
	StageUIDs          []uint32 // STAGE_NAME uids by stage index; names are in the language database
	StagesErr          error    // why StageUIDs is empty
	LeagueStageCount   uint32   // LEAGUE_STAGE objects in the memory pools
}

// ReadGameDB decodes the save's game_db.dat.
func ReadGameDB(a *Archive) (*GameDB, error) {
	f, err := a.ReadFile("game_db.dat")
	if err != nil {
		return nil, err
	}
	pools, err := readPoolCounts(a)
	if err != nil {
		return nil, err
	}
	h, err := ParseGameDBHeader(f.Body)
	if err != nil {
		return nil, fmt.Errorf("game_db.dat header: %w", err)
	}
	db := &GameDB{Header: h, NationCount: pools["NATION"], InjuryCount: pools["INJURY"]}
	db.Currencies, db.CurrencyErr = readCurrencies(f.Body, pools)
	db.Continents, db.Nations, db.NationsErr = readNationData(f.Body, pools)
	db.CompetitionKeys, db.CompetitionKeysErr = readCompetitionKeys(f.Body)
	db.StageUIDs, db.StagesErr = readStageNames(f.Body, pools[stageNamePool])
	db.LeagueStageCount = pools[leagueStagePool]

	clubCount, err := poolSum(pools, "CLUB", "LEAGUE_BODY_CLUB")
	if err != nil {
		return nil, err
	}
	teamCount, err := poolSum(pools, "TEAM", "NATIONAL_TEAM")
	if err != nil {
		return nil, err
	}
	clubs, clubsEnd, err := readClubTable(f.Body, clubCount)
	if err != nil {
		return nil, err
	}
	db.Clubs = clubs
	comps, _, err := splitTable(f.Body, clubsEnd+tableEndSize, competitionTrailer)
	if err != nil {
		return nil, fmt.Errorf("competition table: %w", err)
	}
	if uint32(len(comps)) != pools["COMP"] { //nolint:gosec // record counts fit in uint32
		return nil, fmt.Errorf("%w: %d competitions after the club table, memory pools say %d", ErrCorrupt, len(comps), pools["COMP"])
	}
	for _, c := range comps {
		db.CompetitionUIDs = append(db.CompetitionUIDs, c.UID)
	}
	if db.Teams, _, err = readTeamTable(f.Body, teamCount, h.DatabaseVersion); err != nil {
		return nil, err
	}

	nt, tableStart, err := findNameTables(f.Body[h.Size:], minFirstNames)
	if err != nil {
		return nil, err
	}
	recs, _, err := splitTable(f.Body, h.Size+tableStart, personTrailer)
	if err != nil {
		return nil, err
	}
	if db.Players, db.SkippedPlayers, err = parsePlayers(f.Body, recs, nt); err != nil {
		return nil, err
	}
	return db, nil
}

// FirstTeam returns the index of the club team whose uid is clubUID; a
// club's first team shares its uid (verified for every first team in
// 26.2.0 and 26.3.2 saves).
func (db *GameDB) FirstTeam(clubUID uint32) (int64, bool) {
	for _, t := range db.Teams {
		if !t.National && t.UID == clubUID {
			return int64(t.Index), true
		}
	}
	return 0, false
}
