package fm

import (
	"errors"
	"math/bits"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// squadRolesGame is a Tottenham (547) squad with every attribute 50: 1 (CA
// 100, DM 20, DC 12), 2 (CA 120, DM 15, WBL 20), 3 (CA 90, ST 20) and 5 (CA
// 115, DM 20), plus a Leeds (900) DM (4).
func squadRolesGame() *Game {
	leeds := uniformSquadPlayer(4, 160, map[string]uint8{"DM": 20})
	leeds.TeamIndex = 900
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{
		uniformSquadPlayer(1, 100, map[string]uint8{"DM": 20, "DC": 12}),
		uniformSquadPlayer(2, 120, map[string]uint8{"DM": 15, "WBL": 20}),
		uniformSquadPlayer(3, 90, map[string]uint8{"ST": 20}),
		uniformSquadPlayer(5, 115, map[string]uint8{"DM": 20}),
		leeds,
	}, testWorld, PlayerExtras{})
}

func roleRow(t *testing.T, g SquadRoles, pos, phase, role string) RoleDepth {
	t.Helper()
	i := slices.IndexFunc(g.Roles, func(r RoleDepth) bool { return r.Position == pos && r.Phase == phase && r.Role == role })
	if i < 0 {
		t.Fatalf("no %s %s row for %s in %+v", phase, pos, role, g.Roles)
	}
	return g.Roles[i]
}

type rolePlayerRow struct {
	uid             uint32
	ability, rating int
}

func rolePlayers(r RoleDepth) []rolePlayerRow {
	out := make([]rolePlayerRow, len(r.Players))
	for i, p := range r.Players {
		out[i] = rolePlayerRow{p.UID, p.RoleAbility, p.PositionRating}
	}
	return out
}

func TestSquadRolesRanksEachRole(t *testing.T) {
	v := squadRolesGame().View(false)
	got, err := v.SquadRoles(SquadRolesRequest{TeamIndex: 547, Position: "dm", Phase: ""})
	if err != nil {
		t.Fatal(err)
	}
	if got.Team != "Tottenham Hotspur" {
		t.Errorf("team %q", got.Team)
	}
	// In possession (FM26 rules): 2 rates 95 of his best 100 at DM 15, no
	// pull-up: 95/100 × 120 = 114, below 5 (CA 115) in his best role.
	if r := rolePlayers(roleRow(t, got, "DM", PhaseInPossession, "Defensive Midfielder")); !slices.Equal(r, []rolePlayerRow{{5, 115, 20}, {2, 114, 15}, {1, 100, 20}}) {
		t.Errorf("Defensive Midfielder: %+v", r)
	}
	// Out of possession, on the better side: 2 blends with WBL to 17, rating
	// 97, pulled up to 98; 98/100 × 120 = 117.6.
	if r := rolePlayers(roleRow(t, got, "DM", PhaseOutOfPossession, "Wide Covering Defensive Midfielder")); !slices.Equal(r, []rolePlayerRow{{2, 117, 15}, {5, 115, 20}, {1, 100, 20}}) {
		t.Errorf("Wide Covering Defensive Midfielder: %+v", r)
	}
	inPossessionRows := 0
	for _, r := range got.Roles {
		if r.Position != "DM" {
			t.Errorf("row at %s with a DM filter", r.Position)
		}
		if r.Phase == PhaseInPossession {
			inPossessionRows++
		}
	}
	ipRoles, oopRoles := bits.OnesCount64(positionRoles[positionIndexDM]), bits.OnesCount64(oopPositionRoles[positionIndexDM])
	if inPossessionRows != ipRoles || len(got.Roles) != ipRoles+oopRoles {
		t.Errorf("%d rows, %d in possession; want %d, %d", len(got.Roles), inPossessionRows, ipRoles+oopRoles, ipRoles)
	}
}

func TestSquadRolesLimitPlayersPerRole(t *testing.T) {
	got, err := squadRolesGame().View(false).SquadRoles(SquadRolesRequest{TeamIndex: 547, Position: "DM", Phase: "In_Possession", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if r := rolePlayers(roleRow(t, got, "DM", PhaseInPossession, "Defensive Midfielder")); !slices.Equal(r, []rolePlayerRow{{5, 115, 20}}) {
		t.Errorf("Defensive Midfielder: %+v, want only the best", r)
	}
	for _, r := range got.Roles {
		if len(r.Players) != 1 || r.Phase != PhaseInPossession {
			t.Errorf("%s %s: %d players", r.Phase, r.Role, len(r.Players))
		}
	}
	// No one is rated at SW: no rows, an empty list.
	sw, err := squadRolesGame().View(false).SquadRoles(SquadRolesRequest{TeamIndex: 547, Position: "SW"})
	if err != nil {
		t.Fatal(err)
	}
	if sw.Roles == nil || len(sw.Roles) != 0 {
		t.Errorf("SW rows %#v, want an empty list", sw.Roles)
	}
}

func TestSquadRolesCoverPositionsRatedTenOrMore(t *testing.T) {
	got, err := squadRolesGame().View(false).SquadRoles(SquadRolesRequest{TeamIndex: 547, Phase: PhaseOutOfPossession})
	if err != nil {
		t.Fatal(err)
	}
	if r := roleRow(t, got, "DC", PhaseOutOfPossession, "Centre-Back"); len(r.Players) != 1 || r.Players[0].UID != 1 || r.Players[0].PositionRating != 12 {
		t.Errorf("Centre-Back at DC: %+v, want 1 at 12", r.Players)
	}
	order := func(r RoleDepth) int { return slices.Index(savefile.PositionNames[:], r.Position) }
	for i, r := range got.Roles {
		if r.Phase != PhaseOutOfPossession {
			t.Errorf("%s row with an out-of-possession filter", r.Phase)
		}
		if i > 0 && order(r) < order(got.Roles[i-1]) {
			t.Errorf("row %d at %s after %s", i, r.Position, got.Roles[i-1].Position)
		}
		for _, p := range r.Players {
			if p.UID == 4 || p.PositionRating < bestRolePosition {
				t.Errorf("%s %s: player %+v", r.Position, r.Role, p)
			}
		}
	}
	for _, pos := range []string{"GK", "ML", "AMC"} { // no one rated 10+
		if slices.ContainsFunc(got.Roles, func(r RoleDepth) bool { return r.Position == pos }) {
			t.Errorf("a row at %s", pos)
		}
	}
}

func TestSquadRolesMarkInjuredAndHideFoggedPlayers(t *testing.T) {
	got, err := bestXIGame().View(false).SquadRoles(SquadRolesRequest{TeamIndex: 547, Position: "DC", Phase: PhaseInPossession})
	if err != nil {
		t.Fatal(err)
	}
	r := roleRow(t, got, "DC", PhaseInPossession, "Centre-Back")
	if i := slices.IndexFunc(r.Players, func(p RolePlayer) bool { return p.UID == 5 }); i < 0 || !r.Players[i].Injured {
		t.Errorf("injured player 5: %+v", r.Players)
	}
	// At Leeds (900), 3 and 5 are fully known; 4, 6 and 7 are not.
	fog, err := fogGame().View(true).SquadRoles(SquadRolesRequest{TeamIndex: 900, Position: "AMC", Phase: PhaseInPossession})
	if err != nil {
		t.Fatal(err)
	}
	am := roleRow(t, fog, "AMC", PhaseInPossession, "Attacking Midfielder")
	if got := uidsOf(am.Players); !slices.Equal(got, []uint32{5, 3}) || fog.HiddenByFog != 3 {
		t.Errorf("players %v, hidden %d; want [5 3], 3", got, fog.HiddenByFog)
	}
}

func uidsOf(ps []RolePlayer) []uint32 {
	out := make([]uint32, len(ps))
	for i, p := range ps {
		out[i] = p.UID
	}
	return out
}

func TestSquadRolesRejectBadRequests(t *testing.T) {
	v := squadRolesGame().View(false)
	for _, tt := range []struct {
		r    SquadRolesRequest
		want string
	}{
		{SquadRolesRequest{TeamIndex: 1}, "no team with team_index 1"},
		{SquadRolesRequest{TeamIndex: 547, Position: "XX"}, `position "XX"`},
		{SquadRolesRequest{TeamIndex: 547, Phase: "attack"}, `phase "attack"; use in_possession or out_of_possession`},
		{SquadRolesRequest{TeamIndex: 547, Limit: -1}, "limit -1 is negative"},
	} {
		if _, err := v.SquadRoles(tt.r); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%+v: %v; want an invalid filter mentioning %q", tt.r, err, tt.want)
		}
	}
}

// TestLoadTestSave27bSquadRoles checks KŽ's grid against values pinned
// elsewhere: Fiolić's best out-of-possession role (Winger at AMR, 105) and
// Kalik's Attacking Midfielder at MC (103, get_tactic's slot 7).
func TestLoadTestSave27bSquadRoles(t *testing.T) {
	v := loadTestSaveAt(t, testSave27bPath, LoadOptions{}).View(false)
	tac, _ := v.Tactic()
	for _, tt := range []struct {
		slot             int
		pos, phase, role string
		want             int
	}{
		{8, "AMR", PhaseOutOfPossession, "Winger", 105},
		{7, "MC", PhaseInPossession, "Attacking Midfielder", 103},
	} {
		got, err := v.SquadRoles(SquadRolesRequest{TeamIndex: kzTeam27, Position: tt.pos, Phase: tt.phase})
		if err != nil {
			t.Fatal(err)
		}
		uid := tac.Slots[tt.slot].Player.UID
		r := roleRow(t, got, tt.pos, tt.phase, tt.role)
		if i := slices.IndexFunc(r.Players, func(p RolePlayer) bool { return p.UID == uid }); i < 0 || r.Players[i].RoleAbility != tt.want {
			t.Errorf("%s %s at %s: player %d missing or not %d in %+v", tt.phase, tt.role, tt.pos, uid, tt.want, r.Players)
		}
	}
}
