package fm

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// oopSearchGame has three players with every attribute 50. As a Wide
// Covering Defensive Midfielder at DM, 1 (DM 15, WBR 20) blends with WBR on
// the right side to 17: rating 97, pulled up to 98 of his best 100 (no side:
// 95, pulled up to 97). 2 (DM 20) and 3 (DM 15 only) play their best role. As
// an in-possession Defensive Midfielder (FM26 rules, no pull-up at 15), 1
// rates 95 of 100.
func oopSearchGame() *Game {
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"DM": 15, "WBR": 20}),
		uniformSquadPlayer(2, 97, map[string]uint8{"DM": 20}),
		uniformSquadPlayer(3, 60, map[string]uint8{"DM": 15}),
	}, testWorld, PlayerExtras{})
}

// uniformSquadPlayer is a Tottenham (547) uniformPlayer.
func uniformSquadPlayer(uid uint32, ca int16, positions map[string]uint8) savefile.Player {
	p := rawPlayer(uid, "Player", strconv.Itoa(int(uid)))
	u := uniformPlayer(ca, positions)
	p.Attributes, p.Positions, p.CurrentAbility = u.Attributes, u.Positions, ca
	return p
}

func oopRoleAbilities(ps []Profile) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = -1
		if p.OutOfPossessionRoleAbility != nil {
			out[i] = *p.OutOfPossessionRoleAbility
		}
	}
	return out
}

func TestSearchRanksByOutOfPossessionRoleAbility(t *testing.T) {
	v := oopSearchGame().View(false)
	const wcdm = "Wide Covering Defensive Midfielder"
	for _, tt := range []struct {
		name           string
		f              Filter
		uids           []uint32
		oop, abilities []int
	}{
		{"role", Filter{Position: "DM", OutOfPossessionRole: "wide covering defensive midfielder"}, []uint32{1, 2, 3}, []int{98, 97, 60}, []int{-1, -1, -1}},
		{"named sort", Filter{Position: "dm", OutOfPossessionRole: wcdm, SortBy: "Out_Of_Possession_Role_Ability", Ascending: true}, []uint32{3, 2, 1}, []int{60, 97, 98}, []int{-1, -1, -1}},
		{"minimum", Filter{Position: "DM", OutOfPossessionRole: wcdm, MinOutOfPossessionRoleAbility: 97}, []uint32{1, 2}, []int{98, 97}, []int{-1, -1}},
		{"both roles rank by role_ability", Filter{Position: "DM", Role: "Defensive Midfielder", OutOfPossessionRole: wcdm}, []uint32{2, 1, 3}, []int{97, 98, 60}, []int{97, 95, 60}},
		{"both roles, sorted out of possession", Filter{Position: "DM", Role: "Defensive Midfielder", OutOfPossessionRole: wcdm, SortBy: "out_of_possession_role_ability"}, []uint32{1, 2, 3}, []int{98, 97, 60}, []int{95, 97, 60}},
	} {
		res, err := v.Search(tt.f)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !slices.Equal(uids(res.Players), tt.uids) || !slices.Equal(oopRoleAbilities(res.Players), tt.oop) || !slices.Equal(roleAbilities(res.Players), tt.abilities) {
			t.Errorf("%s: %v with out-of-possession %v, in-possession %v; want %v, %v, %v", tt.name,
				uids(res.Players), oopRoleAbilities(res.Players), roleAbilities(res.Players), tt.uids, tt.oop, tt.abilities)
		}
	}
}

func TestSearchRejectsBadOutOfPossessionRoles(t *testing.T) {
	v := oopSearchGame().View(false)
	for _, tt := range []struct {
		f    Filter
		want string
	}{
		{Filter{OutOfPossessionRole: "Pressing Full-Back"}, `out-of-possession role "Pressing Full-Back" needs a position`},
		{Filter{Position: "DC", OutOfPossessionRole: "Ball-Playing Centre-Back"}, `out-of-possession role "Ball-Playing Centre-Back" is an in-possession role; valid roles: Attacking Midfielder, Central Midfielder, Central Outlet Centre-Forward`},
		{Filter{Position: "ST", OutOfPossessionRole: "Centre-Back"}, `out-of-possession role "Centre-Back" is not offered at ST; roles at ST: Central Outlet Centre-Forward, Centre-Forward, Splitting Outlet Centre-Forward, Tracking Centre-Forward`},
		{Filter{Position: "DC", OutOfPossessionRole: "Stopper"}, `out-of-possession role "Stopper"; valid roles: `},
		{Filter{Position: "SW", OutOfPossessionRole: "Centre-Back"}, "no out-of-possession role is offered at SW"},
		{Filter{Position: "DM", SortBy: "out_of_possession_role_ability"}, "needs an out_of_possession_role"},
		{Filter{MinOutOfPossessionRoleAbility: 100}, "min_out_of_possession_role_ability needs an out_of_possession_role and a position"},
	} {
		_, err := v.Search(tt.f)
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%+v: %v; want an invalid filter mentioning %q", tt.f, err, tt.want)
		}
	}
}

func TestSearchWithFogOfWarHidesOutOfPossessionRoleAbility(t *testing.T) {
	res, err := fogGame().View(true).Search(Filter{Position: "AMC", OutOfPossessionRole: "Attacking Midfielder"})
	if err != nil {
		t.Fatal(err)
	}
	// Full knowledge: 5, 1, 3, 2 by role ability (= current ability); hidden by reputation: 4, 7, 6, 8.
	if want := []uint32{5, 1, 3, 2, 4, 7, 6, 8}; !slices.Equal(uids(res.Players), want) {
		t.Errorf("order %v, want %v", uids(res.Players), want)
	}
	if got, want := oopRoleAbilities(res.Players), []int{150, 120, 100, 90, -1, -1, -1, -1}; !slices.Equal(got, want) {
		t.Errorf("out-of-possession role abilities %v, want %v", got, want)
	}
}

func TestProfileListsBestOutOfPossessionRoles(t *testing.T) {
	// Every attribute 50: the three DL roles rate 100 at DL 20; DR 14 is not 15+.
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{uniformSquadPlayer(1, 150, map[string]uint8{"DL": 20, "DR": 14})}, testWorld, PlayerExtras{})

	got, _ := g.View(false).Profile(1)

	want := []RoleFit{{"Full-Back", "DL", 150}, {"Pressing Full-Back", "DL", 150}, {"Holding Full-Back", "DL", 150}}
	if !slices.Equal(got.BestOutOfPossessionRoles, want) {
		t.Errorf("best out-of-possession roles %+v, want %+v", got.BestOutOfPossessionRoles, want)
	}
	if res, _ := g.View(false).Search(Filter{}); res.Players[0].BestOutOfPossessionRoles != nil {
		t.Errorf("search rows carry best out-of-possession roles: %+v", res.Players[0].BestOutOfPossessionRoles)
	}
	v := fogGame().View(true)
	if own, _ := v.Profile(1); len(own.BestOutOfPossessionRoles) != bestRolesShown {
		t.Errorf("own player: %+v", own.BestOutOfPossessionRoles)
	}
	if unknown, _ := v.Profile(6); unknown.BestOutOfPossessionRoles != nil {
		t.Errorf("unscouted player: %+v", unknown.BestOutOfPossessionRoles)
	}
}
