package fm

import (
	"math"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// SeasonStats are a player's totals over one group of matches: as a row's
// season, the current season's club competitions (league, cups,
// continental); in a SeasonDetail, that row's competitions. Fog of war never
// hides them. Zero counts are left out of the JSON.
type SeasonStats struct {
	Starts  int `json:"starts,omitempty"`
	SubApps int `json:"sub_appearances,omitempty"`
	Goals   int `json:"goals,omitempty"`
	Assists int `json:"assists,omitempty"`
	// AverageRating is over matches with a rating, to two decimals; nil
	// when there are none.
	AverageRating    *float64 `json:"average_rating,omitempty"`
	YellowCards      int      `json:"yellow_cards,omitempty"`
	RedCards         int      `json:"red_cards,omitempty"`
	Minutes          int      `json:"minutes,omitempty"`
	PlayerOfTheMatch int      `json:"player_of_the_match,omitempty"`
}

// seasonStats converts s; nil when all counts are zero.
func seasonStats(s savefile.MatchStats) *SeasonStats {
	out := &SeasonStats{
		Starts: int(s.Starts), SubApps: int(s.SubApps), Goals: int(s.Goals), Assists: int(s.Assists),
		YellowCards: int(s.YellowCards), RedCards: int(s.RedCards),
		Minutes: int(s.Minutes), PlayerOfTheMatch: int(s.PlayerOfTheMatch),
	}
	if s.Rated > 0 {
		// RatingSum is in tenths.
		out.AverageRating = ptr(math.Round(float64(s.RatingSum)*10/float64(s.Rated)) / 100)
	}
	if *out == (SeasonStats{}) {
		return nil
	}
	return out
}

// statsKeeperRating is the GK rating from which the game records a player's
// match stats as a goalkeeper's (code 0xf10a78; verified on four saves). The
// game tests the rating per match; the server uses the current one, so a
// player whose rating crossed 15 this season would get mixed meanings (none
// seen in about 1.2 M scopes).
const statsKeeperRating = 15

// sideStats marks the SeasonDetail stats only one side records: true for
// goalkeepers, false for outfield players (see seasonDetail).
var sideStats = map[string]bool{
	"goals_conceded": true, "clean_sheets": true, "saves_held": true, "saves_parried": true, "saves_tipped": true,
	"penalties_faced": true, "penalties_saved": true, "expected_goals_prevented": true,
	"tackles": false, "tackles_won": false, "headers": false, "headers_won": false, "headers_lost": false,
	"blocks": false, "open_play_crosses_completed": false,
}

// CompetitionClub is the SeasonDetail.Competition of all club competitions.
const CompetitionClub = "club"

// SeasonDetail is a player's totals over one group of competitions, with the
// match-engine counts. Goalkeeper rows get the keeper fields in place of
// tackles, headers, blocks and open-play crosses completed, which share their
// slots. Zero counts are omitted.
type SeasonDetail struct {
	// Competition is "club" (all club competitions), "league", "domestic_cups",
	// "continental", "friendlies" or "internationals".
	Competition string `json:"competition"`
	// Goalkeeper marks a row served with the keeper meanings.
	Goalkeeper bool `json:"goalkeeper,omitempty"`
	SeasonStats
	PenaltiesTaken  int `json:"penalties_taken,omitempty"`
	PenaltiesScored int `json:"penalties_scored,omitempty"`
	MatchesWon      int `json:"matches_won,omitempty"`
	MatchesLost     int `json:"matches_lost,omitempty"`
	// TeamGoals and TeamConceded are the team's goals in the player's matches.
	TeamGoals                int `json:"team_goals,omitempty"`
	TeamConceded             int `json:"team_conceded,omitempty"`
	Passes                   int `json:"passes,omitempty"`
	PassesCompleted          int `json:"passes_completed,omitempty"`
	ProgressivePasses        int `json:"progressive_passes,omitempty"`
	KeyPasses                int `json:"key_passes,omitempty"`
	OpenPlayKeyPasses        int `json:"open_play_key_passes,omitempty"`
	Crosses                  int `json:"crosses,omitempty"`
	CrossesCompleted         int `json:"crosses_completed,omitempty"`
	OpenPlayCrosses          int `json:"open_play_crosses,omitempty"`
	OpenPlayCrossesCompleted int `json:"open_play_crosses_completed,omitempty"`
	ChancesCreated           int `json:"chances_created,omitempty"`
	Shots                    int `json:"shots,omitempty"`
	ShotsOnTarget            int `json:"shots_on_target,omitempty"`
	ShotsOutsideBox          int `json:"shots_outside_box,omitempty"`
	GoalsOutsideBox          int `json:"goals_outside_box,omitempty"`
	FreeKickShots            int `json:"free_kick_shots,omitempty"`
	GoalsFromDirectFreeKicks int `json:"goals_from_direct_free_kicks,omitempty"`
	ShotsBlocked             int `json:"shots_blocked,omitempty"`
	Dribbles                 int `json:"dribbles,omitempty"`
	Offsides                 int `json:"offsides,omitempty"`
	Fouls                    int `json:"fouls,omitempty"`
	FoulsAgainst             int `json:"fouls_against,omitempty"`
	Tackles                  int `json:"tackles,omitempty"`
	TacklesWon               int `json:"tackles_won,omitempty"`
	KeyTackles               int `json:"key_tackles,omitempty"`
	Headers                  int `json:"headers,omitempty"`
	HeadersWon               int `json:"headers_won,omitempty"`
	HeadersLost              int `json:"headers_lost,omitempty"`
	KeyHeaders               int `json:"key_headers,omitempty"`
	Blocks                   int `json:"blocks,omitempty"`
	Interceptions            int `json:"interceptions,omitempty"`
	Clearances               int `json:"clearances,omitempty"`
	Pressures                int `json:"pressures,omitempty"`
	PressuresCompleted       int `json:"pressures_completed,omitempty"`
	PossessionWon            int `json:"possession_won,omitempty"`
	PossessionLost           int `json:"possession_lost,omitempty"`
	MistakesLeadingToGoal    int `json:"mistakes_leading_to_goal,omitempty"`
	Sprints                  int `json:"sprints,omitempty"`
	// Goalkeepers only.
	GoalsConceded  int `json:"goals_conceded,omitempty"`
	CleanSheets    int `json:"clean_sheets,omitempty"`
	SavesHeld      int `json:"saves_held,omitempty"`
	SavesParried   int `json:"saves_parried,omitempty"`
	SavesTipped    int `json:"saves_tipped,omitempty"`
	PenaltiesFaced int `json:"penalties_faced,omitempty"`
	PenaltiesSaved int `json:"penalties_saved,omitempty"`
	// The xG fields are the stored hundredths. The game can show a different
	// second decimal for the same value.
	ExpectedGoals   float64 `json:"expected_goals,omitempty"`
	ExpectedAssists float64 `json:"expected_assists,omitempty"`
	NonPenaltyXG    float64 `json:"non_penalty_expected_goals,omitempty"`
	// ExpectedGoalsPrevented is for goalkeepers only and can be negative.
	ExpectedGoalsPrevented float64 `json:"expected_goals_prevented,omitempty"`
	DistanceKm             float64 `json:"distance_km,omitempty"` // stored in tenths
}

// count converts a stored i16; the signed values include junk below zero,
// which is reported as 0.
func count(v int16) int { return int(max(v, 0)) }

// seasonDetail converts the totals and detail block of one scope; keeper
// selects the goalkeeper meaning of the slots the game shares.
func seasonDetail(competition string, s savefile.MatchStats, keeper bool) SeasonDetail {
	d := s.Detail
	out := SeasonDetail{
		Competition:              competition,
		PenaltiesTaken:           int(s.PenaltiesTaken),
		PenaltiesScored:          int(s.PenaltiesScored),
		MatchesWon:               count(d.MatchesWon),
		MatchesLost:              count(d.MatchesLost),
		TeamGoals:                count(d.TeamGoals),
		TeamConceded:             count(d.TeamConceded),
		Passes:                   count(d.Passes),
		PassesCompleted:          count(d.PassesCompleted),
		ProgressivePasses:        count(d.ProgressivePasses),
		KeyPasses:                count(d.KeyPasses),
		OpenPlayKeyPasses:        count(d.OpenPlayKeyPasses),
		Crosses:                  count(d.Crosses),
		CrossesCompleted:         count(d.CrossesCompleted),
		OpenPlayCrosses:          count(d.OpenPlayCrosses),
		ChancesCreated:           count(d.ChancesCreated),
		Shots:                    count(d.Shots),
		ShotsOnTarget:            count(d.ShotsOnTarget),
		ShotsOutsideBox:          count(d.ShotsOutsideBox),
		GoalsOutsideBox:          count(d.GoalsOutsideBox),
		FreeKickShots:            count(d.FreeKickShots),
		GoalsFromDirectFreeKicks: count(d.GoalsFromDirectFreeKicks),
		ShotsBlocked:             count(d.ShotsBlocked),
		Dribbles:                 count(d.Dribbles),
		Offsides:                 count(d.Offsides),
		Fouls:                    count(d.Fouls),
		FoulsAgainst:             count(d.FoulsAgainst),
		KeyTackles:               count(d.KeyTackles),
		KeyHeaders:               count(d.KeyHeaders),
		Interceptions:            count(d.Interceptions),
		Clearances:               count(d.Clearances),
		Pressures:                count(d.PressuresAttempted),
		PressuresCompleted:       count(d.PressuresCompleted),
		PossessionWon:            count(d.PossessionWon),
		PossessionLost:           count(d.PossessionLost),
		MistakesLeadingToGoal:    count(d.MistakesLeadingToGoal),
		Sprints:                  count(d.Sprints),
		ExpectedGoals:            float64(count(d.ExpectedGoals)) / 100,
		ExpectedAssists:          float64(count(d.ExpectedAssists)) / 100,
		NonPenaltyXG:             float64(count(d.NonPenaltyXG)) / 100,
		DistanceKm:               float64(count(d.Distance)) / 10,
	}
	if keeper {
		out.Goalkeeper = true
		out.GoalsConceded, out.CleanSheets = int(s.GoalsConceded), int(s.CleanSheets)
		out.SavesHeld, out.SavesParried = count(d.Headers), count(d.HeadersWon)
		out.SavesTipped = count(d.OpenPlayCrossesAttempted)
		out.PenaltiesFaced, out.PenaltiesSaved = count(d.Tackles), count(d.TacklesWon)
		// Keepers keep expected goals on target faced × 100 in this slot.
		xgFaced100 := count(d.OpenPlayCrossesCompleted)
		out.ExpectedGoalsPrevented = float64(xgFaced100-100*out.GoalsConceded) / 100
	} else {
		out.Tackles, out.TacklesWon = count(d.Tackles), count(d.TacklesWon)
		out.Headers, out.HeadersWon, out.HeadersLost = count(d.Headers), count(d.HeadersWon), count(d.HeadersLost)
		out.Blocks = count(d.Blocks)
		out.OpenPlayCrossesCompleted = count(d.OpenPlayCrossesCompleted)
	}
	if base := seasonStats(s); base != nil {
		out.SeasonStats = *base
	}
	return out
}

// hasKeeperStats reports whether the game records p's match stats as a
// goalkeeper's.
func (p *Player) hasKeeperStats() bool {
	return len(p.Positions) > savefile.GKPosition && p.Positions[savefile.GKPosition].Rating >= statsKeeperRating
}

// seasonDetails lists the scopes of r that have a count, in the order club,
// league, domestic cups, continental, friendlies, internationals.
func seasonDetails(r savefile.SeasonRecord, keeper bool) []SeasonDetail {
	var out []SeasonDetail
	for _, s := range []struct {
		name  string
		stats *savefile.MatchStats
	}{
		{CompetitionClub, r.Club}, {"league", r.League}, {"domestic_cups", r.Cups},
		{"continental", r.Continental}, {"friendlies", r.Friendlies}, {"internationals", r.Internationals},
	} {
		if s.stats == nil {
			continue
		}
		if d := seasonDetail(s.name, *s.stats, keeper); d != (SeasonDetail{Competition: s.name, Goalkeeper: keeper}) {
			out = append(out, d)
		}
	}
	return out
}
