package fm

import (
	"testing"
	"time"
)

const testSave27Path = "../../data/test_save_27.fm"

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
