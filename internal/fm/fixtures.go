package fm

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// ErrFixturesUnavailable means the save's fixtures could not be read.
var ErrFixturesUnavailable = errors.New("fm: fixtures unavailable")

const (
	defaultFixtureLimit      = 10
	maxCompetitionCandidates = 10
	statusPlayed             = "played"
	statusUpcoming           = "upcoming"
	noScore                  = 0xff
)

// Fixture is a match, played or not. Fog of war hides nothing: results are
// public.
type Fixture struct {
	Date             time.Time `json:"date"`
	Season           int       `json:"season,omitempty"` // the season's start year; 0 for friendlies
	CompetitionIndex int64     `json:"competition_index"`
	Competition      string    `json:"competition,omitempty"`
	Stage            string    `json:"stage,omitempty"` // e.g. "Playoffs – Semi Final – First Leg"
	// The other division of a promotion/relegation playoff between two
	// divisions.
	SecondCompetitionIndex *int64 `json:"second_competition_index,omitempty"`
	SecondCompetition      string `json:"second_competition,omitempty"`
	HomeTeamIndex          int64  `json:"home_team_index"`
	HomeTeam               string `json:"home_team,omitempty"`
	AwayTeamIndex          int64  `json:"away_team_index"`
	AwayTeam               string `json:"away_team,omitempty"`
	Score                  *Score `json:"score,omitempty"` // nil until played
}

// Score is a played fixture's score after 90 minutes, with the score after
// extra time, the shoot-out and the aggregate when they apply. Pairs are
// home, away.
type Score struct {
	Home      int     `json:"home"`
	Away      int     `json:"away"`
	ExtraTime *[2]int `json:"after_extra_time,omitempty"`
	Penalties *[2]int `json:"penalties,omitempty"`
	Aggregate *[2]int `json:"aggregate,omitempty"`
}

// newScore reads the score bytes; nil when the 90-minute score is unset.
func newScore(r *savefile.FixtureResult) *Score {
	if r == nil || r.Home[0] == noScore || r.Away[0] == noScore {
		return nil
	}
	part := func(i int) *[2]int {
		if r.Home[i] == noScore || r.Away[i] == noScore {
			return nil
		}
		return &[2]int{int(r.Home[i]), int(r.Away[i])}
	}
	return &Score{Home: int(r.Home[0]), Away: int(r.Away[0]), ExtraTime: part(1), Penalties: part(2), Aggregate: part(3)}
}

// storedFixture is a fixture whose competition key resolved.
type storedFixture struct {
	savefile.Fixture
	date  time.Time
	label *competitionLabel
}

// resolveFixtures keeps the fixtures whose key resolves, in date order, and
// returns how many it left out.
func resolveFixtures(raw []savefile.Fixture, w World) ([]storedFixture, int) {
	labels := make(map[int32]*competitionLabel)
	out := make([]storedFixture, 0, len(raw))
	for _, f := range raw {
		l, ok := labels[f.Key]
		if !ok {
			if v, resolved := w.keyLabel(f.Key); resolved {
				l = &v
			}
			labels[f.Key] = l
		}
		if l == nil {
			continue
		}
		out = append(out, storedFixture{Fixture: f, date: f.Date.Time(), label: l})
	}
	slices.SortStableFunc(out, func(a, b storedFixture) int {
		return cmp.Or(a.date.Compare(b.date), cmp.Compare(a.Home, b.Home))
	})
	return out, len(raw) - len(out)
}

// FixtureFilter selects fixtures. Nil Team and Competition mean the
// manager's team. Zero From and To leave the range open; both are inclusive.
type FixtureFilter struct {
	Team        *int64
	Competition *int64 // matches either competition of a two-division playoff
	From, To    time.Time
	Status      string // "played", "upcoming" or "" for both
	Limit       int    // per list; default 10, max 200
}

// FixtureList holds played fixtures, most recent first, and the rest,
// soonest first, each cut to the limit.
type FixtureList struct {
	Results       []Fixture `json:"results"`
	Upcoming      []Fixture `json:"upcoming"`
	TotalResults  int       `json:"total_results"`
	TotalUpcoming int       `json:"total_upcoming"`
}

// Fixtures returns the fixtures f selects.
func (v View) Fixtures(f FixtureFilter) (FixtureList, error) {
	if f.Status != "" && f.Status != statusPlayed && f.Status != statusUpcoming {
		return FixtureList{}, fmt.Errorf("%w: status %q; use played or upcoming", ErrInvalidFilter, f.Status)
	}
	if !f.From.IsZero() && !f.To.IsZero() && f.From.After(f.To) {
		return FixtureList{}, fmt.Errorf("%w: from %s is after to %s", ErrInvalidFilter, f.From.Format(time.DateOnly), f.To.Format(time.DateOnly))
	}
	team, err := v.g.queryTeam(f.Team, f.Competition)
	if err != nil {
		return FixtureList{}, err
	}
	if !v.g.hasFixtures {
		return FixtureList{}, ErrFixturesUnavailable
	}
	limit := f.Limit
	if limit <= 0 {
		limit = defaultFixtureLimit
	}
	limit = min(limit, maxLimit)
	var results, upcoming []storedFixture
	for _, s := range v.g.fixtures {
		if !f.matches(s, team) {
			continue
		}
		if newScore(s.Result) != nil {
			if f.Status != statusUpcoming {
				results = append(results, s)
			}
		} else if f.Status != statusPlayed {
			upcoming = append(upcoming, s)
		}
	}
	slices.Reverse(results)
	return FixtureList{
		Results: v.g.renderFixtures(results[:min(len(results), limit)]), Upcoming: v.g.renderFixtures(upcoming[:min(len(upcoming), limit)]),
		TotalResults: len(results), TotalUpcoming: len(upcoming),
	}, nil
}

func (g *Game) renderFixtures(ss []storedFixture) []Fixture {
	out := make([]Fixture, len(ss))
	for i, s := range ss {
		out[i] = g.fixture(s)
	}
	return out
}

func (f FixtureFilter) matches(s storedFixture, team *int64) bool {
	if team != nil && int64(s.Home) != *team && int64(s.Away) != *team {
		return false
	}
	if c := f.Competition; c != nil && s.label.Index != *c && (s.label.SecondIndex < 0 || s.label.SecondIndex != *c) {
		return false
	}
	return (f.From.IsZero() || !s.date.Before(f.From)) && (f.To.IsZero() || !s.date.After(f.To))
}

func (g *Game) fixture(s storedFixture) Fixture {
	f := Fixture{
		Date: s.date, Season: int(s.Season),
		CompetitionIndex: s.label.Index, Competition: s.label.Name, Stage: s.label.Stage,
		HomeTeamIndex: int64(s.Home), HomeTeam: g.world.teamName(int64(s.Home)),
		AwayTeamIndex: int64(s.Away), AwayTeam: g.world.teamName(int64(s.Away)),
		Score: newScore(s.Result),
	}
	if s.label.SecondIndex >= 0 {
		second := s.label.SecondIndex
		f.SecondCompetitionIndex, f.SecondCompetition = &second, s.label.Second
	}
	return f
}

// FindCompetition returns the COMP index named name, ignoring case and
// accents. An exact name wins; otherwise each word must start a word of the
// name. No match or several give ErrInvalidFilter listing candidates.
func (v View) FindCompetition(name string) (int64, error) {
	words := nameWords(name)
	if len(words) == 0 {
		return 0, fmt.Errorf("%w: competition name %q has no letters or digits", ErrInvalidFilter, name)
	}
	want := normalize(name)
	var exact, partial []int64
	for i, n := range v.g.world.Competitions {
		switch {
		case normalize(n) == want:
			exact = append(exact, i)
		case matchesNameStarts(nameWords(n), words):
			partial = append(partial, i)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = partial
	}
	switch len(candidates) {
	case 0:
		return 0, fmt.Errorf("%w: no competition named %q", ErrInvalidFilter, name)
	case 1:
		return candidates[0], nil
	}
	slices.Sort(candidates)
	names := make([]string, 0, maxCompetitionCandidates)
	for _, i := range candidates[:min(len(candidates), maxCompetitionCandidates)] {
		names = append(names, fmt.Sprintf("%s (competition_index %d)", v.g.world.Competitions[i], i))
	}
	return 0, fmt.Errorf("%w: %d competitions match %q: %s; pass competition_index or a fuller name", ErrInvalidFilter, len(candidates), name, strings.Join(names, ", "))
}
