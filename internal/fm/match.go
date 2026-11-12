package fm

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// ErrNoMatch means no played match fits the selection.
var ErrNoMatch = errors.New("fm: no such match")

// MatchDataStatus says what is stored about a played match beyond its score.
type MatchDataStatus string

// MatchDataStatus values.
const (
	// MatchDataStored: the goals and the players' match records.
	MatchDataStored MatchDataStatus = "stored"
	// MatchDataPruned: the goals only; the game has deleted the players'
	// match records.
	MatchDataPruned MatchDataStatus = "pruned"
	// MatchDataPlayersUnreadable: the goals only; the players' match records
	// could not be read.
	MatchDataPlayersUnreadable MatchDataStatus = "goals_only"
	// MatchDataQuickSimulated: the game simulated the match without detail,
	// so only the score is stored.
	MatchDataQuickSimulated MatchDataStatus = "quick_simulated"
	// MatchDataUnsupported: the save's build is not decoded (FM26).
	MatchDataUnsupported MatchDataStatus = "unsupported_build"
)

// eventTypes labels savefile.EventKind values.
var eventTypes = map[savefile.EventKind]string{
	savefile.EventGoal:           "goal",
	savefile.EventOwnGoal:        "own_goal",
	savefile.EventPenalty:        "penalty",
	savefile.EventMissedPenalty:  "missed_penalty",
	savefile.EventShootoutScored: "shootout_scored",
	savefile.EventShootoutMissed: "shootout_missed",
}

// Match is one played match with its events and, when stored, its players.
type Match struct {
	Fixture
	// Events in match order when the match has a MatchID; none then means
	// no goals.
	Events []MatchEvent `json:"events,omitempty"`
	// Players who played, home side first, when MatchData is
	// MatchDataStored.
	Players           []MatchPlayer      `json:"players,omitempty"`
	UnusedSubstitutes []UnusedSubstitute `json:"unused_substitutes,omitempty"`
	// TeamStats are the home and away totals when stored.
	TeamStats []TeamMatchStats `json:"team_stats,omitempty"`
}

// TeamMatchStats is a side's totals in a match.
type TeamMatchStats struct {
	TeamIndex       int64  `json:"team_index"`
	Team            string `json:"team,omitempty"`
	Passes          int    `json:"passes"` // attempted
	PassesCompleted int    `json:"passes_completed"`
	// PassesCompletedByThird is nil when the save does not split them.
	PassesCompletedByThird *ThirdPasses `json:"passes_completed_by_third,omitempty"`
	Tackles                int          `json:"tackles"` // attempted
	TacklesWon             int          `json:"tackles_won"`
	Headers                int          `json:"headers"` // attempted
	HeadersWon             int          `json:"headers_won"`
	ShotsBlocked           int          `json:"shots_blocked"` // the side's own shots that were blocked
	ClearCutChances        int          `json:"clear_cut_chances"`
	Corners                int          `json:"corners"`
	Fouls                  int          `json:"fouls"`
	YellowCards            int          `json:"yellow_cards"`
}

// ThirdPasses splits completed passes by the third they were played in.
type ThirdPasses struct {
	Own    int `json:"own_third"`
	Middle int `json:"middle_third"`
	Final  int `json:"final_third"`
}

// UnusedSubstitute is a squad member who did not play.
type UnusedSubstitute struct {
	Player    TacticPlayer `json:"player"` // with the shirt number
	TeamIndex int64        `json:"team_index"`
	Team      string       `json:"team,omitempty"`
}

// MatchPlayer is one player's line in a match.
type MatchPlayer struct {
	Player    TacticPlayer `json:"player"` // with the shirt number worn
	TeamIndex int64        `json:"team_index"`
	Team      string       `json:"team,omitempty"`
	Performance
}

// Performance is how a player who played did in a match.
type Performance struct {
	Started         bool    `json:"started"` // false: came on as a substitute
	SubbedOnMinute  *int    `json:"subbed_on_minute,omitempty"`
	SubbedOffMinute *int    `json:"subbed_off_minute,omitempty"` // nil unless taken off (nil for a player sent off)
	Rating          float64 `json:"rating"`                      // e.g. 8.19
	XG              float64 `json:"xg"`                          // expected goals, to 2 decimals
	Shots           int     `json:"shots"`
	ShotsOnTarget   int     `json:"shots_on_target"`
	ClearCutChances int     `json:"clear_cut_chances"`
	Passes          int     `json:"passes"` // attempted
	PassesCompleted int     `json:"passes_completed"`
	KeyPasses       int     `json:"key_passes"`
	// ProgressivePasses is checked in game as team totals only.
	ProgressivePasses int     `json:"progressive_passes"`
	Crosses           int     `json:"crosses"` // attempted
	CrossesCompleted  int     `json:"crosses_completed"`
	Tackles           int     `json:"tackles"` // attempted
	TacklesWon        int     `json:"tackles_won"`
	KeyTackles        int     `json:"key_tackles"`
	Interceptions     int     `json:"interceptions"`
	Clearances        int     `json:"clearances"`
	PossessionWon     int     `json:"possession_won"`
	Fouls             int     `json:"fouls"`
	YellowCards       int     `json:"yellow_cards"`
	RedCards          int     `json:"red_cards"`
	DistanceKm        float64 `json:"distance_km"` // to 1 decimal
	Sprints           int     `json:"sprints"`
}

// performance reads a played record.
func performance(r savefile.MatchPlayer) Performance {
	st := r.Stats
	return Performance{
		Started: r.Started(), SubbedOnMinute: minute(st.SubbedOnMinute), SubbedOffMinute: minute(st.SubbedOffMinute),
		Rating: float64(r.Rating) / 100, XG: math.Round(float64(r.XG)*100) / 100,
		Shots: int(st.Shots), ShotsOnTarget: int(st.ShotsOnTarget), ClearCutChances: int(st.ClearCutChances),
		Passes: int(st.Passes), PassesCompleted: int(st.PassesCompleted), KeyPasses: int(st.KeyPasses), ProgressivePasses: int(st.ProgressivePasses),
		Crosses: int(st.Crosses), CrossesCompleted: int(st.CrossesCompleted),
		Tackles: int(st.Tackles), TacklesWon: int(st.TacklesWon), KeyTackles: int(st.KeyTackles),
		Interceptions: int(st.Interceptions), Clearances: int(st.Clearances), PossessionWon: int(st.PossessionWon),
		Fouls: int(st.Fouls), YellowCards: int(st.YellowCards), RedCards: int(st.RedCards),
		DistanceKm: math.Round(float64(st.DistanceMetres)/100) / 10, Sprints: int(st.Sprints),
	}
}

// minute returns m, or nil for 0 (no substitution).
func minute(m uint8) *int {
	if m == 0 {
		return nil
	}
	return ptr(int(m))
}

// MatchEvent is a goal, a missed penalty or a shoot-out kick.
type MatchEvent struct {
	Minute       int    `json:"minute"`
	AddedMinutes int    `json:"added_minutes,omitempty"` // e.g. 3 for 90+3
	Type         string `json:"type"`
	// TeamIndex is the side the event counts for; an own goal's player
	// plays for the other side. Nil for a side byte other than 0 or 1.
	TeamIndex *int64        `json:"team_index,omitempty"`
	Team      string        `json:"team,omitempty"`
	Player    *TacticPlayer `json:"player,omitempty"` // scorer or penalty taker
	Assist    *TacticPlayer `json:"assist,omitempty"`
}

// MatchSelector picks a match: by MatchID, else Team's match on Date, else
// Team's last result. Nil Team means the manager's team.
type MatchSelector struct {
	MatchID *int64
	Team    *int64
	Date    time.Time
}

// Match returns the played match sel picks.
func (v View) Match(sel MatchSelector) (Match, error) {
	if !v.g.hasFixtures {
		return Match{}, ErrFixturesUnavailable
	}
	if sel.MatchID != nil {
		i, ok := v.g.fixtureByMatch[*sel.MatchID]
		if !ok || newScore(v.g.fixtures[i].Result) == nil {
			return Match{}, fmt.Errorf("%w: no match with match_id %d; get_fixtures gives match ids (FM27 saves only)", ErrNoMatch, *sel.MatchID)
		}
		return v.g.match(v.g.fixtures[i]), nil
	}
	team, err := v.g.queryTeam(sel.Team, nil)
	if err != nil {
		return Match{}, err
	}
	onDate := FixtureFilter{From: sel.Date, To: sel.Date}
	var found *storedFixture
	for i, s := range v.g.fixtures {
		if newScore(s.Result) != nil && onDate.matches(s, team) {
			found = &v.g.fixtures[i] // fixtures are in date order: the last one wins
		}
	}
	if found == nil {
		if sel.Date.IsZero() {
			return Match{}, fmt.Errorf("%w: team_index %d has no result in this save", ErrNoMatch, *team)
		}
		return Match{}, fmt.Errorf("%w: team_index %d has no result on %s", ErrNoMatch, *team, sel.Date.Format(time.DateOnly))
	}
	return v.g.match(*found), nil
}

func (g *Game) match(s storedFixture) Match {
	m := Match{Fixture: g.fixture(s)}
	if m.MatchID == nil {
		return m
	}
	m.Events = make([]MatchEvent, len(s.Result.Events))
	for i, e := range s.Result.Events {
		typ, ok := eventTypes[e.Kind]
		if !ok {
			typ = fmt.Sprintf("unknown_%d", e.Kind)
		}
		m.Events[i] = MatchEvent{
			Minute: int(e.Minute), AddedMinutes: int(e.AddedMinutes), Type: typ,
			Player: tacticPlayer(g.persons, e.Scorer), Assist: tacticPlayer(g.persons, e.Assister),
		}
		if team, ok := s.sideTeam(e.Side); ok {
			m.Events[i].TeamIndex, m.Events[i].Team = &team, g.world.teamName(team)
		}
	}
	if m.MatchData == MatchDataStored {
		m.Players, m.UnusedSubstitutes = g.matchPlayerLines(s)
		m.TeamStats = g.teamStats(s)
	}
	return m
}

func (g *Game) matchPlayerLines(s storedFixture) ([]MatchPlayer, []UnusedSubstitute) {
	records := g.matchObjects[int64(s.Result.MatchID)].Players
	var played []MatchPlayer
	var unused []UnusedSubstitute
	for _, r := range records {
		person := TacticPlayer{Unnamed: true}
		if p := tacticPlayer(g.persons, r.Person); p != nil {
			person = *p
		}
		person.ShirtNumber = int(r.Shirt)
		team, _ := s.sideTeam(r.Side) // records have side 0 or 1
		if !r.Played() {
			unused = append(unused, UnusedSubstitute{Player: person, TeamIndex: team, Team: g.world.teamName(team)})
			continue
		}
		played = append(played, MatchPlayer{Player: person, TeamIndex: team, Team: g.world.teamName(team), Performance: performance(r)})
	}
	return played, unused
}

// matchData returns a played result's match id (nil when none) and what is
// stored about it.
func (g *Game) matchData(r *savefile.FixtureResult) (*int64, MatchDataStatus) {
	switch {
	case !r.Decoded:
		return nil, MatchDataUnsupported
	case !r.HasMatchID():
		return nil, MatchDataQuickSimulated
	}
	id := int64(r.MatchID)
	if g.matchObjects == nil {
		return &id, MatchDataPlayersUnreadable
	}
	obj, ok := g.matchObjects[id]
	switch {
	case !ok:
		return &id, MatchDataPruned
	case obj.Players == nil:
		return &id, MatchDataPlayersUnreadable
	}
	return &id, MatchDataStored
}

// sideTeam returns the team index of side 0 (home) or 1 (away); false for
// any other side byte.
func (s storedFixture) sideTeam(side uint8) (int64, bool) {
	switch side {
	case 0:
		return int64(s.Home), true
	case 1:
		return int64(s.Away), true
	}
	return 0, false
}

// teamStats returns the home and away totals of stored match s; nil when
// they were not read.
func (g *Game) teamStats(s storedFixture) []TeamMatchStats {
	teams := g.matchObjects[int64(s.Result.MatchID)].TeamStats
	if teams == nil {
		return nil
	}
	out := make([]TeamMatchStats, len(teams))
	for side := range uint8(len(teams)) {
		t := teams[side]
		team, _ := s.sideTeam(side)
		out[side] = TeamMatchStats{
			TeamIndex: team, Team: g.world.teamName(team),
			Passes: int(t.Passes), PassesCompleted: int(t.PassesCompleted),
			Tackles: int(t.Tackles), TacklesWon: int(t.TacklesWon), Headers: int(t.Headers), HeadersWon: int(t.HeadersWon),
			ShotsBlocked: int(t.ShotsBlocked), ClearCutChances: int(t.ClearCutChances), Corners: int(t.Corners),
			Fouls: int(t.Fouls), YellowCards: int(t.YellowCards),
		}
		if th := t.PassesCompletedByThird; th != nil {
			out[side].PassesCompletedByThird = &ThirdPasses{Own: int(th[0]), Middle: int(th[1]), Final: int(th[2])}
		}
	}
	return out
}
