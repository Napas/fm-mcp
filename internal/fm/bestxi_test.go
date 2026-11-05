package fm

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// bestXIGame is a Tottenham (547) squad with one goalkeeper (1), three
// centre-backs (2, 3 and the injured 5) and a full-back who can cover DC (4),
// plus a Leeds (900) centre-back (6).
func bestXIGame() *Game {
	mk := func(uid uint32, team int64, ca int16, role string, key uint8, positions map[string]uint8) savefile.Player {
		p := rawPlayer(uid, "Player", string(rune('A'-1+uid)))
		r := roleTestPlayer(50, role, key, positions)
		p.Attributes, p.Positions, p.CurrentAbility, p.TeamIndex = r.Attributes, r.Positions, ca, team
		return p
	}
	injured := mk(5, 547, 170, "Centre-Back", 90, map[string]uint8{"DC": 20})
	injured.Injuries = []savefile.Injury{{ID: 27, Start: savefile.Date{Raw: 188, Year: 2025}, DaysMin: 267, DaysMax: 297}}
	players := []savefile.Player{
		mk(1, 547, 120, "Goalkeeper", 80, map[string]uint8{"GK": 20}),
		mk(2, 547, 150, "Centre-Back", 80, map[string]uint8{"DC": 20}),
		mk(3, 547, 140, "Centre-Back", 70, map[string]uint8{"DC": 20}),
		mk(4, 547, 130, "Full-Back", 80, map[string]uint8{"DR": 20, "DC": 15}),
		injured,
		mk(6, 900, 180, "Centre-Back", 90, map[string]uint8{"DC": 20}),
	}
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, players, testWorld, PlayerExtras{})
}

var bestXITestSlots = ipSlots(
	SlotRole{"GK", "", "Goalkeeper"}, SlotRole{"DC", "left", "Centre-Back"},
	SlotRole{"DC", "right", "Centre-Back"}, SlotRole{"DR", "", "Full-Back"},
)

// ipSlots are slots with in-possession roles only.
func ipSlots(roles ...SlotRole) []XISlot {
	out := make([]XISlot, len(roles))
	for i, r := range roles {
		out[i] = XISlot{SlotRole: r}
	}
	return out
}

func pickedUIDs(xi BestXI) []uint32 {
	out := make([]uint32, len(xi.Slots))
	for i, s := range xi.Slots {
		if s.Player != nil {
			out[i] = s.Player.UID
		}
	}
	return out
}

func TestBestXIFillsTheSlotsForTheHighestTotal(t *testing.T) {
	g := bestXIGame()
	xi, err := g.View(false).BestXI(XIRequest{TeamIndex: 547, Slots: bestXITestSlots})
	if err != nil {
		t.Fatal(err)
	}
	got := pickedUIDs(xi)
	if got[0] != 1 || got[3] != 4 || !slices.Contains(got[1:3], 2) || !slices.Contains(got[1:3], 3) {
		t.Errorf("picked %v, want 1, {2, 3}, 4", got)
	}
	if xi.FromTactic || xi.Team != "Tottenham Hotspur" {
		t.Errorf("from tactic %v, team %q", xi.FromTactic, xi.Team)
	}
	if want := []PlayerRef{{UID: 5, Name: "Player E"}}; !slices.Equal(xi.LeftOutInjured, want) {
		t.Errorf("left out injured %+v, want %+v", xi.LeftOutInjured, want)
	}
	for i, s := range xi.Slots {
		if s.Slot != i || s.SlotRole != bestXITestSlots[i].SlotRole || s.OutOfPossession != nil || s.Player.OutOfPossessionRoleAbility != nil {
			t.Errorf("slot %d: %d %+v", i, s.Slot, s.SlotRole)
		}
		at, _ := placeOf(savefile.SlotRole{Position: mustSidedPosition(t, s.Position, s.Side)})
		role, _ := inPossession.named(s.Role)
		if s.Player.StartsAtSlot != nil {
			t.Errorf("slot %d: picked player starts at %d", i, *s.Player.StartsAtSlot)
		}
		if want := g.roleAbility(inPossession, g.byUID[s.Player.UID], role, at); s.Player.RoleAbility != want {
			t.Errorf("slot %d: role ability %d, want %d", i, s.Player.RoleAbility, want)
		}
		checkAlternatives(t, i, s, pickedUIDs(xi))
	}
}

func checkAlternatives(t *testing.T, i int, s XIPick, picked []uint32) {
	t.Helper()
	if len(s.Alternatives) != 3 { // the 3 other fit players; 5 is injured
		t.Errorf("slot %d: alternatives %+v, want 3", i, s.Alternatives)
	}
	for k, a := range s.Alternatives {
		if a.UID == s.Player.UID || a.UID == 5 || a.UID == 6 {
			t.Errorf("slot %d: alternative %d", i, a.UID)
		}
		if k > 0 && a.RoleAbility > s.Alternatives[k-1].RoleAbility {
			t.Errorf("slot %d: alternatives not best first: %+v", i, s.Alternatives)
		}
		if at := slices.Index(picked, a.UID); a.StartsAtSlot == nil || *a.StartsAtSlot != at {
			t.Errorf("slot %d: alternative %d starts at %v, want %d", i, a.UID, a.StartsAtSlot, at)
		}
	}
}

func mustSidedPosition(t *testing.T, code, side string) uint32 {
	t.Helper()
	m, ok := savefile.SidedPosition(code, side)
	if !ok {
		t.Fatalf("position %q %q", code, side)
	}
	return m
}

func TestBestXICanIncludeInjuredPlayers(t *testing.T) {
	xi, err := bestXIGame().View(false).BestXI(XIRequest{TeamIndex: 547, Slots: bestXITestSlots, IncludeInjured: true})
	if err != nil {
		t.Fatal(err)
	}
	picked := pickedUIDs(xi)
	if !slices.Contains(picked, 5) || xi.LeftOutInjured != nil {
		t.Errorf("picked %v, left out %+v; want 5 picked", picked, xi.LeftOutInjured)
	}
	// 5 fit players for 4 slots: the one on the bench has no starts_at_slot.
	for _, s := range xi.Slots {
		for _, a := range s.Alternatives {
			if starts := slices.Contains(picked, a.UID); starts != (a.StartsAtSlot != nil) {
				t.Errorf("slot %d: alternative %d starts at %v; in the XI: %v", s.Slot, a.UID, a.StartsAtSlot, starts)
			}
		}
	}
}

func TestBestXILeavesSlotsEmptyWithoutPlayers(t *testing.T) {
	slots := slices.Repeat(ipSlots(SlotRole{"ST", "", "Poacher"}), 5)
	xi, err := bestXIGame().View(false).BestXI(XIRequest{TeamIndex: 547, Slots: slots})
	if err != nil {
		t.Fatal(err)
	}
	if got := pickedUIDs(xi); slices.Index(got, 0) != 4 {
		t.Errorf("picked %v, want the last slot empty (4 fit players)", got)
	}
}

func TestBestXIDefaultsToTheCurrentTactic(t *testing.T) {
	g := bestXIGame()
	g.tactic = &Tactic{TeamIndex: 547, Slots: []TacticSlot{
		{Slot: 0, InPossession: SlotRole{"GK", "", "Goalkeeper"}},
		{Slot: 1, InPossession: SlotRole{"DR", "", "Full-Back"}, OutOfPossession: SlotRole{"DR", "", "Full-Back"}},
	}}
	xi, err := g.View(false).BestXI(XIRequest{TeamIndex: 547})
	if err != nil {
		t.Fatal(err)
	}
	if got := pickedUIDs(xi); !xi.FromTactic || !slices.Equal(got, []uint32{1, 4}) {
		t.Errorf("from tactic %v, picked %v; want true, [1 4]", xi.FromTactic, got)
	}
	// Slot 0's out-of-possession role is unnamed: in possession only.
	if s := xi.Slots[0]; s.OutOfPossession != nil || s.Player.OutOfPossessionRoleAbility != nil {
		t.Errorf("slot 0: %+v, %+v", s.OutOfPossession, s.Player)
	}
	if s := xi.Slots[1]; s.OutOfPossession == nil || *s.OutOfPossession != (SlotRole{"DR", "", "Full-Back"}) || s.Player.OutOfPossessionRoleAbility == nil {
		t.Errorf("slot 1: %+v, %+v; want Full-Back at DR with its ability", s.OutOfPossession, s.Player)
	}
}

func TestBestXIHidesPlayersFogOfWarHides(t *testing.T) {
	// At Leeds (900), 3 and 5 are fully known; 4, 6 and 7 are not.
	xi, err := fogGame().View(true).BestXI(XIRequest{TeamIndex: 900, Slots: ipSlots(SlotRole{"AMC", "", "Attacking Midfielder"})})
	if err != nil {
		t.Fatal(err)
	}
	s := xi.Slots[0]
	if s.Player == nil || s.Player.UID != 5 || len(s.Alternatives) != 1 || s.Alternatives[0].UID != 3 || xi.HiddenByFog != 3 {
		t.Errorf("picked %+v, alternatives %+v, hidden %d; want 5, [3], 3", s.Player, s.Alternatives, xi.HiddenByFog)
	}
}

// oopRequest asks for one goalkeeper slot with this out-of-possession part.
func oopRequest(oop SlotRole) XIRequest {
	return XIRequest{TeamIndex: 547, Slots: []XISlot{{SlotRole: SlotRole{"GK", "", "Goalkeeper"}, OutOfPossession: &oop}}}
}

// TestBestXIScoresBothPhases: every attribute 50, a Winger at AMR who drops
// to Wide Midfielder at MR. 1 (AMR 20, MR 1) plays his best role in
// possession, 100, but rates 24 at MR, pulled up by (100 − 24) / (6 + 2) = 9
// to 33: 33/100 × 100 = 33, total 133. 2 (AMR 20, MR 20) gets 90 + 90 = 180.
func TestBestXIScoresBothPhases(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"AMR": 20}),
		uniformSquadPlayer(2, 90, map[string]uint8{"AMR": 20, "MR": 20}),
	}, testWorld, PlayerExtras{})
	oop := SlotRole{"MR", "", "wide midfielder"}
	xi, err := g.View(false).BestXI(XIRequest{TeamIndex: 547, Slots: []XISlot{{SlotRole: SlotRole{"AMR", "", "Winger"}, OutOfPossession: &oop}}})
	if err != nil {
		t.Fatal(err)
	}
	s := xi.Slots[0]
	if s.OutOfPossession == nil || *s.OutOfPossession != (SlotRole{"MR", "", "Wide Midfielder"}) {
		t.Errorf("out of possession %+v, want Wide Midfielder at MR", s.OutOfPossession)
	}
	abilities := func(p XIPlayer) [3]int {
		oop := -1
		if p.OutOfPossessionRoleAbility != nil {
			oop = *p.OutOfPossessionRoleAbility
		}
		return [3]int{int(p.UID), p.RoleAbility, oop}
	}
	if s.Player == nil || abilities(*s.Player) != [3]int{2, 90, 90} || len(s.Alternatives) != 1 || abilities(s.Alternatives[0]) != [3]int{1, 100, 33} {
		t.Errorf("picked %+v, alternatives %+v; want 2 (90, 90), then 1 (100, 33)", s.Player, s.Alternatives)
	}
}

// TestBestXIWeighsSlotsAlike: an in-possession-only slot counts its role
// ability twice, like a slot with both phases. Every attribute 50; 1 (CA
// 100, GK 20, ST 20) gets 100 everywhere; 2 (CA 80, GK 15, ST 20) gets 76 in
// goal (rating 95 of 100, no FM26 pull-up at 15) and 80 + 80 up front. 1 in
// goal totals 2 × 100 + 160 = 360 against 2 × 76 + 200 = 352; counting the
// goalkeeper once would put 2 in goal (276 against 260).
func TestBestXIWeighsSlotsAlike(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"GK": 20, "ST": 20}),
		uniformSquadPlayer(2, 80, map[string]uint8{"GK": 15, "ST": 20}),
	}, testWorld, PlayerExtras{})
	cf := SlotRole{"ST", "", "Centre-Forward"}
	xi, err := g.View(false).BestXI(XIRequest{TeamIndex: 547, Slots: []XISlot{
		{SlotRole: SlotRole{"GK", "", "Goalkeeper"}}, {SlotRole: SlotRole{"ST", "", "Poacher"}, OutOfPossession: &cf},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := pickedUIDs(xi); !slices.Equal(got, []uint32{1, 2}) {
		t.Errorf("picked %v, want [1 2]", got)
	}
	if a := xi.Slots[0].Alternatives; len(a) != 1 || a[0].RoleAbility != 76 {
		t.Errorf("goalkeeper alternatives %+v, want 2 at 76", a)
	}
}

// TestBestXIUsesTheSlotsSide: every attribute 50, DM 15 and WBL 20. As a
// Wide Covering Defensive Midfielder at DM left he blends with WBL to 17:
// rating 97, pulled up to 98 of his best 100. At DM right (WBR 1) or without
// a side he rates 95, pulled up to 97; search_players takes the better side.
func TestBestXIUsesTheSlotsSide(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"DM": 15, "WBL": 20}),
	}, testWorld, PlayerExtras{})
	v := g.View(false)
	for side, want := range map[string]int{"left": 98, "right": 97, "": 97} {
		oop := SlotRole{"DM", side, "Wide Covering Defensive Midfielder"}
		xi, err := v.BestXI(XIRequest{TeamIndex: 547, Slots: []XISlot{{SlotRole: SlotRole{"DM", side, "Defensive Midfielder"}, OutOfPossession: &oop}}})
		if err != nil {
			t.Fatal(err)
		}
		if a := xi.Slots[0].Player.OutOfPossessionRoleAbility; a == nil || *a != want {
			t.Errorf("side %q: %v, want %d", side, a, want)
		}
	}
	// In possession, DC 15, DL 20: Wide Centre-Back on the left blends to 17,
	// rating 97 of 100 (no FM26 pull-up at 15); on the right (DR 1) 95.
	wcb := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"DC": 15, "DL": 20}),
	}, testWorld, PlayerExtras{}).View(false)
	for side, want := range map[string]int{"left": 97, "right": 95} {
		xi, err := wcb.BestXI(XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{"DC", side, "Wide Centre-Back"})})
		if err != nil {
			t.Fatal(err)
		}
		if got := xi.Slots[0].Player.RoleAbility; got != want {
			t.Errorf("Wide Centre-Back at DC %s: %d, want %d", side, got, want)
		}
	}
	res, _ := v.Search(Filter{Position: "DM", OutOfPossessionRole: "Wide Covering Defensive Midfielder"})
	if got := oopRoleAbilities(res.Players); !slices.Equal(got, []int{98}) {
		t.Errorf("search_players: %v, want [98]", got)
	}
}

func TestBestXIRejectsBadRequests(t *testing.T) {
	g := bestXIGame()
	other := bestXIGame()
	other.tactic = &Tactic{TeamIndex: 900}
	unnamed := bestXIGame()
	unnamed.tactic = &Tactic{TeamIndex: 547, Slots: []TacticSlot{{Slot: 0, InPossession: SlotRole{Position: "GK"}}}}
	for _, tt := range []struct {
		name string
		g    *Game
		r    XIRequest
		want string
	}{
		{"no tactic", g, XIRequest{TeamIndex: 547}, "no current tactic"},
		{"other team's tactic", other, XIRequest{TeamIndex: 547}, "is team 900's"},
		{"unnamed role", unnamed, XIRequest{TeamIndex: 547}, "slot 0 has no named in-possession role"},
		{"unknown team", g, XIRequest{TeamIndex: 1, Slots: bestXITestSlots}, "no team with team_index 1"},
		{"too many slots", g, XIRequest{TeamIndex: 547, Slots: slices.Repeat(bestXITestSlots, 3)}, "at most 11 slots"},
		{"position", g, XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{"XX", "", "Goalkeeper"})}, `slot 0: position "XX"`},
		{"side", g, XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{"DC", "middle", "Centre-Back"})}, `slot 0: side "middle"`},
		{"side of a single position", g, XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{"GK", "left", "Goalkeeper"})}, `slot 0: side "left"; only DC, DM, MC, AMC and ST take left or right`},
		{"no role", g, XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{Position: "GK"})}, "slot 0 has no in-possession role; pass one offered at GK"},
		{"role", g, XIRequest{TeamIndex: 547, Slots: ipSlots(SlotRole{"GK", "", "Goalkeeper"}, SlotRole{"ST", "", "Goalkeeper"})}, `slot 1: in-possession role "Goalkeeper" is not offered at ST`},
		{"out-of-possession position", g, oopRequest(SlotRole{"XX", "", "Goalkeeper"}), `slot 0 out of possession: position "XX"`},
		{"out-of-possession side", g, oopRequest(SlotRole{"GK", "left", "Goalkeeper"}), `slot 0 out of possession: side "left"`},
		{"no out-of-possession role", g, oopRequest(SlotRole{Position: "GK"}), "slot 0 has no out-of-possession role; pass one offered at GK"},
		{"out-of-possession role", g, oopRequest(SlotRole{"ST", "", "Goalkeeper"}), `slot 0 out of possession: out-of-possession role "Goalkeeper" is not offered at ST`},
	} {
		_, err := tt.g.View(false).BestXI(tt.r)
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v; want an invalid filter mentioning %q", tt.name, err, tt.want)
		}
	}
}

// TestLoadTestSave27bBestXI checks the KŽ tactic's best XI: 11 distinct
// squad players, with a total role ability in and out of possession no
// lower than that of the last match's XI in the same slots.
func TestLoadTestSave27bBestXI(t *testing.T) {
	g := loadTestSaveAt(t, testSave27bPath, LoadOptions{})
	v := g.View(false)
	xi, err := v.BestXI(XIRequest{TeamIndex: kzTeam27, IncludeInjured: true})
	if err != nil {
		t.Fatal(err)
	}
	tac, _ := v.Tactic()
	if !xi.FromTactic || len(xi.Slots) != xiSize {
		t.Fatalf("from tactic %v, %d slots", xi.FromTactic, len(xi.Slots))
	}
	seen := map[uint32]bool{}
	total, lastTotal := 0, 0 // in and out of possession
	for i, s := range xi.Slots {
		if s.Player == nil || seen[s.Player.UID] || !g.byUID[s.Player.UID].plays(kzTeam27) {
			t.Fatalf("slot %d: %+v", i, s.Player)
		}
		seen[s.Player.UID] = true
		if s.OutOfPossession == nil || s.Player.OutOfPossessionRoleAbility == nil {
			t.Fatalf("slot %d: no out-of-possession role ability: %+v", i, s)
		}
		if s.SlotRole != tac.Slots[i].InPossession || *s.OutOfPossession != tac.Slots[i].OutOfPossession {
			t.Errorf("slot %d: %+v → %+v, tactic %+v → %+v", i, s.SlotRole, *s.OutOfPossession, tac.Slots[i].InPossession, tac.Slots[i].OutOfPossession)
		}
		total += s.Player.RoleAbility + *s.Player.OutOfPossessionRoleAbility
		last := tac.Slots[i]
		if a, oop := last.RoleAbility, last.OutOfPossessionRoleAbility; a != nil && oop != nil && g.byUID[last.Player.UID].plays(kzTeam27) {
			lastTotal += *a + *oop
		}
	}
	if total < lastTotal {
		t.Errorf("total role ability %d, below the last match's %d", total, lastTotal)
	}
	// A player best_xi lists in his last-match slot gets get_tactic's values.
	matched := 0
	for i, s := range xi.Slots {
		last := tac.Slots[i]
		if last.Player == nil || last.RoleAbility == nil || last.OutOfPossessionRoleAbility == nil {
			continue
		}
		for _, p := range append([]XIPlayer{*s.Player}, s.Alternatives...) {
			if p.UID != last.Player.UID {
				continue
			}
			matched++
			if p.RoleAbility != *last.RoleAbility || *p.OutOfPossessionRoleAbility != *last.OutOfPossessionRoleAbility {
				t.Errorf("slot %d, %s: %d and %d, get_tactic %d and %d", i, p.Name, p.RoleAbility, *p.OutOfPossessionRoleAbility, *last.RoleAbility, *last.OutOfPossessionRoleAbility)
			}
		}
	}
	if matched == 0 {
		t.Error("no last-match player among the picks and alternatives")
	}
}
