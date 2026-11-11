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
	Started bool `json:"started"` // false: came on as a substitute
	// TakenOff is nil when the records do not tell (see takenOffKnown).
	TakenOff *bool   `json:"taken_off,omitempty"`
	Rating   float64 `json:"rating"` // e.g. 8.19
	XG       float64 `json:"xg"`     // expected goals, to 2 decimals
}

// performance reads a played record; known says whether its side's records
// show who was taken off.
func performance(r savefile.MatchPlayer, known bool) Performance {
	p := Performance{Started: r.Started(), Rating: float64(r.Rating) / 100, XG: math.Round(float64(r.XG)*100) / 100}
	if known {
		p.TakenOff = ptr(r.EndPositions == 0)
	}
	return p
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
	}
	return m
}

func (g *Game) matchPlayerLines(s storedFixture) ([]MatchPlayer, []UnusedSubstitute) {
	records := g.matchPlayers[int64(s.Result.MatchID)]
	known := takenOffKnown(records)
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
		played = append(played, MatchPlayer{Player: person, TeamIndex: team, Team: g.world.teamName(team), Performance: performance(r, known[r.Side])})
	}
	return played, unused
}

// takenOffKnown reports per side whether the records show who was taken
// off: as many starters with no end position as substitutes who came on.
// The game keeps end positions only in some of the human's matches.
func takenOffKnown(records []savefile.MatchPlayer) [2]bool {
	var off, on [2]int
	for _, r := range records {
		switch {
		case r.Side > 1: // savefile reads sides 0 and 1 only
		case r.Started() && r.EndPositions == 0:
			off[r.Side]++
		case !r.Started() && r.EndPositions != 0:
			on[r.Side]++
		}
	}
	return [2]bool{off[0] == on[0], off[1] == on[1]}
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
	if g.matchPlayers == nil {
		return &id, MatchDataPlayersUnreadable
	}
	players, ok := g.matchPlayers[id]
	switch {
	case !ok:
		return &id, MatchDataPruned
	case players == nil:
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
