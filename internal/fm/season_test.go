package fm

import (
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func TestSeasonStatsAreVisibleWhateverTheScouting(t *testing.T) {
	g := fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		6: {Club: &savefile.MatchStats{RatingSum: 1185, Starts: 16, SubApps: 2, Rated: 18, Assists: 1, RedCards: 1, YellowCards: 7},
			League: &savefile.MatchStats{Starts: 12, Minutes: 1000, Detail: savefile.MatchDetail{Passes: 300}}},
		7: {Club: &savefile.MatchStats{SubApps: 3}},
		8: {Club: &savefile.MatchStats{}}, // no count: no season
	})
	tests := []struct {
		uid  uint32
		want *SeasonStats
	}{
		{6, &SeasonStats{Starts: 16, SubApps: 2, Assists: 1, AverageRating: ptr(6.58), YellowCards: 7, RedCards: 1}},
		{7, &SeasonStats{SubApps: 3}}, // no rated match: no average
		{1, nil},
		{8, nil},
	}
	wantDetail := []SeasonDetail{
		{Competition: "club", SeasonStats: SeasonStats{Starts: 16, SubApps: 2, Assists: 1, AverageRating: ptr(6.58), YellowCards: 7, RedCards: 1}},
		{Competition: "league", SeasonStats: SeasonStats{Starts: 12, Minutes: 1000}, Passes: 300},
	}
	for _, fog := range []bool{true, false} {
		v := g.View(fog)
		for _, tt := range tests {
			p, _ := v.Profile(tt.uid)
			if !reflect.DeepEqual(p.Season, tt.want) {
				t.Errorf("fog %v: uid %d season = %+v, want %+v", fog, tt.uid, p.Season, tt.want)
			}
			if tt.uid == 6 && !reflect.DeepEqual(p.SeasonDetail, wantDetail) {
				t.Errorf("fog %v: season detail = %+v, want %+v", fog, p.SeasonDetail, wantDetail)
			}
		}
	}
	if p, _ := g.View(true).Profile(6); p.Scouting != ScoutingRequired {
		t.Errorf("uid 6 scouting = %s; the test needs a player fog of war hides", p.Scouting)
	}
}

func TestSeasonAverageRatingRoundsToHundredths(t *testing.T) {
	for _, tt := range []struct {
		sum   uint16
		rated uint8
		want  float64
	}{{1185, 18, 6.58}, {1547, 21, 7.37}, {700, 1, 70}, {67, 1, 6.7}} {
		s := seasonStats(savefile.MatchStats{RatingSum: tt.sum, Starts: tt.rated, Rated: tt.rated})
		if s.AverageRating == nil || *s.AverageRating != tt.want {
			t.Errorf("%d over %d = %v, want %v", tt.sum, tt.rated, s.AverageRating, tt.want)
		}
	}
}

func TestSeasonStatsJSONLeavesOutZeros(t *testing.T) {
	b, err := json.Marshal(seasonStats(savefile.MatchStats{RatingSum: 1185, Starts: 16, SubApps: 2, Rated: 18, Assists: 1, RedCards: 1, YellowCards: 7}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"starts":16,"sub_appearances":2,"assists":1,"average_rating":6.58,"yellow_cards":7,"red_cards":1}`
	if got := string(b); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	if b, _ := json.Marshal(Profile{}); strings.Contains(string(b), "season") {
		t.Errorf("profile without season stats: %s", b)
	}
}

func TestSeasonStatsAreNilWithoutAnyCount(t *testing.T) {
	if s := seasonStats(savefile.MatchStats{}); s != nil {
		t.Errorf("all-zero season = %+v, want nil", s)
	}
}

// matkovicLeague is Anton Matković's Ligue 1 row, 2035-12-09 (26.3.2).
var matkovicLeague = savefile.MatchStats{
	RatingSum: 1077, Minutes: 1088, Starts: 13, SubApps: 2, Rated: 15, Goals: 7, Assists: 4, PlayerOfTheMatch: 2,
	Detail: savefile.MatchDetail{
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

func TestSeasonStatsCarryMinutesAndPlayerOfTheMatch(t *testing.T) {
	s := seasonStats(savefile.MatchStats{Minutes: 1608, PlayerOfTheMatch: 3, Starts: 20})
	if s.Minutes != 1608 || s.PlayerOfTheMatch != 3 {
		t.Fatalf("season = %+v", s)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"minutes":1608`, `"player_of_the_match":3`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("%s lacks %s", b, want)
		}
	}
}

func TestProfileListsSeasonDetailByCompetition(t *testing.T) {
	club := savefile.MatchStats{Minutes: 1608, Starts: 20, SubApps: 2, PenaltiesTaken: 1, PenaltiesScored: 1, PlayerOfTheMatch: 3}
	cups := savefile.MatchStats{Minutes: 180, Starts: 2, Goals: 2, Detail: savefile.MatchDetail{Fouls: 3}}
	continental := savefile.MatchStats{Minutes: 540, Starts: 6, Goals: 3, Detail: savefile.MatchDetail{Fouls: 4}}
	friendlies := savefile.MatchStats{Minutes: 90, Starts: 1, Goals: 1}
	internationals := savefile.MatchStats{Minutes: 2, SubApps: 1}
	g := fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		5: {Club: &club, League: &matkovicLeague, Cups: &cups, Continental: &continental, Friendlies: &friendlies,
			Internationals: &internationals},
	})
	p, _ := g.View(false).Profile(5)
	var got []string
	for _, d := range p.SeasonDetail {
		got = append(got, d.Competition)
	}
	if want := []string{"club", "league", "domestic_cups", "continental", "friendlies", "internationals"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("competitions = %v, want %v", got, want)
	}
	if c := p.SeasonDetail[0]; c.PenaltiesTaken != 1 || c.PenaltiesScored != 1 || c.PlayerOfTheMatch != 3 || c.Minutes != 1608 {
		t.Errorf("club row = %+v", c)
	}
	for _, w := range []SeasonDetail{
		{Competition: "domestic_cups", SeasonStats: SeasonStats{Starts: 2, Goals: 2, Minutes: 180}, Fouls: 3},
		{Competition: "continental", SeasonStats: SeasonStats{Starts: 6, Goals: 3, Minutes: 540}, Fouls: 4},
		{Competition: "friendlies", SeasonStats: SeasonStats{Starts: 1, Goals: 1, Minutes: 90}},
		{Competition: "internationals", SeasonStats: SeasonStats{SubApps: 1, Minutes: 2}},
	} {
		i := slices.IndexFunc(p.SeasonDetail, func(d SeasonDetail) bool { return d.Competition == w.Competition })
		if !reflect.DeepEqual(p.SeasonDetail[i], w) {
			t.Errorf("%s row = %+v\nwant %+v", w.Competition, p.SeasonDetail[i], w)
		}
	}
	want := SeasonDetail{
		Competition: "league",
		SeasonStats: SeasonStats{
			Starts: 13, SubApps: 2, Goals: 7, Assists: 4, AverageRating: ptr(7.18), Minutes: 1088, PlayerOfTheMatch: 2,
		},
		MatchesWon: 8, Passes: 386, PassesCompleted: 320, ProgressivePasses: 37, KeyPasses: 31,
		Crosses: 17, CrossesCompleted: 10, ChancesCreated: 9, Shots: 29, ShotsOnTarget: 17, ShotsBlocked: 1,
		Dribbles: 21, Offsides: 4, Fouls: 11, Tackles: 21, TacklesWon: 17, Headers: 62, HeadersWon: 24, KeyHeaders: 3,
		Interceptions: 28, Clearances: 9, PossessionWon: 77, PossessionLost: 120, Sprints: 209,
		ExpectedGoals: 4.26, ExpectedAssists: 2.68, DistanceKm: 160.6,
		MatchesLost: 1, TeamGoals: 31, TeamConceded: 16, OpenPlayKeyPasses: 31, OpenPlayCrosses: 16,
		OpenPlayCrossesCompleted: 10, ShotsOutsideBox: 7, FoulsAgainst: 11, HeadersLost: 38, Blocks: 2,
		Pressures: 84, PressuresCompleted: 20, NonPenaltyXG: 4.26,
	}
	if !reflect.DeepEqual(p.SeasonDetail[1], want) {
		t.Errorf("league row = %+v\nwant %+v", p.SeasonDetail[1], want)
	}
	b, _ := json.Marshal(p.SeasonDetail[1])
	for _, s := range []string{`"competition":"league"`, `"expected_assists":2.68`, `"distance_km":160.6`, `"player_of_the_match":2`, `"key_passes":31`,
		`"non_penalty_expected_goals":4.26`, `"pressures":84`, `"open_play_crosses_completed":10`, `"headers_lost":38`, `"matches_lost":1`} {
		if !strings.Contains(string(b), s) {
			t.Errorf("%s lacks %s", b, s)
		}
	}
	if strings.Contains(string(b), "key_tackles") {
		t.Errorf("zero field in %s", b)
	}
}

func TestSeasonDetailWithFriendliesOnly(t *testing.T) {
	g := fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		5: {Friendlies: &savefile.MatchStats{Starts: 2, Minutes: 150, Detail: savefile.MatchDetail{Passes: 40}}},
	})
	p, _ := g.View(false).Profile(5)
	if p.Season != nil {
		t.Errorf("season = %+v, want nil", p.Season)
	}
	want := []SeasonDetail{{Competition: "friendlies", SeasonStats: SeasonStats{Starts: 2, Minutes: 150}, Passes: 40}}
	if !reflect.DeepEqual(p.SeasonDetail, want) {
		t.Errorf("detail = %+v, want %+v", p.SeasonDetail, want)
	}
}

func TestSeasonDetailDropsNegativeValues(t *testing.T) {
	d := seasonDetail("league", savefile.MatchStats{Starts: 1, Detail: savefile.MatchDetail{Fouls: -5, ExpectedGoals: -27, Distance: -3, Passes: 7}}, false)
	if d.Fouls != 0 || d.ExpectedGoals != 0 || d.DistanceKm != 0 || d.Passes != 7 {
		t.Errorf("detail = %+v", d)
	}
	b, _ := json.Marshal(d)
	for _, s := range []string{"fouls", "expected_goals", "distance_km", "-"} {
		if strings.Contains(string(b), s) {
			t.Errorf("%s contains %q", b, s)
		}
	}
}

func TestSeasonDetailLeavesOutScopesWithoutACount(t *testing.T) {
	g := fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		5: {Club: &savefile.MatchStats{}, League: &savefile.MatchStats{Detail: savefile.MatchDetail{Fouls: -2}},
			Friendlies: &savefile.MatchStats{Starts: 1}},
		6: {Club: &savefile.MatchStats{}},
	})
	p, _ := g.View(false).Profile(5)
	want := []SeasonDetail{{Competition: "friendlies", SeasonStats: SeasonStats{Starts: 1}}}
	if !reflect.DeepEqual(p.SeasonDetail, want) {
		t.Errorf("detail = %+v, want %+v", p.SeasonDetail, want)
	}
	if p, _ := g.View(false).Profile(6); p.SeasonDetail != nil {
		t.Errorf("all-zero detail = %+v, want nil", p.SeasonDetail)
	}
}

// samiLeague is Bruno Sami's Ligue 1 row, 2035-12-09 (26.3.2): the game
// writes keeper stats into seven of the outfield slots.
var samiLeague = savefile.MatchStats{
	RatingSum: 1077, Minutes: 1440, Starts: 16, Rated: 16, PlayerOfTheMatch: 1, GoalsConceded: 22, CleanSheets: 3,
	Detail: savefile.MatchDetail{
		TeamGoals: 35, TeamConceded: 22, MatchesWon: 8, MatchesLost: 2, Passes: 327, PassesCompleted: 317,
		Tackles: 1, TacklesWon: 1, Headers: 27, HeadersWon: 25, MistakesLeadingToGoal: 1, Distance: 1049,
		Interceptions: 2, PossessionLost: 11, PossessionWon: 109, Clearances: 6,
		ProgressivePasses: 7, OpenPlayCrossesAttempted: 6, OpenPlayCrossesCompleted: 1238, Blocks: 79,
	},
}

func TestSeasonDetailSplitsKeeperSlots(t *testing.T) {
	keeper := seasonDetail("league", samiLeague, true)
	want := SeasonDetail{
		Competition: "league", Goalkeeper: true,
		SeasonStats: SeasonStats{Starts: 16, AverageRating: ptr(6.73), Minutes: 1440, PlayerOfTheMatch: 1},
		MatchesWon:  8, MatchesLost: 2, TeamGoals: 35, TeamConceded: 22, Passes: 327, PassesCompleted: 317,
		ProgressivePasses: 7, Interceptions: 2, Clearances: 6, PossessionWon: 109, PossessionLost: 11,
		MistakesLeadingToGoal: 1, DistanceKm: 104.9,
		GoalsConceded: 22, CleanSheets: 3, SavesHeld: 27, SavesParried: 25,
		SavesTipped: 6, PenaltiesFaced: 1, PenaltiesSaved: 1, ExpectedGoalsPrevented: -9.62,
	}
	if !reflect.DeepEqual(keeper, want) {
		t.Errorf("keeper row = %+v\nwant %+v", keeper, want)
	}
	b, _ := json.Marshal(keeper)
	for _, s := range []string{`"goalkeeper":true`, `"saves_held":27`, `"expected_goals_prevented":-9.62`, `"goals_conceded":22`, `"clean_sheets":3`} {
		if !strings.Contains(string(b), s) {
			t.Errorf("%s lacks %s", b, s)
		}
	}
	for _, s := range []string{"headers", "headers_won", "tackles", "tackles_won", "blocks", "open_play_crosses_completed", "saves", "shots_on_target_faced"} {
		if strings.Contains(string(b), `"`+s+`"`) {
			t.Errorf("keeper row has %s: %s", s, b)
		}
	}

	// The same bytes for an outfield player keep their outfield meaning.
	outfield := seasonDetail("league", samiLeague, false)
	if outfield.Headers != 27 || outfield.HeadersWon != 25 || outfield.Tackles != 1 || outfield.Blocks != 79 ||
		outfield.OpenPlayCrossesCompleted != 1238 || outfield.SavesHeld != 0 || outfield.GoalsConceded != 0 ||
		outfield.ExpectedGoalsPrevented != 0 || outfield.Goalkeeper {
		t.Errorf("outfield row = %+v", outfield)
	}
	if b, _ := json.Marshal(outfield); strings.Contains(string(b), "goalkeeper") {
		t.Errorf("outfield row has the goalkeeper flag: %s", b)
	}
}

func TestProfileUsesTheKeeperRule(t *testing.T) {
	stats := samiLeague
	for _, tt := range []struct {
		gk     uint8
		keeper bool
		held   int
	}{{20, true, 27}, {15, true, 27}, {14, false, 0}, {1, false, 0}} {
		p := rawPlayer(9, "Bruno", "Sami")
		p.Index, p.Positions[savefile.GKPosition] = 9, tt.gk
		g := NewGame(Info{Date: gameDate}, []savefile.Player{p}, testWorld,
			PlayerExtras{Season: map[uint32]savefile.SeasonRecord{9: {League: &stats}}})
		prof, _ := g.View(false).Profile(9)
		if len(prof.SeasonDetail) != 1 || prof.SeasonDetail[0].Goalkeeper != tt.keeper || prof.SeasonDetail[0].SavesHeld != tt.held {
			t.Errorf("GK %d: detail = %+v, want goalkeeper %v", tt.gk, prof.SeasonDetail, tt.keeper)
		}
	}
}

func TestSeasonDetailLeavesOutAKeeperScopeWithoutACount(t *testing.T) {
	if got := seasonDetails(savefile.SeasonRecord{League: &savefile.MatchStats{}}, true); got != nil {
		t.Errorf("all-zero keeper detail = %+v, want nil", got)
	}
}

func TestSearchSortsBySeasonStats(t *testing.T) {
	g := fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		1: {Club: &savefile.MatchStats{Goals: 3, Assists: 5, Starts: 10, Minutes: 900, Rated: 10, RatingSum: 700, PlayerOfTheMatch: 1}},
		5: {Club: &savefile.MatchStats{Goals: 7, Assists: 1, Starts: 5, SubApps: 3, Minutes: 500, Rated: 8, RatingSum: 600, PlayerOfTheMatch: 2}},
		6: {Club: &savefile.MatchStats{SubApps: 2, Minutes: 30}}, // no rated match
	})
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"goals", Filter{SortBy: "season_goals", Limit: 2}, []uint32{5, 1}},
		{"case", Filter{SortBy: "SEASON_GOALS", Limit: 2}, []uint32{5, 1}},
		{"assists", Filter{SortBy: "season_assists", Limit: 2}, []uint32{1, 5}},
		{"appearances", Filter{SortBy: "season_appearances", Limit: 3}, []uint32{1, 5, 6}},
		{"minutes ascending", Filter{SortBy: "season_minutes", Ascending: true, MinSeasonMinutes: 1}, []uint32{6, 5, 1}},
		// Players without a rated match come last, most reputable first.
		{"average rating", Filter{SortBy: "season_average_rating", Limit: 3}, []uint32{5, 1, 4}},
		{"player of the match", Filter{SortBy: "season_player_of_the_match", Limit: 2}, []uint32{5, 1}},
		{"min minutes", Filter{MinSeasonMinutes: 600}, []uint32{1}},
		{"min minutes is inclusive", Filter{MinSeasonMinutes: 500, SortBy: "season_minutes"}, []uint32{1, 5}},
		{"average rating ascending", Filter{SortBy: "season_average_rating", Ascending: true, Limit: 3}, []uint32{1, 5, 4}},
	}
	for _, fog := range []bool{false, true} {
		for _, tt := range tests {
			res, err := g.View(fog).Search(tt.f)
			if err != nil {
				t.Fatalf("fog %v, %s: %v", fog, tt.name, err)
			}
			var got []uint32
			for _, p := range res.Players {
				got = append(got, p.UID)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("fog %v, %s: got %v, want %v", fog, tt.name, got, tt.want)
			}
		}
	}
}

func TestSeasonSearchNeedsSeasonStats(t *testing.T) {
	for _, f := range []Filter{{SortBy: "season_goals"}, {MinSeasonMinutes: 90}} {
		_, err := fogGame().View(false).Search(f)
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "season stats") || !strings.Contains(err.Error(), "min_season_minutes") {
			t.Errorf("Search(%+v) without season stats: err = %v, want ErrInvalidFilter naming season stats", f, err)
		}
		if _, err := fogGameWithSeason(map[uint32]savefile.SeasonRecord{}).View(false).Search(f); err != nil {
			t.Errorf("Search(%+v) with no matches yet: %v", f, err)
		}
	}
}

func per90Game() *Game {
	club := func(minutes uint16, progressive, npxg100 int16) savefile.SeasonRecord {
		return savefile.SeasonRecord{Club: &savefile.MatchStats{Starts: 1, Minutes: minutes, Detail: savefile.MatchDetail{ProgressivePasses: progressive, NonPenaltyXG: npxg100}}}
	}
	return fogGameWithSeason(map[uint32]savefile.SeasonRecord{
		1: club(900, 45, 270), // 4.5 and 0.27 per 90
		5: club(500, 30, 300), // 5.4 and 0.54
		6: club(30, 10, 0),    // 30 per 90, under the default minimum minutes
		7: club(450, 20, 0),   // 4.0, exactly the default minimum
	})
}

func TestSearchSortsAndFiltersPer90(t *testing.T) {
	g := per90Game()
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"sort leaves out players under 450 minutes", Filter{SortBy: "per90_progressive_passes"}, []uint32{5, 1, 7}},
		{"min_season_minutes replaces the default", Filter{SortBy: "per90_progressive_passes", MinSeasonMinutes: 1}, []uint32{6, 5, 1, 7}},
		{"ascending", Filter{SortBy: "PER90_progressive_passes", Ascending: true}, []uint32{7, 1, 5}},
		{"minimum", Filter{MinPer90: map[string]float64{"non_penalty_expected_goals": 0.3}}, []uint32{5}},
		{"minimum is inclusive", Filter{MinPer90: map[string]float64{"progressive_passes": 4}, SortBy: "per90_progressive_passes"}, []uint32{5, 1, 7}},
		{"minimum with minutes", Filter{MinPer90: map[string]float64{"non_penalty_expected_goals": 0.2}, MinSeasonMinutes: 600}, []uint32{1}},
	}
	for _, fog := range []bool{false, true} {
		for _, tt := range tests {
			res, err := g.View(fog).Search(tt.f)
			if err != nil {
				t.Fatalf("fog %v, %s: %v", fog, tt.name, err)
			}
			if got := uids(res.Players); !slices.Equal(got, tt.want) {
				t.Errorf("fog %v, %s: got %v, want %v", fog, tt.name, got, tt.want)
			}
		}
	}
}

func TestSearchRowsCarryTheirPer90Values(t *testing.T) {
	res, err := per90Game().View(false).Search(Filter{SortBy: "per90_progressive_passes", MinPer90: map[string]float64{"non_penalty_expected_goals": 0.3}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"progressive_passes": 5.4, "non_penalty_expected_goals": 0.54}
	if len(res.Players) != 1 || !reflect.DeepEqual(res.Players[0].Per90, want) {
		t.Errorf("rows = %+v, want one row with per90 %v", res.Players, want)
	}
	res, _ = per90Game().View(false).Search(Filter{})
	for _, p := range res.Players {
		if p.Per90 != nil {
			t.Errorf("uid %d per90 = %v without a per-90 sort or filter", p.UID, p.Per90)
		}
	}
}

func TestPer90SearchRejectsBadNames(t *testing.T) {
	for _, f := range []Filter{
		{SortBy: "per90_goalz"},
		{SortBy: "per90_minutes"},
		{MinPer90: map[string]float64{"goalz": 1}},
	} {
		_, err := per90Game().View(false).Search(f)
		if !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("Search(%+v): err = %v, want ErrInvalidFilter", f, err)
		}
	}
	_, err := fogGame().View(false).Search(Filter{MinPer90: map[string]float64{"goals": 0.5}})
	if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "season stats") {
		t.Errorf("per-90 filter without season stats: err = %v", err)
	}
}

func TestPer90LeavesOutStatsOfTheOtherSide(t *testing.T) {
	keeper := rawPlayer(1, "Keeper", "One")
	keeper.Index, keeper.Positions = 1, [15]uint8{20}
	outfield := rawPlayer(2, "Defender", "Two")
	outfield.Index = 2
	g := NewGame(Info{Date: gameDate}, []savefile.Player{keeper, outfield}, testWorld, PlayerExtras{Season: map[uint32]savefile.SeasonRecord{
		1: {Club: &savefile.MatchStats{Starts: 10, Minutes: 900, GoalsConceded: 10}},
		2: {Club: &savefile.MatchStats{Starts: 10, Minutes: 900, Detail: savefile.MatchDetail{Tackles: 20}}},
	}})
	v := g.View(false)
	res, err := v.Search(Filter{Gender: "any", SortBy: "per90_goals_conceded", Ascending: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{1, 2}) || len(res.Players[1].Per90) > 0 {
		t.Errorf("fewest conceded: %v with per90 %v, %v; want the keeper first and no outfield value", got, res.Players[0].Per90, res.Players[1].Per90)
	}
	res, err = v.Search(Filter{Gender: "any", MinPer90: map[string]float64{"tackles": 0}})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{2}) {
		t.Errorf("tackles minimum: got %v, want only the outfielder", got)
	}
}

func TestPer90AcceptsNegativeMinimums(t *testing.T) {
	if _, err := per90Game().View(false).Search(Filter{MinPer90: map[string]float64{"expected_goals_prevented": -0.5}}); err != nil {
		t.Errorf("negative minimum: %v", err)
	}
}

func TestPer90FieldsAreTheSeasonDetailCounts(t *testing.T) {
	want := []string{
		"assists", "blocks", "chances_created", "clean_sheets", "clearances", "crosses", "crosses_completed",
		"distance_km", "dribbles", "expected_assists", "expected_goals", "expected_goals_prevented",
		"fouls", "fouls_against", "free_kick_shots", "goals", "goals_conceded", "goals_from_direct_free_kicks",
		"goals_outside_box", "headers", "headers_lost", "headers_won", "interceptions", "key_headers",
		"key_passes", "key_tackles", "matches_lost", "matches_won", "mistakes_leading_to_goal",
		"non_penalty_expected_goals", "offsides", "open_play_crosses", "open_play_crosses_completed",
		"open_play_key_passes", "passes", "passes_completed", "penalties_faced", "penalties_saved",
		"penalties_scored", "penalties_taken", "player_of_the_match", "possession_lost", "possession_won",
		"pressures", "pressures_completed", "progressive_passes", "red_cards", "saves_held", "saves_parried",
		"saves_tipped", "shots", "shots_blocked", "shots_on_target", "shots_outside_box", "sprints",
		"tackles", "tackles_won", "team_conceded", "team_goals", "yellow_cards",
	}
	if got := slices.Sorted(maps.Keys(per90Fields)); !slices.Equal(got, want) {
		t.Errorf("per90Fields = %v", got)
	}
}

// TestSideStatsAreTheSlotsOneSideLeavesEmpty ties sideStats to seasonDetail:
// with every slot set, a stat is non-zero for one side only exactly when
// sideStats marks it for that side.
func TestSideStatsAreTheSlotsOneSideLeavesEmpty(t *testing.T) {
	var s savefile.MatchStats
	s.GoalsConceded, s.CleanSheets, s.Starts, s.Minutes = 1, 1, 1, 90
	detail := reflect.ValueOf(&s.Detail).Elem()
	for i := range detail.NumField() {
		detail.Field(i).SetInt(200) // above the 100 xG-faced offset, so xG prevented is non-zero
	}
	keeper, outfield := seasonDetail("x", s, true), seasonDetail("x", s, false)
	got := map[string]bool{}
	for name := range per90Fields {
		kv := reflect.ValueOf(keeper).FieldByIndex(per90Fields[name]).IsZero()
		ov := reflect.ValueOf(outfield).FieldByIndex(per90Fields[name]).IsZero()
		switch {
		case !kv && ov:
			got[name] = true
		case kv && !ov:
			got[name] = false
		}
	}
	if !maps.Equal(got, sideStats) {
		t.Errorf("one-side stats = %v, sideStats = %v", got, sideStats)
	}
}
