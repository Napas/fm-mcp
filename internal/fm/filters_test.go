package fm

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func TestSearchHeightTraitPersonalityAndReleaseClauseFilters(t *testing.T) {
	g := testGame()
	for uid, h := range map[uint32]int{1: 195, 2: 185, 3: 180, 4: 187} { // 5 has no height
		g.byUID[uid].HeightCM = h
	}
	g.byUID[2].traits = 1<<17 | 1<<37 // Argues With Officials, Dives Into Tackles
	g.byUID[4].traits = 1 << 17
	vicario := g.byUID[1] // Model Professional: professionalism 20, temperament 10, age 28
	vicario.Attributes[attrProfessionalism], vicario.Attributes[attrTemperament] = 20, 10
	g.byUID[1].Contract = &Contract{ReleaseFee: 20_000_000}
	g.byUID[2].Contract = &Contract{ReleaseFee: 60_000_000}
	g.byUID[4].Contract = &Contract{}

	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"min height", Filter{Club: "tottenham", MinHeightCM: 186}, []uint32{1, 4}},
		{"max height leaves out unknown heights", Filter{Club: "tottenham", MaxHeightCM: 186}, []uint32{2}},
		{"trait", Filter{Gender: "men", Traits: []string{"argues with officials"}}, []uint32{2, 4}},
		{"all traits", Filter{Gender: "men", Traits: []string{"Argues With Officials", "Dives Into Tackles"}}, []uint32{2}},
		{"personality", Filter{Gender: "men", Personalities: []string{"model professional"}}, []uint32{1}},
		{"any personality", Filter{Club: "tottenham", Personalities: []string{"Model Professional", "Balanced"}}, []uint32{2, 1, 4, 5}},
		{"release clause", Filter{Club: "tottenham", MaxReleaseClause: 50_000_000}, []uint32{1}},
	}
	for _, tt := range tests {
		res, err := g.View(false).Search(tt.f)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := uids(res.Players); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFogOfWarHidesTraitsPersonalityAndReleaseClauses(t *testing.T) {
	v := fogGame().View(true)
	for _, f := range []Filter{
		{Traits: []string{"Marks Opponent Tightly"}},
		{Personalities: []string{"Balanced"}},
		{MaxReleaseClause: 5_000_000},
	} {
		res, err := v.Search(f)
		if err != nil {
			t.Fatal(err)
		}
		if res.NeedsScouting == 0 || slices.Contains(uids(res.Players), 6) {
			t.Errorf("%+v: players %v, needs_scouting %d; want unscouted uid 6 counted, not listed", f, uids(res.Players), res.NeedsScouting)
		}
	}
}

func TestPartialScoutingShowsTraitsButNotPersonality(t *testing.T) {
	v := fogGame().View(true) // uid 4: partial knowledge
	res, err := v.Search(Filter{Traits: []string{"Marks Opponent Tightly"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(uids(res.Players), 4) {
		t.Errorf("trait search %v lacks partly scouted uid 4", uids(res.Players))
	}
	res, err = v.Search(Filter{Personalities: []string{"Balanced"}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(uids(res.Players), 4) {
		t.Errorf("personality search %v lists partly scouted uid 4", uids(res.Players))
	}
}

func TestMinMonthsAtClubCountsWholeMonthsAtMonthEnds(t *testing.T) {
	end := time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)
	mk := func(uid uint32, joined savefile.Date) savefile.Player {
		p := rawPlayer(uid, "Player", "X")
		p.JoinedClub = joined
		return p
	}
	g := NewGame(Info{Date: end}, []savefile.Player{
		mk(1, savefile.Date{Raw: 61, Year: 2025}), // 2025-03-02: 29 days
		mk(2, savefile.Date{Raw: 59, Year: 2025}), // 2025-02-28: the clamped month end
	}, testWorld, PlayerExtras{})
	res, err := g.View(false).Search(Filter{Gender: "any", MinMonthsAtClub: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{2}) {
		t.Errorf("got %v, want [2]", got)
	}
}

func TestSearchMinRoleAbility(t *testing.T) {
	v := roleSearchGame().View(false)
	res, err := v.Search(Filter{Position: "DC", Role: "Overlapping Centre-Back", MinRoleAbility: 137})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{3, 2}) {
		t.Errorf("got %v, want [3 2] (abilities 140, 137; 1 has 120)", got)
	}
}

func TestSearchRejectsBadProfileFilters(t *testing.T) {
	v := testGame().View(false)
	for _, tt := range []struct {
		f    Filter
		want string
	}{
		{Filter{Traits: []string{"Shoots A Lot"}}, `trait "Shoots A Lot"`},
		{Filter{Personalities: []string{"Grumpy"}}, `personality "Grumpy"`},
		{Filter{MinRoleAbility: 100}, "min_role_ability needs a role"},
		{Filter{MinHeightCM: -1}, "min_height_cm"},
		{Filter{MinMonthsAtClub: 1201}, "min_months_at_club"},
		{Filter{MinSeasonMinutes: -1}, "min_season_minutes"},
	} {
		if _, err := v.Search(tt.f); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%+v: %v; want an invalid filter mentioning %q", tt.f, err, tt.want)
		}
	}
}

// TestPersonalityLabelsListEveryLabel samples the personality rule and
// checks each label it gives is one the personality filter accepts.
func TestPersonalityLabelsListEveryLabel(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed keeps the sample reproducible
	attr := func() int { return 1 + r.IntN(20) }
	seen := map[string]bool{}
	for range 500_000 {
		in := personalityInput{
			determination: attr(), ambition: attr(), loyalty: attr(), pressure: attr(), professionalism: attr(),
			sportsmanship: attr(), temperament: attr(), leadership: attr(),
			age: 18 + r.IntN(15), negative: r.IntN(2) == 0, devoted: r.IntN(2) == 0,
		}
		seen[in.label()] = true
	}
	for l := range seen {
		if !slices.Contains(personalityLabels, l) {
			t.Errorf("personalityLabels lacks %q", l)
		}
	}
	if len(seen) != len(personalityLabels) {
		t.Errorf("sampled %d labels, personalityLabels has %d", len(seen), len(personalityLabels))
	}
}

func TestContractReleaseFeeIsTheLowestMinimumFeeReleaseClause(t *testing.T) {
	c := newContract(&savefile.Contract{Clauses: []savefile.Clause{
		{Type: 1, Value: 1_000_000},  // relegation release clause: not a minimum fee
		{Type: 16, Value: 9_000_000}, // foreign clubs
		{Type: 0, Value: 12_000_000},
		{Type: 18, Value: -1}, // no amount
	}}, testWorld)
	if c.ReleaseFee != 9_000_000 {
		t.Errorf("release fee = %d, want 9000000", c.ReleaseFee)
	}
	if c := newContract(&savefile.Contract{Clauses: []savefile.Clause{{Type: 1, Value: 1_000_000}}}, testWorld); c.ReleaseFee != 0 {
		t.Errorf("release fee without a minimum fee clause = %d, want 0", c.ReleaseFee)
	}
}
