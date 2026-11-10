package mcpserver

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/fm"
	"github.com/Napas/fm-mcp/internal/savefile"
)

func TestRealMatchStartingFlagsMCP(t *testing.T) {
	path := os.Getenv("FM_MATCH_STARTERS_SAVE")
	if path == "" {
		t.Skip("set FM_MATCH_STARTERS_SAVE to a copy of the Caen 2026-08-03 save")
	}
	h := connect(t, New(Options{DefaultSave: path, Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) { return fm.Load(path, fm.LoadOptions{}) }}))
	for _, id := range []int{395, 1128} {
		var report fm.MatchReport
		if res := h.call("get_match_report", map[string]any{"match_id": id}, &report); res.IsError {
			t.Fatal(allText(res))
		}
		counts := map[int64]int{}
		foundBomo := false
		for _, p := range report.Players {
			if p.Started {
				counts[p.TeamIndex]++
			}
			if p.UID != nil && *p.UID == 2000182073 {
				foundBomo = true
				if !p.Started {
					t.Errorf("Bomo should start in report %d", id)
				}
			}
		}
		if !foundBomo || counts[report.HomeTeamIndex] != 11 || counts[report.AwayTeamIndex] != 11 {
			t.Errorf("report %d: Bomo found %v, starts %v", id, foundBomo, counts)
		}
	}
	var history fm.PlayerMatches
	if res := h.call("get_player_match_stats", map[string]any{"uid": 2000182073}, &history); res.IsError {
		t.Fatal(allText(res))
	}
	checked := 0
	for _, m := range history.Matches {
		if m.MatchID != nil && (*m.MatchID == 395 || *m.MatchID == 1128) {
			checked++
			if !m.Stats.Started {
				t.Errorf("history marks Bomo as a substitute in %d", *m.MatchID)
			}
		}
	}
	if checked != 2 {
		t.Fatalf("only %d of Bomo's two match rows checked", checked)
	}
}

func TestMatchReportTools(t *testing.T) {
	p := raw(123, "Match", "Player", 547, 100, 3)
	p.Index = 55
	kickoff := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	h := connect(t, New(Options{DefaultSave: "/test.fm", Logf: func(string, ...any) {}, Load: func(string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{ManagerTeamIndex: 547}, []savefile.Player{p}, world, fm.PlayerExtras{MatchReports: &savefile.MatchReports{Reports: []savefile.MatchReport{{ID: 395, Kickoff: kickoff, Home: 547, Away: 20739, Players: []savefile.MatchPlayer{{Person: 55, Played: true, Started: true, Passes: 33, PassesCompleted: 28, Rating: 6.7}}}}}}), nil
	}}))
	var report fm.MatchReport
	if res := h.call("get_match_report", map[string]any{"match_id": 395}, &report); res.IsError {
		t.Fatal(allText(res))
	}
	if report.MatchID == nil || *report.MatchID != 395 || len(report.Players) != 1 || report.Players[0].Name != "Match Player" || report.Players[0].Passes != 33 {
		t.Fatalf("report %+v", report)
	}
	var history fm.PlayerMatches
	if res := h.call("get_player_match_stats", map[string]any{"uid": 123}, &history); res.IsError {
		t.Fatal(allText(res))
	}
	if history.Total != 1 || len(history.Matches) != 1 || !strings.Contains(history.Coverage, "Retained") {
		t.Fatalf("history %+v", history)
	}
	for _, id := range []int{999, -1} {
		if res := h.call("get_match_report", map[string]any{"match_id": id}, nil); !res.IsError {
			t.Fatalf("accepted report id %d", id)
		}
	}
}
