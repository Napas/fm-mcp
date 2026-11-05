package fm

import (
	"math/bits"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func mustOOPRole(t *testing.T, name string) uint64 {
	t.Helper()
	r, ok := roleNamed(name, oopRoleAttributesByRole, savefile.OutOfPossessionRoleName)
	if !ok {
		t.Fatalf("no out-of-possession role %q", name)
	}
	return r
}

// sidedAt is a position with an optional side ("left", "right" or "").
func sidedAt(t *testing.T, code, side string) place {
	t.Helper()
	at := positionAt(t, code)
	at.mask = mustSidedPosition(t, code, side)
	return at
}

// uniformPlayer has every attribute at 50, so every role rates 100 at a
// natural position, and the given position ratings (others 1).
func uniformPlayer(ca int16, positions map[string]uint8) savefile.Player {
	p := roleTestPlayer(50, "", 0, positions)
	p.CurrentAbility = ca
	return p
}

func TestEveryOutOfPossessionRoleHasAttributes(t *testing.T) {
	named := 0
	for b := range 64 {
		role := uint64(1) << b
		name, ok := savefile.OutOfPossessionRoleName(role)
		if _, has := oopRoleAttributesByRole[role]; ok != has {
			t.Errorf("role bit %#x (%q): named %v, attributes %v", role, name, ok, has)
		}
		if ok {
			named++
		}
	}
	if named != 39 {
		t.Errorf("%d named out-of-possession roles, want 39", named)
	}
}

func TestOutOfPossessionRoleAttributesMatchTheGame(t *testing.T) {
	// docs/format/role-ability.md, out-of-possession table (26.3.2 and 27.0.0).
	for name, want := range map[string][3]string{
		"Centre-Back":        {"Heading Marking Tackling Anticipation Positioning Strength JumpingReach", "Decisions Pace Bravery Aggression Composure Concentration", ""},
		"Pressing Full-Back": {"Marking Tackling Anticipation Positioning Teamwork WorkRate Acceleration Aggression", "Decisions Stamina Pace Bravery Agility Concentration", ""},
		"Central Outlet Midfielder": {"OffTheBall Anticipation Decisions Teamwork Balance Concentration",
			"Marking FirstTouch Strength Composure", "WorkRate Stamina"},
	} {
		a := oopRoleAttributesByRole[mustOOPRole(t, name)]
		if got := [3]string{roleNames(a.key), roleNames(a.preferable), roleNames(a.negative)}; got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

func TestOutOfPossessionPositionRoles(t *testing.T) {
	// 0x491d7c4 without a side context (27.0.0).
	for code, want := range map[string]uint64{
		"GK": 0x7, "SW": 0, "DL": 0xe00, "DC": 0x1f8, "DR": 0xe00, "DM": 0xf8000,
		"ML": 0x3800000000, "MC": 0x38100000, "MR": 0x3800000000,
		"AML": 0x3c000000000, "AMC": 0x3c0000000, "AMR": 0x3c000000000,
		"ST": 0x3c0000000000, "WBL": 0x7000, "WBR": 0x7000,
	} {
		if got := oopPositionRoles[positionAt(t, code).index]; got != want {
			t.Errorf("%s: %#x, want %#x", code, got, want)
		}
	}
	for _, roles := range oopPositionRoles {
		for r := roles; r != 0; r &= r - 1 {
			if _, ok := oopRoleAttributesByRole[uint64(1)<<bits.TrailingZeros64(r)]; !ok {
				t.Errorf("offered role bit %#x has no attributes", uint64(1)<<bits.TrailingZeros64(r))
			}
		}
	}
}

// TestOutOfPossessionRatingBlends checks 0x491ab5c's blending: from position
// rating 12, the slot rating is averaged with a higher neighbour. With every
// attribute 50 the result is 100 minus the position penalty.
func TestOutOfPossessionRatingBlends(t *testing.T) {
	for _, tt := range []struct {
		role, pos, side string
		positions       map[string]uint8
		want            int
	}{
		{"Pressing Full-Back", "DL", "", map[string]uint8{"DL": 14, "WBL": 20}, 97}, // (14+20)/2 = 17: 3 %
		{"Pressing Full-Back", "DL", "", map[string]uint8{"DL": 14}, 85},            // 14: 15 %
		{"Pressing Full-Back", "DR", "", map[string]uint8{"DR": 14, "WBR": 18, "WBL": 20}, 96},
		{"Wide Centre-Back", "DC", "", map[string]uint8{"DC": 14, "DL": 20, "DR": 16}, 97}, // no side: the higher
		{"Wide Centre-Back", "DC", "left", map[string]uint8{"DC": 14, "DL": 16, "DR": 20}, 95},
		{"Dropping Defensive Midfielder", "DM", "", map[string]uint8{"DM": 13, "DC": 19}, 96},
		{"Dropping Defensive Midfielder", "DM", "", map[string]uint8{"DM": 12, "DC": 20}, 96},                  // 12 blends: 16
		{"Dropping Defensive Midfielder", "DM", "", map[string]uint8{"DM": 11, "DC": 20}, 64},                  // under 12: no blend, 36 %
		{"Wide Covering Defensive Midfielder", "DM", "", map[string]uint8{"DM": 14, "WBL": 20, "WBR": 20}, 85}, // needs a side
		{"Wide Covering Central Midfielder", "MC", "", map[string]uint8{"MC": 14, "ML": 20, "MR": 20}, 85},     // needs a side
		{"Pressing Wing-Back", "WBL", "", map[string]uint8{"WBL": 14, "ML": 20}, 97},
		{"Splitting Outlet Attacking Midfielder", "AMC", "", map[string]uint8{"AMC": 14, "AML": 20}, 85}, // needs a side
		{"Splitting Outlet Attacking Midfielder", "AMC", "left", map[string]uint8{"AMC": 14, "AML": 20}, 97},
		{"Centre-Back", "DC", "", map[string]uint8{"DC": 14, "DL": 20}, 85}, // no blending role
	} {
		in := newRoleInputs(uniformPlayer(150, tt.positions))
		if got := in.oopRating(mustOOPRole(t, tt.role), sidedAt(t, tt.pos, tt.side)); got != tt.want {
			t.Errorf("%s at %s %s: %d, want %d", tt.role, tt.pos, tt.side, got, tt.want)
		}
	}
}

func TestOutOfPossessionAbility(t *testing.T) {
	in := newRoleInputs(uniformPlayer(60, map[string]uint8{"DC": 20}))
	if in.oopBest != 100 {
		t.Fatalf("best out-of-possession rating %d, want 100", in.oopBest)
	}
	for _, tt := range []struct {
		role, pos string
		want      int
	}{
		{"Centre-Back", "DC", 60}, // the best role: current ability, never halved
		// DL 1: rating 24 (76 % penalty), pulled up by (100 − 24) / (6 + 2)
		// = 9 to 33; 33/100 × 60 = 19.8.
		{"Full-Back", "DL", 19},
	} {
		if got, ok := in.oopAbility(mustOOPRole(t, tt.role), positionAt(t, tt.pos), roleRules26); !ok || got != tt.want {
			t.Errorf("%s at %s: %d, %v; want %d", tt.role, tt.pos, got, ok, tt.want)
		}
	}
	if got, ok := in.oopAbility(1<<63, positionAt(t, "DC"), roleRules26); ok {
		t.Errorf("unknown role: %d", got)
	}
	// FM26 save, DL 16: rating 96 pulls up to 98; 98/100 × 60 = 58.8.
	fm26 := newRoleInputs(uniformPlayer(60, map[string]uint8{"DC": 20, "DL": 16}))
	if got, _ := fm26.oopAbility(mustOOPRole(t, "Full-Back"), positionAt(t, "DL"), roleRules26); got != 58 {
		t.Errorf("FM26 save, Full-Back at DL 16: %d, want 58", got)
	}
	// No position rated 10 or more: no best rating, so the ability is the rating.
	none := newRoleInputs(uniformPlayer(60, nil))
	if got, _ := none.oopAbility(mustOOPRole(t, "Full-Back"), positionAt(t, "DL"), roleRules26); none.oopBest != 0 || got != 24 {
		t.Errorf("best %d, ability %d; want 0, 24", none.oopBest, got)
	}
}

// TestEveryOutOfPossessionRoleBlendsWithItsNeighbour checks each blending
// role bit of 0x491ab5c: slot 14 and neighbour 20 blend to 17 (rating 97).
func TestEveryOutOfPossessionRoleBlendsWithItsNeighbour(t *testing.T) {
	for _, tt := range []struct {
		role, pos, side, neighbour string
	}{
		{"Wide Centre-Back", "DC", "left", "DL"},
		{"Wide Centre-Back", "DC", "right", "DR"},
		{"Stopping Wide Centre-Back", "DC", "left", "DL"},
		{"Covering Wide Centre-Back", "DC", "right", "DR"},
		{"Pressing Full-Back", "DL", "", "WBL"},
		{"Pressing Full-Back", "DR", "", "WBR"},
		{"Wide Covering Defensive Midfielder", "DM", "left", "WBL"},
		{"Wide Covering Defensive Midfielder", "DM", "right", "WBR"},
		{"Tracking Wide Midfielder", "ML", "", "WBL"},
		{"Tracking Wide Midfielder", "MR", "", "WBR"},
		{"Pressing Wing-Back", "WBL", "", "ML"},
		{"Pressing Wing-Back", "WBR", "", "MR"},
		{"Wide Covering Central Midfielder", "MC", "left", "ML"},
		{"Wide Covering Central Midfielder", "MC", "right", "MR"},
		{"Tracking Winger", "AML", "", "ML"},
		{"Tracking Winger", "AMR", "", "MR"},
		{"Dropping Defensive Midfielder", "DM", "", "DC"},
		{"Pressing Defensive Midfielder", "DM", "", "MC"},
		{"Tracking Attacking Midfielder", "AMC", "", "MC"},
		{"Pressing Central Midfielder", "MC", "", "AMC"},
		{"Inside Outlet Winger", "AML", "", "AMC"},
		{"Tracking Centre-Forward", "ST", "", "AMC"},
		{"Splitting Outlet Attacking Midfielder", "AMC", "left", "AML"},
		{"Splitting Outlet Attacking Midfielder", "AMC", "right", "AMR"},
		{"Splitting Outlet Centre-Forward", "ST", "left", "AML"},
		{"Splitting Outlet Centre-Forward", "ST", "right", "AMR"},
	} {
		in := newRoleInputs(uniformPlayer(150, map[string]uint8{tt.pos: 14, tt.neighbour: 20}))
		if got := in.oopRating(mustOOPRole(t, tt.role), sidedAt(t, tt.pos, tt.side)); got != 97 {
			t.Errorf("%s at %s %s with %s 20: %d, want 97", tt.role, tt.pos, tt.side, tt.neighbour, got)
		}
	}
}

// TestOutOfPossessionRulesAreFM27s: every build gets 27.0.0's rules.
func TestOutOfPossessionRulesAreFM27s(t *testing.T) {
	for _, rules := range []roleRules{roleRules26, roleRules27} {
		if got := rules.outOfPossession(); got != roleRules27 {
			t.Errorf("%d: %d, want roleRules27", rules, got)
		}
	}
}
