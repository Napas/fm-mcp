package fm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// zazSlots is FC Metz's "ZaZ - Autumn 3.39 ST" (real KŽ save, 26.3.2):
// in- and out-of-possession position masks and role values (as stored, with the duty bit) by slot.
func zazSlots() [11]savefile.TacticSlot {
	ipPos := []uint32{1, 0x20, 0x200010, 0x100010, 0x40, 0x200400, 0x10, 0x800, 0x1000, 0x100400, 0x4000}
	oopPos := []uint32{1, 4, 0x200010, 0x100010, 8, 0x200080, 0x10, 0x800, 0x1000, 0x100080, 0x2000}
	ipRole := []uint64{0x401000, 0x4000800000, 0x40000000800000, 0x40000000800000, 0x4000800000, 0x100000000800000, 0x1200000, 0x8800000, 0x8800000, 0x100000000800000, 0x80800000}
	oopRole := []uint64{0x400001, 0x200200, 0x200008, 0x200008, 0x200200, 0x208000, 0x200008, 0x4000400000, 0x4000400000, 0x208000, 0x40400000}
	var out [11]savefile.TacticSlot
	for i := range out {
		out[i] = savefile.TacticSlot{
			InPossession:    savefile.SlotRole{Position: ipPos[i], Role: ipRole[i]},
			OutOfPossession: savefile.SlotRole{Position: oopPos[i], Role: oopRole[i]},
		}
	}
	return out
}

// emptyLineup is a selection with no players.
func emptyLineup() [26]int32 {
	var l [26]int32
	for i := range l {
		l[i] = -1
	}
	return l
}

// tacticPlayers are Spurs players with person indices 100 (Vicario), 101
// (Romero) and 102 (Bergvall).
func tacticPlayers() []savefile.Player {
	vicario := rawPlayer(1, "Guglielmo", "Vicario")
	vicario.Index = 100
	romero := rawPlayer(2, "Cristian", "Romero")
	romero.Index = 101
	bergvall := rawPlayer(4, "Lucas", "Bergvall")
	bergvall.Index = 102
	return []savefile.Player{vicario, romero, bergvall}
}

func tacticGame(t *savefile.Tactics) *Game {
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, tacticPlayers(), testWorld, PlayerExtras{Tactics: t})
}

func zazTactics() *savefile.Tactics {
	sel := emptyLineup()
	sel[0], sel[2], sel[10] = 100, 101, 999 // 999 is no player
	sel[11], sel[13] = 102, 100
	return &savefile.Tactics{
		Human: 88351, Team: 547, Selection: sel, Active: 0,
		Tactics:     []savefile.Tactic{{Name: "ZaZ - Autumn 3.39 ST", Mentality: 6, Slots: zazSlots()}},
		LastLineups: map[int32]savefile.LastLineup{547: {Opponent: 670, Players: emptyLineup()}},
	}
}

func TestTacticNamesSlotsAndSelectedPlayers(t *testing.T) {
	tac, ok := tacticGame(zazTactics()).View(true).Tactic()
	if !ok {
		t.Fatal("no tactic")
	}

	if tac.TeamIndex != 547 || tac.Team != "Tottenham Hotspur" || tac.Name != "ZaZ - Autumn 3.39 ST" || tac.Mentality != "Attacking" || tac.LineupSource != LineupSelected {
		t.Errorf("tactic = %+v", tac)
	}
	if len(tac.Slots) != 11 {
		t.Fatalf("%d slots, want 11", len(tac.Slots))
	}
	want := []TacticSlot{
		{Slot: 0,
			InPossession:    SlotRole{Position: "GK", Role: "Ball-Playing Goalkeeper"},
			OutOfPossession: SlotRole{Position: "GK", Role: "Goalkeeper"},
			Player:          &TacticPlayer{UID: 1, Name: "Guglielmo Vicario"},
			RoleAbility:     ptr(49)}, // GK rated 1
		{Slot: 1,
			InPossession:    SlotRole{Position: "WBR", Role: "Advanced Wing-Back"},
			OutOfPossession: SlotRole{Position: "DR", Role: "Full-Back"}},
		{Slot: 2,
			InPossession:    SlotRole{Position: "DC", Side: "right", Role: "Overlapping Centre-Back"},
			OutOfPossession: SlotRole{Position: "DC", Side: "right", Role: "Centre-Back"},
			Player:          &TacticPlayer{UID: 2, Name: "Cristian Romero"},
			RoleAbility:     ptr(45)},
	}
	if !reflect.DeepEqual(tac.Slots[:3], want) {
		t.Errorf("slots 0-2 = %+v, want %+v", tac.Slots[:3], want)
	}
	st := tac.Slots[10]
	if st.InPossession != (SlotRole{Position: "ST", Role: "Channel Forward"}) ||
		st.OutOfPossession != (SlotRole{Position: "AMC", Role: "Attacking Midfielder"}) || !reflect.DeepEqual(st.Player, &TacticPlayer{Unnamed: true}) {
		t.Errorf("slot 10 = %+v, player %+v; want ST / AMC with an unnamed player (999 is not a player)", st, st.Player)
	}
	if !reflect.DeepEqual(tac.Bench, []TacticPlayer{{UID: 4, Name: "Lucas Bergvall"}, {UID: 1, Name: "Guglielmo Vicario"}}) {
		t.Errorf("bench = %+v", tac.Bench)
	}
}

func TestTacticJSON(t *testing.T) {
	tac, _ := tacticGame(zazTactics()).View(false).Tactic()
	b, err := json.Marshal(tac)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"team_index":547`, `"name":"ZaZ - Autumn 3.39 ST"`, `"mentality":"Attacking"`, `"lineup_source":"selected"`,
		`"in_possession":{"position":"DC","side":"right","role":"Overlapping Centre-Back"}`,
		`"player":{"uid":2,"name":"Cristian Romero"},"role_ability":45`, `"bench":[{"uid":4`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON lacks %s: %s", want, b)
		}
	}
}

func TestTacticHidesRoleAbilityWithCurrentAbility(t *testing.T) {
	players := tacticPlayers()
	players[1].TeamIndex, players[1].CurrentReputation = 20739, 0 // Romero, unscouted at another club
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, players, testWorld, PlayerExtras{Tactics: zazTactics()})

	fogged, _ := g.View(true).Tactic()
	clear, _ := g.View(false).Tactic()

	if fogged.Slots[0].RoleAbility == nil || fogged.Slots[2].RoleAbility != nil {
		t.Errorf("with fog of war: Vicario %v, Romero %v; want only Vicario's", fogged.Slots[0].RoleAbility, fogged.Slots[2].RoleAbility)
	}
	if clear.Slots[2].RoleAbility == nil || *clear.Slots[2].RoleAbility != 45 {
		t.Errorf("without fog of war: Romero %v, want 45", clear.Slots[2].RoleAbility)
	}
	if again, _ := g.View(false).Tactic(); again.Slots[2].RoleAbility == nil {
		t.Error("the fogged view changed the game's tactic")
	}
}

func TestTacticFallsBackToTheLastMatchLineup(t *testing.T) {
	tactics := zazTactics()
	tactics.Selection = emptyLineup()
	last := emptyLineup()
	last[0], last[5], last[12] = 101, 100, 102
	tactics.LastLineups[547] = savefile.LastLineup{Opponent: 670, Players: last}
	tactics.LastLineups[20739] = savefile.LastLineup{Players: emptyLineup()}

	tac, ok := tacticGame(tactics).View(true).Tactic()

	if !ok || tac.LineupSource != LineupLastMatch {
		t.Fatalf("tactic = %+v, %v; want the last match's lineup", tac, ok)
	}
	if p := tac.Slots[0].Player; p == nil || p.UID != 2 {
		t.Errorf("slot 0 player = %+v, want Romero", p)
	}
	if p := tac.Slots[5].Player; p == nil || p.UID != 1 {
		t.Errorf("slot 5 player = %+v, want Vicario", p)
	}
	if !reflect.DeepEqual(tac.Bench, []TacticPlayer{{UID: 4, Name: "Lucas Bergvall"}}) {
		t.Errorf("bench = %+v", tac.Bench)
	}
}

func TestTacticWithoutAnyLineup(t *testing.T) {
	tactics := zazTactics()
	tactics.Selection = emptyLineup()
	delete(tactics.LastLineups, 547)

	tac, ok := tacticGame(tactics).View(true).Tactic()

	if !ok || tac.LineupSource != LineupNone || tac.Bench != nil {
		t.Fatalf("tactic = %+v, %v", tac, ok)
	}
	for _, s := range tac.Slots {
		if s.Player != nil {
			t.Errorf("slot %d has player %+v", s.Slot, s.Player)
		}
	}
}

func TestTacticPicksTheActiveTactic(t *testing.T) {
	tactics := zazTactics()
	tactics.Tactics = append(tactics.Tactics, savefile.Tactic{Name: "Second", Mentality: 4})

	for _, tt := range []struct {
		active int32
		want   string
	}{{1, "Second"}, {0, "ZaZ - Autumn 3.39 ST"}, {-1, "ZaZ - Autumn 3.39 ST"}, {2, "ZaZ - Autumn 3.39 ST"}} {
		tactics.Active = tt.active
		if tac, ok := tacticGame(tactics).View(false).Tactic(); !ok || tac.Name != tt.want {
			t.Errorf("active %d: %q, %v; want %q", tt.active, tac.Name, ok, tt.want)
		}
	}
}

func TestTacticLeavesUnknownValuesEmpty(t *testing.T) {
	tactics := zazTactics()
	tactics.Tactics[0].Mentality = 9
	tactics.Tactics[0].Slots[0].InPossession = savefile.SlotRole{Position: 0x11, Role: 1 << 63}

	tac, _ := tacticGame(tactics).View(false).Tactic()

	if tac.Mentality != "" || tac.Slots[0].InPossession != (SlotRole{}) {
		t.Errorf("mentality %q, slot 0 %+v; want both empty", tac.Mentality, tac.Slots[0].InPossession)
	}
}

func TestNoTacticWithoutASavedOne(t *testing.T) {
	tactics := zazTactics()
	tactics.Tactics = nil

	for name, tt := range map[string]*savefile.Tactics{"no tactics_man.dat": nil, "no tactic": tactics} {
		if tac, ok := tacticGame(tt).View(false).Tactic(); ok {
			t.Errorf("%s: tactic %+v", name, tac)
		}
	}
}

func TestCheckTactics(t *testing.T) {
	tactics := *zazTactics()

	if got, w := checkTactics(tactics, nil, 547); got == nil || got.Team != 547 || len(got.Tactics) != 1 || w != nil {
		t.Errorf("own team: %v, %q", got, w)
	}
	if got, w := checkTactics(tactics, nil, noTeam); got == nil || len(got.Tactics) != 1 || w != nil {
		t.Errorf("unknown manager team: %v, %q", got, w)
	}
	got, w := checkTactics(tactics, nil, 652)
	if got == nil || got.Tactics != nil || !reflect.DeepEqual(got.LastLineups, tactics.LastLineups) || len(w) != 1 || !strings.Contains(w[0], "team 547") {
		t.Errorf("other team: %v, %q; want the last lineups without tactics and a warning naming the team", got, w)
	}
	noTeamEntry := savefile.Tactics{Team: -1, Active: -1}
	if _, w := checkTactics(noTeamEntry, nil, 547); w != nil {
		t.Errorf("no team entry: warnings %q", w)
	}
	for _, err := range []error{savefile.ErrUnsupported, savefile.ErrCorrupt} {
		got, w := checkTactics(savefile.Tactics{}, err, 547)
		if got != nil || len(w) != 1 || !strings.HasPrefix(w[0], "current tactic and last-match lineups unavailable: ") {
			t.Errorf("%v: %v, %q; want no tactics and a warning", err, got, w)
		}
	}
	tactics.BadFormations = 3
	if got, w := checkTactics(tactics, nil, 547); got == nil || !reflect.DeepEqual(w, []string{"3 teams' last-match formations could not be decoded and are missing"}) {
		t.Errorf("bad formations: %v, %q", got, w)
	}
}

func TestCheckTacticsWarnsWhenLastLineupsWereScanned(t *testing.T) {
	walkErr := &savefile.WalkError{
		Part: "a team's set pieces or records",
		Err:  fmt.Errorf("%w: set-piece routines version 0x3d at offset 900", savefile.ErrCorrupt),
	}
	tactics := *zazTactics()
	tactics.WalkErr = walkErr
	want := "tactics layout not fully recognised; last lineups found by searching the file (stopped at a team's set pieces or records)"
	if got, w := checkTactics(tactics, nil, 547); got == nil || len(got.Tactics) != 1 || !reflect.DeepEqual(w, []string{want}) {
		t.Errorf("first entry read: %v, %q; want the tactic and %q", got, w, want)
	}
	noEntry := savefile.Tactics{Team: -1, Active: -1, LastLineups: tactics.LastLineups, WalkErr: walkErr}
	if got, w := checkTactics(noEntry, nil, 547); got == nil || !reflect.DeepEqual(w, []string{"current tactic unavailable; " + want}) {
		t.Errorf("first entry not read: %v, %q; want %q", got, w, "current tactic unavailable; "+want)
	}
}

func TestCheckTacticsTwoHumans(t *testing.T) {
	tacticErr := fmt.Errorf("%w: 2 humans in tactics_man.dat", savefile.ErrUnsupported)
	tactics := savefile.Tactics{Team: -1, Active: -1, LastLineups: zazTactics().LastLineups, TacticErr: tacticErr}
	got, w := checkTactics(tactics, nil, 547)
	if got == nil || !reflect.DeepEqual(got.LastLineups, tactics.LastLineups) || !reflect.DeepEqual(w, []string{"current tactic unavailable: " + tacticErr.Error()}) {
		t.Errorf("got %v, %q; want the last lineups and one warning", got, w)
	}
	// The walk also failed: the tactic warning is not repeated.
	tactics.WalkErr = &savefile.WalkError{Part: "a manager's header", Err: savefile.ErrCorrupt}
	if _, w := checkTactics(tactics, nil, 547); len(w) != 2 || strings.HasPrefix(w[1], "current tactic") {
		t.Errorf("warnings %q; want the tactic warning, then the layout warning without a prefix", w)
	}
}

func TestLoadTestSaveHasNoTactic(t *testing.T) {
	g := loadTestSave(t)

	if tac, ok := g.View(false).Tactic(); ok {
		t.Errorf("tactic = %+v; the test save holds none", tac)
	}
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "tactic") {
			t.Errorf("warning %q", w)
		}
	}
}

// TestRealSavesTactic checks FC Metz's tactic and last match in the real KŽ
// saves: on 2035-12-09 (FM_REAL_SAVE_9DEC) the XI and bench are selected and
// played the last match, against Reims; on 2035-12-02 (FM_REAL_SAVE) the last
// match's XI, against Marseille, is used. Never point these at the game's own
// save folder.
func TestRealSavesTactic(t *testing.T) {
	for _, tt := range []struct {
		env      string
		date     time.Time
		source   string
		xi       []string
		bench    int
		shirts   metzShirts
		opponent opponentWant
	}{
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), LineupSelected,
			[]string{"Bruno Sami", "Danilo Veiga", "Joseph Nduquidi", "Fernando Medina", "Mathias Fjørtoft Løvik", "Thomas Tacalfred", "Karl Vassiljev", "Anton Matković", "Luke Reddy", "Samal Edubatey", "Stephen Ly"}, 9,
			metzShirts{88, 22, 5, 18, 32, 8, 43, 17, 16, 6, 1, 77, 70, 25, 15, 4, 3, 13, 28, 27},
			opponentWant{1545, "Reims", "Balanced", "4-2-3-1", "4-2-2-1-1"}}, // out of possession DM ×2, MR, ML, AMC, ST
		{"FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC), LineupLastMatch,
			[]string{"Bruno Sami", "Danilo Veiga", "Carlo Bernardi", "Chris Ashby", "Louis Leroux", "Samal Edubatey", "Joseph Nduquidi", "Anton Matković", "Luke Reddy", "Thomas Tacalfred", "Stephen Ly"}, -1,
			metzShirts{88, 22, 19, 4, 41, 6, 5, 17, 16, 8, 1, 77, 27, 25, 15, 70, 3, 13, 18, 28},
			opponentWant{670, "Marseille", "Positive", "4-2-3-1", "4-2-2-2"}},
	} {
		t.Run(tt.env, func(t *testing.T) {
			g := loadRealSave(t, tt.env, tt.date)
			for _, w := range g.Info.Warnings {
				if strings.Contains(w, "tactic") {
					t.Errorf("warning %q", w)
				}
			}
			tac, ok := g.View(true).Tactic()
			if !ok {
				t.Fatal("no tactic")
			}
			if tac.Name != "ZaZ - Autumn 3.39 ST" || tac.TeamIndex != g.Info.ManagerTeamIndex || tac.TeamIndex != 652 || tac.LineupSource != tt.source || tac.Mentality != "Attacking" {
				t.Errorf("tactic %q, team %d (manager's %d), source %q, mentality %q", tac.Name, tac.TeamIndex, g.Info.ManagerTeamIndex, tac.LineupSource, tac.Mentality)
			}
			for _, s := range tac.Slots {
				if s.Player == nil {
					continue
				}
				if p, ok := g.byUID[s.Player.UID]; !ok || p.Name != s.Player.Name {
					t.Errorf("slot %d: uid %d is not %s", s.Slot, s.Player.UID, s.Player.Name)
				}
			}
			if xi := slotNames(tac.Slots); !slices.Equal(xi, tt.xi) {
				t.Errorf("XI = %q, want %q", xi, tt.xi)
			}
			if s := tac.Slots[0].InPossession; s.Position != "GK" {
				t.Errorf("slot 0 = %+v, want GK", s)
			}
			if s := tac.Slots[10]; s.InPossession.Position != "ST" || s.OutOfPossession.Position != "AMC" {
				t.Errorf("slot 10 = %+v, want ST in possession, AMC out of it", s)
			}
			if tt.bench >= 0 && len(tac.Bench) != tt.bench {
				t.Errorf("bench = %d players, want %d", len(tac.Bench), tt.bench)
			}

			checkRealLastLineups(t, g, tt.xi, tt.shirts, tt.opponent)
		})
	}
}
