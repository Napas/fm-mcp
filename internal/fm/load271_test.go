package fm

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// testSave271Path is a 27.1.0 KŽ save (game date 2028-03-11) that also
// holds match objects written by 27.0.0.
const testSave271Path = "../../data/test_save_27_1.fm"

// TestLoadTestSave271 checks that a 27.1.0 save loads with its tactic and
// every match object read. Its build is not verified in game, so that
// warning stays.
func TestLoadTestSave271(t *testing.T) {
	g := loadTestSaveAt(t, testSave271Path, LoadOptions{})

	want := time.Date(2028, 3, 11, 0, 0, 0, 0, time.UTC)
	if !g.Info.Date.Equal(want) || g.Info.Build != "27.1.0+2505117" || g.Info.ManagerTeamIndex != kzTeam27 {
		t.Errorf("date %s, build %q, team %d; want %s, 27.1.0+2505117, %d", g.Info.Date.Format(time.DateOnly), g.Info.Build, g.Info.ManagerTeamIndex, want.Format(time.DateOnly), kzTeam27)
	}
	// Every warning the save gives; any other is a regression.
	expected := []string{
		"database version 3756 has not been checked",
		"of 470 league stages have no readable table",
		"match objects without a fixture: 232", // 2026 matches whose fixtures the game dropped
		"no language database",                 // loaded without the game's files
	}
	unverified := false
	for _, w := range g.Info.Warnings {
		i := slices.IndexFunc(expected, func(e string) bool { return strings.Contains(w, e) })
		switch {
		case i < 0:
			t.Errorf("warning %q", w)
		case i == 0:
			unverified = true
		}
	}
	if !unverified {
		t.Errorf("no unverified-build warning in %q", g.Info.Warnings)
	}
	v := g.View(false)
	if tac, ok := v.Tactic(); !ok || len(tac.Slots) != 11 {
		t.Errorf("tactic %+v, %v; want KŽ's 11 slots", tac, ok)
	}
	if b := v.Budgets(); b == nil {
		t.Error("no budgets for KŽ")
	}
	m, err := v.Match(MatchSelector{MatchID: ptr(int64(51085))}) // KŽ 3–1 Wolfsberger, a 27.1.0 object
	if err != nil || len(m.Players) == 0 || len(m.TeamStats) != 2 {
		t.Errorf("match 51085: %d players, %d team stats, err %v; want both", len(m.Players), len(m.TeamStats), err)
	}
}
