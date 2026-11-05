package fm

import (
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	testSave27Path  = "../../data/test_save_27.fm"
	testSave27bPath = "../../data/test_save_27b.fm" // after Kauno Žalgiris's first match
	kzTeam27        = 16521                         // Kauno Žalgiris's team index
	enzoUID27       = 14229525                      // Enzo Fernández
	salahUID27      = 98028755                      // Mohamed Salah
	mikelionisUID27 = 47055959                      // Deividas Mikelionis, KŽ's goalkeeper
)

func TestLoadTestSave27Info(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	want := time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)
	if !g.Info.Date.Equal(want) || g.Info.Build != "27.0.0+2493426" || g.Info.ManagerClub != "FK Kauno Žalgiris" {
		t.Errorf("date %s, build %q, club %q; want date %s", g.Info.Date.Format(time.DateOnly), g.Info.Build, g.Info.ManagerClub, want.Format(time.DateOnly))
	}
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "3754") { // unverified-build warnings name the version
			t.Errorf("warning %q", w)
		}
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
	// As on the Tactics screen (2026-01-23, by role): the CM and AM drop to
	// the two CMs, the winger to wide midfield.
	for i, want := range map[int]struct{ ip, oop SlotRole }{
		6: {SlotRole{"MC", "right", "Central Midfielder"}, SlotRole{"MC", "right", "Central Midfielder"}},
		7: {SlotRole{"MC", "left", "Attacking Midfielder"}, SlotRole{"MC", "left", "Central Midfielder"}},
		8: {SlotRole{"AMR", "", "Winger"}, SlotRole{"MR", "", "Wide Midfielder"}},
	} {
		if s := tac.Slots[i]; s.InPossession != want.ip || s.OutOfPossession != want.oop {
			t.Errorf("slot %d: %+v → %+v, want %+v → %+v", i, s.InPossession, s.OutOfPossession, want.ip, want.oop)
		}
	}
	// By hand (0x4918654): Baldassarra 115 → 119 of 124, Kalik 111 → 117 of
	// 124, both ability 110. The 26.3.2 rules give 102 and 98.
	for i, want := range map[int]int{5: 105, 7: 103} {
		if a := tac.Slots[i].RoleAbility; a == nil || *a != want {
			t.Errorf("slot %d: role ability %v, want %d", i, a, want)
		}
	}
	// Out of possession (0x491aa68), by hand: Fiolić as Wide Midfielder at MR
	// 12 rates 105 less 20 % = 84 of his best 104, pulled up to 90; 90/104 × 105.
	if a := tac.Slots[8].OutOfPossessionRoleAbility; a == nil || *a != 90 {
		t.Errorf("slot 8: out-of-possession role ability %v, want 90", a)
	}
	// His best out-of-possession role gives his current ability, 105; the
	// other two values are this code's output, not checked in game.
	want := []RoleFit{{"Winger", "AMR", 105}, {"Inside Outlet Winger", "AMR", 103}, {"Wide Outlet Winger", "AMR", 103}}
	if prof, _ := v.Profile(tac.Slots[8].Player.UID); !slices.Equal(prof.BestOutOfPossessionRoles, want) {
		t.Errorf("Fiolić's best out-of-possession roles %+v, want %+v", prof.BestOutOfPossessionRoles, want)
	}
	for i, s := range tac.Slots {
		if s.OutOfPossessionRoleAbility == nil {
			t.Errorf("slot %d: no out-of-possession role ability", i)
		}
	}
	l, err := v.LastLineup(kzTeam27)
	if err != nil {
		t.Fatal(err)
	}
	if l.Opponent != "FK Banga" || l.InPossessionShape != "4-1-2-2-1" || l.OutOfPossessionShape != "4-1-4-1" || l.Slots[0].Player == nil || l.Slots[0].Player.ShirtNumber != 1 {
		t.Errorf("last lineup against %q, shapes %q / %q, goalkeeper %+v", l.Opponent, l.InPossessionShape, l.OutOfPossessionShape, l.Slots[0].Player)
	}
	// As the match report's OOP view shows (2026-01-23): the last match pairs
	// slots 6 to 8 as stored, unlike the tactic.
	for i, want := range map[int]struct {
		name string
		oop  SlotRole
	}{
		6: {"Vykintas Slivka", SlotRole{"MR", "", "Wide Midfielder"}},
		7: {"Anthony Kalik", SlotRole{"MC", "right", "Central Midfielder"}},
		8: {"Ivan Fiolić", SlotRole{"MC", "left", "Central Midfielder"}},
		9: {"Leon Kreković", SlotRole{"ML", "", "Wide Midfielder"}},
	} {
		if s := l.Slots[i]; s.Player == nil || s.Player.Name != want.name || s.OutOfPossession != want.oop {
			t.Errorf("last lineup slot %d: %+v → %+v, want %s → %+v", i, s.Player, s.OutOfPossession, want.name, want.oop)
		}
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
		for _, uid := range []uint32{enzoUID27, salahUID27} {
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

// Levels shown in game (2026-01-23 save; 27b is the same). Akusobi: KŽ B,
// Lithuania. Kadrić: on loan at FK Novi Pazar, Serbia.
func TestLoadTestSave27bReputationLevel(t *testing.T) {
	v := loadTestSaveAt(t, testSave27bPath, LoadOptions{}).View(false)

	for uid, name := range map[uint32]string{2000591747: "Davis Akusobi", 62220551: "Haris Kadrić"} {
		p, ok := v.Profile(uid)
		if !ok || p.Name != name {
			t.Fatalf("uid %d: %q, %v; want %s", uid, p.Name, ok, name)
		}
		if p.ReputationLevel != "National" {
			t.Errorf("%s: reputation level %q, want National", name, p.ReputationLevel)
		}
	}
}

// Values the game showed in the 2026-01-23 save; 27b holds the same ones.
func TestLoadTestSave27bMatchesGame(t *testing.T) {
	g := loadTestSaveAt(t, testSave27bPath, LoadOptions{})

	for _, tt := range []struct {
		uid  uint32
		wage int64
		end  string
	}{
		{enzoUID27, 250000, "2031-06-30"},  // €10.2M–17.24M a year shown
		{salahUID27, 351046, "2028-06-30"}, // €20.4M–26.03M a year shown
	} {
		p := g.byUID[tt.uid]
		if p == nil || p.Contract == nil {
			t.Fatalf("uid %d: no contract", tt.uid)
		}
		if end := p.Contract.End.Format(time.DateOnly); p.Contract.WeeklyWage != tt.wage || end != tt.end {
			t.Errorf("%s: wage %d, end %s; want %d, %s", p.Name, p.Contract.WeeklyWage, end, tt.wage, tt.end)
		}
	}
	p := g.byUID[mikelionisUID27] // the friendly against FK Banga
	if p == nil {
		t.Fatal("no Mikelionis")
	}
	f := g.auxOf(p).season.Friendlies
	if f == nil || f.Starts != 1 || f.GoalsConceded != 2 || f.RatingSum != 64 || f.Rated != 1 || f.Detail.Passes != 7 || f.Detail.PassesCompleted != 4 {
		t.Errorf("Mikelionis friendlies %+v, want 1 start, 2 conceded, 6.4, 4 of 7 passes", f)
	}
}
