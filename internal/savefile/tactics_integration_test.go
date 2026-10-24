package savefile

import (
	"slices"
	"testing"
	"time"
)

// Real-save values decoded from tactics_man.dat of the KŽ saves (26.3.2).
const (
	metzTeam   = 652
	kzHuman    = 158530
	zazPresets = 19
)

// openTacticsSave opens the copy of a real save named by env and reads its
// tactics and the tactics_man.dat body, skipping when env is unset and
// failing when the save is not of game date want.
func openTacticsSave(t *testing.T, env string, want time.Time) (Tactics, []byte) {
	t.Helper()
	a, file := openRealArchive(t, env)
	s, err := ReadSummary(a)
	if err != nil {
		t.Fatal(err)
	}
	requireGameDate(t, env, file, s.GameDate.Time(), want)
	tac, err := ReadTactics(a)
	if err != nil {
		t.Fatal(err)
	}
	f, err := a.ReadFile("tactics_man.dat")
	if err != nil {
		t.Fatal(err)
	}
	return tac, f.Body
}

// checkZaZ checks FC Metz's tactic, identical in the 12-02 and 12-09 saves,
// and the human's four team entries: the first team, FC Metz 2 and team
// 38 394 hold the tactic, team 86 128 none.
func checkZaZ(t *testing.T, tac Tactics, body []byte) {
	t.Helper()
	if tac.Human != kzHuman || tac.Team != metzTeam || tac.Active != 0 || len(tac.Tactics) != 1 {
		t.Fatalf("human %d, team %d, active %d, %d tactics; want %d, %d, 0, 1", tac.Human, tac.Team, tac.Active, len(tac.Tactics), kzHuman, metzTeam)
	}
	if got, want := tac.Tactics[0], zazTactic(); got != want {
		t.Errorf("tactic = %+v\nwant %+v", got, want)
	}
	if tac.WalkErr != nil || tac.TacticErr != nil {
		t.Errorf("walk error %v, tactic error %v; want none", tac.WalkErr, tac.TacticErr)
	}
	for i, e := range walkedEntries(t, body, kzHuman, []int32{metzTeam, 19023, 38394, 86128}, []int32{0, 0, 0, -1}) {
		want := 1
		if i == 3 {
			want = 0
		}
		if len(e.tactics) != want || want == 1 && e.tactics[0] != zazTactic() {
			t.Errorf("team %d: tactics %+v, want %d copies of ZaZ", e.team, e.tactics, want)
		}
		if i > 0 && e.selection != emptySelection() {
			t.Errorf("team %d: selection %v, want empty", e.team, e.selection)
		}
	}
}

// walkedEntries walks body and checks that it holds one human with the given
// team entries and active indices; it returns the entries.
func walkedEntries(t *testing.T, body []byte, human uint32, teams, active []int32) []teamEntry {
	t.Helper()
	w := tacticsWalker{r: newReader(body)}
	if _, err := w.walk(); err != nil || len(w.humans) != 1 || w.humans[0].human != human {
		t.Fatalf("walk error %v, %d humans; want nil, 1 (%d)", err, len(w.humans), human)
	}
	var gotTeams, gotActive []int32
	for _, e := range w.humans[0].teams {
		gotTeams = append(gotTeams, e.team)
		gotActive = append(gotActive, e.active)
	}
	if !slices.Equal(gotTeams, teams) || !slices.Equal(gotActive, active) {
		t.Errorf("team entries %v, active %v; want %v, %v", gotTeams, gotActive, teams, active)
	}
	return w.humans[0].teams
}

// TestTacticsWalkMatchesScan checks that the structural walk reaches the
// last-lineup table where the scan finds it.
func TestTacticsWalkMatchesScan(t *testing.T) {
	for _, tt := range []struct {
		name, env string // env "" = the test save
		at, n     int    // 0 = not pinned
	}{
		{"test save", "", 8704, 2413},
		{"12-02", "FM_REAL_SAVE", 31267, 15016},
		{"12-09", "FM_REAL_SAVE_9DEC", 31267, 15016},
		{"12-14", "FM_REAL_SAVE_14DEC", 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var a *Archive
			if tt.env == "" {
				a = openTestSave(t)
			} else {
				a, _ = openRealArchive(t, tt.env)
			}
			f, err := a.ReadFile("tactics_man.dat")
			if err != nil {
				t.Fatal(err)
			}
			w := tacticsWalker{r: newReader(f.Body)}
			walked, err := w.walk()
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			scanned, err := findLastLineups(f.Body, 0)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if walked.at != scanned.at || walked.presets != scanned.presets || len(walked.lineups) != len(scanned.lineups) {
				t.Errorf("walk: table at %d, %d presets, %d lineups; scan: %d, %d, %d",
					walked.at, walked.presets, len(walked.lineups), scanned.at, scanned.presets, len(scanned.lineups))
			}
			if tt.at != 0 && (walked.at != tt.at || len(walked.lineups) != tt.n) {
				t.Errorf("table at %d with %d lineups, want %d, %d", walked.at, len(walked.lineups), tt.at, tt.n)
			}
			if walked.presets != zazPresets {
				t.Errorf("%d presets, want %d", walked.presets, zazPresets)
			}
		})
	}
}

func TestRealSaveTactics(t *testing.T) {
	tac, body := openTacticsSave(t, "FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC))
	checkZaZ(t, tac, body)
	if tac.Selection != emptySelection() {
		t.Errorf("selection = %v, want empty", tac.Selection)
	}
	// The XI of the last match, against Marseille.
	want := emptySelection()
	copy(want[:], []int32{183247, 40427, 169046, 146891, 62597, 172728, 55473, 77416, 163497, 182575, 147261})
	got := tac.LastLineups[metzTeam]
	if got.Opponent != 670 || [11]int32(got.Players[:11]) != [11]int32(want[:11]) {
		t.Errorf("Metz last lineup = %+v, want opponent 670, XI %v", got, want[:11])
	}
	if got.Formation == nil || *got.Formation != *zazFormation() {
		t.Errorf("Metz formation = %+v, want %+v", got.Formation, zazFormation())
	}
	// Squad numbers seen in game: Sami 88, Veiga 22, Bernardi 19, Ashby 4,
	// Leroux 41, Edubatey 6, Nduquidi 5, Matković 17, Reddy 16, Tacalfred 8, Ly 1.
	if xi := [11]uint8(got.ShirtNumbers[:11]); xi != [11]uint8{88, 22, 19, 4, 41, 6, 5, 17, 16, 8, 1} {
		t.Errorf("Metz shirt numbers = %v", xi)
	}
	// Marseille, Metz's opponent.
	checkMetzOpponent(t, tac, 670, 5, [11]SlotRole{
		{0x1, 0x1}, {0x4, 0x100000000000}, {0x200010, 0x1000000}, {0x100010, 0x1000000}, {0x8, 0x10000000000000},
		{0x200080, 0x400000000000}, {0x100080, 0x8000}, {0x800, 0x80}, {0x2000, 0x100000000}, {0x1000, 0x80}, {0x4000, 0x400},
	}, [11]SlotRole{
		{0x1, 0x4}, {0x4, 0x800}, {0x200010, 0x8}, {0x100010, 0x20}, {0x8, 0x400}, {0x200080, 0x10000}, {0x100080, 0x40000},
		{0x100, 0x1000000000}, {0x200, 0x800000000}, {0x204000, 0x100000000000}, {0x104000, 0x40000000000},
	})
	checkAllFormations(t, tac, 15016)
}

// checkMetzOpponent checks the last match of FC Metz's last opponent: its
// opponent, mentality and both phases' positions and roles.
func checkMetzOpponent(t *testing.T, tac Tactics, team int32, mentality uint8, wantIP, wantOOP [tacticSlots]SlotRole) {
	t.Helper()
	l := tac.LastLineups[team]
	if l.Opponent != metzTeam || l.Formation == nil || l.Formation.Mentality != mentality {
		t.Fatalf("team %d last lineup = %+v; want opponent %d, mentality %d", team, l, metzTeam, mentality)
	}
	for i, s := range l.Formation.Slots {
		if s.InPossession != wantIP[i] {
			t.Errorf("team %d slot %d in possession = %+v, want %+v", team, i, s.InPossession, wantIP[i])
		}
		if s.OutOfPossession != wantOOP[i] {
			t.Errorf("team %d slot %d out of possession = %+v, want %+v", team, i, s.OutOfPossession, wantOOP[i])
		}
	}
}

// checkAllFormations checks that every last-lineup record's formation
// decodes, with the goalkeeper in slot 0 and 11 distinct positions per phase.
func checkAllFormations(t *testing.T, tac Tactics, want int) {
	t.Helper()
	if n := len(tac.LastLineups); n != want || tac.BadFormations != 0 {
		t.Fatalf("%d last lineups, %d bad formations; want %d, 0", n, tac.BadFormations, want)
	}
	for team, l := range tac.LastLineups {
		f := l.Formation
		if f == nil {
			t.Fatalf("team %d: no formation", team)
		}
		for _, phase := range []func(TacticSlot) SlotRole{
			func(s TacticSlot) SlotRole { return s.InPossession },
			func(s TacticSlot) SlotRole { return s.OutOfPossession },
		} {
			seen := map[uint32]bool{}
			for _, s := range f.Slots {
				seen[phase(s).Position] = true
			}
			if phase(f.Slots[0]).Position != 0x1 || len(seen) != 11 {
				t.Fatalf("team %d: formation %+v; want GK in slot 0 and 11 positions", team, *f)
			}
		}
	}
}

func TestRealSave9DecTactics(t *testing.T) {
	tac, body := openTacticsSave(t, "FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC))
	checkZaZ(t, tac, body)
	if tac.Selection != zazSelection() {
		t.Errorf("selection = %v, want %v", tac.Selection, zazSelection())
	}
	if n := len(tac.LastLineups); n != 15016 {
		t.Errorf("%d last lineups, want 15016", n)
	}
	// The selected XI played the last match, against Reims.
	xi := zazSelection()
	if got := tac.LastLineups[metzTeam]; got.Opponent != 1545 || [11]int32(got.Players[:11]) != [11]int32(xi[:11]) {
		t.Errorf("Metz last lineup = %+v", got)
	}
	if f := tac.LastLineups[metzTeam].Formation; f == nil || *f != *zazFormation() {
		t.Errorf("Metz formation = %+v, want %+v", f, zazFormation())
	}
	// Squad numbers seen in game (2035-12-14): XI Sami 88 … Ly 1, bench
	// Haddad 77 … Karaboué 27.
	if got, want := tac.LastLineups[metzTeam].ShirtNumbers, zazShirtNumbers(); [20]uint8(got[:20]) != [20]uint8(want[:20]) {
		t.Errorf("Metz shirt numbers = %v, want %v", got[:20], want[:20])
	}
	// Reims, Metz's opponent.
	checkMetzOpponent(t, tac, 1545, 4, [11]SlotRole{
		{0x1, 0x20000000000000}, {0x4, 0x8}, {0x200010, 0x1000000}, {0x100010, 0x1000000}, {0x8, 0x10000000000000},
		{0x200080, 0x8000}, {0x100080, 0x10}, {0x800, 0x8000000}, {0x2000, 0x20000000000}, {0x1000, 0x200000000000}, {0x4000, 0x80000},
	}, [11]SlotRole{
		{0x1, 0x4}, {0x4, 0x800}, {0x200010, 0x20}, {0x100010, 0x8}, {0x8, 0x800}, {0x200080, 0x10000}, {0x100080, 0x20000},
		{0x100, 0x800000000}, {0x200, 0x800000000}, {0x2000, 0x40000000}, {0x4000, 0x100000000000},
	})
	checkAllFormations(t, tac, 15016)
}

func TestTestSaveTactics(t *testing.T) {
	a := openTestSave(t)
	f, err := a.ReadFile("tactics_man.dat")
	if err != nil {
		t.Fatal(err)
	}
	if f.Header.Version != tacticsVersion {
		t.Fatalf("version %d, want %d", f.Header.Version, tacticsVersion)
	}
	tac, err := parseTactics(f.Body)
	if err != nil {
		t.Fatal(err)
	}
	if tac.Human != 88351 || tac.Team != 547 || tac.Active != -1 || len(tac.Tactics) != 0 || tac.Selection != emptySelection() {
		t.Errorf("got human %d, team %d, active %d, %d tactics, selection %v; want 88351, 547, -1, none, empty",
			tac.Human, tac.Team, tac.Active, len(tac.Tactics), tac.Selection)
	}
	if n := len(tac.LastLineups); n != 2413 {
		t.Errorf("%d last lineups, want 2413", n)
	}
	// Tottenham has not played yet.
	if l := tac.LastLineups[547]; l.Players != emptySelection() || l.Opponent != -1 {
		t.Errorf("Tottenham last lineup = %+v, want empty", l)
	}
	checkAllFormations(t, tac, 2413)
	for _, e := range walkedEntries(t, f.Body, 88351, []int32{547, 27845, 27846}, []int32{-1, -1, -1}) {
		if len(e.tactics) != 0 || e.selection != emptySelection() {
			t.Errorf("team %d: %d tactics, selection %v; want none, empty", e.team, len(e.tactics), e.selection)
		}
	}
}
