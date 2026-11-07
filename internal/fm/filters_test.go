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

func TestSearchProfileFilters(t *testing.T) {
	g := testGame()
	for uid, h := range map[uint32]int{1: 195, 2: 185, 3: 180, 4: 187} { // 5 has no height
		g.byUID[uid].HeightCM = h
	}
	g.byUID[2].traits = 1<<17 | 1<<37 // Argues With Officials, Dives Into Tackles
	g.byUID[4].traits = 1 << 17
	vicario := g.byUID[1] // Model Professional: professionalism 20, temperament 10, age 28
	vicario.Attributes[attrProfessionalism], vicario.Attributes[attrTemperament] = 20, 10
	g.byUID[1].Contract = &Contract{ReleaseFee: 20_000_000, transferFlags: savefile.CoreFlagTransferListed | savefile.CoreFlagUnavailableForLoan}
	g.byUID[2].Contract = &Contract{ReleaseFee: 60_000_000, transferFlags: savefile.CoreFlagLoanListed}
	g.byUID[4].Contract = &Contract{transferFlags: savefile.CoreFlagUnavailableForLoan}
	g.byUID[4].Positions[2].Rating, g.byUID[4].Positions[3].Rating = 16, 12 // Bergvall: DL 16, DC 12

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
		{"any of positions", Filter{Gender: "men", Positions: []string{"dl", "GK"}}, []uint32{1, 4}},
		{"min position rating", Filter{Gender: "men", Position: "DC", MinPositionRating: 19}, []uint32{2}},
		{"retraining candidates", Filter{Gender: "men", Position: "DC", MinPositionRating: 10}, []uint32{2, 4, 3}},
		{"positions with a min rating", Filter{Gender: "men", Positions: []string{"DL", "DC"}, MinPositionRating: 17}, []uint32{2, 3}},
		{"natural only", Filter{Gender: "men", Position: "DC", MinPositionRating: 20}, []uint32{2}},
		{"max attribute", Filter{Club: "tottenham", MaxAttributes: map[string]int{"aggression": 15}}, []uint32{1, 4, 5}},
		{"top reputation", Filter{Club: "tottenham", MinReputation: 7800, MaxReputation: 10000}, []uint32{2}},
		{"reputation at the top of the scale", Filter{Club: "tottenham", MinReputation: 10000}, []uint32{}},
		{"min reputation", Filter{Club: "tottenham", MinReputation: 7000}, []uint32{2, 1}},
		{"max reputation", Filter{Club: "tottenham", MaxReputation: 7000}, []uint32{4, 5}},
		{"reputation range", Filter{Club: "tottenham", MinReputation: 6000, MaxReputation: 7600}, []uint32{1, 4}},
		{"transfer listed", Filter{Club: "tottenham", TransferStatus: []string{"transfer_listed"}}, []uint32{1}},
		{"any transfer status", Filter{Club: "tottenham", TransferStatus: []string{"Loan_Listed", "transfer_listed"}}, []uint32{2, 1}},
		{"unavailable for loan", Filter{Club: "tottenham", TransferStatus: []string{"unavailable_for_loan"}}, []uint32{1, 4}},
		{"exclude unavailable for loan keeps players without a contract", Filter{Club: "tottenham", ExcludeTransferStatus: []string{"Unavailable_For_Loan"}}, []uint32{2, 5}},
		{"exclude any transfer status", Filter{Club: "tottenham", ExcludeTransferStatus: []string{"loan_listed", "transfer_listed"}}, []uint32{4, 5}},
		{"include one status, exclude another", Filter{Club: "tottenham", TransferStatus: []string{"unavailable_for_loan"}, ExcludeTransferStatus: []string{"transfer_listed"}}, []uint32{4}},
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
		{Filter{Positions: []string{"DC", "XX"}}, `position "XX"`},
		{Filter{Position: "DC", Positions: []string{"DL"}}, "use position or positions, not both"},
		{Filter{Positions: []string{"DC", "DL"}, Role: "Centre-Back"}, "role and out_of_possession_role need position, not positions"},
		{Filter{Positions: []string{"DC"}, OutOfPossessionRole: "Centre-Back"}, "role and out_of_possession_role need position, not positions"},
		{Filter{MinPositionRating: 10}, "min_position_rating needs a position or positions"},
		{Filter{Position: "DC", MinPositionRating: 21}, "min_position_rating 21 is outside 1-20"},
		{Filter{Position: "DC", MinPositionRating: -1}, "min_position_rating -1 is outside 1-20"},
		{Filter{MaxAttributes: map[string]int{"Speed": 10}}, `attribute "Speed"`},
		{Filter{MaxAttributes: map[string]int{"Pace": 0}}, "Pace maximum 0 is outside 1-20"},
		{Filter{MinReputation: -1}, "min_reputation -1 is outside 0-10000"},
		{Filter{MaxReputation: 10001}, "max_reputation 10001 is outside 0-10000"},
		{Filter{MaxReputation: -1}, "max_reputation -1 is outside 0-10000"},
		{Filter{TransferStatus: []string{"listed"}}, `transfer_status "listed"; valid values: transfer_listed, loan_listed, unavailable_for_loan`},
		{Filter{ExcludeTransferStatus: []string{"listed"}}, `exclude_transfer_status "listed"; valid values: transfer_listed, loan_listed, unavailable_for_loan`},
		{Filter{TransferStatus: []string{"loan_listed"}, ExcludeTransferStatus: []string{"Loan_Listed"}}, "in both transfer_status and exclude_transfer_status: loan_listed"},
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

// TestSearchRoleAbilityBelowAccomplished: a role at a position rated 12
// scores as in squad_roles.
func TestSearchRoleAbilityBelowAccomplished(t *testing.T) {
	v := squadRolesGame().View(false)
	res, err := v.Search(Filter{Gender: "any", Position: "DC", MinPositionRating: 10, Role: "Ball-Playing Centre-Back"})
	if err != nil {
		t.Fatal(err)
	}
	grid, err := v.SquadRoles(SquadRolesRequest{TeamIndex: 547, Position: "DC", Phase: PhaseInPossession})
	if err != nil {
		t.Fatal(err)
	}
	want := roleRow(t, grid, "DC", PhaseInPossession, "Ball-Playing Centre-Back").Players[0].RoleAbility
	if got := uids(res.Players); !slices.Equal(got, []uint32{1}) || res.Players[0].RoleAbility == nil || *res.Players[0].RoleAbility != want {
		t.Errorf("players %v, role abilities %v; want [1] at %d", got, roleAbilities(res.Players), want)
	}
}

func TestSearchRowsCarryFilteredPositionRatings(t *testing.T) {
	g := testGame()
	g.byUID[4].Positions[2].Rating, g.byUID[4].Positions[3].Rating = 16, 12 // Bergvall: DL 16, DC 12
	res, err := g.View(false).Search(Filter{Gender: "men", Positions: []string{"DC", "dc", "DL"}, MinPositionRating: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint32][]PositionRating{
		2: {{"DC", 20}, {"DL", 1}},
		4: {{"DC", 12}, {"DL", 16}},
		3: {{"DC", 18}, {"DL", 1}},
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{2, 4, 3}) {
		t.Fatalf("players %v", got)
	}
	for _, p := range res.Players {
		if !slices.Equal(p.PositionRatings, want[p.UID]) {
			t.Errorf("%d: position ratings %+v, want %+v", p.UID, p.PositionRatings, want[p.UID])
		}
	}
	if res, err = g.View(false).Search(Filter{Gender: "men"}); err != nil {
		t.Fatal(err)
	}
	if res.Players[0].PositionRatings != nil {
		t.Errorf("rows without a position filter carry %+v", res.Players[0].PositionRatings)
	}
}

func TestContractTransferStatus(t *testing.T) {
	for flags, want := range map[uint8][]string{
		0:    nil,
		0x80: nil, // bit 7 is not a status
		0x41: {"transfer_listed", "unavailable_for_loan"},
		0x03: {"transfer_listed", "loan_listed"},
		0x02: {"loan_listed"},
	} {
		c := newContract(&savefile.Contract{CoreFlags: flags}, testWorld)
		if got := c.TransferStatus(); !slices.Equal(got, want) {
			t.Errorf("flags %#x: %v, want %v", flags, got, want)
		}
	}
}

// Fog of war never hides the transfer status: the game shows it on every
// player's profile.
func TestFogOfWarShowsTransferStatus(t *testing.T) {
	g := fogGame()
	g.byUID[6].Contract.transferFlags = savefile.CoreFlagTransferListed
	v := g.View(true)
	res, err := v.Search(Filter{TransferStatus: []string{"transfer_listed"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{6}) || res.NeedsScouting != 0 {
		t.Fatalf("search = %v, needs scouting %d; want [6], 0", got, res.NeedsScouting)
	}
	if got := res.Players[0].TransferStatus; !slices.Equal(got, []string{"transfer_listed"}) {
		t.Errorf("row transfer status = %v", got)
	}
	if p, ok := v.Profile(6); !ok || !slices.Equal(p.TransferStatus, []string{"transfer_listed"}) {
		t.Errorf("profile 6 found %v, transfer status %v", ok, p.TransferStatus)
	}
}

// A loanee's status comes from the main contract, not the loan.
func TestLoaneeTransferStatusIsTheParentClubs(t *testing.T) {
	g := fogGame()
	g.byUID[3].Contract.transferFlags = savefile.CoreFlagTransferListed
	g.byUID[3].Loan.transferFlags = savefile.CoreFlagLoanListed
	v := g.View(false)
	if p, ok := v.Profile(3); !ok || !slices.Equal(p.TransferStatus, []string{"transfer_listed"}) {
		t.Errorf("profile 3 found %v, transfer status %v; want [transfer_listed]", ok, p.TransferStatus)
	}
	for status, want := range map[string][]uint32{"transfer_listed": {3}, "loan_listed": {}} {
		res, err := v.Search(Filter{TransferStatus: []string{status}})
		if err != nil {
			t.Fatal(err)
		}
		if got := uids(res.Players); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", status, got, want)
		}
	}
}
