package fm

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// ErrNoMatch means no played match fits the selection.
var ErrNoMatch = errors.New("fm: no such match")

// MatchDataStatus says what is stored about a played match beyond its score.
type MatchDataStatus string

// MatchDataStatus values.
const (
	// MatchDataGoals: the goals, missed penalties and shoot-out kicks.
	MatchDataGoals MatchDataStatus = "goals_only"
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

// Match is one played match with its events.
type Match struct {
	Fixture
	// Events in match order when the match has a MatchID; none then means
	// no goals.
	Events []MatchEvent `json:"events,omitempty"`
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
		i := slices.IndexFunc(v.g.fixtures, func(s storedFixture) bool {
			return newScore(s.Result) != nil && s.Result.MatchID != savefile.NoMatchID && int64(s.Result.MatchID) == *sel.MatchID
		})
		if i < 0 {
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
		var team *int64
		switch e.Side {
		case 0:
			team = ptr(int64(s.Home))
		case 1:
			team = ptr(int64(s.Away))
		}
		typ, ok := eventTypes[e.Kind]
		if !ok {
			typ = fmt.Sprintf("unknown_%d", e.Kind)
		}
		m.Events[i] = MatchEvent{
			Minute: int(e.Minute), AddedMinutes: int(e.AddedMinutes), Type: typ,
			TeamIndex: team,
			Player:    tacticPlayer(g.persons, e.Scorer), Assist: tacticPlayer(g.persons, e.Assister),
		}
		if team != nil {
			m.Events[i].Team = g.world.teamName(*team)
		}
	}
	return m
}

// matchData returns a played result's match id (nil when none) and what its
// stored match data holds.
func matchData(r *savefile.FixtureResult) (*int64, MatchDataStatus) {
	switch {
	case !r.Decoded:
		return nil, MatchDataUnsupported
	case r.MatchID == savefile.NoMatchID:
		return nil, MatchDataQuickSimulated
	}
	id := int64(r.MatchID)
	return &id, MatchDataGoals
}
