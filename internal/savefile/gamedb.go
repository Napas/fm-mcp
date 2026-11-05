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

// UnsetYear is the year of unset dates, e.g. 1900-01-01 joined dates and
// 1900-01-02 contract ends.
const UnsetYear = 1900

func (d Date) before(o Date) bool {
	return d.Year < o.Year || d.Year == o.Year && d.dayOfYear() < o.dayOfYear()
}

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
	DatabaseVersion         uint16 // constant per build: see versions.go
	Const7                  uint16 // always 7
	Const0                  uint16 // always 0
	Unknown790              uint32 // UnknownNNN: meaning unknown; NNN is the writer's object offset
	OriginalDatabaseVersion uint16
	Unknown7C6              uint16
	Label                   string // "2620" in the test save
	Date7D8                 Date
	Unknown7DC              uint32
	Timestamp               uint64 // ms since 0001-01-01 (inference); 0 before version 3754
	Year                    uint16 // capped by the writer at 2025 in 26.x; 2026 seen in 27.0.0 (cap unknown)
	Unknown7CA              uint16
	GameDate                Date // the in-game date
	Unknown830              uint32
	Flag80D                 uint8
	Flag80E                 uint8
	Unknown808              uint32
	Size                    int // header length; the next section starts here
}

// headerTimestampGate is the first database version seen with the u64
// timestamp; the game's own gate may be lower.
const headerTimestampGate = dbVersion27_0

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
	}
	if h.DatabaseVersion >= headerTimestampGate {
		h.Timestamp = r.u64()
	}
	h.Year = r.u16()
	h.Unknown7CA = r.u16()
	h.GameDate = r.date()
	h.Unknown830 = r.u32()
	h.Flag80D = r.u8()
	h.Flag80E = r.u8()
	h.Unknown808 = r.u32()
	if r.err != nil {
		return GameDBHeader{}, r.err
	}
	h.Size = r.off
	return h, nil
}

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
	db.Continents, db.Nations, db.NationsErr = readNationData(f.Body, pools, h.DatabaseVersion)
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
	clubs, clubsEnd, err := readClubTable(f.Body, clubCount, h.DatabaseVersion)
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
	if db.Players, db.SkippedPlayers, err = parsePlayers(f.Body, recs, nt, h.DatabaseVersion); err != nil {
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
