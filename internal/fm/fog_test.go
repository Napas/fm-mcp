package fm

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// fogGame is a save managed from Tottenham (team 547, club 416). Person
// indices equal uids.
func fogGame() *Game { return fogGameWithSeason(nil) }

// fogGameWithSeason is fogGame with season stats by person index.
func fogGameWithSeason(season map[uint32]savefile.SeasonRecord) *Game {
	mk := func(uid uint32, team int64, rep uint16, ca int16) savefile.Player {
		p := rawPlayer(uid, "Player", strconv.FormatUint(uint64(uid), 10))
		p.Index, p.TeamIndex, p.CurrentReputation, p.CurrentAbility, p.PotentialAbility = uid, team, rep, ca, ca+10
		p.Value = int32(ca) * 100_000
		p.Hidden = [8]uint8{12, 12, 12, 12, 12, 12, 12, 12}
		p.Traits = 0x8000
		p.ClubAffinities = []savefile.ClubAffinity{{Club: 20, Value: 100, Reason: 2}}
		p.Contract = &savefile.Contract{TeamIndex: team, WeeklyWage: int32(ca) * 100, PlayingTime: 3, OriginalPlayingTime: 2, PlayingTimeScore: 88, SquadNumber: 19, Clauses: []savefile.Clause{{Type: 0, Value: 1_000_000, Extra: 0xffff}}}
		return p
	}
	own := mk(1, 547, 1000, 120)
	youth := mk(2, 27845, 500, 90)
	loanee := mk(3, 900, 900, 100)
	loanee.Contract.TeamIndex = 547
	loanee.Loan = &savefile.Contract{TeamIndex: 900}
	famous := mk(4, 900, 8500, 160) // reputation alone: 80
	famous.FutureContract = &savefile.Contract{TeamIndex: 547, WeeklyWage: 30_000}
	scouted := mk(5, 900, 7000, 150) // stored 100
	unknown := mk(6, 900, 5200, 140) // reputation alone: 30
	weak := mk(7, 900, 6600, 130)    // stored 40, reputation 50
	free := mk(8, -1, 100, 110)      // stored 90
	free.Contract = nil
	scoutedLevels := map[uint32]uint8{5: 100, 7: 40, 8: 90}
	info := Info{Date: gameDate, ManagerTeamIndex: 547}
	return NewGame(info, []savefile.Player{own, youth, loanee, famous, scouted, unknown, weak, free}, testWorld,
		PlayerExtras{Scouted: scoutedLevels, Season: season})
}

func TestBaseKnowledgeFollowsReputation(t *testing.T) {
	for rep, want := range map[int]int{10000: 80, 8000: 80, 7999: 50, 6500: 50, 6499: 30, 5000: 30, 4999: 0, 0: 0} {
		if got := baseKnowledge(rep); got != want {
			t.Errorf("baseKnowledge(%d) = %d, want %d", rep, got, want)
		}
	}
}

func TestViewReportsKnowledgeWithFogOfWar(t *testing.T) {
	v := fogGame().View(true)
	tests := []struct {
		uid      uint32
		level    int
		scouting Scouting
	}{
		{1, 100, ScoutingFull},    // own first team
		{2, 100, ScoutingFull},    // own youth team
		{3, 100, ScoutingFull},    // own player out on loan
		{4, 80, ScoutingPartial},  // reputation
		{5, 100, ScoutingFull},    // stored
		{6, 30, ScoutingRequired}, // reputation
		{7, 50, ScoutingRequired}, // reputation beats the stored level
		{8, 90, ScoutingPartial},  // stored
	}
	for _, tt := range tests {
		p, ok := v.Profile(tt.uid)
		if !ok {
			t.Fatalf("no player %d", tt.uid)
		}
		if p.Knowledge == nil || *p.Knowledge != tt.level || p.Scouting != tt.scouting {
			t.Errorf("player %d: knowledge %v scouting %q, want %d %q", tt.uid, p.Knowledge, p.Scouting, tt.level, tt.scouting)
		}
	}
}

func TestViewWithoutFogOfWarShowsEverything(t *testing.T) {
	p, _ := fogGame().View(false).Profile(6)

	if p.Knowledge != nil || p.Scouting != "" {
		t.Errorf("knowledge %v scouting %q, want neither without fog of war", p.Knowledge, p.Scouting)
	}
	if p.CurrentAbility == nil || *p.CurrentAbility != 140 || p.Value == nil || p.Attributes.Get("Consistency") == 0 || p.Attributes.Get("Ambition") != 12 ||
		p.Personality != "Balanced" || len(p.Traits) != 1 ||
		*p.Contract.Wage != 14_000 || len(p.Contract.Clauses) != 1 || p.PlayingTime != "Regular Starter" || p.OriginalPlayingTime != "Important Player" || p.ActualPlayingTime != "Important Player" {
		t.Errorf("profile hides values: %+v", p)
	}
}

func TestPlayingTimeIsHiddenLikeOtherContractDetails(t *testing.T) {
	v := fogGame().View(true)
	for uid, shown := range map[uint32]bool{1: true, 3: false, 4: false, 5: true, 6: false} {
		p, _ := v.Profile(uid)
		want := [3]string{}
		if shown {
			want = [3]string{"Regular Starter", "Important Player", "Important Player"}
		}
		if got := [3]string{p.PlayingTime, p.OriginalPlayingTime, p.ActualPlayingTime}; got != want {
			t.Errorf("player %d (%s): playing, original and actual playing time %q; want %q", uid, p.Scouting, got, want)
		}
	}
}

// The game shows squad numbers on every club's squad screen.
func TestFogOfWarNeverHidesTheSquadNumber(t *testing.T) {
	v := fogGame().View(true)
	for _, uid := range []uint32{1, 4, 6} {
		if p, _ := v.Profile(uid); p.SquadNumber != 19 {
			t.Errorf("player %d (%s): squad number %d, want 19", uid, p.Scouting, p.SquadNumber)
		}
	}
}

func TestLoyaltyBonusIsHiddenLikeTheWage(t *testing.T) {
	g := fogGame()
	for _, p := range g.byUID {
		p.LoyaltyBonusRemaining = 50_000
	}
	v := g.View(true)
	for uid, shown := range map[uint32]bool{1: true, 3: true, 4: false, 5: true, 6: false} {
		p, _ := v.Profile(uid)
		if got := p.LoyaltyBonusRemaining != nil && *p.LoyaltyBonusRemaining == 50_000; got != shown || p.Contract.Wage != nil != shown {
			t.Errorf("player %d (%s): loyalty bonus %v, wage %v; want both shown: %v", uid, p.Scouting, p.LoyaltyBonusRemaining, p.Contract.Wage, shown)
		}
	}
	if p, _ := g.View(false).Profile(6); p.LoyaltyBonusRemaining == nil {
		t.Error("loyalty bonus hidden without fog of war")
	}
}

func TestPersonalityIsHiddenLikeHiddenAttributesAndTraitsLikeAttributes(t *testing.T) {
	v := fogGame().View(true)
	for _, tt := range []struct {
		uid                 uint32
		personality, traits bool
	}{{1, true, true}, {3, true, true}, {4, false, true}, {5, true, true}, {6, false, false}, {8, false, true}} {
		p, _ := v.Profile(tt.uid)
		if (p.Personality != "") != tt.personality || (p.Traits != nil) != tt.traits {
			t.Errorf("player %d (%s): personality %q, traits %q; want shown %v, %v", tt.uid, p.Scouting, p.Personality, p.Traits, tt.personality, tt.traits)
		}
		if (p.FavouredClubs != nil) != tt.personality {
			t.Errorf("player %d (%s): favoured clubs %v; want shown %v like personality", tt.uid, p.Scouting, p.FavouredClubs, tt.personality)
		}
		if (p.Personality != "") != (p.Attributes.Get("Ambition") != 0) {
			t.Errorf("player %d: personality %q but ambition %d", tt.uid, p.Personality, p.Attributes.Get("Ambition"))
		}
	}
}

// The game lists favoured clubs with a value of 50 or more (in game: Leroux's
// 40 and 8 are not shown, Čuber Potočnik's 50 is), by short name.
func TestProfileListsFavouredClubsFrom50OfKnownClubs(t *testing.T) {
	raw := rawPlayer(1, "Anton", "Matković")
	raw.ClubAffinities = []savefile.ClubAffinity{
		{Club: 416, Value: 100, Reason: 1}, // supporter
		{Club: 20, Value: 50, Reason: 2},   // as a player
		{Club: 9, Value: 75, Reason: 4},    // a women's club, unlabelled reason
		{Club: 20, Value: 49, Reason: 2},   // below the game's cut
		{Club: 7, Value: 100, Reason: 1},   // not in the club table
		{Club: 276, Value: 60, Reason: 2},  // a club without a first team
		{Club: 20, Value: -50, Reason: 2},  // disliked
	}
	p, _ := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{raw}, testWorld, PlayerExtras{}).View(false).Profile(1)

	spurs, leeds, chelsea := int64(547), int64(900), int64(20739)
	want := []FavouredClub{
		{Club: "Spurs", TeamIndex: &spurs, As: "supporter"},
		{Club: "Leeds United", TeamIndex: &leeds, As: "player"},
		{Club: "Chelsea Women", TeamIndex: &chelsea},
		{Club: "Svendborg fB", As: "player"},
	}
	if !reflect.DeepEqual(p.FavouredClubs, want) {
		t.Errorf("favoured clubs %+v, want %+v", p.FavouredClubs, want)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"favoured_clubs":[{"club":"Spurs","team_index":547,"as":"supporter"},{"club":"Leeds United","team_index":900,"as":"player"},{"club":"Chelsea Women","team_index":20739},{"club":"Svendborg fB","as":"player"}]`) {
		t.Errorf("profile JSON: %s", b)
	}
}

func TestProfileLeavesOutEmptyFavouredClubs(t *testing.T) {
	raw := rawPlayer(1, "Louis", "Leroux")
	raw.ClubAffinities = []savefile.ClubAffinity{{Club: 19201, Value: 40, Reason: 2}, {Club: 416, Value: 8, Reason: 2}}
	p, _ := NewGame(Info{Date: gameDate}, []savefile.Player{raw}, testWorld, PlayerExtras{}).View(false).Profile(1)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if p.FavouredClubs != nil || strings.Contains(string(b), "favoured_clubs") {
		t.Errorf("profile JSON: %s", b)
	}
}

func TestNoTraitsAreLeftOut(t *testing.T) {
	g := fogGame()
	g.byUID[1].traits = 0
	p, _ := g.View(false).Profile(1)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if p.Traits != nil || strings.Contains(string(b), `"traits"`) || !strings.Contains(string(b), `"personality":"Balanced"`) {
		t.Errorf("profile JSON: %s", b)
	}
}

func TestNoLoyaltyBonusIsLeftOut(t *testing.T) {
	p, _ := fogGame().View(false).Profile(1)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if p.LoyaltyBonusRemaining != nil || strings.Contains(string(b), "loyalty_bonus_remaining") {
		t.Errorf("profile without a loyalty bonus carries one: %s", b)
	}
}

func TestPartialKnowledgeHidesAbilityWageClausesAndHiddenAttributes(t *testing.T) {
	p, _ := fogGame().View(true).Profile(4)

	if p.CurrentAbility != nil || p.PotentialAbility != nil {
		t.Errorf("ability shown: %v %v", p.CurrentAbility, p.PotentialAbility)
	}
	if p.Value == nil || *p.Value != 16_000_000 {
		t.Errorf("value = %v, want 16000000", p.Value)
	}
	if p.Contract == nil || p.Contract.Wage != nil || p.Contract.Clauses != nil || p.Contract.TeamIndex != 900 {
		t.Errorf("contract = %+v, want team without wage or clauses", p.Contract)
	}
	if p.FutureContract == nil || p.FutureContract.Wage != nil || p.FutureContract.TeamIndex != 547 {
		t.Errorf("future contract = %+v, want team without wage", p.FutureContract)
	}
	for _, name := range []string{"Consistency", "Dirtiness", "ImportantMatches", "InjuryProneness", "Versatility",
		"Adaptability", "Ambition", "Loyalty", "Pressure", "Professionalism", "Sportsmanship", "Temperament", "Controversy"} {
		if p.Attributes.Get(name) != 0 {
			t.Errorf("hidden attribute %s shown", name)
		}
	}
	if p.Personality != "" {
		t.Errorf("personality %q shown", p.Personality)
	}
	if !slices.Equal(p.Traits, []string{"Marks Opponent Tightly"}) {
		t.Errorf("traits = %q, want them shown with the attributes", p.Traits)
	}
	if p.Attributes.Get("Pace") != 10 || p.Attributes.Get("Eccentricity") != 10 {
		t.Errorf("visible attributes = %v", p.Attributes)
	}
}

func TestRequiredScoutingHidesAttributesAndValueAndLeavesThemOutOfJSON(t *testing.T) {
	g := fogGame()
	g.byUID[6].NotForSale, g.byUID[6].Value, g.byUID[6].LoyaltyBonusRemaining = true, 0, 50_000
	p, _ := g.View(true).Profile(6)

	if p.Attributes != (Attributes{}) || p.Value != nil || p.NotForSale || p.CurrentAbility != nil {
		t.Errorf("profile = %+v", p)
	}
	if len(p.Positions) != 15 || p.Contract == nil || p.Name == "" {
		t.Errorf("basic facts hidden: %+v", p)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"attributes"`, `"current_ability"`, `"potential_ability"`, `"value"`, `"wage"`, `"clauses"`, `"not_for_sale"`, `"playing_time"`, `"original_playing_time"`, `"actual_playing_time"`, `"loyalty_bonus_remaining"`, `"personality"`, `"traits"`} {
		if strings.Contains(s, key) {
			t.Errorf("JSON has %s: %s", key, s)
		}
	}
	if !strings.Contains(s, `"knowledge":30`) || !strings.Contains(s, `"scouting":"required"`) || !strings.Contains(s, `"squad_number":19`) {
		t.Errorf("JSON lacks knowledge: %s", s)
	}
}

func TestNotForSaleProfileHasNoValue(t *testing.T) {
	g := fogGame()
	g.byUID[4].NotForSale, g.byUID[4].Value = true, 0

	for _, fog := range []bool{false, true} {
		p, _ := g.View(fog).Profile(4)
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.Value != nil || !p.NotForSale || strings.Contains(string(b), `"value"`) || !strings.Contains(string(b), `"not_for_sale":true`) {
			t.Errorf("fog %v: %s", fog, b)
		}
	}
}

func TestSearchWithFogOfWarMatchesOnlyVisibleValues(t *testing.T) {
	v := fogGame().View(true)
	tests := []struct {
		name  string
		f     Filter
		want  []uint32
		total int
	}{
		{"min ability", Filter{MinCurrentAbility: 95}, []uint32{5, 1, 3}, 3},
		{"max ability", Filter{MaxCurrentAbility: 200}, []uint32{5, 1, 3, 2}, 4},
		{"min potential", Filter{MinPotentialAbility: 1}, []uint32{5, 1, 3, 2}, 4},
		{"max value", Filter{MaxValue: 20_000_000}, []uint32{5, 1, 3, 2, 4, 8}, 6},
		{"max wage", Filter{MaxWage: 20_000}, []uint32{5, 1, 3, 2}, 4}, // 8 has no contract, so no wage
		{"visible attribute", Filter{MinAttributes: map[string]int{"Pace": 5}}, []uint32{5, 1, 3, 2, 4, 8}, 6},
		{"hidden attribute", Filter{MinAttributes: map[string]int{"Consistency": 5}}, []uint32{5, 1, 3, 2}, 4},
		{"personality attribute", Filter{MinAttributes: map[string]int{"Professionalism": 5}}, []uint32{5, 1, 3, 2}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			got, total := res.Players, res.Total
			if !slices.Equal(uids(got), tt.want) || total != tt.total {
				t.Errorf("got %v (total %d), want %v (total %d)", uids(got), total, tt.want, tt.total)
			}
		})
	}
}

func TestSearchWithFogOfWarSortsHiddenKeysLastByReputation(t *testing.T) {
	v := fogGame().View(true)
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		// Full: 5 (150), 1 (120), 3 (100), 2 (90); hidden by reputation: 4, 7, 6, 8.
		{"default", Filter{}, []uint32{5, 1, 3, 2, 4, 7, 6, 8}},
		{"ascending", Filter{Ascending: true}, []uint32{2, 3, 1, 5, 4, 7, 6, 8}},
		// Value is visible from knowledge 80: 4 (16M) and 8 (11M) join.
		{"value", Filter{SortBy: "value"}, []uint32{4, 5, 1, 8, 3, 2, 7, 6}},
		{"hidden attribute", Filter{SortBy: "Consistency", Limit: 5}, []uint32{1, 2, 3, 5, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Players
			if !slices.Equal(uids(got), tt.want) {
				t.Errorf("got %v, want %v", uids(got), tt.want)
			}
		})
	}
}

func TestSearchTreatsNotForSaleAsUnknownValue(t *testing.T) {
	g := fogGame()
	g.byUID[5].NotForSale, g.byUID[5].Value = true, 0
	v := g.View(false)

	res, err := v.Search(Filter{MaxValue: 300_000_000})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Players
	if slices.Contains(uids(got), 5) {
		t.Errorf("max value matched a player not for sale: %v", uids(got))
	}
	res, err = v.Search(Filter{SortBy: "value"})
	if err != nil {
		t.Fatal(err)
	}
	got = res.Players
	if ids := uids(got); ids[len(ids)-1] != 5 {
		t.Errorf("sort by value = %v, want the player not for sale last", ids)
	}
}

func TestSquadAndDepthWithFogOfWar(t *testing.T) {
	v := fogGame().View(true)

	squad, err := v.Squad(900)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(squad); !slices.Equal(got, []uint32{5, 3, 4, 7, 6}) {
		t.Errorf("Squad(900) = %v, want known abilities first, then by reputation", got)
	}
	if squad[2].CurrentAbility != nil {
		t.Errorf("squad shows hidden ability: %+v", squad[2])
	}
	loaned, err := v.LoanedOut(547)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(loaned); !slices.Equal(got, []uint32{3}) {
		t.Errorf("LoanedOut(547) = %v", got)
	}
	depth, err := v.Depth(900)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range depth {
		for _, p := range d.Players {
			if p.Scouting == "" {
				t.Errorf("%s: depth player %d without scouting", d.Position, p.UID)
			}
		}
	}
}

func TestProfilesDoNotShareContractsWithTheGame(t *testing.T) {
	v := fogGame().View(true)
	first, _ := v.Profile(1)
	*first.Contract.Wage = 0
	first.Contract.Clauses[0].Amount = 0

	again, _ := v.Profile(1)

	if *again.Contract.Wage != 12_000 || again.Contract.Clauses[0].Amount == 0 {
		t.Errorf("editing a profile changed the game: %+v", again.Contract)
	}
}

func TestNegativeValueIsUnknown(t *testing.T) {
	g := fogGame()
	g.byUID[1].Value = -1 // free agents are saved with -1
	v := g.View(false)

	if p, _ := v.Profile(1); p.Value != nil {
		t.Errorf("value = %d, want none", *p.Value)
	}
	res, err := v.Search(Filter{MaxValue: 20_000_000, SortBy: "value", Ascending: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); slices.Contains(got, 1) {
		t.Errorf("max value matched an unknown value: %v", got)
	}
}

func TestSearchCountsPlayersThatNeedScouting(t *testing.T) {
	g := fogGame()
	g.byUID[6].NotForSale, g.byUID[6].Value = true, 0
	tests := []struct {
		name          string
		fog           bool
		f             Filter
		want          []uint32
		needsScouting int
	}{
		// Potential is visible only for 1, 2, 3 and 5; 4, 6, 7 and 8 might match.
		{"hidden potential", true, Filter{MinPotentialAbility: 150}, []uint32{5}, 4},
		{"visible filters still apply", true, Filter{MinPotentialAbility: 150, TeamIndex: ptr(int64(900))}, []uint32{5}, 3},
		// Values are visible from knowledge 80, so only 6 and 7 are hidden; 6
		// is not for sale, which fog of war also hides.
		{"hidden value", true, Filter{MaxValue: 20_000_000}, []uint32{5, 1, 3, 2, 4, 8}, 2},
		// A visible attribute below the minimum rules 4 and 8 out.
		{"visible miss beats a hidden value", true, Filter{MinAttributes: map[string]int{"Pace": 15}, MinCurrentAbility: 1}, nil, 2},
		// 8 is a free agent: his wage is unknown, not hidden.
		{"free agent wage", true, Filter{MaxWage: 20_000, FreeAgentsOnly: true}, nil, 0},
		{"sort on a hidden key", true, Filter{SortBy: "potential_ability", TeamIndex: ptr(int64(900))}, []uint32{5, 3, 4, 7, 6}, 0},
		{"no fog of war", false, Filter{MinPotentialAbility: 150, MaxValue: 20_000_000}, []uint32{4, 5}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := g.View(tt.fog).Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(uids(res.Players), tt.want) || res.NeedsScouting != tt.needsScouting {
				t.Errorf("got %v, needs scouting %d; want %v, %d", uids(res.Players), res.NeedsScouting, tt.want, tt.needsScouting)
			}
		})
	}
}

func TestSearchSortsFreeAgentsAfterKnownWages(t *testing.T) {
	v := fogGame().View(false)

	res, err := v.Search(Filter{SortBy: "wage", Ascending: true})
	if err != nil {
		t.Fatal(err)
	}
	if ids := uids(res.Players); ids[len(ids)-1] != 8 {
		t.Errorf("sort by wage = %v, want the free agent (8) last", ids)
	}
}

func TestFogOfWarHidesOtherClubsBankBalances(t *testing.T) {
	teams := make(map[int64]*Team, len(testWorld.Teams))
	for i, team := range testWorld.Teams {
		withBalance := *team
		withBalance.BankBalance = 1_000_000 + i
		teams[i] = &withBalance
	}
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, nil, World{Teams: teams}, PlayerExtras{})

	fog := g.View(true)
	for _, i := range []int64{547, 27845} { // own first team and U21
		if team, _ := fog.Team(i); team.BankBalance != 1_000_000+i {
			t.Errorf("own team %d balance = %d", i, team.BankBalance)
		}
	}
	leeds, _ := fog.Team(900)
	if b, _ := json.Marshal(leeds); leeds.BankBalance != 0 || strings.Contains(string(b), "bank_balance") {
		t.Errorf("Team(900) under fog of war = %s, want no bank balance", b)
	}
	if found := findTeams(t, fog, "leeds", 1); found[0].BankBalance != 0 {
		t.Errorf("FindTeams(leeds) balance = %d, want hidden", found[0].BankBalance)
	}
	if found := findTeams(t, fog, "spurs", 1); found[0].BankBalance != 1_000_547 {
		t.Errorf("FindTeams(spurs) balance = %d, want shown", found[0].BankBalance)
	}
	if leeds, _ := g.View(false).Team(900); leeds.BankBalance != 1_000_900 {
		t.Errorf("Team(900) without fog of war balance = %d", leeds.BankBalance)
	}
	g.Info.ManagerTeamIndex = -1
	if team, _ := g.View(true).Team(547); team.BankBalance != 0 {
		t.Errorf("unknown manager's club: Team(547) balance = %d, want hidden", team.BankBalance)
	}
}

// TestSortByKeyComputesEachKeyOnce checks the order (visible keys first,
// then by reputation) and that each player's key is computed once.
func TestSortByKeyComputesEachKeyOnce(t *testing.T) {
	mk := func(uid uint32, rep int, s sight) seen {
		p := &Player{}
		p.UID, p.Reputation.Current = uid, rep
		return seen{p, s}
	}
	xs := []seen{mk(1, 10, 0), mk(2, 50, fullKnowledge), mk(3, 90, 0), mk(4, 20, fullKnowledge), mk(5, 30, fullKnowledge)}
	calls := map[uint32]int{}
	key := func(p *Player, s sight) (int, bool) {
		calls[p.UID]++
		return int(p.UID % 3), s.full()
	}
	sortByKey(xs, key, false)

	got := make([]uint32, len(xs))
	for i, x := range xs {
		got[i] = x.p.UID
	}
	// Keys 2, 1, 2 for players 2, 4, 5; 3 and 1 hidden, by reputation.
	if want := []uint32{2, 5, 4, 3, 1}; !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
	for uid, n := range calls {
		if n != 1 {
			t.Errorf("player %d: key computed %d times", uid, n)
		}
	}
}
