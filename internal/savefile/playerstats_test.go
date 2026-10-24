package savefile

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

// statsScope encodes one scope block: u8 6, the 17-byte totals, the 120-byte
// detail block.
func statsScope(s MatchStats) []byte {
	b := []byte{statsScopeVersion}
	b = binary.LittleEndian.AppendUint16(b, s.RatingSum)
	b = binary.LittleEndian.AppendUint16(b, s.Minutes)
	b = append(b, s.Starts, s.SubApps, s.Rated, s.Goals, s.Assists, s.GoalsConceded, s.PenaltiesTaken, s.PenaltiesScored,
		s.PlayerOfTheMatch, s.RedCards, s.YellowCards, s.CleanSheets, 0)
	detail := make([]byte, statsDetailSize)
	for _, f := range detailFields(&s.Detail) {
		binary.LittleEndian.PutUint16(detail[2*f.index:], uint16(*f.field)) //nolint:gosec // bit reinterpretation of i16
	}
	return append(b, detail...)
}

// statsSlots encodes scopes as u8 present + block each; nil is absent.
func statsSlots(scopes []*MatchStats) []byte {
	var b []byte
	for _, s := range scopes {
		if s == nil {
			b = append(b, 0)
			continue
		}
		b = append(append(b, 1), statsScope(*s)...)
	}
	return b
}

type statsTeam struct {
	team   uint32
	scopes [statsTeamScopes]*MatchStats
}

type statsRecord struct {
	person  uint32
	scopes  [statsScopes]*MatchStats
	teams   []statsTeam
	seasons int // 8-byte summary items
}

func (rec statsRecord) encode(b []byte) []byte {
	b = binary.LittleEndian.AppendUint32(append(b, 1), rec.person)
	b = append(b, make([]byte, statsRecordHead)...)
	b = append(b, statsSlots(rec.scopes[:])...)
	b = append(b, make([]byte, statsRecordMiddle)...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(rec.teams))) //nolint:gosec // test sizes
	for _, t := range rec.teams {
		b = binary.LittleEndian.AppendUint32(append(b, 0), t.team)
		b = append(b, statsSlots(t.scopes[:])...)
	}
	b = binary.LittleEndian.AppendUint32(b, uint32(rec.seasons)) //nolint:gosec // test sizes
	for range rec.seasons {
		b = append(b, 2, 0xa1, 0x04, 3, 18, 0, 1, 18)
	}
	b = append(b, recentRatings)
	return append(b, make([]byte, recentRatingSize*recentRatings)...)
}

func statsBody(recs ...statsRecord) []byte {
	var b []byte
	for _, rec := range recs {
		b = rec.encode(b)
	}
	return append(append(b, 0), make([]byte, statsTrailerSize)...)
}

// ashby is Chris Ashby's season in the real KŽ save (2035-12-02).
var ashby = MatchStats{RatingSum: 1185, Starts: 16, SubApps: 2, Rated: 18, Assists: 1, RedCards: 1, YellowCards: 7}

// matkovicLeague is Anton Matković's Ligue 1 row in the real save of 2035-12-09.
var matkovicLeague = MatchStats{
	RatingSum: 1077, Minutes: 1088, Starts: 13, SubApps: 2, Rated: 15, Goals: 7, Assists: 4, PlayerOfTheMatch: 2,
	Detail: MatchDetail{
		MatchesWon: 8, Passes: 386, PassesCompleted: 320, Tackles: 21, TacklesWon: 17, Headers: 62, HeadersWon: 24,
		Dribbles: 21, Fouls: 11, Shots: 29, ShotsOnTarget: 17, Distance: 1606, Offsides: 4, KeyPasses: 31,
		KeyTackles: 0, KeyHeaders: 3, Interceptions: 28, ChancesCreated: 9, PossessionLost: 120, PossessionWon: 77,
		ExpectedGoals: 426, ExpectedAssists: 268, Clearances: 9, Sprints: 209, ProgressivePasses: 37,
		Crosses: 17, CrossesCompleted: 10, ShotsBlocked: 1,
		TeamGoals: 31, TeamConceded: 16, MatchesLost: 1, FoulsAgainst: 11, HeadersLost: 38, ShotsOutsideBox: 7,
		OpenPlayKeyPasses: 31, NonPenaltyXG: 426, OpenPlayCrosses: 16, OpenPlayCrossesAttempted: 16, OpenPlayCrossesCompleted: 10,
		Blocks: 2, PressuresAttempted: 84, PressuresCompleted: 20,
	},
}

func TestParseSeasonStats(t *testing.T) {
	league := matkovicLeague
	friendly := MatchStats{RatingSum: 338, Starts: 2, SubApps: 3, Rated: 5}
	cup := MatchStats{RatingSum: 153, Starts: 2, Rated: 2, Goals: 2, Assists: 1, PenaltiesTaken: 1, PenaltiesScored: 1}
	continental := MatchStats{RatingSum: 546, Starts: 7, Rated: 7, Goals: 3, Assists: 4, PenaltiesTaken: 1}
	striker := MatchStats{RatingSum: 1547, Starts: 19, SubApps: 2, Rated: 21, Goals: 10, Assists: 7}

	var full statsRecord
	full.person = 146891
	full.scopes[0], full.scopes[1], full.scopes[seasonScope] = &friendly, &league, &ashby
	full.teams = []statsTeam{{team: 20758, scopes: [statsTeamScopes]*MatchStats{1: &league, 5: &league}}}
	full.seasons = 2

	var onlyFriendlies statsRecord
	onlyFriendlies.person = 7
	onlyFriendlies.scopes[0] = &friendly

	var other statsRecord
	other.person = 9
	other.scopes[seasonScope] = &striker

	var cups statsRecord
	cups.person = 11
	cups.scopes[2], cups.scopes[3] = &cup, &continental

	international := MatchStats{RatingSum: 476, Starts: 6, Rated: 6, Goals: 7, PenaltiesTaken: 2, PenaltiesScored: 2}
	var internationals statsRecord
	internationals.person = 13
	internationals.scopes[4] = &international

	tests := []struct {
		name string
		body []byte
		want map[uint32]SeasonRecord
	}{
		{"every part present", statsBody(full, onlyFriendlies, other), map[uint32]SeasonRecord{
			146891: {Club: &ashby, Friendlies: &friendly, League: &league},
			7:      {Friendlies: &friendly},
			9:      {Club: &striker},
		}},
		{"no season", statsBody(onlyFriendlies), map[uint32]SeasonRecord{7: {Friendlies: &friendly}}},
		{"cups and continental", statsBody(cups), map[uint32]SeasonRecord{11: {Cups: &cup, Continental: &continental}}},
		{"internationals", statsBody(internationals), map[uint32]SeasonRecord{13: {Internationals: &international}}},
		{"empty", statsBody(), map[uint32]SeasonRecord{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSeasonStats(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Slots 6 and 7 hold the calendar year's club and international matches.
func TestParseSeasonStatsIgnoresCalendarYearSlots(t *testing.T) {
	s := MatchStats{RatingSum: 70, Starts: 1, Rated: 1}
	var rec statsRecord
	rec.person = 5
	rec.scopes[6], rec.scopes[7] = &s, &s
	got, err := parseSeasonStats(statsBody(rec))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want no records", got)
	}
}

func TestParseSeasonStatsRejects(t *testing.T) {
	var rec statsRecord
	rec.person = 146891
	rec.scopes[seasonScope] = &ashby
	good := statsBody(rec)
	scopeAt := 1 + 4 + statsRecordHead + seasonScope // presence flag of the season scope

	teamCountAt := scopeAt - seasonScope + statsScopes + statsScopeSize + statsRecordMiddle
	if good[teamCountAt+8] != recentRatings {
		t.Fatalf("team count is not at %d", teamCountAt)
	}
	mutate := func(at int, v byte) []byte {
		b := append([]byte(nil), good...)
		b[at] = v
		return b
	}
	tests := []struct {
		name string
		body []byte
		want error
	}{
		{"truncated", good[:len(good)-1], ErrTruncated},
		{"trailing bytes", append(append([]byte(nil), good...), 0), ErrCorrupt},
		{"bad record flag", mutate(0, 2), ErrCorrupt},
		{"bad scope flag", mutate(scopeAt, 2), ErrCorrupt},
		{"bad scope version", mutate(scopeAt+1, 5), ErrCorrupt},
		{"rated beyond appearances", mutate(scopeAt+1+7, 19), ErrCorrupt},
		{"duplicate person", statsBody(rec, rec), ErrCorrupt},
		{"huge team count", binary.LittleEndian.AppendUint32(append([]byte(nil), good[:teamCountAt]...), 0xffffffff), ErrTruncated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseSeasonStats(tt.body); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestStatsScopeReadsFixedOffsets writes a scope block byte by byte, without
// the parser's field table, so that each field is pinned to its offset.
func TestStatsScopeReadsFixedOffsets(t *testing.T) {
	totals := make([]byte, statsTotalsSize)
	binary.LittleEndian.PutUint16(totals[0:], 1623) // rating sum
	binary.LittleEndian.PutUint16(totals[2:], 1608) // minutes
	totals[4], totals[5], totals[6] = 20, 2, 22     // starts, sub appearances, rated
	totals[7], totals[8], totals[9] = 10, 8, 99     // goals, assists, goals conceded
	totals[10], totals[11], totals[12] = 4, 3, 5    // penalties taken, penalties scored, player of the match
	totals[13], totals[14] = 1, 6                   // red, yellow cards
	totals[15], totals[16] = 98, 97                 // clean sheets, offset 16 (not read)

	// Every i16 slot holds 1000 + its index, then the fields below are
	// overwritten at their byte offsets.
	detail := make([]byte, statsDetailSize)
	for i := 0; i < statsDetailSize; i += 2 {
		binary.LittleEndian.PutUint16(detail[i:], uint16(1000+i/2)) //nolint:gosec // test values
	}
	put := func(at int, v int16) {
		binary.LittleEndian.PutUint16(detail[at:], uint16(v)) //nolint:gosec // bit reinterpretation of i16
	}
	put(4, 8)     // d2 matches won
	put(22, -5)   // d11 fouls, negative
	put(46, 1606) // d23 distance
	put(72, 426)  // d36 expected goals
	put(74, 268)  // d37 expected assists
	put(116, 1)   // d58 shots blocked
	put(94, -250) // d47, not decoded

	b := append([]byte{1, statsScopeVersion}, totals...)
	b = append(b, detail...)
	got, ok := newReader(b).statsScope()
	if !ok {
		t.Fatal("scope not read")
	}
	want := MatchStats{
		RatingSum: 1623, Minutes: 1608, Starts: 20, SubApps: 2, Rated: 22, Goals: 10, Assists: 8,
		PenaltiesTaken: 4, PenaltiesScored: 3, PlayerOfTheMatch: 5, RedCards: 1, YellowCards: 6,
		GoalsConceded: 99, CleanSheets: 98,
		Detail: MatchDetail{
			TeamGoals: 1000, TeamConceded: 1001, MatchesLost: 1003, FoulsAgainst: 1012, MistakesLeadingToGoal: 1020,
			OpenPlayCrossesAttempted: 1025, OpenPlayCrossesCompleted: 1026, Blocks: 1032, GoalsFromDirectFreeKicks: 1033,
			FreeKickShots: 1040, GoalsOutsideBox: 1041, OpenPlayCrosses: 1045, ShotsOutsideBox: 1046,
			PressuresAttempted: 1051, PressuresCompleted: 1052,
			OpenPlayKeyPasses: 1053, NonPenaltyXG: 1054, HeadersLost: 1059,
			MatchesWon: 8, Passes: 1004, PassesCompleted: 1005, Tackles: 1006, TacklesWon: 1007,
			Headers: 1008, HeadersWon: 1009, Dribbles: 1010, Fouls: -5, Shots: 1013, ShotsOnTarget: 1014,
			Distance: 1606, Offsides: 1024, KeyPasses: 1027, KeyTackles: 1028, KeyHeaders: 1029,
			Interceptions: 1030, ChancesCreated: 1031, PossessionLost: 1034, PossessionWon: 1035,
			ExpectedGoals: 426, ExpectedAssists: 268, Clearances: 1038, Sprints: 1048,
			ProgressivePasses: 1050, Crosses: 1056, CrossesCompleted: 1057, ShotsBlocked: 1,
		},
	}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// Only the kept scopes are allocated: a save holds hundreds of thousands of
// scope slots, most of them absent or not kept.
func TestParseSeasonStatsAllocatesOnlyKeptScopes(t *testing.T) {
	const players = 2000
	recs := make([]statsRecord, players)
	for i := range recs {
		recs[i].person = uint32(i) //nolint:gosec // test sizes
		recs[i].scopes[1], recs[i].scopes[seasonScope] = &matkovicLeague, &ashby
	}
	body := statsBody(recs...)
	allocs := testing.AllocsPerRun(3, func() {
		if _, err := parseSeasonStats(body); err != nil {
			t.Fatal(err)
		}
	})
	if allocs >= 3*players {
		t.Errorf("%v allocations for %d players with 2 scopes each", allocs, players)
	}
}
