package fm

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// ErrMatchPlayersUnavailable means the save's match player records were not
// read (FM26 saves, or a load warning says why).
var ErrMatchPlayersUnavailable = errors.New("fm: match player records unavailable")

// PlayerMatchFilter selects a player's matches. Zero From and To leave the
// range open; both are inclusive.
type PlayerMatchFilter struct {
	From, To time.Time
	Limit    int // default 10, max 200
}

// PlayerMatchList is a player's stored matches, newest first, cut to the
// limit; Total counts all that fit the filter.
type PlayerMatchList struct {
	Player  TacticPlayer  `json:"player"`
	Matches []PlayerMatch `json:"matches"`
	Total   int           `json:"total"`
}

// PlayerMatch is one match a player played in.
type PlayerMatch struct {
	MatchID          int64     `json:"match_id"`
	Date             time.Time `json:"date"`
	CompetitionIndex int64     `json:"competition_index"`
	Competition      string    `json:"competition,omitempty"`
	Stage            string    `json:"stage,omitempty"`
	TeamIndex        int64     `json:"team_index"`
	Team             string    `json:"team,omitempty"`
	OpponentIndex    int64     `json:"opponent_team_index"`
	Opponent         string    `json:"opponent,omitempty"`
	Home             bool      `json:"home"`
	Score            *Score    `json:"score,omitempty"` // home side first
	ShirtNumber      int       `json:"shirt_number,omitempty"`
	Performance
	Goals    int `json:"goals"` // including penalties
	Assists  int `json:"assists"`
	OwnGoals int `json:"own_goals,omitempty"`
}

// PlayerMatches returns the stored matches the player played in, for any team.
func (v View) PlayerMatches(uid uint32, f PlayerMatchFilter) (PlayerMatchList, error) {
	if err := checkDateRange(f.From, f.To); err != nil {
		return PlayerMatchList{}, err
	}
	p, ok := v.g.byUID[uid]
	if !ok {
		return PlayerMatchList{}, fmt.Errorf("%w: no player with uid %d", ErrInvalidFilter, uid)
	}
	if v.g.matchObjects == nil {
		return PlayerMatchList{}, ErrMatchPlayersUnavailable
	}
	limit := listLimit(f.Limit)
	out := PlayerMatchList{Player: TacticPlayer{UID: p.UID, Name: p.Name}, Matches: []PlayerMatch{}}
	for _, at := range slices.Backward(v.g.playerMatches[uid]) {
		if !inDateRange(v.g.fixtures[at.fixture].date, f.From, f.To) {
			continue
		}
		out.Total++
		if len(out.Matches) < limit {
			out.Matches = append(out.Matches, v.g.playerMatch(at))
		}
	}
	return out, nil
}

func (g *Game) playerMatch(at playedMatch) PlayerMatch {
	s := g.fixtures[at.fixture]
	records := g.matchObjects[int64(s.Result.MatchID)].Players
	r := records[at.record]
	team, _ := s.sideTeam(r.Side) // records have side 0 or 1
	opponent, _ := s.sideTeam(1 - r.Side)
	f := g.fixture(s)
	m := PlayerMatch{
		MatchID: int64(s.Result.MatchID), Date: s.date,
		CompetitionIndex: f.CompetitionIndex, Competition: f.Competition, Stage: f.Stage,
		TeamIndex: team, Team: g.world.teamName(team), OpponentIndex: opponent, Opponent: g.world.teamName(opponent),
		Home: r.Side == 0, Score: f.Score, ShirtNumber: int(r.Shirt), Performance: performance(r, takenOffKnown(records)[r.Side]),
	}
	for _, e := range s.Result.Events {
		switch e.Kind {
		case savefile.EventGoal, savefile.EventPenalty:
			if e.Scorer == r.Person {
				m.Goals++
			}
			if e.Assister == r.Person {
				m.Assists++
			}
		case savefile.EventOwnGoal:
			if e.Scorer == r.Person {
				m.OwnGoals++
			}
		}
	}
	return m
}
