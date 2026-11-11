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
