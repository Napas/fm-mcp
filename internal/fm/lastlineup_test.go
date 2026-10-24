package fm

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// lastLineupTactics holds Spurs' last match against Chelsea in Metz's
// formation: Vicario (1) in goal, Romero (17) at right centre-back, person
// 999 (not a player, 9) up front, and on the bench Bergvall (15) and person
// 998 (not a player, no number). Slot 1 is empty but has a number.
func lastLineupTactics() *savefile.Tactics {
	players := emptyLineup()
	players[0], players[2], players[10], players[12], players[14] = 100, 101, 999, 102, 998
	var shirts [26]uint8
	for i := range shirts {
		shirts[i] = 255
	}
	shirts[0], shirts[1], shirts[2], shirts[10], shirts[12] = 1, 5, 17, 9, 15
	t := zazTactics()
	t.LastLineups = map[int32]savefile.LastLineup{
		547:   {Opponent: 20739, Players: players, ShirtNumbers: shirts, Formation: &savefile.Formation{Mentality: 6, Slots: zazSlots()}},
		900:   {Opponent: -1, Players: emptyLineup(), Formation: &savefile.Formation{Mentality: 4, Slots: zazSlots()}},
		20739: {Opponent: 547, Players: emptyLineup()},
	}
	return t
}

func TestLastLineup(t *testing.T) {
	got, err := tacticGame(lastLineupTactics()).View(true).LastLineup(547)
	if err != nil {
		t.Fatal(err)
	}

	if got.TeamIndex != 547 || got.Team != "Tottenham Hotspur" || got.OpponentIndex != 20739 || got.Opponent != "Chelsea" ||
		got.Mentality != "Attacking" || got.InPossessionShape != "3-2-2-2-1" || got.OutOfPossessionShape != "5-2-3" {
		t.Errorf("last lineup = %+v", got)
	}
	if len(got.Slots) != 11 {
		t.Fatalf("%d slots, want 11", len(got.Slots))
	}
	want := []TacticSlot{
		{Slot: 0,
			InPossession:    SlotRole{Position: "GK", Role: "Ball-Playing Goalkeeper"},
			OutOfPossession: SlotRole{Position: "GK", Role: "Goalkeeper"},
			Player:          &TacticPlayer{UID: 1, Name: "Guglielmo Vicario", ShirtNumber: 1}},
		{Slot: 1,
			InPossession:    SlotRole{Position: "WBR", Role: "Advanced Wing-Back"},
			OutOfPossession: SlotRole{Position: "DR", Role: "Full-Back"}},
		{Slot: 2,
			InPossession:    SlotRole{Position: "DC", Side: "right", Role: "Overlapping Centre-Back"},
			OutOfPossession: SlotRole{Position: "DC", Side: "right", Role: "Centre-Back"},
			Player:          &TacticPlayer{UID: 2, Name: "Cristian Romero", ShirtNumber: 17}},
	}
	if !reflect.DeepEqual(got.Slots[:3], want) {
		t.Errorf("slots 0-2 = %+v, want %+v", got.Slots[:3], want)
	}
	st := got.Slots[10]
	if st.InPossession.Position != "ST" || st.OutOfPossession.Position != "AMC" || !reflect.DeepEqual(st.Player, &TacticPlayer{ShirtNumber: 9, Unnamed: true}) {
		t.Errorf("slot 10 = %+v, player %+v; want ST / AMC with an unnamed number 9", st, st.Player)
	}
	if !reflect.DeepEqual(got.Bench, []TacticPlayer{{UID: 4, Name: "Lucas Bergvall", ShirtNumber: 15}, {Unnamed: true}}) {
		t.Errorf("bench = %+v", got.Bench)
	}
}

func TestLastLineupJSON(t *testing.T) {
	l, err := tacticGame(lastLineupTactics()).View(false).LastLineup(547)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"team_index":547`, `"opponent_team_index":20739`, `"opponent":"Chelsea"`, `"mentality":"Attacking"`,
		`"in_possession_shape":"3-2-2-2-1"`, `"out_of_possession_shape":"5-2-3"`,
		`"in_possession":{"position":"DC","side":"right","role":"Overlapping Centre-Back"}`, `"bench":[{"uid":4`,
		`"player":{"uid":2,"name":"Cristian Romero","shirt_number":17}`, `"player":{"shirt_number":9,"unnamed_player":true}`,
		`{"unnamed_player":true}]`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON lacks %s: %s", want, b)
		}
	}
}

func TestLastLineupWithoutFormation(t *testing.T) {
	tactics := lastLineupTactics()
	l := tactics.LastLineups[547]
	l.Formation = nil
	tactics.LastLineups[547] = l

	got, err := tacticGame(tactics).View(false).LastLineup(547)

	if err != nil || got.OpponentIndex != 20739 || got.Mentality != "" || got.InPossessionShape != "" || got.OutOfPossessionShape != "" {
		t.Fatalf("last lineup = %+v, %v; want the opponent without a formation", got, err)
	}
	if s := got.Slots[0]; s.InPossession != (SlotRole{}) || s.OutOfPossession != (SlotRole{}) || s.Player == nil || s.Player.UID != 1 {
		t.Errorf("slot 0 = %+v, want Vicario without positions", s)
	}
}

func TestLastLineupErrors(t *testing.T) {
	v := tacticGame(lastLineupTactics()).View(false)
	for _, tt := range []struct {
		team int64
		want error
	}{
		{12345, ErrInvalidFilter}, // no such team
		{900, ErrNoLastMatch},     // a record without an opponent
		{1233, ErrNoLastMatch},    // no record
	} {
		if l, err := v.LastLineup(tt.team); !errors.Is(err, tt.want) {
			t.Errorf("team %d: %+v, %v; want %v", tt.team, l, err, tt.want)
		}
	}
	if _, err := v.LastLineup(900); errors.Is(err, ErrLastLineupsUnavailable) {
		t.Errorf("a team without a match: %v; want only ErrNoLastMatch", err)
	}
	if _, err := tacticGame(nil).View(false).LastLineup(547); !errors.Is(err, ErrLastLineupsUnavailable) || errors.Is(err, ErrNoLastMatch) {
		t.Errorf("without tactics_man.dat: %v; want ErrLastLineupsUnavailable", err)
	}
}

func TestLastLineupWhenTheTacticBelongsToAnotherTeam(t *testing.T) {
	kept, warnings := checkTactics(*lastLineupTactics(), nil, 652)
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 652}, tacticPlayers(), testWorld, PlayerExtras{Tactics: kept})

	if _, ok := g.View(false).Tactic(); ok || len(warnings) != 1 {
		t.Errorf("tactic served, warnings %q; want no tactic and one warning", warnings)
	}
	if l, err := g.View(false).LastLineup(547); err != nil || l.Slots[0].Player == nil {
		t.Errorf("last lineup = %+v, %v; want Spurs' with Vicario", l, err)
	}
}

func TestShape(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []string
		want string
	}{
		{"4-4-2", []string{"GK", "DR", "DC", "DC", "DL", "MR", "MC", "MC", "ML", "ST", "ST"}, "4-4-2"},
		{"4-2-3-1", []string{"GK", "DR", "DC", "DC", "DL", "DM", "DM", "AMR", "AMC", "AML", "ST"}, "4-2-3-1"},
		{"wing-backs with defensive midfielders", []string{"GK", "DC", "DC", "DC", "WBR", "WBL", "DM", "MC", "AMC", "ST", "ST"}, "3-3-1-1-2"},
		{"sweeper (never decoded: stored in the GK cell)", []string{"GK", "SW", "DC", "DC", "DR", "DL", "MC", "MC", "AMC", "ST", "ST"}, ""},
		{"order does not matter", []string{"ST", "DC", "GK", "MC", "DC", "MC", "DC", "AML", "AMR", "MR", "ML"}, "3-4-2-1"},
		{"unknown position", []string{"GK", "DR", "DC", "DC", "DL", "MR", "MC", "MC", "ML", "ST", ""}, ""},
		{"no slots", nil, ""},
	} {
		if got := shape(tt.in); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

// metzShirts are the shirt numbers of FC Metz's last XI, then bench, by
// place (squad numbers seen in game, 2035-12-14).
type metzShirts []int

// opponentWant is the expected last match of FC Metz's last opponent.
type opponentWant struct {
	team                          int64
	name, mentality               string
	inPossession, outOfPossession string // shapes
}

// checkRealLastLineups checks the last matches of FC Metz and its opponent
// in a real KŽ save.
func checkRealLastLineups(t *testing.T, g *Game, xi []string, shirts metzShirts, opp opponentWant) {
	t.Helper()
	v := g.View(true)
	metz, err := v.LastLineup(652)
	if err != nil {
		t.Fatal(err)
	}
	if metz.OpponentIndex != opp.team || !strings.Contains(metz.Opponent, opp.name) || metz.Mentality != "Attacking" ||
		metz.InPossessionShape != "3-2-2-2-1" || metz.OutOfPossessionShape != "5-2-3" {
		t.Errorf("Metz last match: opponent %d %q, mentality %q, shapes %q / %q", metz.OpponentIndex, metz.Opponent, metz.Mentality, metz.InPossessionShape, metz.OutOfPossessionShape)
	}
	tac, _ := v.Tactic()
	for i, s := range metz.Slots {
		if s.InPossession != tac.Slots[i].InPossession || s.OutOfPossession != tac.Slots[i].OutOfPossession {
			t.Errorf("slot %d: %+v, want the tactic's %+v", i, s, tac.Slots[i])
		}
	}
	if names := slotNames(metz.Slots); !slices.Equal(names, xi) {
		t.Errorf("XI = %q, want %q", names, xi)
	}
	var got metzShirts
	for _, s := range metz.Slots {
		got = append(got, s.Player.ShirtNumber)
	}
	for _, p := range metz.Bench {
		got = append(got, p.ShirtNumber)
	}
	if !slices.Equal(got, shirts) {
		t.Errorf("shirt numbers = %v, want %v", got, shirts)
	}
	back, err := v.LastLineup(opp.team)
	if err != nil || back.OpponentIndex != 652 || !strings.Contains(back.Opponent, "Metz") || back.Mentality != opp.mentality ||
		back.InPossessionShape != opp.inPossession || back.OutOfPossessionShape != opp.outOfPossession {
		t.Errorf("opponent's last match = %+v, %v; want against Metz, %+v", back, err, opp)
	}
}

// slotNames returns the name of each slot's player; "" for an empty slot.
func slotNames(slots []TacticSlot) []string {
	names := make([]string, len(slots))
	for i, s := range slots {
		if s.Player != nil {
			names[i] = s.Player.Name
		}
	}
	return names
}
