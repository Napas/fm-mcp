package fm

import (
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func matchGame(reports *savefile.MatchReports) *Game {
	p := savefile.Player{Index: 10, UID: 900, FirstName: "Test", LastName: "Player", TeamIndex: 1}
	return NewGame(Info{ManagerTeamIndex: 1, Date: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)}, []savefile.Player{p}, fixtureWorld(), PlayerExtras{
		Fixtures: []savefile.Fixture{{Key: 0, Home: 1, Away: 2, Date: day(2026, 7, 22), Result: played(1, 1)}}, MatchReports: reports,
	})
}

func oneReport(id uint32) savefile.MatchReport {
	return savefile.MatchReport{ID: id, Kickoff: time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC), Home: 1, Away: 2, Players: []savefile.MatchPlayer{
		{Person: 10, Played: true, Started: true, Rating: 6.7, Passes: 33, PassesCompleted: 28},
		{Person: 9999, Side: 1, Played: true},
		{Person: 10, Played: false},
	}}
}

func TestMatchReportsLinkFixturesAndResolvePeople(t *testing.T) {
	g := matchGame(&savefile.MatchReports{Reports: []savefile.MatchReport{oneReport(395)}})
	for _, fog := range []bool{false, true} {
		v := g.View(fog)
		fs, err := v.Fixtures(FixtureFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs.Results) != 1 || fs.Results[0].MatchID == nil || *fs.Results[0].MatchID != 395 {
			t.Fatalf("fixtures: %+v", fs)
		}
		r, err := v.MatchReport(395)
		if err != nil {
			t.Fatal(err)
		}
		if r.Score == nil || r.Score.Home != 1 || r.HomeTeam != "Home FC" || r.Players[0].Name != "Test Player" || r.Players[0].UID == nil || *r.Players[0].UID != 900 || !r.Players[1].Unnamed || r.Players[2].EngineRating != nil {
			t.Fatalf("report: %+v", r)
		}
		h, err := v.PlayerMatchStats(900, 1)
		if err != nil || h.Total != 1 || len(h.Matches) != 1 || h.Matches[0].Stats.Passes != 33 {
			t.Fatalf("history: %+v %v", h, err)
		}
	}
}

func TestMatchReportsMissingAndAmbiguous(t *testing.T) {
	for _, reports := range []*savefile.MatchReports{nil, {}, {Reports: []savefile.MatchReport{oneReport(1), oneReport(2)}}} {
		v := matchGame(reports).View(true)
		fs, err := v.Fixtures(FixtureFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if fs.Results[0].MatchID != nil {
			t.Fatal("linked absent/ambiguous report")
		}
		if _, err := v.MatchReport(500); err == nil {
			t.Fatal("missing report should be unavailable")
		}
		if _, err := v.PlayerMatchStats(999, 10); err == nil {
			t.Fatal("accepted unknown uid")
		}
	}
}

func TestMatchReportDoesNotLinkAmbiguousFixtures(t *testing.T) {
	g := matchGame(&savefile.MatchReports{Reports: []savefile.MatchReport{oneReport(395)}})
	g.fixtures = append(g.fixtures, g.fixtures[0])
	g.resolveMatchReports(&savefile.MatchReports{Reports: []savefile.MatchReport{oneReport(395)}}, nil)
	fs, err := g.View(true).Fixtures(FixtureFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs.Results {
		if f.MatchID != nil {
			t.Fatal("same-day fixture ambiguity should not guess a report link")
		}
	}
}
