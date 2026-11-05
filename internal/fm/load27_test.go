package fm

import (
	"strings"
	"testing"
	"time"
)

const (
	testSave27Path  = "../../data/test_save_27.fm"
	testSave27bPath = "../../data/test_save_27b.fm" // after Kauno Žalgiris's first match
	kzTeam27        = 16521                         // Kauno Žalgiris's team index
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
	l, err := v.LastLineup(kzTeam27)
	if err != nil {
		t.Fatal(err)
	}
	if l.Opponent != "FK Banga" || l.InPossessionShape != "4-1-2-2-1" || l.Slots[0].Player == nil || l.Slots[0].Player.ShirtNumber != 1 {
		t.Errorf("last lineup against %q, shape %q, goalkeeper %+v", l.Opponent, l.InPossessionShape, l.Slots[0].Player)
	}
}

// Serves KŽ's A Lyga table: nine teams, FK Banga first and KŽ fourth.
func TestLoadTestSave27Table(t *testing.T) {
	kz := int64(kzTeam27)
	for _, path := range []string{testSave27Path, testSave27bPath} {
		got, err := loadTestSaveAt(t, path, LoadOptions{}).View(false).Tables(&kz, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(got) != 1 || len(got[0].Rows) != 9 || got[0].Rows[3].Team != "FK Kauno Žalgiris" || got[0].Rows[0].Team != "FK Banga" {
			t.Errorf("%s: tables %+v", path, got)
		}
	}
}

// Ben Fox's injury in data/test_save_27b.fm: index 118 from 2026-01-01,
// 213–243 days.
func TestLoadTestSave27Injuries(t *testing.T) {
	g := loadTestSaveAt(t, testSave27bPath, LoadOptions{})

	injured := 0
	for _, p := range g.players {
		if p.Injured() {
			injured++
		}
	}
	if injured != 1632 {
		t.Errorf("%d injured players, want 1632", injured)
	}
	p := g.byUID[215502]
	if p == nil {
		t.Fatal("no Ben Fox")
	}
	if len(p.Injuries) != 1 {
		t.Fatalf("Ben Fox: %+v", p.Injuries)
	}
	if in := p.Injuries[0]; in.ID != 118 || in.Since.Format(time.DateOnly) != "2026-01-01" ||
		in.ReturnEarliest.Format(time.DateOnly) != "2026-08-02" || in.ReturnLatest.Format(time.DateOnly) != "2026-09-02" {
		t.Errorf("Ben Fox: %+v, want index 118 since 2026-01-01, back 2026-08-02..09-02", in)
	}
}

// 33 arranged moves count, 17 of them dated before the game date.
func TestLoadTestSave27ArrangedMoves(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})

		arranged := 0
		for _, a := range g.aux {
			if a.interest.arranged != nil {
				arranged++
			}
		}
		if arranged != 33 {
			t.Errorf("%s: %d players with an arranged move, want 33", path, arranged)
		}
		for _, uid := range []uint32{
			2000365091, // Cavan Sullivan, 2028-01-01
			14217414,   // Lucas Beltrán, 2025-01-01 to his current club
		} {
			p := g.byUID[uid]
			if p == nil {
				t.Fatalf("%s: no player with uid %d", path, uid)
			}
			if g.aux[p.auxIdx].interest.arranged == nil {
				t.Errorf("%s: uid %d has no arranged move", path, uid)
			}
		}
	}
}

// Offset 105 says 2026-09-02 for Enzo Fernández and 2026-08-05 for Mohamed
// Salah; the game shows 2026-01-19 for both.
func TestLoadTestSave27JoinedClub(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})

		for _, p := range g.players {
			if p.JoinedClub != nil && p.JoinedClub.After(g.Info.Date) {
				t.Errorf("%s: %s joined %s, after the game date", path, p.Name, p.JoinedClub.Format(time.DateOnly))
				break
			}
		}
		for _, uid := range []uint32{
			14229525, // Enzo Fernández
			98028755, // Mohamed Salah
		} {
			p := g.byUID[uid]
			if p == nil {
				t.Fatalf("%s: no player with uid %d", path, uid)
			}
			if p.JoinedClub == nil || p.JoinedClub.Format(time.DateOnly) != "2026-01-19" {
				t.Errorf("%s: %s joined %v, want 2026-01-19", path, p.Name, p.JoinedClub)
			}
		}
	}
}
