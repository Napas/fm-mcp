package savefile

import (
	"maps"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testSavePath = "../../data/test_save.fm"

// openTestSave opens the FM 26.2.0 test save, skipping when it is absent.
func openTestSave(t *testing.T) *Archive {
	t.Helper()
	if _, err := os.Stat(testSavePath); err != nil {
		t.Skipf("test save not available: %v", err)
	}
	a, err := Open(testSavePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestTestSaveContainer(t *testing.T) {
	a := openTestSave(t)

	want := Header{Magic: "fmf.", Version: 8, DirectoryOffset: 46481102, DataOffset: 26, Byte25: 3}
	if a.Header() != want {
		t.Errorf("Header = %+v, want %+v", a.Header(), want)
	}
	dir := a.Directory()
	if dir.Name != "test_save" {
		t.Errorf("root = %q, want test_save", dir.Name)
	}
	var rgman int
	for _, e := range dir.Files {
		if strings.HasPrefix(e.Path, "rgman/") {
			rgman++
		}
	}
	if len(dir.Files) != 1215 || rgman != 1139 {
		t.Errorf("got %d files (%d under rgman/), want 1215 (1139)", len(dir.Files), rgman)
	}
}

func TestTestSaveEveryFileDecompresses(t *testing.T) {
	a := openTestSave(t)

	for _, e := range a.Directory().Files {
		f, err := a.ReadFile(e.Path)
		if err != nil {
			t.Errorf("%s: %v", e.Path, err)
			continue
		}
		if f.Header.Extension != path.Ext(e.Path) {
			t.Errorf("%s: header extension %q", e.Path, f.Header.Extension)
		}
	}
}

func TestTestSaveGameDBFile(t *testing.T) {
	a := openTestSave(t)

	f, err := a.ReadFile("game_db.dat")
	if err != nil {
		t.Fatal(err)
	}
	if f.Header.Version != 4000 {
		t.Errorf("version = %d, want 4000", f.Header.Version)
	}
	if want := 122062842 - FileHeaderSize; len(f.Body) != want {
		t.Errorf("body = %d bytes, want %d", len(f.Body), want)
	}

	h, err := ParseGameDBHeader(f.Body)
	if err != nil {
		t.Fatal(err)
	}
	if h.DatabaseVersion != 3727 || h.OriginalDatabaseVersion != 3727 || h.Label != "2620" {
		t.Errorf("header = %+v", h)
	}
	if want := time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC); !h.GameDate.Time().Equal(want) {
		t.Errorf("game date = %s, want %s", h.GameDate.Time(), want)
	}
	if h.Size != 48 {
		t.Errorf("Size = %d, want 48", h.Size)
	}
}

func TestTestSavePlayers(t *testing.T) {
	a := openTestSave(t)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC); !db.Header.GameDate.Time().Equal(want) {
		t.Errorf("game date = %s, want %s", db.Header.GameDate.Time(), want)
	}
	players := db.Players

	if len(players) != 37011 {
		t.Errorf("got %d players, want 37011", len(players))
	}
	byName := map[string]Player{}
	spurs := 0
	for _, p := range players {
		byName[p.FirstName+" "+p.LastName] = p
		if p.TeamIndex == 547 {
			spurs++
		}
	}
	if spurs != 27 {
		t.Errorf("got %d players at club 547 (Tottenham), want 27", spurs)
	}

	m := byName["James Maddison"]
	if m.UID != 28093567 || m.Index != 8804 || m.NationIndex != 139 || m.TeamIndex != 547 {
		t.Errorf("Maddison identity = %+v", m)
	}
	if want := time.Date(1996, 11, 23, 0, 0, 0, 0, time.UTC); !m.BirthDate.Time().Equal(want) {
		t.Errorf("Maddison born %s, want %s", m.BirthDate.Time(), want)
	}
	if m.CurrentAbility != 151 || m.PotentialAbility != 163 {
		t.Errorf("Maddison CA/PA = %d/%d, want 151/163", m.CurrentAbility, m.PotentialAbility)
	}
	if m.HomeReputation != 7500 || m.CurrentReputation != 7550 || m.WorldReputation != 6500 {
		t.Errorf("Maddison reputations = %d/%d/%d", m.HomeReputation, m.CurrentReputation, m.WorldReputation)
	}

	if m.Height != 175 || m.Value < 51_000_000 || m.Value > 52_000_000 || m.Female {
		t.Errorf("Maddison height %d, value %d, female %v", m.Height, m.Value, m.Female)
	}

	if c := m.Contract; c == nil || c.WeeklyWage != 147000 || c.TeamIndex != 547 || !c.End.Time().Equal(time.Date(2028, 6, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Maddison contract = %+v", m.Contract)
	}
	if len(m.Injuries) != 1 || m.Injuries[0].ID != 27 || !m.Injuries[0].Start.Time().AddDate(0, 0, int(m.Injuries[0].DaysMin)).Equal(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Maddison injuries = %+v", m.Injuries)
	}
	// PLAYER_INJURY in memory_pools.dat counts 1 321 injuries.
	if injuries := injuryTotal(players); injuries != 1321 || db.InjuryCount != 136 {
		t.Errorf("got %d injuries and %d injury types, want 1321 and 136", injuries, db.InjuryCount)
	}
	loans := 0
	for _, p := range players {
		if p.Loan != nil {
			loans++
		}
	}
	if loans != 3247 {
		t.Errorf("%d players on loan, want 3247 (the LOAN_CONTRACT pool)", loans)
	}

	v := byName["Guglielmo Vicario"]
	if want := time.Date(1996, 10, 7, 0, 0, 0, 0, time.UTC); !v.BirthDate.Time().Equal(want) {
		t.Errorf("Vicario born %s, want %s", v.BirthDate.Time(), want)
	}
	if v.Positions[0] != 20 || v.Height != 194 {
		t.Errorf("Vicario GK rating = %d, want 20", v.Positions[0])
	}
	if c := v.Contract; c == nil || c.PlayingTime != 15 {
		t.Errorf("Vicario contract = %+v, want playing time 15 (First-Choice Goalkeeper)", v.Contract)
	}
	// Club affinities: Matković (then at NK Osijek) supports R. Madrid;
	// Necip Uysal's 100 for Beşiktaş makes him Devoted.
	checkKnownPlayers(t, players, map[uint32][]ClubAffinity{
		2000301456: {{Club: 1086, Value: 100, Reason: 1}}, // Anton Matković
		2000196612: nil,                                   // Jaka Čuber Potočnik
		70031727:   {{Club: 1177, Value: 100, Reason: 1}}, // Necip Uysal
	}, func(p Player, want []ClubAffinity) {
		if !slices.Equal(p.ClubAffinities, want) {
			t.Errorf("%s %s: club affinities %+v, want %+v", p.FirstName, p.LastName, p.ClubAffinities, want)
		}
	})
	// No status has changed yet in the 26.2.0 test save.
	for _, p := range players {
		if c := p.Contract; c != nil && c.OriginalPlayingTime != c.PlayingTime {
			t.Errorf("%s %s: playing time %d, original %d; want equal", p.FirstName, p.LastName, c.PlayingTime, c.OriginalPlayingTime)
			break
		}
	}
}

// openRealArchive opens the copy of a real save named by env, skipping when
// env is unset. The archive closes when the test ends. Never point env at the
// game's own save folder.
func openRealArchive(t *testing.T, env string) (*Archive, string) {
	t.Helper()
	file := os.Getenv(env)
	if file == "" {
		t.Skipf("%s not set", env)
	}
	a, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a, file
}

// openRealSave opens the real save named by env and reads its database,
// failing when the save is not of game date want.
func openRealSave(t *testing.T, env string, want time.Time) (*Archive, *GameDB) {
	t.Helper()
	a, file := openRealArchive(t, env)
	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}
	requireGameDate(t, env, file, db.Header.GameDate.Time(), want)
	return a, db
}

// requireGameDate fails the test when the save named by env is of another day.
func requireGameDate(t *testing.T, env, file string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s: %s is of %s, not %s", env, file, got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

// TestRealSave runs the parser on a copy of a real save named by
// FM_REAL_SAVE. Never point it at the game's own save folder.
func TestRealSave(t *testing.T) {
	a, db := openRealSave(t, "FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC))
	if len(db.Players) < 10000 {
		t.Errorf("only %d players", len(db.Players))
	}
	t.Logf("build db %d, %d players", db.Header.DatabaseVersion, len(db.Players))
	if db.CurrencyErr != nil {
		t.Errorf("currencies: %v", db.CurrencyErr)
	}
	checkKnownCurrencies(t, db.Currencies)
	checkNationalSquads(t, db)
	checkNations(t, db, true)
	checkRealSaveCompetitionKeys(t, db)
	checkRealSaveFixtures(t, a, len(db.CompetitionKeys))
	checkRealSaveTables(t, a, db)
	if db.InjuryCount != 136 {
		t.Errorf("%d injury types, want 136", db.InjuryCount)
	}
	// Facts seen in game on the KŽ save of 2035-12-02.
	checkKnownInjuries(t, db.Players)
	checkFranceSquad(t, db)
	checkKnownPlayingTimes(t, db.Players)
	checkClubBudgets(t, db, metzUID, Budgets{Transfer: 8312454, WeeklyWage: 1069817})
	checkKnownLoyaltyBonuses(t, db.Players, map[uint32]loyalty{
		2002078397: {482888, 2299}, // Chris Ashby
		2002114753: {772867, 2972}, // Bruno Sami
	})
	checkKnownPersonalities(t, db.Players, map[uint32]personality{
		2002078397: {[8]uint8{7, 11, 16, 12, 15, 17, 17, 3}, 0x8000},              // Chris Ashby: Marks Opponent Tightly
		2002114753: {[8]uint8{13, 19, 13, 8, 13, 8, 10, 6}, 0x0080000000000000},   // Bruno Sami
		28124569:   {[8]uint8{12, 14, 14, 15, 17, 15, 16, 2}, 0x0100000000000002}, // Tino Livramento
	})
	checkKnownClubAffinities(t, db)

	fog, err := ReadFogOfWar(a)
	if err != nil {
		t.Fatal(err)
	}
	uids := make(map[uint32]uint32, len(db.Players)) // person index → uid
	for _, p := range db.Players {
		uids[p.Index] = p.UID
	}
	human, err := ReadHumanIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fog.For(human); !ok {
		t.Errorf("fog_of_war.dat has no entry for human %d", human)
	}
	for _, h := range fog.Humans {
		for person := range h.Players {
			// Skipped player records are missing from db.Players.
			if _, ok := uids[person]; !ok && db.SkippedPlayers == 0 {
				t.Errorf("human %d knows person %d, which is not a player", h.HumanIndex, person)
			}
		}
		t.Logf("human %d: stored knowledge of %d players", h.HumanIndex, len(h.Players))
	}

	season, err := ReadSeasonStats(a)
	if err != nil {
		t.Fatal(err)
	}
	for person := range season {
		if _, ok := uids[person]; !ok && db.SkippedPlayers == 0 {
			t.Errorf("season stats for person %d, who is not a player", person)
		}
	}
	t.Logf("season stats for %d players", len(season))
	// Chris Ashby in the KŽ save of 2035-12-02; the game shows 16 (2)
	// appearances, average rating 6.58, 7 yellow and 1 red card.
	if uids[ashbyIndex] == ashbyUID {
		if got := season[ashbyIndex].Club; got == nil || withoutExtras(*got) != ashby {
			t.Errorf("Ashby = %+v, want %+v", got, ashby)
		}
	}
}

// checkKeeperSlots checks the game's keeper rule on every kept scope: with a
// GK rating of 15 or more the game writes no headers lost (d59), else d59 is
// headers minus headers won.
func checkKeeperSlots(t *testing.T, db *GameDB, season map[uint32]SeasonRecord) {
	t.Helper()
	keepers, others := 0, 0
	for _, p := range db.Players {
		rec := season[p.Index]
		for _, s := range []*MatchStats{rec.Club, rec.Friendlies, rec.League, rec.Cups, rec.Continental, rec.Internationals} {
			if s == nil {
				continue
			}
			d := s.Detail
			keeper := p.Positions[0] >= 15
			if keeper {
				keepers++
			} else {
				others++
			}
			if keeper && d.HeadersLost != 0 || !keeper && d.HeadersLost != d.Headers-d.HeadersWon {
				t.Errorf("person %d (GK %d): headers %d, won %d, lost %d", p.Index, p.Positions[0], d.Headers, d.HeadersWon, d.HeadersLost)
				return
			}
		}
	}
	t.Logf("keeper rule holds on %d keeper and %d outfield scopes", keepers, others)
}

// withoutExtras clears the fields beyond rating, appearances, goals, assists
// and cards.
func withoutExtras(s MatchStats) MatchStats {
	s.Minutes, s.PenaltiesTaken, s.PenaltiesScored, s.PlayerOfTheMatch = 0, 0, 0, 0
	s.Detail = MatchDetail{}
	return s
}

// TestRealSave9Dec checks season stats against the game's Season Stats screens
// in the KŽ save of 2035-12-09, named by FM_REAL_SAVE_9DEC.
func TestRealSave9Dec(t *testing.T) {
	a, db := openRealSave(t, "FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC))
	season, err := ReadSeasonStats(a)
	if err != nil {
		t.Fatal(err)
	}
	byUID := make(map[uint32]uint32, len(db.Players))
	for _, p := range db.Players {
		byUID[p.UID] = p.Index
	}
	record := func(uid uint32) SeasonRecord {
		t.Helper()
		idx, ok := byUID[uid]
		if !ok {
			t.Fatalf("uid %d not in the save", uid)
		}
		return season[idx]
	}
	// scope fails the test when got is absent, else returns it.
	scope := func(name string, got *MatchStats) MatchStats {
		t.Helper()
		if got == nil {
			t.Fatalf("%s scope is absent", name)
		}
		return *got
	}
	check := func(name string, got MatchStats, want MatchStats) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %+v, want %+v", name, got, want)
		}
	}

	// Anton Matković.
	m := record(2000301456)
	check("Matković league", scope("league", m.League), matkovicLeague)
	club := scope("club", m.Club)
	if club.Minutes != 1608 || club.PlayerOfTheMatch != 3 || club.PenaltiesTaken != 1 || club.PenaltiesScored != 1 {
		t.Errorf("Matković club = %+v", club)
	}
	f := scope("friendlies", m.Friendlies)
	if f.Starts != 3 || f.SubApps != 1 || f.Goals != 1 || f.Assists != 2 || f.PlayerOfTheMatch != 1 {
		t.Errorf("Matković friendlies = %+v", f)
	}

	// Carlo Bernardi.
	bl := scope("Bernardi league", record(2002100552).League)
	check("Bernardi league", bl, MatchStats{
		RatingSum: 877, Minutes: 679, Starts: bl.Starts, SubApps: bl.SubApps, Rated: 13,
		Goals: bl.Goals, Assists: bl.Assists, PlayerOfTheMatch: bl.PlayerOfTheMatch,
		RedCards: bl.RedCards, YellowCards: bl.YellowCards,
		Detail: MatchDetail{
			MatchesWon: 7, Passes: 384, PassesCompleted: 336, Tackles: 13, TacklesWon: 13, Headers: 46, HeadersWon: 22,
			Dribbles: 1, Fouls: 4, Shots: 2, ShotsOnTarget: 1, Distance: 916, Offsides: 0, KeyPasses: 6,
			KeyTackles: 1, KeyHeaders: 11, Interceptions: 17, ChancesCreated: 1, PossessionLost: 51, PossessionWon: 67,
			ExpectedGoals: 8, ExpectedAssists: 55, Clearances: 17, Sprints: 163, ProgressivePasses: 56,
			Crosses: 2, CrossesCompleted: 2, ShotsBlocked: 4,
			TeamGoals: 23, TeamConceded: 11, MatchesLost: 2, FoulsAgainst: 1, HeadersLost: 24, ShotsOutsideBox: 1,
			OpenPlayKeyPasses: 6, NonPenaltyXG: 8, OpenPlayCrosses: 2, OpenPlayCrossesAttempted: 2, OpenPlayCrossesCompleted: 2,
			Blocks: 5, PressuresAttempted: 37, PressuresCompleted: 5,
		},
	})

	// Erling Haaland: Carabao Cup.
	hc := scope("Haaland cups", record(29179241).Cups)
	check("Haaland cups", hc, MatchStats{
		RatingSum: 153, Minutes: 161, Starts: hc.Starts, SubApps: hc.SubApps, Rated: hc.Rated, Goals: 2, Assists: 1,
		PenaltiesTaken: 1, PenaltiesScored: 1, PlayerOfTheMatch: hc.PlayerOfTheMatch,
		RedCards: hc.RedCards, YellowCards: hc.YellowCards,
		Detail: MatchDetail{
			MatchesWon: 1, Passes: 57, PassesCompleted: 46, Tackles: 0, TacklesWon: 0, Headers: 9, HeadersWon: 1,
			Dribbles: 2, Fouls: 0, Shots: 4, ShotsOnTarget: 3, Distance: 161, Offsides: 3, KeyPasses: 1,
			KeyTackles: 0, KeyHeaders: 0, Interceptions: 2, ChancesCreated: 0, PossessionLost: 32, PossessionWon: 4,
			ExpectedGoals: 62, ExpectedAssists: 11, Clearances: 2, Sprints: 7, ProgressivePasses: 0,
			Crosses: 0, CrossesCompleted: 0, ShotsBlocked: 0,
			TeamGoals: 5, TeamConceded: 3, FoulsAgainst: 6, HeadersLost: 8, ShotsOutsideBox: 1,
			OpenPlayKeyPasses: 1, NonPenaltyXG: 62,
		},
	})

	// Francesco Pio Esposito.
	e := record(2000146883)
	el := scope("Esposito league", e.League)
	check("Esposito league", el, MatchStats{
		RatingSum: el.RatingSum, Minutes: 933, Starts: el.Starts, SubApps: el.SubApps, Rated: el.Rated,
		Goals: el.Goals, Assists: el.Assists, PenaltiesTaken: 2, PenaltiesScored: 2,
		PlayerOfTheMatch: el.PlayerOfTheMatch, RedCards: el.RedCards, YellowCards: el.YellowCards,
		Detail: MatchDetail{
			MatchesWon: 7, Passes: 307, PassesCompleted: 271, Tackles: 2, TacklesWon: 1, Headers: 72, HeadersWon: 22,
			Dribbles: 10, Fouls: 10, Shots: 66, ShotsOnTarget: 32, Distance: 1056, Offsides: 8, KeyPasses: 17,
			KeyTackles: 0, KeyHeaders: 1, Interceptions: 5, ChancesCreated: 2, PossessionLost: 124, PossessionWon: 22,
			ExpectedGoals: 812, ExpectedAssists: 8, Clearances: 1, Sprints: 61, ProgressivePasses: 14,
			Crosses: 4, CrossesCompleted: 2, ShotsBlocked: 0,
			TeamGoals: 17, TeamConceded: 9, MatchesLost: 1, FoulsAgainst: 19, HeadersLost: 50, ShotsOutsideBox: 19,
			GoalsOutsideBox: 2, FreeKickShots: 4, OpenPlayKeyPasses: 17, NonPenaltyXG: 735, OpenPlayCrosses: 4,
			OpenPlayCrossesAttempted: 4, OpenPlayCrossesCompleted: 2, Blocks: 1, PressuresAttempted: 22, PressuresCompleted: 5,
		},
	})
	ec := scope("Esposito continental", e.Continental)
	if ec.PenaltiesTaken != 1 || ec.PenaltiesScored != 0 {
		t.Errorf("Esposito continental pens = %d taken, %d scored; want 1, 0", ec.PenaltiesTaken, ec.PenaltiesScored)
	}
	// Slot 4 equals the game's Euro Championship qualifiers row.
	ei := scope("Esposito internationals", e.Internationals)
	if ei.Starts != 6 || ei.SubApps != 0 || ei.Goals != 7 || ei.PenaltiesScored != 2 || ei.PlayerOfTheMatch != 2 {
		t.Errorf("Esposito internationals = %+v", ei)
	}
	if m.Internationals == nil || m.Internationals.SubApps != 1 || m.Internationals.Rated != 0 {
		t.Errorf("Matković internationals = %+v, want 0 (1) without a rating", m.Internationals)
	}
	if d := club.Detail; d.PressuresAttempted != 122 || d.HeadersLost != 49 {
		t.Errorf("Matković club pressures %d, headers lost %d; want 122, 49", d.PressuresAttempted, d.HeadersLost)
	}

	// Bruno Sami, goalkeeper. League values equal the game's Season Stats
	// (2035-12-28); shots on target faced is not shown there.
	sami := record(2002114753)
	for _, k := range []struct {
		name                                string
		got                                 MatchStats
		conceded, cleanSheets               uint8
		faced, saved, held, parried, tipped int16
		onTarget, xgotFaced, mistakes       int16
	}{
		{"league", scope("Sami league", sami.League), 22, 3, 1, 1, 27, 25, 6, 79, 1238, 1},
		{"club", scope("Sami club", sami.Club), 27, 6, 2, 1, 40, 31, 10, 106, 1567, 1},
	} {
		d := k.got.Detail
		if k.got.GoalsConceded != k.conceded || k.got.CleanSheets != k.cleanSheets ||
			d.Tackles != k.faced || d.TacklesWon != k.saved || d.Headers != k.held || d.HeadersWon != k.parried ||
			d.OpenPlayCrossesAttempted != k.tipped || d.Blocks != k.onTarget || d.OpenPlayCrossesCompleted != k.xgotFaced ||
			d.MistakesLeadingToGoal != k.mistakes || d.HeadersLost != 0 {
			t.Errorf("Sami %s = %+v", k.name, k.got)
		}
	}

	checkKeeperSlots(t, db, season)
	checkCompetitionBonuses(t, db)
	checkClubBudgets(t, db, metzUID, Budgets{Transfer: 8329098, WeeklyWage: 1069817})
	checkKnownLoyaltyBonuses(t, db.Players, map[uint32]loyalty{
		2002100552: {780434, 3363}, // Carlo Bernardi
		2000191040: {932281, 4018}, // Louis Leroux
	})
	checkKnownPersonalities(t, db.Players, map[uint32]personality{
		2002100552: {[8]uint8{10, 16, 14, 16, 15, 18, 15, 6}, 0x400008000},    // Carlo Bernardi
		2000191040: {[8]uint8{11, 11, 13, 11, 13, 12, 11, 5}, 0x800001500080}, // Louis Leroux
	})
	checkKnownClubAffinities(t, db)
	checkNations(t, db, true)
}

// checkKnownClubAffinities checks club affinities. In game on 2035-12-14,
// Matković's profile shows "As a player: Osijek" and "As a supporter:
// R. Madrid", and Čuber Potočnik's "As a player: Maribor"; the bytes are the
// same on 12-02 and 12-09. Leroux's are from the bytes.
func checkKnownClubAffinities(t *testing.T, db *GameDB) {
	t.Helper()
	checkKnownPlayers(t, db.Players, map[uint32][]ClubAffinity{
		2000301456: {{Club: 1115, Value: 100, Reason: 1}, {Club: 190, Value: 100, Reason: 2}}, // Anton Matković
		2000196612: {{Club: 1054, Value: 50, Reason: 2}},                                      // Jaka Čuber Potočnik
		2000191040: {{Club: 19201, Value: 40, Reason: 2}, {Club: 474, Value: 8, Reason: 2}},   // Louis Leroux
	}, func(p Player, want []ClubAffinity) {
		if !slices.Equal(p.ClubAffinities, want) {
			t.Errorf("%s %s: club affinities %+v, want %+v", p.FirstName, p.LastName, p.ClubAffinities, want)
		}
	})
	for i, name := range map[int64][2]string{1115: {"R. Madrid", "R. Madrid"}, 190: {"NK Osijek", "Osijek"}, 474: {"FC Metz", "FC Metz"}, 1054: {"NK Maribor", "Maribor"}} {
		if c := db.Clubs[i]; c.Name != name[0] || c.ShortName != name[1] {
			t.Errorf("club %d is %q / %q, want %q / %q", i, c.Name, c.ShortName, name[0], name[1])
		}
	}
}

// TestRealSave14Dec checks FC Metz's budgets against the game's Recruitment
// Budgets screen in the KŽ save of 2035-12-14, named by FM_REAL_SAVE_14DEC:
// transfer budget €9.63M, wage budget €16.43M p/a available plus €47.93M p/a
// spent.
func TestRealSave14Dec(t *testing.T) {
	_, db := openRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	checkClubBudgets(t, db, metzUID, Budgets{Transfer: 8329074, WeeklyWage: 1069817})
	checkKnownClubAffinities(t, db)
	checkNations(t, db, true)
}

// Nation indices of the KŽ saves and the test save (ranks of the language
// table's uids).
const (
	nationUSA       = 120
	nationEngland   = 139
	nationFrance    = 143
	nationIsrael    = 149
	nationAustralia = 177
	nationBrazil    = 189
	nationEcuador   = 192
)

// checkNations checks the continent table, nation → continent links and
// nation factors. Continents follow geography (Israel plays in Europe,
// Australia in Asia); the United States and Ecuador are the two nations
// whose record has two parsing tails. withFactors adds the KŽ saves'
// factors of France and the United States.
func checkNations(t *testing.T, db *GameDB, withFactors bool) {
	t.Helper()
	if db.NationsErr != nil {
		t.Fatalf("nations: %v", db.NationsErr)
	}
	if len(db.Continents) != 7 || len(db.Nations) != int(db.NationCount) {
		t.Fatalf("%d continents, %d nations (pool %d)", len(db.Continents), len(db.Nations), db.NationCount)
	}
	for i, want := range []float32{0.85, 0.85, 0.99, 0.85, 0.85, 1, 0} {
		if c := db.Continents[i]; c.Strength != want || c.Index != uint32(i) || c.UID != uint32(i)+1 { //nolint:gosec // small index
			t.Errorf("continent %d = %+v, want strength %v", i, c, want)
		}
	}
	for nation, want := range map[int]int32{
		nationUSA: 3, nationEngland: 2, nationFrance: 2, nationIsrael: 2, nationAustralia: 1, nationBrazil: 5, nationEcuador: 5,
	} {
		if got := db.Nations[nation].Continent; got != want {
			t.Errorf("nation %d continent %d, want %d", nation, got, want)
		}
	}
	perContinent := map[int32]int{}
	for _, n := range db.Nations {
		perContinent[n.Continent]++
	}
	if want := map[int32]int{-1: 22, 0: 57, 1: 47, 2: 57, 3: 43, 4: 15, 5: 10}; !maps.Equal(perContinent, want) {
		t.Errorf("nations per continent %v, want %v", perContinent, want)
	}
	if withFactors && (db.Nations[nationFrance].MensFactor != 169 || db.Nations[nationUSA].MensFactor != 140) {
		t.Errorf("factors France %d, USA %d; want 169, 140", db.Nations[nationFrance].MensFactor, db.Nations[nationUSA].MensFactor)
	}
}

// metzUID is FC Metz's club uid, the human's club in the KŽ saves.
const metzUID = 844

// checkClubBudgets checks the budgets of the club whose first team has uid.
func checkClubBudgets(t *testing.T, db *GameDB, uid uint32, want Budgets) {
	t.Helper()
	team, ok := db.FirstTeam(uid)
	if !ok {
		t.Fatalf("no first team with uid %d", uid)
	}
	club := db.Clubs[db.Teams[team].Container]
	if club.Budgets == nil || *club.Budgets != want {
		t.Errorf("%s budgets = %+v, want %+v", club.Name, club.Budgets, want)
	}
}

// personality is a player's hidden attributes and trait mask.
type personality struct {
	hidden [8]uint8
	traits uint64
}

// checkKnownPersonalities checks the inputs of personalities and traits
// seen in game.
func checkKnownPersonalities(t *testing.T, players []Player, want map[uint32]personality) {
	t.Helper()
	checkKnownPlayers(t, players, want, func(p Player, w personality) {
		if p.Hidden != w.hidden || p.Traits != w.traits {
			t.Errorf("%s %s: hidden %v, traits %#x; want %v, %#x", p.FirstName, p.LastName, p.Hidden, p.Traits, w.hidden, w.traits)
		}
	})
}

// loyalty is a main contract's total loyalty bonus and weekly instalment, GBP.
type loyalty struct{ total, weekly int32 }

// checkKnownLoyaltyBonuses checks the loyalty fields of Metz players whose
// remaining bonus was seen in game.
func checkKnownLoyaltyBonuses(t *testing.T, players []Player, want map[uint32]loyalty) {
	t.Helper()
	checkKnownPlayers(t, players, want, func(p Player, w loyalty) {
		if c := p.Contract; c == nil || c.LoyaltyBonus != w.total || c.WeeklyLoyalty != w.weekly {
			t.Errorf("%s %s: contract %+v, want loyalty bonus %d, weekly %d", p.FirstName, p.LastName, c, w.total, w.weekly)
		}
	})
}

// checkKnownPlayers calls check for each player in want, by uid, and reports
// the uids missing from players.
func checkKnownPlayers[W any](t *testing.T, players []Player, want map[uint32]W, check func(Player, W)) {
	t.Helper()
	missing := maps.Clone(want)
	for _, p := range players {
		if w, ok := want[p.UID]; ok {
			check(p, w)
			delete(missing, p.UID)
		}
	}
	if len(missing) != 0 {
		t.Errorf("players not found: %v", missing)
	}
}

// checkCompetitionBonuses checks the game's "Qualifying for the UEFA
// Champions League €71K" bonus of Carlo Bernardi (£60 964 at 1.1571305 =
// €70 543) and that Louis Leroux, who has none in game, has no such bonus.
func checkCompetitionBonuses(t *testing.T, db *GameDB) {
	t.Helper()
	const championsLeagueUID = 1301394
	contracts := map[uint32]*Contract{}
	for _, p := range db.Players {
		contracts[p.UID] = p.Contract
	}
	b := contracts[2002100552]
	want := []CompetitionBonus{{Type: 10, Competition: 11, Target: 637, Value: 60964}}
	if b == nil || !slices.Equal(b.Bonuses, want) {
		t.Errorf("Bernardi contract = %+v, want bonuses %+v", b, want)
	}
	if len(db.CompetitionUIDs) <= 637 || db.CompetitionUIDs[637] != championsLeagueUID {
		t.Errorf("competition 637 is not the UEFA Champions League (uid %d)", championsLeagueUID)
	}
	if l := contracts[2000191040]; l == nil || len(l.Bonuses) != 0 {
		t.Errorf("Leroux contract = %+v, want no competition bonuses", l)
	}
}

// checkKnownInjuries checks injuries seen in game in the KŽ save at
// 2035-12-02: Veiga "a bruised ankle, 1–2 days" (index 84 = uid 100) and
// Bernardi "a tight groin, 1–3 days" (88 = uid 104), plus Diakhaté's bruised
// ankle from 2035-11-30. Every injury matches an injury_manager.dat entry
// (8 631 entries, 28 for persons not decoded as players).
func checkKnownInjuries(t *testing.T, players []Player) {
	t.Helper()
	if total := injuryTotal(players); total != 8603 {
		t.Errorf("got %d injuries, want 8603", total)
	}
	want := map[uint32]Injury{
		83261126:   {ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}, // Danilo Veiga
		2002100552: {ID: 88, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 3}, // Carlo Bernardi
		2002183764: {ID: 84, Start: Date{Raw: 0x434e, Year: 2035}, DaysMin: 0, DaysMax: 3}, // Aziz Diakhaté
	}
	checkKnownPlayers(t, players, want, func(p Player, w Injury) {
		if len(p.Injuries) != 1 || p.Injuries[0] != w {
			t.Errorf("%s %s injuries = %+v, want [%+v]", p.FirstName, p.LastName, p.Injuries, w)
		}
	})
}

// TestRealSave14DecPlayingTime checks main-contract playing time bytes,
// playing-time scores and squad numbers in the KŽ save of 2035-12-14, named by
// FM_REAL_SAVE_14DEC. Squad numbers and playing times equal the game's
// squad-number screen that day. Never point it at the game's own save folder.
func TestRealSave14DecPlayingTime(t *testing.T) {
	_, db := openRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	type want struct {
		index             uint32
		current, original PlayingTime
		score             int8
		squadNumber       uint8
	}
	checkKnownPlayers(t, db.Players, map[uint32]want{
		2002100552: {169046, 3, 2, 88, 19},  // Carlo Bernardi
		2000478936: {102147, 4, 2, -15, 28}, // Ilan Assongo
		2000191040: {62597, 3, 2, 86, 41},   // Louis Leroux
		2002078397: {146891, 22, 2, 80, 4},  // Chris Ashby
		2002095003: {163497, 1, 4, 81, 16},  // Luke Reddy
		2002078767: {147261, 4, 4, 100, 1},  // Stephen Ly
	}, func(p Player, w want) {
		c := p.Contract
		if p.Index != w.index || c == nil || c.PlayingTime != w.current || c.OriginalPlayingTime != w.original || c.PlayingTimeScore != w.score || c.SquadNumber != w.squadNumber {
			t.Errorf("%s %s (index %d): contract %+v, want index %d, playing time %d, original %d, score %d, squad number %d",
				p.FirstName, p.LastName, p.Index, c, w.index, w.current, w.original, w.score, w.squadNumber)
		}
	})
}

// checkKnownPlayingTimes checks main contracts' playing time and original
// playing time in the KŽ save of 2035-12-02; the playing time matches the
// Metz squad screen for all twelve.
func checkKnownPlayingTimes(t *testing.T, players []Player) {
	t.Helper()
	type playingTimes struct {
		index             uint32
		current, original PlayingTime
		name              string
	}
	want := map[uint32]playingTimes{
		2002095003: {163497, 1, 4, "Star Player"},               // Luke Reddy
		2002104234: {172728, 2, 2, "Important Player"},          // Samal Edubatey
		2000301456: {77416, 2, 2, "Important Player"},           // Anton Matković
		2002114753: {183247, 15, 15, "First-Choice Goalkeeper"}, // Bruno Sami
		2002206940: {285144, 10, 13, "Breakthrough Prospect"},   // Mounir Haddad
		2002078397: {146891, 3, 2, "Regular Starter"},           // Chris Ashby
		2000191040: {62597, 3, 2, "Regular Starter"},            // Louis Leroux
		83261126:   {40427, 3, 2, "Regular Starter"},            // Danilo Veiga
		2000111544: {55473, 3, 1, "Regular Starter"},            // Joseph Nduquidi
		2002115328: {183822, 4, 4, "Squad Player"},              // Pablo Vilas
		2002099166: {167660, 4, 4, "Squad Player"},              // Pablo Rodríguez
		2002078767: {147261, 4, 4, "Squad Player"},              // Stephen Ly
	}
	checkKnownPlayers(t, players, want, func(p Player, w playingTimes) {
		c := p.Contract
		if p.Index != w.index || c == nil || c.PlayingTime != w.current || c.OriginalPlayingTime != w.original {
			t.Errorf("%s %s (index %d): contract %+v, want index %d, playing time %d, original %d", p.FirstName, p.LastName, p.Index, c, w.index, w.current, w.original)
			return
		}
		if name, _ := c.PlayingTime.Name(); name != w.name {
			t.Errorf("%s %s: playing time %q, want %q", p.FirstName, p.LastName, name, w.name)
		}
	})
}

func injuryTotal(players []Player) int {
	total := 0
	for _, p := range players {
		total += len(p.Injuries)
	}
	return total
}

// checkFranceSquad checks France's senior squad (team 589) as the game shows it.
func checkFranceSquad(t *testing.T, db *GameDB) {
	t.Helper()
	want := []string{"Leonardi", "Kambwala", "Yoro", "Bitshiabu", "Gusto", "Bouaddi", "Camavinga", "Methalie", "Charron",
		"Bahoya", "Tel", "Chevalier", "Saliba", "Lukeba", "Gadou", "Álvarez", "Kanda-Tshongo", "Pfister", "Doué",
		"Thuram", "Gonçalves", "Ekitiké", "Cherki"}
	slices.Sort(want)
	if len(db.Teams) <= 589 {
		t.Errorf("only %d teams", len(db.Teams))
		return
	}
	if got := squadSurnames(db, 589); !slices.Equal(got, want) {
		t.Errorf("France (team 589) = %v, want %v", got, want)
	}
}

const (
	ashbyIndex = 146891
	ashbyUID   = 2002078397
)

// gameDBDir is the installed game's bundled database folder.
func gameDBDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	dir := filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Football Manager 26",
		"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("game not installed: %v", err)
	}
	return dir
}

func TestInstalledLangDBNations(t *testing.T) {
	dir := gameDBDir(t)
	for _, file := range []string{"2600/2600_fm/lang_db.dat", "2620/2620_fm/lang_update_db.dat"} {
		ldb, err := OpenLangDB(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		nations, err := ldb.Nations()
		_ = ldb.Close()
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		byUID := map[uint32]LangName{}
		for _, n := range nations {
			byUID[n.UID] = n
		}
		if len(nations) != 251 || byUID[765].Name != "England" || byUID[1649].Code != "ARG" {
			t.Errorf("%s: %d nations, 765 = %+v, 1649 = %+v", file, len(nations), byUID[765], byUID[1649])
		}
	}
}

func TestInstalledLangDBInjuries(t *testing.T) {
	dir := gameDBDir(t)
	for _, file := range []string{"2600/2600_fm/lang_db.dat", "2620/2620_fm/lang_update_db.dat"} {
		ldb, err := OpenLangDB(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		injuries, err := ldb.Injuries()
		_ = ldb.Close()
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		names := NamesByRank(injuries, 136)
		if names[27] != "torn anterior cruciate ligament" || names[84] != "bruised ankle" || names[88] != "tight groin" {
			t.Errorf("%s: %d injuries; 27 = %q, 84 = %q, 88 = %q", file, len(injuries), names[27], names[84], names[88])
		}
	}
}

func TestTestSaveSummary(t *testing.T) {
	a := openTestSave(t)

	s, err := ReadSummary(a)
	if err != nil {
		t.Fatal(err)
	}

	if s.ManagerName != "Napas Mačiulaitis" || s.ClubShortName != "Spurs" || s.ClubUID != 728 || s.Build != "26.2.0+2276383" {
		t.Errorf("Summary = %+v", s)
	}
}

func TestTestSaveClubsTeamsAndCompetitions(t *testing.T) {
	a := openTestSave(t)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}

	if len(db.Clubs) != 25777 || len(db.Teams) != 59370 || len(db.CompetitionUIDs) != 8093 {
		t.Fatalf("got %d clubs, %d teams, %d competitions", len(db.Clubs), len(db.Teams), len(db.CompetitionUIDs))
	}
	spurs := db.Teams[547]
	club := db.Clubs[spurs.Container]
	if spurs.UID != 728 || spurs.Container != 416 || club.Name != "Tottenham Hotspur" || club.ShortName != "Spurs" || club.Women {
		t.Errorf("team 547 = %+v, club = %+v", spurs, club)
	}
	if db.CompetitionUIDs[spurs.Division] != 11 || db.CompetitionUIDs[637] != 1301394 {
		t.Errorf("Spurs division %d → uid %d; comp 637 → uid %d", spurs.Division, db.CompetitionUIDs[spurs.Division], db.CompetitionUIDs[637])
	}
	checkClubBudgets(t, db, 728, Budgets{Transfer: 45400000, WeeklyWage: 3770160})
	if c := db.Clubs[db.Teams[20739].Container]; c.Name != "Chelsea" || !c.Women {
		t.Errorf("team 20739 club = %+v, want Chelsea women", c)
	}
	if n := db.Teams[1233]; !n.National || n.Container != 187 {
		t.Errorf("team 1233 = %+v, want Argentina's national team (container 187)", n)
	}
}

func TestTestSaveNations(t *testing.T) {
	a := openTestSave(t)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}

	checkNations(t, db, false)
}

func TestTestSaveNationalSquads(t *testing.T) {
	a := openTestSave(t)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}
	checkNationalSquads(t, db)
	if got := squadSurnames(db, 1233); len(got) != 23 || !slices.Contains(got, "Romero") || !slices.Contains(got, "Messi") {
		t.Errorf("Argentina (team 1233) = %v, want 23 players with Romero and Messi", got)
	}
}

// checkNationalSquads checks that a player's national team field and the
// national teams' player lists agree both ways.
func checkNationalSquads(t *testing.T, db *GameDB) {
	t.Helper()
	listed := map[uint32]int32{}
	for _, team := range db.Teams {
		if !team.National {
			continue
		}
		for _, person := range team.Players {
			if prev, ok := listed[uint32(person)]; ok { //nolint:gosec // person indices are non-negative
				t.Errorf("person %d is in national teams %d and %d", person, prev, team.Index)
			}
			listed[uint32(person)] = int32(team.Index) //nolint:gosec // team indices fit in int32
		}
	}
	called := 0
	for _, p := range db.Players {
		team, ok := listed[p.Index]
		switch {
		case p.NationalTeamRef >= 0:
			called++
			if !ok || team != p.NationalTeamRef {
				t.Errorf("player %d has national team %d but is listed in %d (%v)", p.Index, p.NationalTeamRef, team, ok)
			}
		case ok:
			t.Errorf("player %d is listed in national team %d without a national team field", p.Index, team)
		}
	}
	t.Logf("%d players called up to national teams", called)
}

// squadSurnames returns the sorted surnames of the players listed in team.
func squadSurnames(db *GameDB, team int) []string {
	byIndex := make(map[int32]Player, len(db.Players))
	for _, p := range db.Players {
		byIndex[int32(p.Index)] = p //nolint:gosec // person indices fit in int32
	}
	var out []string
	for _, person := range db.Teams[team].Players {
		if p, ok := byIndex[person]; ok {
			out = append(out, p.LastName)
		}
	}
	slices.Sort(out)
	return out
}

func TestInstalledLangDBCompetitions(t *testing.T) {
	ldb, err := OpenLangDB(filepath.Join(gameDBDir(t), "2620/2620_fm/lang_update_db.dat"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()

	comps, err := ldb.Competitions()
	if err != nil {
		t.Fatal(err)
	}
	names := map[uint32]string{}
	for _, c := range comps {
		names[c.UID] = c.Name
	}
	if names[11] != "Premier League" || names[1301394] != "UEFA Champions League" {
		t.Errorf("%d competitions; 11 = %q, 1301394 = %q", len(comps), names[11], names[1301394])
	}
}

func TestTestSaveFogOfWar(t *testing.T) {
	a := openTestSave(t)

	f, err := ReadFogOfWar(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Humans) != 1 {
		t.Fatalf("%d humans, want 1", len(f.Humans))
	}
	human, err := ReadHumanIndex(a)
	if err != nil || human != 88351 {
		t.Fatalf("ReadHumanIndex = %d, %v; want 88351", human, err)
	}
	h, ok := f.For(human)
	if !ok {
		t.Fatalf("human 88351 missing; found human %d", f.Humans[0].HumanIndex)
	}
	levels := map[uint8]int{}
	for _, l := range h.Players {
		levels[l]++
	}
	if len(h.Players) != 73 || levels[100] != 53 || levels[80] != 15 || levels[60] != 5 {
		t.Errorf("%d players, levels %v; want 73 with 100×53, 80×15, 60×5", len(h.Players), levels)
	}
	// Fábio Carvalho (uid 28122195) and Bailey Palmer (uid 2000510958).
	if h.Players[9480] != 100 || h.Players[54212] != 60 {
		t.Errorf("Carvalho = %d, Palmer = %d; want 100, 60", h.Players[9480], h.Players[54212])
	}
}

func TestTestSaveSeasonStats(t *testing.T) {
	a := openTestSave(t)

	season, err := ReadSeasonStats(a)
	if err != nil {
		t.Fatal(err)
	}
	withClub := 0
	for _, rec := range season {
		if rec.Club != nil {
			withClub++
		}
	}
	if withClub != 890 {
		t.Errorf("%d players with season stats, want 890", withClub)
	}
	// Edinson Cavani (uid 78005742): the database's 2025 season at Boca.
	want := MatchStats{RatingSum: 1916, Starts: 28, Rated: 28, Goals: 16, Assists: 5, RedCards: 1}
	if got := season[22267].Club; got == nil || withoutExtras(*got) != want {
		t.Errorf("Cavani = %+v, want %+v", got, want)
	}
}

func TestTestSaveCurrencies(t *testing.T) {
	a := openTestSave(t)

	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}

	if db.CurrencyErr != nil || len(db.Currencies) != 173 {
		t.Fatalf("%d currencies, err %v; want 173", len(db.Currencies), db.CurrencyErr)
	}
	checkKnownCurrencies(t, db.Currencies)
}

// near reports whether rate is within 1e-6 of want.
func near(rate float32, want float64) bool { return math.Abs(float64(rate)-want) < 1e-6 }

// checkKnownCurrencies checks GBP, EUR and USD, whose records are the same
// in every save seen.
func checkKnownCurrencies(t *testing.T, currencies []Currency) {
	t.Helper()
	byUID := map[uint32]Currency{}
	for _, c := range currencies {
		byUID[c.UID] = c
	}
	if eur := byUID[28]; eur.Symbol != "€" || eur.Name != "European Euro" || eur.ShortName != "Euro" || !near(eur.PerGBP, 1.15713) {
		t.Errorf("uid 28 = %+v, want the euro at 1.15713", eur)
	}
	if gbp := byUID[86]; gbp.PerGBP != 1 {
		t.Errorf("uid 86 = %+v, want GBP at 1", gbp)
	}
	if usd := byUID[87]; usd.Symbol != "$" || !near(usd.PerGBP, 1.350825) {
		t.Errorf("uid 87 = %+v, want USD at 1.350825", usd)
	}
}

// TestRealSave16JanPersonFields checks the ACTUAL_PERSON fields before the
// contracts marker in the KŽ save of 2036-01-16, named by
// FM_REAL_SAVE_16JAN, against the values the player-interest research
// decoded (docs/superpowers/specs/2026-10-23-player-interest).
func TestRealSave16JanPersonFields(t *testing.T) {
	_, db := openRealSave(t, "FM_REAL_SAVE_16JAN", time.Date(2036, 1, 16, 0, 0, 0, 0, time.UTC))
	byUID := make(map[uint32]*Player, len(db.Players))
	for i := range db.Players {
		byUID[db.Players[i].UID] = &db.Players[i]
	}
	player := func(uid uint32) *Player {
		t.Helper()
		p := byUID[uid]
		if p == nil {
			t.Fatalf("no player with uid %d", uid)
		}
		return p
	}

	if got := player(2002115699).TransferIDs; !slices.Equal(got, []uint32{10183, 10810}) { // Unai Marco
		t.Errorf("Marco: transfer ids %v, want [10183 10810]", got)
	}
	for uid, want := range map[uint32]int64{
		2002115328: 474,  // Pablo Vilas: FC Metz
		2002077775: 474,  // Jean-Pierre Bayal: FC Metz
		2002112631: 491,  // Julien Barthes
		2002117488: 498,  // Gianfranco Fiaschetti
		2002098987: 677,  // Giovanni Lotito
		2002127265: 1122, // Lau Jiménez
		2000416174: -1,   // Gabriel Ahovey
	} {
		if p := player(uid); p.PreviousClub != want {
			t.Errorf("%s %s (uid %d): previous club %d, want %d", p.FirstName, p.LastName, uid, p.PreviousClub, want)
		}
	}
	for uid, want := range map[uint32]Date{
		2002077775: {Raw: 246, Year: 2035}, // Bayal: 2035-09-03
		2002112631: {Raw: 228, Year: 2035}, // Barthes: 2035-08-16
	} {
		if got := player(uid).JoinedClubPerson; got.dayOfYear() != want.Raw || got.Year != want.Year {
			t.Errorf("uid %d: joined %d/%d, want day %d of %d", uid, got.dayOfYear(), got.Year, want.Raw, want.Year)
		}
	}

	labels, err := os.ReadFile("../../docs/superpowers/specs/2026-10-23-player-interest/labels-2036-01-16.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(labels)), "\n")[1:]
	for _, line := range lines {
		uid, err := strconv.ParseUint(strings.Split(line, "\t")[1], 10, 32)
		if err != nil {
			t.Fatalf("labels: %q: %v", line, err)
		}
		if p := player(uint32(uid)); p.Retiring || p.OnlyOneClub {
			t.Errorf("%s %s (uid %d): retiring %v, only one club %v; want neither", p.FirstName, p.LastName, uid, p.Retiring, p.OnlyOneClub)
		}
	}

	var retiring, onlyOne, previous, transfers int
	for _, p := range db.Players {
		if p.Retiring {
			retiring++
		}
		if p.OnlyOneClub {
			onlyOne++
		}
		if p.PreviousClub != -1 {
			previous++
			if p.PreviousClub < 0 || p.PreviousClub >= int64(len(db.Clubs)) {
				t.Errorf("uid %d: previous club %d is not a club index", p.UID, p.PreviousClub)
			}
		}
		if len(p.TransferIDs) > 0 {
			transfers++
		}
	}
	t.Logf("%d labelled players; of %d players: %d retiring, %d only one club, %d with a previous club, %d with transfer ids",
		len(lines), len(db.Players), retiring, onlyOne, previous, transfers)
}

// TestRealSave16JanContractEvents checks the contract ring, kind and core
// flags in the KŽ save of 2036-01-16 against the player-interest research.
func TestRealSave16JanContractEvents(t *testing.T) {
	_, db := openRealSave(t, "FM_REAL_SAVE_16JAN", time.Date(2036, 1, 16, 0, 0, 0, 0, time.UTC))
	byUID := make(map[uint32]*Player, len(db.Players))
	for i := range db.Players {
		byUID[db.Players[i].UID] = &db.Players[i]
	}
	main := func(uid uint32) *Contract {
		t.Helper()
		p := byUID[uid]
		if p == nil || p.Contract == nil {
			t.Fatalf("uid %d: no player or main contract", uid)
		}
		return p.Contract
	}

	lotito := main(2002098987)
	found := false
	for _, e := range lotito.Events {
		if e.Open() && e.Type == 12 && e.ID == 650 && e.Status == 3 {
			found = true
		}
	}
	if !found {
		t.Errorf("Lotito: events %+v, want an open event of type 12, id 650, status 3", lotito.Events)
	}

	cissokho := main(2002099764)
	var types []uint8
	for _, e := range cissokho.Events {
		if e.Open() {
			t.Errorf("Cissokho: open event %+v, want all closed", e)
		}
		types = append(types, e.Type)
	}
	if !slices.Equal(types, []uint8{37, 37, 19}) {
		t.Errorf("Cissokho: event types %v, want [37 37 19]", types)
	}

	if barthes := main(2002112631); len(barthes.Events) != 0 {
		t.Errorf("Barthes: events %+v, want none", barthes.Events)
	}

	for uid, want := range map[uint32][2]uint8{ // kind, core flags
		2002112631: {1, 0x00}, // Julien Barthes
		2002114070: {0, 0x00}, // Zvonko Nešić
		2002116366: {1, 0x40}, // Álvaro Garrido (bit 6)
	} {
		c := main(uid)
		if !c.Full || c.Kind != want[0] || c.CoreFlags != want[1] {
			t.Errorf("uid %d: full %v, kind %d, core flags %#x; want kind %d, flags %#x", uid, c.Full, c.Kind, c.CoreFlags, want[0], want[1])
		}
	}
}
