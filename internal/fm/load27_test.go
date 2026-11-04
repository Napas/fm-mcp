package fm

import (
	"strings"
	"testing"
	"time"
)

const (
	testSave27Path  = "../../data/test_save_27.fm"
	testSave27bPath = "../../data/test_save_27b.fm" // after Kauno Žalgiris's first match
)

func TestLoadTestSave27Info(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	want := time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)
	if !g.Info.Date.Equal(want) || g.Info.Build != "27.0.0+2493426" || g.Info.ManagerClub != "FK Kauno Žalgiris" {
		t.Errorf("date %s, build %q, club %q; want date %s", g.Info.Date.Format(time.DateOnly), g.Info.Build, g.Info.ManagerClub, want.Format(time.DateOnly))
	}
}

func TestLoadTestSave27SeasonStats(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	if !g.hasSeason {
		t.Errorf("season stats not loaded; warnings %q", g.Info.Warnings)
	}
}

func TestLoadTestSave27Fixtures(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	if !g.hasFixtures || len(g.fixtures) == 0 {
		t.Errorf("fixtures not loaded (%d); warnings %q", len(g.fixtures), g.Info.Warnings)
	}
}

func TestLoadTestSave27Tactics(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})
		for _, w := range g.Info.Warnings {
			if strings.Contains(w, "tactic") || strings.Contains(w, "lineup") || strings.Contains(w, "formation") {
				t.Errorf("%s: warning %q", path, w)
			}
		}
	}
}

// After its first match Kauno Žalgiris's tactic has no selection, so its
// players come from the last match, against FK Banga.
func TestLoadTestSave27bTactic(t *testing.T) {
	v := loadTestSaveAt(t, testSave27bPath, LoadOptions{}).View(false)

	tac, ok := v.Tactic()
	if !ok || tac.Name != "4-3-3 DM Wide" || tac.Mentality != "Balanced" || tac.LineupSource != LineupLastMatch {
		t.Fatalf("tactic %q, %q, %q, %v", tac.Name, tac.Mentality, tac.LineupSource, ok)
	}
	if p := tac.Slots[0].Player; p == nil || p.Name != "Deividas Mikelionis" {
		t.Errorf("goalkeeper %+v, want Deividas Mikelionis", p)
	}
	l, err := v.LastLineup(16521)
	if err != nil {
		t.Fatal(err)
	}
	if l.Opponent != "FK Banga" || l.InPossessionShape != "4-1-2-2-1" || l.Slots[0].Player == nil || l.Slots[0].Player.ShirtNumber != 1 {
		t.Errorf("last lineup against %q, shape %q, goalkeeper %+v", l.Opponent, l.InPossessionShape, l.Slots[0].Player)
	}
}
