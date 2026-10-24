package fm

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func mustRole(t *testing.T, name string) uint64 {
	t.Helper()
	r, ok := inPossessionRoleNamed(name)
	if !ok {
		t.Fatalf("no in-possession role %q", name)
	}
	return r
}

func positionAt(t *testing.T, code string) place {
	t.Helper()
	i := slices.Index(savefile.PositionNames[:], code)
	if i < 0 {
		t.Fatalf("no position %q", code)
	}
	return placeAt(i)
}

// roleTestPlayer has every attribute at attr, the given role's key
// attributes at key (when role is set), position ratings 1 except those
// given, and current ability 150.
func roleTestPlayer(attr uint8, role string, key uint8, positions map[string]uint8) savefile.Player {
	var p savefile.Player
	for i := range p.Attributes {
		p.Attributes[i] = attr
	}
	if r, ok := inPossessionRoleNamed(role); ok {
		for _, i := range roleAttributesByRole[r].key {
			p.Attributes[i] = key
		}
	}
	for i := range p.Positions {
		p.Positions[i] = 1
	}
	for code, v := range positions {
		p.Positions[slices.Index(savefile.PositionNames[:], code)] = v
	}
	p.CurrentAbility = 150
	return p
}

func roleNames(idx []int) string {
	names := make([]string, len(idx))
	for i, a := range idx {
		names[i] = savefile.AttributeNames[a]
	}
	return strings.Join(names, " ")
}

// TestRoleAttributesMatchTheGame checks the key, preferable and negative
// attributes of the six roles whose shading was checked in game
// (2035-12-14) and of the Goalkeeper.
func TestRoleAttributesMatchTheGame(t *testing.T) {
	for name, want := range map[string][3]string{
		"Channel Midfielder": {"Crossing OffTheBall Passing FirstTouch Technique WorkRate Acceleration Composure",
			"Dribbling LongShots Vision Anticipation Decisions Flair Stamina Pace Agility", ""},
		"Wide Centre-Back": {"Heading Marking Tackling Anticipation Positioning Strength JumpingReach",
			"Dribbling Decisions WorkRate Acceleration Stamina Pace Bravery Aggression Agility Composure Concentration", "Passing"},
		"Wide Midfielder": {"Crossing Passing Decisions Technique Teamwork WorkRate Stamina Pace",
			"Dribbling OffTheBall Vision Anticipation FirstTouch Acceleration Agility Composure", ""},
		"No Nonsense Goalkeeper": {"Handling AerialReach CommandOfArea Communication Positioning Reflexes Agility Concentration",
			"Anticipation Decisions OneOnOnes", "Passing Eccentricity"},
		"Inside Forward": {"Dribbling OffTheBall Anticipation FirstTouch Technique Acceleration Agility Composure",
			"Crossing Finishing LongShots Passing Vision Flair WorkRate Stamina Pace Balance", ""},
		"Channel Forward": {"Dribbling Finishing OffTheBall FirstTouch Technique WorkRate Acceleration Composure",
			"Crossing Heading Passing Anticipation Decisions Stamina Pace Balance Agility", ""},
		"Goalkeeper": {"Handling AerialReach CommandOfArea Communication Positioning Reflexes Agility Concentration",
			"Kicking Throwing Anticipation Decisions OneOnOnes", "Eccentricity"},
	} {
		a := roleAttributesByRole[mustRole(t, name)]
		got := [3]string{roleNames(a.key), roleNames(a.preferable), roleNames(a.negative)}
		if got != want {
			t.Errorf("%s: key, preferable, negative = %q, want %q", name, got, want)
		}
	}
}

func TestEveryNamedInPossessionRoleHasAttributes(t *testing.T) {
	n := 0
	for bit := range 64 {
		role := uint64(1) << bit
		_, named := savefile.InPossessionRoleName(role)
		a, ok := roleAttributesByRole[role]
		if named != ok {
			t.Errorf("role bit %d: named %v, attributes %v", bit, named, ok)
		}
		if ok {
			n++
			if len(a.key) == 0 {
				t.Errorf("role bit %d has no key attributes", bit)
			}
		}
	}
	if n != 50 {
		t.Errorf("%d roles with attributes, want 50", n)
	}
	for i, roles := range positionRoles {
		for bit := range 64 {
			if _, ok := roleAttributesByRole[1<<bit]; roles&(1<<bit) != 0 && !ok {
				t.Errorf("%s offers role bit %d, which has no attributes", savefile.PositionNames[i], bit)
			}
		}
	}
}

func TestInPossessionRoleNamedIgnoresCase(t *testing.T) {
	if r, ok := inPossessionRoleNamed("overlapping centre-back"); !ok || r != 0x40000000000000 {
		t.Errorf("got %#x, %v", r, ok)
	}
	if r, ok := inPossessionRoleNamed("Sweeper Keeper"); ok {
		t.Errorf("an out-of-possession role gave %#x", r)
	}
}

// TestRoleBitConstantsNameTheirRoles pins each role-bit constant to the
// roles its comment names, and the position masks of the side blends.
func TestRoleBitConstantsNameTheirRoles(t *testing.T) {
	for _, tt := range []struct {
		mask  uint64
		names []string
	}{
		{wideOverlappingCentreBacks, []string{"Wide Centre-Back", "Overlapping Centre-Back"}},
		{insideFullBack, []string{"Inside Full-Back"}},
		{insideWideForwards, []string{"Inside Forward", "Playmaking Winger", "Inside Winger"}},
		{falseNine, []string{"False Nine"}},
		{advancedCentreBack, []string{"Advanced Centre-Back"}},
		{completeForward, []string{"Complete Forward"}},
		{threshold100Roles, []string{"Free Role", "Wide Forward", "Box To Box Playmaker"}},
		{threshold80Roles, []string{"Advanced Centre-Back", "Enganche", "Regista", "False Nine", "Playmaking Winger"}},
		{threshold80BestRoles, []string{"Advanced Centre-Back", "False Nine", "Playmaking Winger"}},
		{threshold60Roles, []string{"Half Back", "Advanced Wing-Back"}},
	} {
		var want uint64
		for _, n := range tt.names {
			want |= mustRole(t, n)
		}
		if tt.mask != want {
			t.Errorf("%v: mask %#x, want %#x", tt.names, tt.mask, want)
		}
	}
	const attackerLeft, attackerRight, left, right = 0x10000, 0x8000, 0x100000, 0x200000
	mask := func(codes ...string) uint32 {
		var m uint32
		for _, c := range codes {
			m |= positionAt(t, c).mask
		}
		return m
	}
	if want := mask("DL", "WBL", "ML", "AML") | attackerLeft | left; leftPositions != want {
		t.Errorf("leftPositions %#x, want %#x", leftPositions, want)
	}
	if want := mask("DR", "WBR", "MR", "AMR") | attackerRight | right; rightPositions != want {
		t.Errorf("rightPositions %#x, want %#x", rightPositions, want)
	}
}

// Role names are unique ignoring case, so a name gives one role.
func TestInPossessionRoleNamesAreUnique(t *testing.T) {
	seen := map[string]uint64{}
	for role := range roleAttributesByRole {
		n, _ := savefile.InPossessionRoleName(role)
		if other, ok := seen[strings.ToLower(n)]; ok {
			t.Errorf("%q names roles %#x and %#x", n, other, role)
		}
		seen[strings.ToLower(n)] = role
	}
}

func TestPositionPenalty(t *testing.T) {
	for p, want := range map[int]int{20: 0, 18: 2, 15: 5, 14: 15, 13: 17, 12: 20, 11: 36, 1: 76} {
		if got := positionPenalty(p); got != want {
			t.Errorf("rating %d: penalty %d%%, want %d%%", p, got, want)
		}
	}
}

func TestRoleRatingWeighsKeyPreferableAndNegativeAttributes(t *testing.T) {
	gk := mustRole(t, "Goalkeeper")
	allFifty := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"GK": 20}))
	keyEighty := newRoleInputs(roleTestPlayer(50, "Goalkeeper", 80, map[string]uint8{"GK": 20}))
	eccentric := roleTestPlayer(50, "", 0, map[string]uint8{"GK": 20})
	eccentric.Attributes[slices.Index(savefile.AttributeNames[:], "Eccentricity")] = 100
	eccentricIn := newRoleInputs(eccentric)

	for name, tt := range map[string]struct {
		in   roleInputs
		want int
	}{"all 50": {allFifty, 100}, "key 80": {keyEighty, 141}, "eccentricity 100": {eccentricIn, 87}} {
		if got := tt.in.rating(gk, positionAt(t, "GK")); got != tt.want {
			t.Errorf("%s: rating %d, want %d", name, got, tt.want)
		}
	}
}

func TestRoleRatingAppliesThePositionPenalty(t *testing.T) {
	gk := mustRole(t, "Goalkeeper")
	for p, want := range map[uint8]int{20: 100, 18: 98, 15: 95, 14: 85, 13: 83, 12: 80, 11: 64, 1: 24} {
		in := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"GK": p}))
		if got := in.rating(gk, positionAt(t, "GK")); got != want {
			t.Errorf("GK rating %d: role rating %d, want %d", p, got, want)
		}
	}
}

func TestRoleRatingBlendsTheNeighbouringPosition(t *testing.T) {
	ocb := mustRole(t, "Overlapping Centre-Back")
	f9 := mustRole(t, "False Nine")
	dc := positionAt(t, "DC")
	in := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"DC": 14, "DR": 20, "DL": 10}))
	for name, tt := range map[string]struct {
		at   place
		want int
	}{
		"no side: the better full-back rating": {dc, 97},
		"left: DL is lower":                    {place{index: dc.index, mask: 0x100010}, 85},
		"right: DR":                            {place{index: dc.index, mask: 0x200010}, 97},
	} {
		if got := in.rating(ocb, tt.at); got != tt.want {
			t.Errorf("%s: %d, want %d", name, got, tt.want)
		}
	}
	if in := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"DC": 11, "DR": 20})); in.rating(ocb, dc) != 64 {
		t.Errorf("DC 11 does not blend: %d, want 64", in.rating(ocb, dc))
	}
	st := positionAt(t, "ST")
	for _, tt := range []struct {
		st, amc uint8
		want    int
	}{{12, 20, 96}, {12, 14, 56}, {12, 1, 80}, {11, 20, 64}} {
		in := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"ST": tt.st, "AMC": tt.amc}))
		if got := in.rating(f9, st); got != tt.want {
			t.Errorf("False Nine ST %d AMC %d: %d, want %d", tt.st, tt.amc, got, tt.want)
		}
	}
}

func TestBestRoleRating(t *testing.T) {
	for name, tt := range map[string]struct {
		p    savefile.Player
		want int
	}{
		"goalkeeper":                  {roleTestPlayer(50, "", 0, map[string]uint8{"GK": 20}), 100},
		"positions under 10 skipped":  {roleTestPlayer(50, "", 0, map[string]uint8{"GK": 9}), 0},
		"position 10 counts":          {roleTestPlayer(50, "", 0, map[string]uint8{"GK": 10}), 60},
		"AWB under 60 does not count": {roleTestPlayer(10, "Advanced Wing-Back", 12, map[string]uint8{"WBL": 20}), 22},
		"inside forward":              {roleTestPlayer(50, "Inside Forward", 70, map[string]uint8{"AML": 20, "AMR": 14}), 132},
	} {
		if got := newRoleInputs(tt.p).best; got != tt.want {
			t.Errorf("%s: best %d, want %d", name, got, tt.want)
		}
	}
}

func TestRoleAbility(t *testing.T) {
	in := newRoleInputs(roleTestPlayer(50, "Inside Forward", 70, map[string]uint8{"AML": 20, "AMR": 14}))
	for _, tt := range []struct {
		role, pos string
		want      int
	}{
		{"Inside Forward", "AML", 150}, // the best role: current ability
		{"Inside Forward", "AMR", 135}, // AMR 14: raised towards the best before scaling
		{"Winger", "AML", 137},
		{"Goalkeeper", "AML", 118},
	} {
		if got, ok := in.ability(mustRole(t, tt.role), positionAt(t, tt.pos)); !ok || got != tt.want {
			t.Errorf("%s at %s: %d, %v; want %d", tt.role, tt.pos, got, ok, tt.want)
		}
	}
	if got, ok := in.ability(1<<63, positionAt(t, "AML")); ok {
		t.Errorf("unknown role: %d", got)
	}
}

func TestRoleAbilityHalvesUnderTheRoleThreshold(t *testing.T) {
	acb := mustRole(t, "Advanced Centre-Back")
	for ca, want := range map[int16]int{70: 35, 90: 90} {
		p := roleTestPlayer(50, "", 0, map[string]uint8{"DC": 20})
		p.CurrentAbility = ca
		in := newRoleInputs(p)
		if got, _ := in.ability(acb, positionAt(t, "DC")); got != want {
			t.Errorf("current ability %d: %d, want %d", ca, got, want)
		}
	}
}

func TestRoleAbilityWithoutABestRoleIsTheRating(t *testing.T) {
	in := newRoleInputs(roleTestPlayer(50, "", 0, map[string]uint8{"GK": 9}))
	if got, _ := in.ability(mustRole(t, "Goalkeeper"), positionAt(t, "GK")); got != 56 {
		t.Errorf("%d, want 56", got)
	}
}

// sideBits are the position-mask side bits by golden-file side.
var sideBits = map[string]uint32{"-": 0, "left": 0x100000, "right": 0x200000}

// TestRoleAbilityMatchesTheModel checks rating, best rating and role ability
// against cases computed by the research Python model. The cases cover every
// role offered at every position and each blending, threshold and floor rule.
func TestRoleAbilityMatchesTheModel(t *testing.T) {
	data, err := os.ReadFile("testdata/role_ability_golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	num := func(s string) int {
		v, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			t.Fatal(err)
		}
		return int(v)
	}
	players := map[string]roleInputs{}
	cases := 0
	for line := range strings.Lines(string(data)) {
		f := strings.Fields(line)
		switch {
		case len(f) == 0 || f[0] == "#":
		case f[0] == "p" && len(f) == 3+len(savefile.PositionNames)+len(savefile.AttributeNames):
			var p savefile.Player
			p.CurrentAbility = int16(num(f[2])) //nolint:gosec // 1..200
			for i := range p.Positions {
				p.Positions[i] = uint8(num(f[3+i])) //nolint:gosec // 1..20
			}
			for i := range p.Attributes {
				p.Attributes[i] = uint8(num(f[3+len(p.Positions)+i])) //nolint:gosec // 0..100
			}
			players[f[1]] = newRoleInputs(p)
		case f[0] == "c" && len(f) == 8:
			in, ok := players[f[1]]
			side, sideOK := sideBits[f[4]]
			if !ok || !sideOK {
				t.Fatalf("bad case %q", line)
			}
			role, at := uint64(num(f[2])), positionAt(t, f[3]) //nolint:gosec // a role bit
			at.mask |= side
			ability, _ := in.ability(role, at)
			if got, want := [3]int{in.rating(role, at), in.best, ability}, [3]int{num(f[5]), num(f[6]), num(f[7])}; got != want {
				t.Errorf("%s: rating, best, ability = %v, want %v", strings.TrimSpace(line), got, want)
			}
			cases++
		default:
			t.Fatalf("bad line %q", line)
		}
	}
	if cases < 100 {
		t.Errorf("%d cases", cases)
	}
}

// TestRealSaveRoleAbility checks FC Metz's XI on 2035-12-09
// (FM_REAL_SAVE_9DEC): each slot's in-possession role rating, the player's
// best role rating and his role ability, from the game's formula. The game
// showed 4 stars for Reddy, 3.5 for Tacalfred, Edubatey, Matković and Ly, 3
// for Sami, Vassiljev and Løvik, 2.5 for Nduquidi and Veiga, 2 for Medina.
// Never point it at the game's own save folder.
func TestRealSaveRoleAbility(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC))
	tac, ok := g.View(true).Tactic()
	if !ok {
		t.Fatal("no tactic")
	}
	want := []struct {
		name                  string
		rating, best, ability int
	}{
		{"Bruno Sami", 131, 133, 125},
		{"Danilo Veiga", 123, 126, 115},
		{"Joseph Nduquidi", 118, 132, 109},
		{"Fernando Medina", 120, 131, 107},
		{"Mathias Fjørtoft Løvik", 123, 125, 123},
		{"Thomas Tacalfred", 135, 135, 127},
		{"Karl Vassiljev", 125, 129, 125},
		{"Anton Matković", 128, 130, 127},
		{"Luke Reddy", 129, 132, 124},
		{"Samal Edubatey", 133, 133, 133},
		{"Stephen Ly", 138, 141, 127},
	}
	for i, w := range want {
		s := tac.Slots[i]
		if s.Player == nil || s.Player.Name != w.name {
			t.Fatalf("slot %d player %+v, want %s", i, s.Player, w.name)
		}
		if s.RoleAbility == nil || *s.RoleAbility != w.ability {
			t.Errorf("%s: role_ability %v, want %d", w.name, s.RoleAbility, w.ability)
		}
		in := g.auxOf(g.byUID[s.Player.UID]).roles
		at := positionAt(t, s.InPossession.Position)
		at.mask |= sideBits[s.InPossession.Side]
		if r := in.rating(mustRole(t, s.InPossession.Role), at); r != w.rating || in.best != w.best {
			t.Errorf("%s: rating %d, best %d; want %d, %d", w.name, r, in.best, w.rating, w.best)
		}
	}

	// Manual test 3.12: Metz's Overlapping Centre-Backs and Matković's best roles.
	res, err := g.View(true).Search(Filter{TeamIndex: ptr(int64(652)), Position: "DC", Role: "Overlapping Centre-Back"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range res.Players {
		got = append(got, p.Name+" "+strconv.Itoa(*p.RoleAbility))
	}
	if want := []string{"Karl Vassiljev 120", "Chris Ashby 115", "Carlo Bernardi 112", "Pablo Vilas 110", "Joseph Nduquidi 109", "Fernando Medina 107"}; !slices.Equal(got, want) {
		t.Errorf("Overlapping Centre-Backs %q, want %q", got, want)
	}
	matkovic := findPlayers(t, g.View(true), "anton matkovic", 1)[0]
	prof, _ := g.View(true).Profile(matkovic.UID)
	if want := []RoleFit{{"Wide Forward", "AMR", 129}, {"Inside Forward", "AMR", 127}, {"Channel Forward", "ST", 127}}; !slices.Equal(prof.BestRoles, want) {
		t.Errorf("Matković's best roles %+v, want %+v", prof.BestRoles, want)
	}
}

// TestRealSave14DecRoleAbility checks six FC Metz players' role lists on
// the Overview screen on 2035-12-14 (FM_REAL_SAVE_14DEC): each list's roles
// at the player's position, in the game's order (ties keep the game's
// order), with their role abilities from the game's formula, and that his
// best role gives his current ability and leads best_roles. Never point it
// at the game's own save folder.
func TestRealSave14DecRoleAbility(t *testing.T) {
	type fit struct {
		role    string
		ability int
	}
	want := []struct {
		uid      uint32
		name     string
		position string
		roles    []fit // the game's list, best first
	}{
		{2002099166, "Pablo Rodríguez", "MC", []fit{
			{"Channel Midfielder", 129}, {"Advanced Playmaker", 128}, {"Midfield Playmaker", 126},
			{"Attacking Midfielder", 121}, {"Wide Central Midfielder", 107},
		}},
		{2002115328, "Pablo Vilas", "DC", []fit{
			{"Wide Centre-Back", 119}, {"Centre-Back", 119}, {"No-Nonsense Centre-Back", 115},
			{"Overlapping Centre-Back", 110}, {"Ball-Playing Centre-Back", 106},
		}},
		{2002098873, "Asier Castillo", "ML", []fit{
			{"Wide Midfielder", 127}, {"Winger", 124}, {"Inside Winger", 120}, {"Playmaking Winger", 120},
		}},
		{2002206940, "Mounir Haddad", "GK", []fit{
			{"No Nonsense Goalkeeper", 107}, {"Goalkeeper", 102}, {"Ball-Playing Goalkeeper", 99},
		}},
		{2000301456, "Anton Matković", "AMR", []fit{
			{"Wide Forward", 129}, {"Inside Forward", 127}, {"Winger", 126}, {"Inside Winger", 123},
			{"Playmaking Winger", 120},
		}},
		{2002078767, "Stephen Ly", "ST", []fit{
			{"Centre-Forward", 130}, {"Channel Forward", 127}, {"Deep Lying Forward", 127},
			{"Poacher", 123}, {"Target Forward", 120},
		}},
	}
	g := loadRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	v := g.View(true)
	for _, w := range want {
		p, ok := g.byUID[w.uid]
		if !ok || p.Name != w.name {
			t.Errorf("uid %d: %+v, want %s", w.uid, p, w.name)
			continue
		}
		for _, f := range w.roles {
			if got := g.roleAbility(p, mustRole(t, f.role), positionAt(t, w.position)); got != f.ability {
				t.Errorf("%s: %s at %s %d, want %d", w.name, f.role, w.position, got, f.ability)
			}
		}
		if w.roles[0].ability != p.CurrentAbility {
			t.Errorf("%s: best role %d, current ability %d", w.name, w.roles[0].ability, p.CurrentAbility)
		}
		// best_roles breaks ties by role bit, so Vilas's first is Centre-Back.
		prof, _ := v.Profile(w.uid)
		tied := func(f fit) bool { return f.ability == w.roles[0].ability }
		if len(prof.BestRoles) == 0 || prof.BestRoles[0].Position != w.position ||
			prof.BestRoles[0].RoleAbility != w.roles[0].ability ||
			!slices.ContainsFunc(w.roles, func(f fit) bool { return tied(f) && f.role == prof.BestRoles[0].Role }) {
			t.Errorf("%s: best_roles %+v, want a best role at %s, %d", w.name, prof.BestRoles, w.position, w.roles[0].ability)
		}
	}
}

// TestRealSavesStoredRolesAreOffered checks that search_players accepts every
// stored in-possession position and role, as get_tactic and get_last_lineup
// name them: the human tactic and every team's last lineup in each real save,
// including lineups without an opponent, which get_last_lineup does not serve.
// Never point these at the game's own save folder.
func TestRealSavesStoredRolesAreOffered(t *testing.T) {
	for _, s := range []struct {
		env  string
		date time.Time
	}{
		{"FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC)},
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC)},
		{"FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(s.env, func(t *testing.T) {
			g := loadRealSave(t, s.env, s.date)
			v := g.View(false)
			pairs := map[SlotRole]int{} // by position and role, without side
			add := func(r SlotRole) {
				if r.Position != "" || r.Role != "" {
					pairs[SlotRole{Position: r.Position, Role: r.Role}]++
				}
			}
			tac, ok := v.Tactic()
			if !ok {
				t.Fatal("no tactic")
			}
			for _, sl := range tac.Slots {
				add(sl.InPossession)
			}
			for team, l := range g.lastLineups {
				if l.Formation == nil {
					t.Fatalf("team %d: no formation", team)
				}
				for _, sl := range l.Formation.Slots {
					add(slotRole(sl.InPossession, sl.InPossession.InPossessionRoleName))
				}
			}
			slots := 0
			for r, n := range pairs {
				slots += n
				if r.Position == "" || r.Role == "" {
					t.Errorf("%d slots with position %q and role %q", n, r.Position, r.Role)
					continue
				}
				if _, err := v.Search(Filter{Position: r.Position, Role: r.Role, Limit: 1}); err != nil {
					t.Errorf("%d slots with %s %q: %v", n, r.Position, r.Role, err)
				}
			}
			if slots < 165000 {
				t.Errorf("%d slots, want at least 165 000", slots)
			}
			t.Logf("%d slots, %d position and role pairs", slots, len(pairs))
		})
	}
}

// roleSearchGame has three centre-backs at Tottenham and a right-back:
// 1 (CA 120) is built for Overlapping Centre-Back, 2 (CA 160) for
// Ball-Playing Centre-Back, 3 (CA 140) is all-round, 4 (CA 170) plays DR.
func roleSearchGame() *Game {
	mk := func(uid uint32, born uint16, ca int16, attr uint8, role string, key uint8, pos string) savefile.Player {
		p := rawPlayer(uid, "Player", strconv.Itoa(int(uid)))
		r := roleTestPlayer(attr, role, key, nil)
		p.Attributes, p.CurrentAbility = r.Attributes, ca
		p.Positions[slices.Index(savefile.PositionNames[:], pos)] = 20
		p.BirthDate = savefile.Date{Raw: 100, Year: born}
		return p
	}
	players := []savefile.Player{
		mk(1, 1999, 120, 50, "Overlapping Centre-Back", 80, "DC"),
		mk(2, 1998, 160, 50, "Advanced Centre-Back", 90, "DC"),
		mk(3, 1997, 140, 50, "", 0, "DC"),
		mk(4, 1996, 170, 50, "", 0, "DR"),
	}
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, players, testWorld, PlayerExtras{})
}

func roleAbilities(ps []Profile) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = -1
		if p.RoleAbility != nil {
			out[i] = *p.RoleAbility
		}
	}
	return out
}

func TestSearchRanksByRoleAbility(t *testing.T) {
	v := roleSearchGame().View(false)
	for _, tt := range []struct {
		name      string
		f         Filter
		uids      []uint32
		abilities []int
	}{
		{"role", Filter{Position: "DC", Role: "overlapping centre-back"}, []uint32{3, 2, 1}, []int{140, 137, 120}},
		{"named sort", Filter{Position: "dc", Role: "Overlapping Centre-Back", SortBy: "Role_Ability", Ascending: true}, []uint32{1, 2, 3}, []int{120, 137, 140}},
		{"other sort", Filter{Position: "DC", Role: "Overlapping Centre-Back", SortBy: "age"}, []uint32{3, 2, 1}, []int{140, 137, 120}},
		{"current ability", Filter{Position: "DC", Role: "Overlapping Centre-Back", SortBy: "current_ability"}, []uint32{2, 3, 1}, []int{137, 140, 120}},
		{"no role", Filter{Position: "DC"}, []uint32{2, 3, 1}, []int{-1, -1, -1}},
	} {
		res, err := v.Search(tt.f)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !slices.Equal(uids(res.Players), tt.uids) || !slices.Equal(roleAbilities(res.Players), tt.abilities) {
			t.Errorf("%s: %v with role abilities %v; want %v, %v", tt.name, uids(res.Players), roleAbilities(res.Players), tt.uids, tt.abilities)
		}
	}
}

func TestSearchRejectsBadRoles(t *testing.T) {
	v := roleSearchGame().View(false)
	for _, tt := range []struct {
		f    Filter
		want string
	}{
		{Filter{Role: "Overlapping Centre-Back"}, "needs a position"},
		{Filter{Position: "DC", Role: "Sweeper Keeper"}, `role "Sweeper Keeper"`},
		{Filter{Position: "DC", SortBy: "role_ability"}, "needs a role"},
		{Filter{Position: "ST", Role: "Goalkeeper"}, `role "Goalkeeper" is not offered at ST; roles at ST: `},
		{Filter{Position: "DM", Role: "Regista"}, `role "Regista" is not offered at DM; roles at DM: `},
		{Filter{Position: "SW", Role: "Centre-Back"}, "no role is offered at SW"},
	} {
		_, err := v.Search(tt.f)
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%+v: %v; want an invalid filter mentioning %q", tt.f, err, tt.want)
		}
	}
}

// TestSearchRoleErrorsListOnlyOfferedRoles checks that errors suggest only
// roles the game offers: at the position, or at any position for an unknown
// name.
func TestSearchRoleErrorsListOnlyOfferedRoles(t *testing.T) {
	v := roleSearchGame().View(false)
	_, err := v.Search(Filter{Position: "ST", Role: "Goalkeeper"})
	if err == nil || !strings.Contains(err.Error(), "Channel Forward") || strings.Contains(err.Error(), "Inside Forward") {
		t.Errorf("roles at ST: %v", err)
	}
	_, err = v.Search(Filter{Position: "DC", Role: "Sweeper Keeper"})
	if err == nil || !strings.Contains(err.Error(), "Overlapping Centre-Back") || strings.Contains(err.Error(), "Regista") {
		t.Errorf("unknown role: %v", err)
	}
}

func TestSearchWithFogOfWarHidesRoleAbilityWithCurrentAbility(t *testing.T) {
	res, err := fogGame().View(true).Search(Filter{Position: "AMC", Role: "Attacking Midfielder"})
	if err != nil {
		t.Fatal(err)
	}
	// Full knowledge: 5, 1, 3, 2 by role ability (= current ability); hidden by reputation: 4, 7, 6, 8.
	if want := []uint32{5, 1, 3, 2, 4, 7, 6, 8}; !slices.Equal(uids(res.Players), want) {
		t.Errorf("order %v, want %v", uids(res.Players), want)
	}
	if got, want := roleAbilities(res.Players), []int{150, 120, 100, 90, -1, -1, -1, -1}; !slices.Equal(got, want) {
		t.Errorf("role abilities %v, want %v", got, want)
	}
}

func TestProfileListsBestRoles(t *testing.T) {
	p := rawPlayer(1, "Luke", "Reddy")
	p.Attributes, p.Positions = roleTestPlayer(50, "Inside Forward", 70, nil).Attributes, [15]uint8{}
	p.Positions[slices.Index(savefile.PositionNames[:], "AML")] = 20
	p.Positions[slices.Index(savefile.PositionNames[:], "AMR")] = 14 // not 15+: left out
	p.CurrentAbility = 150
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{p}, testWorld, PlayerExtras{})

	got, _ := g.View(false).Profile(1)

	want := []RoleFit{{"Inside Forward", "AML", 150}, {"Wide Forward", "AML", 146}, {"Inside Winger", "AML", 145}}
	if !slices.Equal(got.BestRoles, want) {
		t.Errorf("best roles %+v, want %+v", got.BestRoles, want)
	}
	if res, _ := g.View(false).Search(Filter{}); res.Players[0].BestRoles != nil {
		t.Errorf("search rows carry best roles: %+v", res.Players[0].BestRoles)
	}
}

func TestProfileBestRolesAreDistinctAndHiddenWithCurrentAbility(t *testing.T) {
	v := fogGame().View(true)
	// All attributes 50 at AMC 20 and MC 18: every AMC role ties at the current ability.
	own, _ := v.Profile(1)
	want := []RoleFit{{"Attacking Midfielder", "AMC", 120}, {"Advanced Playmaker", "AMC", 120}, {"Free Role", "AMC", 120}}
	if !slices.Equal(own.BestRoles, want) {
		t.Errorf("own player: %+v, want %+v", own.BestRoles, want)
	}
	if unknown, _ := v.Profile(6); unknown.BestRoles != nil {
		t.Errorf("unscouted player: %+v", unknown.BestRoles)
	}
}
