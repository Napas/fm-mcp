package fm

import (
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Nation indices with the KŽ saves' factors and continents.
const (
	nationFrance = 143 // factor 169, Europe
	nationUSA    = 120 // factor 140, North America
	nationGone   = 7   // a defunct nation: no continent
	nationWorld  = 8   // a nation on a continent of strength 0
)

var (
	france = NationScale{Factor: 169, ContinentStrength: 0.99, HasContinent: true}
	usa    = NationScale{Factor: 140, ContinentStrength: 0.85, HasContinent: true}
)

// repWorld has Metz's first team (France) and Jacksonville Armada U23 (USA).
var repWorld = World{
	Teams: map[int64]*Team{
		652:   {Index: 652, ClubIndex: 844, Name: "FC Metz", NationIndex: nationFrance},
		70000: {Index: 70000, ClubIndex: 5000, Name: "Jacksonville Armada U23", NationIndex: nationUSA},
		71000: {Index: 71000, ClubIndex: 5001, Name: "Old club", NationIndex: nationGone},
		72000: {Index: 72000, ClubIndex: 5002, Name: "World club", NationIndex: nationWorld},
		585:   {Index: 585, ClubIndex: noTeam, Name: "France", National: true, NationIndex: nationFrance},
	},
	NationScales: map[int64]NationScale{
		nationFrance: france, nationUSA: usa, nationGone: {Factor: 100},
		nationWorld: {Factor: 169, HasContinent: true},
	},
}

// Thresholds from the research samples: Metz (France, Europe) and
// Jacksonville Armada U23 (USA, North America).
func TestReputationThresholds(t *testing.T) {
	tests := []struct {
		name           string
		scale          NationScale
		tc, tn, tr, tl int
	}{
		{"France", france, 6645, 3956, 2082, 416},
		{"USA", usa, 3675, 2327, 1225, 245},
	}
	for _, tt := range tests {
		s0, s1 := tt.scale.nationScale(), tt.scale.continentScale()
		tc, tn, tr, tl := reputationThresholds(s0, s1)
		if tc != tt.tc || tn != tt.tn || tr != tt.tr || tl != tt.tl {
			t.Errorf("%s: thresholds %d %d %d %d, want %d %d %d %d", tt.name, tc, tn, tr, tl, tt.tc, tt.tn, tt.tr, tt.tl)
		}
	}
}

func TestReputationThresholdsClampAndFallBack(t *testing.T) {
	// A weak nation: the national cut is raised to 2150.
	tc, tn, tr, tl := reputationThresholds(0.05, 0.7)
	if tc != 3675 || tn != 2150 || tr != 87 || tl != 17 {
		t.Errorf("s0 0.05: %d %d %d %d", tc, tn, tr, tl)
	}
	// A strong one: the national cut is capped at 5250 below the regional
	// cut 5500 (s0 2.2), which drops to 4000; local 1100 stays.
	tc, tn, tr, tl = reputationThresholds(2.2, 1)
	if tc != 6750 || tn != 5250 || tr != 4000 || tl != 1100 {
		t.Errorf("s0 2.2: %d %d %d %d", tc, tn, tr, tl)
	}
	// A continental cut above 7250 drops to 6500.
	if tc, _, _, _ := reputationThresholds(1, 1.2); tc != 6500 {
		t.Errorf("s1 1.2: continental cut %d, want 6500", tc)
	}
}

func TestNationScales(t *testing.T) {
	// The game cuts the rounded factor to int8 (0x2fbe778): 128 wraps to
	// -128, 258 to 2.
	for factor, want := range map[int16]float32{
		169: 0.85, 140: 0.7, 145: 0.75, 0: 0.05, 4: 0.05, 187: 0.95,
		1265: 6.35, 1275: 0.05, 2580: 0.1, -300: 0.05,
	} {
		if got := (NationScale{Factor: factor}).nationScale(); got != want {
			t.Errorf("factor %d: %v, want %v", factor, got, want)
		}
	}
	if got := france.continentScale(); got != float32(0.99)+(float32(0.99)-1) {
		t.Errorf("Europe: %v", got)
	}
}

func TestReputationLevelWords(t *testing.T) {
	tests := []struct {
		cr, wr int
		want   string
	}{
		{0, 7250, "Worldwide"},
		{9000, 7249, "Continental"},
		{6645, 3626, "Continental"},
		{6644, 3626, "National"},
		{6671, 3625, "National"}, // Bernardi's current reputation with a world one just too low
		{3956, 0, "National"},
		{3955, 0, "Regional"},
		{2082, 0, "Regional"},
		{2081, 0, "Local"},
		{416, 0, "Local"},
		{415, 0, "Obscure"},
	}
	s0, s1 := france.nationScale(), france.continentScale()
	for _, tt := range tests {
		if got := reputationWords[reputationLevel(tt.cr, tt.wr, s0, s1)]; got != tt.want {
			t.Errorf("cr %d, wr %d: %q, want %q", tt.cr, tt.wr, got, tt.want)
		}
	}
}

func basics(team, nation int64, cr, wr int) PlayerBasics {
	return PlayerBasics{TeamIndex: team, NationIndex: nation, Reputation: Reputation{Current: cr, World: wr}}
}

func TestReputationLevelUsesHisTeamsNationThenHisNationality(t *testing.T) {
	tests := []struct {
		name string
		b    PlayerBasics
		want string
	}{
		{"Bernardi at Metz", basics(652, 150, 6671, 3645), "Continental"},
		{"Cardozo at Jacksonville", basics(70000, 104, 3046, 1841), "National"},
		{"an American at Metz", basics(652, nationUSA, 3000, 1000), "Regional"},
		{"a Frenchman at Jacksonville", basics(70000, nationFrance, 3000, 1000), "National"},
		{"national team", basics(585, nationUSA, 3000, 1000), "Regional"},
		{"free agent: his nationality", basics(noTeam, nationUSA, 3000, 1000), "National"},
		{"free agent without a known nation", basics(noTeam, 999, 6800, 3700), "Continental"},
		// No continent at the club's nation: its factor, his nationality's continent.
		{"club in a defunct nation", basics(71000, nationUSA, 2200, 0), "National"},
		// Continent strength 0 at the club's nation: his nationality's
		// continent (0x2fbe7cc), else 1.
		{"club on a strength-0 continent", basics(72000, nationUSA, 3000, 1000), "National"},
		{"both on a strength-0 continent", basics(72000, nationWorld, 3000, 1000), "Regional"},
	}
	for _, tt := range tests {
		if got := repWorld.reputationLevel(tt.b); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestReputationLevelIsLeftOutWhereItsInputsAreUnknown(t *testing.T) {
	woman := basics(652, nationFrance, 9000, 9000)
	woman.Female = true
	coach := basics(652, nationFrance, 9000, 9000)
	coach.PlayerCoach = true

	for name, b := range map[string]PlayerBasics{"woman": woman, "player-coach": coach} {
		if got := repWorld.reputationLevel(b); got != "" {
			t.Errorf("%s: %q, want none", name, got)
		}
	}
	if got := testWorld.reputationLevel(basics(547, 139, 9000, 9000)); got != "" {
		t.Errorf("without nation scales: %q, want none", got)
	}
}

// The game shows the reputation word for unscouted players (Cardozo, 12-14).
func TestFogOfWarNeverHidesTheReputationLevel(t *testing.T) {
	raw := rawPlayer(2000280809, "Rahul", "Cardozo")
	raw.TeamIndex, raw.CurrentReputation, raw.WorldReputation = 70000, 3046, 1841
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 652}, []savefile.Player{raw}, repWorld, PlayerExtras{})

	p, _ := g.View(true).Profile(2000280809)

	if p.Scouting != ScoutingRequired || p.ReputationLevel != "National" {
		t.Errorf("scouting %q, reputation level %q; want required and National", p.Scouting, p.ReputationLevel)
	}
}
