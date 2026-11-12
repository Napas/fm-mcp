package savefile

import (
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"
)

// matchRecordBytes encodes a player record of matchPlayerSize bytes.
func matchRecordBytes(p MatchPlayer) []byte {
	b := make([]byte, matchPlayerSize)
	b[0], b[7], b[12] = 2, 2, 2
	binary.LittleEndian.PutUint32(b[1:], uint32(p.Person)) //nolint:gosec // signed field
	b[5], b[6] = p.Shirt, p.Side
	binary.LittleEndian.PutUint32(b[8:], p.StartPositions)
	binary.LittleEndian.PutUint32(b[13:], p.EndPositions)
	binary.LittleEndian.PutUint32(b[25:], math.Float32bits(p.XG))
	binary.LittleEndian.PutUint16(b[136:], p.Rating)
	st := p.Stats
	b[41], b[42], b[45], b[60], b[63], b[64] = st.Shots, st.ShotsOnTarget, st.ClearCutChances, st.Fouls, st.YellowCards, st.RedCards
	b[67], b[71] = st.SubbedOffMinute, st.SubbedOnMinute
	b[86], b[87], b[88], b[89], b[90] = st.Passes, st.PassesCompleted, st.KeyPasses, st.Crosses, st.CrossesCompleted
	b[92], b[93], b[94], b[98] = st.Tackles, st.TacklesWon, st.KeyTackles, st.Interceptions
	binary.LittleEndian.PutUint32(b[140:], math.Float32bits(st.DistanceMetres))
	b[151], b[153], b[186], b[190] = st.PossessionWon, st.Clearances, st.ProgressivePasses, st.Sprints
	return b
}

// testSquad is a side of 11 starters (person base..base+10, one of them
// taken off), a substitute who came on and an unused substitute.
func testSquad(side uint8, base int32) []MatchPlayer {
	out := make([]MatchPlayer, 0, 13)
	for i := range int32(11) {
		end := uint32(0x10)
		if i == 10 {
			end = 0
		}
		out = append(out, MatchPlayer{Person: base + i, Shirt: uint8(i + 1), Side: side, StartPositions: 0x10, EndPositions: end, XG: 0.25, Rating: 690}) //nolint:gosec // small test data
	}
	return append(out,
		MatchPlayer{Person: base + 11, Shirt: 12, Side: side, EndPositions: 0x4000, XG: 0.1, Rating: 640},
		MatchPlayer{Person: base + 12, Shirt: 13, Side: side, Rating: 670})
}

// testSides are a home testSquad from person home and an away one from away.
func testSides(home, away int32) []MatchPlayer {
	return append(testSquad(0, home), testSquad(1, away)...)
}

// matchObjectBytes encodes a decompressed match object: the 01 00 01 prefix,
// filler that holds no record, then the records of both sides.
func matchObjectBytes(players []MatchPlayer) []byte {
	b := append([]byte{1, 0, 1, 0x3c, 0x1f}, make([]byte, 300)...)
	for _, p := range players {
		b = append(b, matchRecordBytes(p)...)
		b = append(b, make([]byte, 30)...) // records are often longer than the fixed part
	}
	return append(b, make([]byte, 200)...)
}

type testPackEntry struct {
	id    int32
	frame []byte
}

// matchPackBody encodes match_stats_cfg.dat's body with one pack per list of
// entries, their frames stored in reverse order to show that offsets, not
// order, locate them.
func matchPackBody(packs ...[]testPackEntry) []byte {
	b := binary.LittleEndian.AppendUint32(nil, 1027)
	b = append(b, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(packs))) //nolint:gosec // small test data
	for i, entries := range packs {
		b = binary.LittleEndian.AppendUint32(b, uint32(i)) //nolint:gosec // small test data
		b = binary.LittleEndian.AppendUint32(b, 5)
		b = append(b, "pks_"+string(rune('0'+i))...)
		b = binary.LittleEndian.AppendUint32(b, 0x01068000)
		b = append(b, 1, 1)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(entries))) //nolint:gosec // small test data
		var data []byte
		offsets := make([]int, len(entries))
		for j := len(entries) - 1; j >= 0; j-- {
			offsets[j] = len(data)
			data = append(data, entries[j].frame...)
		}
		for j, e := range entries {
			b = binary.LittleEndian.AppendUint32(b, uint32(e.id))         //nolint:gosec // signed field
			b = binary.LittleEndian.AppendUint32(b, uint32(offsets[j]))   //nolint:gosec // small test data
			b = binary.LittleEndian.AppendUint32(b, uint32(len(e.frame))) //nolint:gosec // small test data
		}
		b = binary.LittleEndian.AppendUint32(b, 0x01068000)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(data))) //nolint:gosec // small test data
		b = append(b, 3)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(data))) //nolint:gosec // small test data
		b = append(b, data...)
		b = append(b, 1)
	}
	return b
}

// TestParseMatchPack uses matchObjectBytes, which has no room for the away
// side's team block, so its objects have no team stats.
func TestParseMatchPack(t *testing.T) {
	first := testSides(100, 200)
	second := testSides(300, 400)
	body := matchPackBody(
		[]testPackEntry{{1022, zstdFrame(t, matchObjectBytes(first))}},
		[]testPackEntry{{7, zstdFrame(t, matchObjectBytes(second))}, {8, zstdFrame(t, []byte("not a match object"))}},
	)

	got, err := parseMatchPack(body, maxMatchPackDecoded)
	if err != nil {
		t.Fatalf("parseMatchPack: %v", err)
	}
	want := []MatchObject{{MatchID: 1022, Players: first}, {MatchID: 7, Players: second}, {MatchID: 8}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("objects = %+v, want %+v (match 8 unreadable)", got, want)
	}
}

func TestParseMatchPackBudget(t *testing.T) {
	frame := zstdFrame(t, matchObjectBytes(testSides(100, 200)))
	body := matchPackBody([]testPackEntry{{1, frame}, {2, frame}, {3, frame}})
	size := len(matchObjectBytes(testSides(100, 200)))

	if _, err := parseMatchPack(body, 3*size); err != nil {
		t.Errorf("within budget: %v", err)
	}
	if _, err := parseMatchPack(body, 3*size-1); !errors.Is(err, ErrCorrupt) {
		t.Errorf("over budget: err = %v, want ErrCorrupt", err)
	}
}

func TestScanMatchPlayersRejectsBadSides(t *testing.T) {
	ten := append(testSquad(0, 100)[1:], testSquad(1, 200)...)
	dup := testSides(100, 100)
	for name, players := range map[string][]MatchPlayer{"ten starters": ten, "duplicate person": dup} {
		if _, _, err := scanMatchPlayers(matchObjectBytes(players)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v, want ErrCorrupt", name, err)
		}
	}
}

// TestScanMatchPlayersSkipsLookalikes checks that bytes shaped like a record
// but with position bits outside the known ones, a rating out of range or a
// non-finite xG are not read as players.
func TestScanMatchPlayersSkipsLookalikes(t *testing.T) {
	players := testSides(100, 200)
	obj := matchObjectBytes(players)
	for _, bad := range []MatchPlayer{
		{Person: 900, Side: 0, StartPositions: 0x02020201, Rating: 690}, // junk position bits
		{Person: 901, Side: 0, StartPositions: 0x10, Rating: 2000},      // rating too high
		{Person: 902, Side: 1, StartPositions: 0x10, XG: float32(math.NaN()), Rating: 690},
		{Person: 903, Side: 2, StartPositions: 0x10, Rating: 690},     // side byte 2
		{Person: -5, Side: 0, StartPositions: 0x10, Rating: 690},      // negative person
		{Person: 1 << 30, Side: 0, StartPositions: 0x10, Rating: 690}, // float bytes read as a person
		{Person: 904, Side: 0, StartPositions: 0x10, Rating: 200},     // rating too low
		{Person: 906, Side: 0, StartPositions: 0x10, Rating: 690, Stats: MatchPlayerStats{DistanceMetres: float32(math.Inf(1))}},
	} {
		obj = append(obj, matchRecordBytes(bad)...)
	}
	faced := matchRecordBytes(MatchPlayer{Person: 905, Side: 0, StartPositions: 0x10, Rating: 690})
	binary.LittleEndian.PutUint32(faced[29:], math.Float32bits(50)) // xG faced out of range
	obj = append(obj, faced...)

	got, _, err := scanMatchPlayers(obj)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, players) {
		t.Errorf("players = %+v, want only the two squads", got)
	}
}

// TestScanMatchPlayersStepsOverRecords checks that bytes inside a record
// that look like another record (seen 139 bytes into goalkeeper records) are
// not read: the scan resumes after a record's fixed part.
func TestScanMatchPlayersStepsOverRecords(t *testing.T) {
	players := testSides(100, 200)
	first := append(matchRecordBytes(players[0]), make([]byte, 200)...) // a longer record
	copy(first[139:], matchRecordBytes(MatchPlayer{Person: 999, Side: 0, EndPositions: 0x10, Rating: 512}))
	obj := append([]byte{1, 0, 1}, first...)
	for _, p := range players[1:] {
		obj = append(obj, matchRecordBytes(p)...)
	}
	got, _, err := scanMatchPlayers(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > 0 {
		got[0].Stats = players[0].Stats // the planted record overwrites these bytes
	}
	if !reflect.DeepEqual(got, players) {
		t.Errorf("%d players, want %d: a record inside another was read", len(got), len(players))
	}
}

func TestParseMatchPackRejectsBadContainer(t *testing.T) {
	frame := zstdFrame(t, matchObjectBytes(testSides(100, 200)))
	ok := matchPackBody([]testPackEntry{{1, frame}})
	badThree := append([]byte(nil), ok...)
	badThree[len(ok)-1-len(frame)-4-1] = 4 // the u8 3 before the data length
	tooMany := append([]byte(nil), ok...)
	binary.LittleEndian.PutUint32(tooMany[6:], maxMatchPacks+1)
	pastData := append([]byte(nil), ok...)
	entry := 4 + 2 + 4 + 4 + 4 + 5 + 4 + 2 + 4 // the first entry's id
	binary.LittleEndian.PutUint32(pastData[entry+4:], 1<<20)
	pastSize := append([]byte(nil), ok...)
	binary.LittleEndian.PutUint32(pastSize[entry+8:], 1<<20)
	badEnd := append([]byte(nil), ok...)
	badEnd[len(badEnd)-1] = 2

	for name, body := range map[string][]byte{"too many packs": tooMany, "entry past the data": pastData, "size past the data": pastSize, "pack end byte": badEnd, "pack byte 3": badThree, "truncated": ok[:len(ok)-10]} {
		if _, err := parseMatchPack(body, maxMatchPackDecoded); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestReadMatchObjectsGatesVersion(t *testing.T) {
	a := openSave(t, buildSave(t, testInner{path: matchPackFile, data: innerFile("tad.", 5, "")}))
	if _, err := ReadMatchObjects(a); !errors.Is(err, ErrUnsupported) {
		t.Errorf("version 5: err = %v, want ErrUnsupported", err)
	}
}

// TestTestSave27MatchObjects checks the 07-15 match (Shamrock Rovers 3–2
// Kauno Žalgiris) against the game's match report.
func TestTestSave27MatchObjects(t *testing.T) {
	objs, err := ReadMatchObjects(openTestSaveAt(t, testSave27HomegrownPath))
	if err != nil {
		t.Fatal(err)
	}
	unread := 0
	for _, o := range objs {
		if o.Players == nil {
			unread++
		}
	}
	if len(objs) != 821 || unread != 0 {
		t.Fatalf("%d objects, %d unreadable; want 821 and 0", len(objs), unread)
	}
	var m *MatchObject
	for i := range objs {
		if objs[i].MatchID == 1022 {
			m = &objs[i]
		}
	}
	if m == nil {
		t.Fatal("no match 1022")
	}
	var played, cameOn int
	var xg [2]float64
	byPerson := map[int32]MatchPlayer{}
	for _, p := range m.Players {
		byPerson[p.Person] = p
		xg[p.Side] += float64(p.XG)
		if p.Played() {
			played++
		}
		if p.Played() && !p.Started() {
			cameOn++
		}
	}
	if len(m.Players) != 44 || played != 32 || cameOn != 10 || math.Round(xg[0]*100) != 198 || math.Round(xg[1]*100) != 218 {
		t.Errorf("%d records, %d played, %d came on, xG %.3f–%.3f; want 44, 32, 10, 1.98–2.18", len(m.Players), played, cameOn, xg[0], xg[1])
	}
	if byrne := byPerson[10498]; byrne.Rating != 819 || byrne.Side != 0 || byrne.Shirt != 29 {
		t.Errorf("Byrne = %+v, want rating 819, home, shirt 29", byrne)
	}
	want := [2]TeamStats{
		{Passes: 468, PassesCompleted: 402, PassesCompletedByThird: &[3]uint16{118, 215, 69}, Tackles: 6, TacklesWon: 4, Headers: 26, HeadersWon: 16, ShotsBlocked: 1, ClearCutChances: 5, Corners: 6, Fouls: 13, YellowCards: 3},
		{Passes: 500, PassesCompleted: 435, PassesCompletedByThird: &[3]uint16{156, 223, 56}, Tackles: 12, TacklesWon: 7, Headers: 25, HeadersWon: 8, ShotsBlocked: 4, ClearCutChances: 3, Corners: 4, Fouls: 7, YellowCards: 2},
	}
	if m.TeamStats == nil {
		t.Fatal("match 1022 has no team stats")
	}
	if got := *m.TeamStats; !reflect.DeepEqual(got, want) {
		t.Errorf("team stats = %+v, want the report's %+v", got, want)
	}
	for _, o := range objs {
		if o.TeamStats == nil {
			t.Errorf("match %d has no team stats", o.MatchID)
		}
		checkTeamSums(t, o)
	}
}

// TestParseMatchPackEmptyIsRead checks that a pack file with no objects
// reads as an empty list, not as unread.
func TestParseMatchPackEmptyIsRead(t *testing.T) {
	got, err := parseMatchPack(matchPackBody([]testPackEntry{}), maxMatchPackDecoded)
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("parseMatchPack = %v, %v; want an empty, non-nil list", got, err)
	}
}

func TestHasMatchID(t *testing.T) {
	for id, want := range map[int32]bool{0: true, 1022: true, NoMatchID: false, -7: false} {
		if got := (&FixtureResult{MatchID: id}).HasMatchID(); got != want {
			t.Errorf("HasMatchID(%d) = %v, want %v", id, got, want)
		}
	}
}

// teamBlockBytes encodes the block before a side's first player record.
func teamBlockBytes(s TeamStats) []byte {
	b := make([]byte, teamBlockSize)
	at := func(rel int) int { return teamBlockSize + rel }
	b[at(teamShotsBlocked)], b[at(teamClearCutChances)], b[at(teamCorners)] = s.ShotsBlocked, s.ClearCutChances, s.Corners
	b[at(teamFouls)], b[at(teamYellowCards)] = s.Fouls, s.YellowCards
	for rel, v := range map[int]uint16{
		teamPasses: s.Passes, teamPassesCompleted: s.PassesCompleted, teamTackles: s.Tackles, teamTacklesWon: s.TacklesWon,
		teamHeaders: s.Headers, teamHeadersWon: s.HeadersWon,
	} {
		binary.LittleEndian.PutUint16(b[at(rel):], v)
	}
	if th := s.PassesCompletedByThird; th != nil {
		for i, v := range th {
			binary.LittleEndian.PutUint16(b[at(teamThirdsCompleted+4*i):], v)
		}
	}
	return b
}

// sidesObjectBytes encodes an object with a team block per side.
func sidesObjectBytes(home, away []MatchPlayer, teams [2]TeamStats) []byte {
	b := append([]byte{1, 0, 1, 0x3c, 0x1f}, make([]byte, 50)...)
	for side, players := range [][]MatchPlayer{home, away} {
		b = append(b, teamBlockBytes(teams[side])...)
		for _, p := range players {
			b = append(b, matchRecordBytes(p)...)
		}
	}
	return append(b, make([]byte, 100)...)
}

var testTeams = [2]TeamStats{
	{Passes: 468, PassesCompleted: 402, PassesCompletedByThird: &[3]uint16{118, 215, 69}, Tackles: 6, TacklesWon: 4, Headers: 26, HeadersWon: 16, ShotsBlocked: 1, ClearCutChances: 5, Corners: 6, Fouls: 13, YellowCards: 3},
	{Passes: 500, PassesCompleted: 435, Tackles: 12, TacklesWon: 7, Headers: 25, HeadersWon: 8, ShotsBlocked: 4, ClearCutChances: 3, Corners: 4, Fouls: 7, YellowCards: 2},
}

func TestReadMatchObjectTeamStats(t *testing.T) {
	home, away := testSquad(0, 100), testSquad(1, 200)
	players, teams, err := readMatchObject(sidesObjectBytes(home, away, testTeams))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(players, append(home, away...)) || !reflect.DeepEqual(teams, &testTeams) {
		t.Errorf("teams = %+v, want %+v", teams, testTeams)
	}
}

// TestReadMatchObjectRejectsBadTeamStats checks that completed > attempted
// drops the team stats but keeps the players.
func TestReadMatchObjectRejectsBadTeamStats(t *testing.T) {
	home, away := testSquad(0, 100), testSquad(1, 200)
	for name, change := range map[string]func(*TeamStats){
		"more tackles won than made": func(s *TeamStats) { s.TacklesWon = 13 },
		"more headers won than made": func(s *TeamStats) { s.HeadersWon = 26 },
		"blocked shots out of range": func(s *TeamStats) { s.ShotsBlocked = 41 },
		"chances out of range":       func(s *TeamStats) { s.ClearCutChances = 41 },
		"yellow cards out of range":  func(s *TeamStats) { s.YellowCards = 41 },
		"more passes completed":      func(s *TeamStats) { s.PassesCompleted = 501 },
		"fouls out of range":         func(s *TeamStats) { s.Fouls = 200 },
		"corners out of range":       func(s *TeamStats) { s.Corners = 41 },
		"passes out of range":        func(s *TeamStats) { s.Passes, s.PassesCompleted = 9000, 9000 },
		"tackles out of range":       func(s *TeamStats) { s.Tackles, s.TacklesWon = 201, 0 },
		"headers out of range":       func(s *TeamStats) { s.Headers, s.HeadersWon = 201, 0 },
	} {
		bad := testTeams
		change(&bad[1])
		players, teams, err := readMatchObject(sidesObjectBytes(home, away, bad))
		if err != nil || len(players) != len(home)+len(away) || teams != nil {
			t.Errorf("%s: players %d, teams %+v, err %v; want the players and no team stats", name, len(players), teams, err)
		}
	}
}

// TestReadMatchObjectThirdsOnlyWhenSplit checks that completed passes by
// third are kept only when they add up to the completed total (objects
// without a setup section hold zeros).
func TestReadMatchObjectThirdsOnlyWhenSplit(t *testing.T) {
	teams := testTeams
	zeros := [3]uint16{}
	teams[0].PassesCompletedByThird = &zeros
	teams[1].Passes, teams[1].PassesCompleted = 0, 0 // a side with no passes: zeros add up, but there is no split
	_, got, err := readMatchObject(sidesObjectBytes(testSquad(0, 100), testSquad(1, 200), teams))
	if err != nil || got == nil || got[0].PassesCompletedByThird != nil || got[0].PassesCompleted != 402 || got[1].PassesCompletedByThird != nil {
		t.Errorf("teams = %+v, %v; want both sides without the split", got, err)
	}
}

// TestReadMatchObjectTeamStatsNeedRoom checks that team stats are not read
// when the home records start less than a team block into the object or
// the away block would overlap the home records; the players still are.
func TestReadMatchObjectTeamStatsNeedRoom(t *testing.T) {
	home, away := testSquad(0, 100), testSquad(1, 200)
	early := append([]byte{1, 0, 1}, teamBlockBytes(testTeams[0])[teamBlockSize-100:]...) // 103 bytes before the home records
	for _, p := range home {
		early = append(early, matchRecordBytes(p)...)
	}
	early = append(early, teamBlockBytes(testTeams[1])...)
	for _, p := range away {
		early = append(early, matchRecordBytes(p)...)
	}
	// The away block starts 6 bytes inside the last home record, whose last
	// bytes are 0: the stats read as plausible, so only the overlap check
	// can reject them.
	overlap := append([]byte{1, 0, 1}, teamBlockBytes(testTeams[0])...)
	for _, p := range home {
		overlap = append(overlap, matchRecordBytes(p)...)
	}
	overlap = append(overlap, teamBlockBytes(testTeams[1])[6:]...)
	for _, p := range away {
		overlap = append(overlap, matchRecordBytes(p)...)
	}
	for name, obj := range map[string][]byte{"home too early": early, "away overlaps home": overlap} {
		players, teams, err := readMatchObject(obj)
		if err != nil || len(players) != len(home)+len(away) || teams != nil {
			t.Errorf("%s: players %d, teams %+v, err %v; want the players and no team stats", name, len(players), teams, err)
		}
	}
}

func TestMatchRecordReadsStats(t *testing.T) {
	want := MatchPlayer{Person: 7, Shirt: 23, Side: 1, StartPositions: 0x10, EndPositions: 0x10, XG: 0.4, Rating: 711, Stats: MatchPlayerStats{
		Shots: 1, ShotsOnTarget: 2, ClearCutChances: 3, Fouls: 4, YellowCards: 5, RedCards: 6, SubbedOffMinute: 7, SubbedOnMinute: 8,
		Passes: 73, PassesCompleted: 65, KeyPasses: 9, Crosses: 10, CrossesCompleted: 11, Tackles: 12, TacklesWon: 13, KeyTackles: 14,
		Interceptions: 15, DistanceMetres: 9183.5, PossessionWon: 16, Clearances: 17, ProgressivePasses: 18, Sprints: 19,
	}}

	got, ok := matchRecord(matchRecordBytes(want))

	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("matchRecord = %+v, %v; want %+v", got, ok, want)
	}
}

// TestReadMatchObjectDistanceUnits checks that distances read as metres:
// objects with a setup section store metres, the others kilometres.
func TestReadMatchObjectDistanceUnits(t *testing.T) {
	for name, tc := range map[string]struct {
		stored float32
		setup  bool
	}{"metres with a setup section": {9500, true}, "kilometres without one": {9.5, false}} {
		t.Run(name, func(t *testing.T) {
			players := testSides(100, 200)
			players[0].Stats.DistanceMetres = tc.stored
			obj := matchObjectBytes(players)
			if tc.setup {
				obj = append(obj, "FM27.fm27-rel-03-launch-freepreview.2493426"...)
			}

			got, _, err := readMatchObject(obj)

			if err != nil || got[0].Stats.DistanceMetres != 9500 {
				t.Errorf("distance = %v (err %v), want 9500 metres", got[0].Stats.DistanceMetres, err)
			}
		})
	}
}

// TestTestSave27MatchPlayerStats checks match 12438 (FK TransINVEST 1–1
// Kauno Žalgiris, 2026-11-08) against the in-game player stats, and on
// every object that the players' fouls, yellow cards and tackles add up to
// the team totals.
func TestTestSave27MatchPlayerStats(t *testing.T) {
	objs, err := ReadMatchObjects(openTestSaveAt(t, testSave27MatchesPath))
	if err != nil {
		t.Fatal(err)
	}
	byPerson := map[int32]MatchPlayer{}
	unread := 0
	for _, o := range objs {
		if o.Players == nil {
			unread++
		}
		if o.MatchID == 12438 {
			for _, p := range o.Players {
				byPerson[p.Person] = p
			}
			want := [2]sideSums{
				{shots: 6, onTarget: 3, progressive: 33, fouls: 8, yellow: 1, tackles: 10, tacklesWon: 5, clearCut: 0, metres: 114_900},
				{shots: 12, onTarget: 7, progressive: 55, fouls: 12, yellow: 1, tackles: 12, tacklesWon: 10, clearCut: 2, metres: 117_200},
			}
			if got := matchSums(o.Players); got != want {
				t.Errorf("match 12438 sums = %+v, want the report's %+v", got, want)
			}
		}
		checkTeamSums(t, o)
	}
	if len(objs) != 3872 || unread != 0 {
		t.Errorf("%d objects, %d unreadable; want 3872 and 0", len(objs), unread)
	}
	for name, tc := range map[string]struct {
		person int32
		want   MatchPlayerStats
		km     float64
	}{
		"Hernández": {29473, MatchPlayerStats{Passes: 73, PassesCompleted: 65, Tackles: 4, TacklesWon: 4, KeyTackles: 2, Interceptions: 6, Clearances: 2, PossessionWon: 16, Sprints: 4}, 9.2},
		"Moutachy":  {21036, MatchPlayerStats{Passes: 47, PassesCompleted: 42, KeyPasses: 3, Crosses: 7, CrossesCompleted: 3, Tackles: 2, TacklesWon: 1, Interceptions: 3, Clearances: 1, PossessionWon: 7, Sprints: 4}, 12.1},
		"Kalik":     {8813, MatchPlayerStats{ClearCutChances: 1, SubbedOffMinute: 65, Passes: 70, PassesCompleted: 63, Crosses: 1, Interceptions: 2, PossessionWon: 3, Sprints: 4}, 8.9},
		"Šluta":     {20782, MatchPlayerStats{Fouls: 2, YellowCards: 1, SubbedOnMinute: 33, Passes: 28, PassesCompleted: 24, Tackles: 1, Interceptions: 1, Clearances: 1, PossessionWon: 5, Sprints: 2}, 7.9},
		"Kloniūnas": {20616, MatchPlayerStats{SubbedOffMinute: 33, Passes: 21, PassesCompleted: 18, Clearances: 1, PossessionWon: 3, Sprints: 1}, 4.1},
	} {
		got := byPerson[tc.person].Stats
		km := math.Round(float64(got.DistanceMetres)/100) / 10
		got.DistanceMetres = 0
		got.Shots, got.ShotsOnTarget, got.ProgressivePasses = 0, 0, 0 // checked as team sums above
		if !reflect.DeepEqual(got, tc.want) || km != tc.km {
			t.Errorf("%s = %+v, %.1f km; want %+v, %.1f km", name, got, km, tc.want, tc.km)
		}
	}
}

type sideSums struct {
	shots, onTarget, progressive, fouls, yellow, tackles, tacklesWon, clearCut int
	metres                                                                     int // to the nearest 100
}

func matchSums(ps []MatchPlayer) [2]sideSums {
	var out [2]sideSums
	var metres [2]float64
	for _, p := range ps {
		s, st := &out[p.Side], p.Stats
		s.shots += int(st.Shots)
		s.onTarget += int(st.ShotsOnTarget)
		s.progressive += int(st.ProgressivePasses)
		s.fouls += int(st.Fouls)
		s.yellow += int(st.YellowCards)
		s.tackles += int(st.Tackles)
		s.tacklesWon += int(st.TacklesWon)
		s.clearCut += int(st.ClearCutChances)
		metres[p.Side] += float64(st.DistanceMetres)
	}
	for side := range out {
		out[side].metres = int(math.Round(metres[side]/100) * 100)
	}
	return out
}

// checkTeamSums checks that o's players add up to its team totals.
func checkTeamSums(t *testing.T, o MatchObject) {
	t.Helper()
	if o.TeamStats == nil {
		return
	}
	sums := matchSums(o.Players)
	for side, ts := range o.TeamStats {
		s := sums[side]
		if s.fouls != int(ts.Fouls) || s.yellow != int(ts.YellowCards) || s.tackles != int(ts.Tackles) || s.tacklesWon != int(ts.TacklesWon) || s.clearCut != int(ts.ClearCutChances) {
			t.Errorf("match %d side %d: player sums %+v, team %+v", o.MatchID, side, s, ts)
		}
		if s.metres < 50_000 {
			t.Errorf("match %d side %d: players ran %d metres", o.MatchID, side, s.metres)
		}
	}
}
