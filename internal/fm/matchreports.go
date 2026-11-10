package fm

import (
	"fmt"
	"slices"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// MatchReport uses real per-match data. It is independent of season scopes.
type MatchReport struct {
	Fixture
	Kickoff     time.Time     `json:"kickoff"`
	Source      string        `json:"source"`
	Players     []MatchPlayer `json:"players"`
	Limitations []string      `json:"limitations"`
}

type MatchPlayer struct {
	UID             *uint32  `json:"uid,omitempty"`
	Name            string   `json:"name,omitempty"`
	Unnamed         bool     `json:"unnamed_player,omitempty"`
	TeamIndex       int64    `json:"team_index"`
	ShirtNumber     int      `json:"shirt_number"`
	Played          bool     `json:"played"`
	Started         bool     `json:"started"`
	EngineRating    *float64 `json:"engine_rating,omitempty"`
	Goals           int      `json:"goals"`
	Assists         int      `json:"assists"`
	Shots           int      `json:"shots"`
	ShotsOnTarget   int      `json:"shots_on_target"`
	Passes          int      `json:"passes"`
	PassesCompleted int      `json:"passes_completed"`
	KeyPasses       int      `json:"key_passes"`
	Tackles         *int     `json:"tackles,omitempty"`
	TacklesWon      *int     `json:"tackles_won,omitempty"`
	Dribbles        int      `json:"dribbles"`
	Fouls           int      `json:"fouls"`
	Interceptions   int      `json:"interceptions"`
	ExpectedGoals   float64  `json:"expected_goals"`
	ExpectedAssists float64  `json:"expected_assists"`
	DistanceKM      float64  `json:"distance_km"`
}

// reportKey joins reports to fixtures without guessing from team names or
// last-lineup order. More than one report for a key is left unlinked.
type reportKey struct {
	day        string
	home, away int64
}

func fixtureReportKey(date time.Time, home, away int64) reportKey {
	return reportKey{date.Format(time.DateOnly), home, away}
}

func (g *Game) resolveMatchReports(raw *savefile.MatchReports, players []savefile.Player) {
	if raw == nil {
		return
	}
	g.matchReports = make(map[uint32]MatchReport)
	g.fixtureReports = make(map[reportKey]*uint32)
	byPerson := make(map[uint32]*Player, len(players))
	for _, p := range players {
		byPerson[p.Index] = g.byUID[p.UID]
	}
	fixtures := make(map[reportKey][]storedFixture)
	for _, f := range g.fixtures {
		key := fixtureReportKey(f.date, int64(f.Home), int64(f.Away))
		fixtures[key] = append(fixtures[key], f)
	}
	for _, r := range raw.Reports {
		id := r.ID
		key := fixtureReportKey(r.Kickoff, int64(r.Home), int64(r.Away))
		f := Fixture{Date: time.Date(r.Kickoff.Year(), r.Kickoff.Month(), r.Kickoff.Day(), 0, 0, 0, 0, time.UTC), HomeTeamIndex: int64(r.Home), AwayTeamIndex: int64(r.Away), HomeTeam: g.world.teamName(int64(r.Home)), AwayTeam: g.world.teamName(int64(r.Away))}
		if matches := fixtures[key]; len(matches) == 1 {
			f = g.fixture(matches[0])
		}
		f.MatchID = &id
		out := MatchReport{Fixture: f, Kickoff: r.Kickoff, Source: "match_stats_cfg.dat", Players: make([]MatchPlayer, 0, len(r.Players)), Limitations: []string{
			"Engine ratings can exist for substitutes the game leaves unrated; they are not guaranteed UI match ratings.",
			"Minutes, possession, match events and goalkeeper-specific counters are not decoded. A missing report is unavailable, not a match with zero stats.",
			"Supported layout: FM27 match format 60. Fields validated against save season counters; not yet checked against the FM27 match screen.",
		}}
		for _, p := range r.Players {
			team := int64(r.Home)
			if p.Side == 1 {
				team = int64(r.Away)
			}
			row := MatchPlayer{TeamIndex: team, ShirtNumber: int(p.Shirt), Played: p.Played, Started: p.Started, Goals: p.Goals, Assists: p.Assists, Shots: p.Shots, ShotsOnTarget: p.ShotsOnTarget, Passes: p.Passes, PassesCompleted: p.PassesCompleted, KeyPasses: p.KeyPasses, Dribbles: p.Dribbles, Fouls: p.Fouls, Interceptions: p.Interceptions, ExpectedGoals: p.ExpectedGoals, ExpectedAssists: p.ExpectedAssists, DistanceKM: p.DistanceKM}
			if p.Played {
				rating := p.Rating
				row.EngineRating = &rating
			}
			if player := byPerson[p.Person]; player != nil {
				uid := player.UID
				row.UID = &uid
				row.Name = player.Name
				if !player.hasKeeperStats() {
					tackles, won := p.Tackles, p.TacklesWon
					row.Tackles = &tackles
					row.TacklesWon = &won
				}
			} else {
				row.Unnamed = true
			}
			out.Players = append(out.Players, row)
		}
		g.matchReports[id] = out
		if matches := fixtures[key]; len(matches) == 1 && newScore(matches[0].Result) != nil {
			if _, exists := g.fixtureReports[key]; exists {
				g.fixtureReports[key] = nil
			} else {
				g.fixtureReports[key] = &id
			}
		}
	}
}

// MatchReport returns one indexed report. Public match stats ignore fog of war.
func (v View) MatchReport(id uint32) (MatchReport, error) {
	if v.g.matchReports == nil {
		return MatchReport{}, fmt.Errorf("match reports unavailable; game_overview warnings say why")
	}
	if r, ok := v.g.matchReports[id]; ok {
		return r, nil
	}
	return MatchReport{}, fmt.Errorf("match report %d unavailable: use match_id from get_fixtures; old reports may have been discarded or use an unsupported format", id)
}

type PlayerMatch struct {
	Fixture
	Stats MatchPlayer `json:"stats"`
}

type PlayerMatches struct {
	Matches  []PlayerMatch `json:"matches"`
	Total    int           `json:"total_available"`
	Coverage string        `json:"coverage"`
}

// PlayerMatchStats lists only appearances in retained, decoded reports.
func (v View) PlayerMatchStats(uid uint32, limit int) (PlayerMatches, error) {
	if _, ok := v.g.byUID[uid]; !ok {
		return PlayerMatches{}, fmt.Errorf("unknown player uid %d", uid)
	}
	if v.g.matchReports == nil {
		return PlayerMatches{}, fmt.Errorf("match reports unavailable; game_overview warnings say why")
	}
	rows := make([]PlayerMatch, 0)
	for _, r := range v.g.matchReports {
		for _, p := range r.Players {
			if p.UID != nil && *p.UID == uid && p.Played {
				rows = append(rows, PlayerMatch{Fixture: r.Fixture, Stats: p})
			}
		}
	}
	slices.SortFunc(rows, func(a, b PlayerMatch) int {
		if c := b.Date.Compare(a.Date); c != 0 {
			return c
		}
		if *a.MatchID < *b.MatchID {
			return -1
		}
		if *a.MatchID > *b.MatchID {
			return 1
		}
		return 0
	})
	if limit <= 0 {
		limit = 10
	}
	limit = min(limit, maxLimit)
	return PlayerMatches{Matches: rows[:min(limit, len(rows))], Total: len(rows), Coverage: "Retained decoded reports only, not a complete career history. engine_rating may exist when FM shows no rating; minutes are unavailable."}, nil
}
