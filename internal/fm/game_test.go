package fm

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func testGame() *Game {
	mk := func(uid uint32, first, last string, club int64, ca, pa int16, year uint16, rep uint16) savefile.Player {
		p := rawPlayer(uid, first, last)
		p.TeamIndex, p.CurrentAbility, p.PotentialAbility = club, ca, pa
		p.BirthDate = savefile.Date{Raw: 100, Year: year}
		p.CurrentReputation = rep
		return p
	}
	vicario := mk(1, "Guglielmo", "Vicario", 547, 144, 163, 1996, 7550)
	vicario.Positions = [15]uint8{20}
	romero := mk(2, "Cristian", "Romero", 547, 156, 168, 1998, 7800)
	romero.Positions = [15]uint8{1, 1, 1, 20}
	romero.Attributes[45] = 100 // Aggression 20
	romero.NationalTeamRef = 1233
	axel := mk(3, "Axel", "Romero", -1, 65, 105, 2004, 3250)
	axel.Positions = [15]uint8{1, 1, 1, 18}
	bergvall := mk(4, "Lucas", "Bergvall", 547, 140, 181, 2006, 6000)
	napas := mk(5, "Napas", "Mačiulaitis", 547, 1, 1, 1990, 1)
	thompson := mk(6, "Alyssa", "Thompson", 20739, 172, 185, 2004, 7000)
	thompson.Female = true
	thompson.Positions = [15]uint8{1, 1, 1, 18}
	return NewGame(Info{Date: gameDate}, []savefile.Player{vicario, romero, axel, bergvall, napas, thompson}, testWorld, PlayerExtras{})
}

func uids(ps []Profile) []uint32 {
	out := make([]uint32, len(ps))
	for i, p := range ps {
		out[i] = p.UID
	}
	return out
}

func TestProfileByUID(t *testing.T) {
	v := testGame().View(false)

	if p, ok := v.Profile(2); !ok || p.Name != "Cristian Romero" {
		t.Errorf("Profile(2) = %v, %v", p, ok)
	}
	if _, ok := v.Profile(99); ok {
		t.Error("Profile(99) found a player")
	}
}

func TestFindPlayersMatchesAllWordsIgnoringAccentsMostReputableFirst(t *testing.T) {
	g := testGame()

	if got := uids(findPlayers(t, g.View(false), "romero", 10)); !slices.Equal(got, []uint32{2, 3}) {
		t.Errorf("romero = %v, want [2 3]", got)
	}
	if got := uids(findPlayers(t, g.View(false), "Napas Maciulaitis", 10)); !slices.Equal(got, []uint32{5}) {
		t.Errorf("Napas Maciulaitis = %v, want [5]", got)
	}
	if got := uids(findPlayers(t, g.View(false), "romero", 1)); !slices.Equal(got, []uint32{2}) {
		t.Errorf("limit 1 = %v, want [2]", got)
	}
}

func TestFindPlayersMatchesTheStartOfNameWords(t *testing.T) {
	mk := func(uid uint32, first, last string, rep uint16) savefile.Player {
		p := rawPlayer(uid, first, last)
		p.CurrentReputation = rep
		return p
	}
	g := NewGame(Info{Date: gameDate}, []savefile.Player{
		mk(1, "Stephen", "Ly", 3000),
		mk(2, "Stephen", "Reilly", 5000),
		mk(3, "Stephen", "Connolly", 4000),
		mk(4, "Heung-Min", "Son", 8000),
		mk(5, "Youssouf", "M'Changama", 6000),
		mk(6, "Bradley", "Traoré-Barcola", 7000),
	}, testWorld, PlayerExtras{})
	v := g.View(false)
	for query, want := range map[string][]uint32{
		"Stephen Ly":     {1},
		"steph":          {2, 3, 1},
		"eilly":          nil,
		"son heung":      {4},
		"min":            {4},
		"barcola":        {6},
		"traore":         {6},
		"traore-barcola": {6},
		"changama":       {5},
		"m'changama":     {5},
		"mchangama":      {5},
	} {
		if got := uids(findPlayers(t, v, query, 10)); !slices.Equal(got, want) {
			t.Errorf("%q = %v, want %v", query, got, want)
		}
	}
}

func TestSquadListsClubPlayersByAbility(t *testing.T) {
	g := testGame()

	squad, err := g.View(false).Squad(547)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(squad); !slices.Equal(got, []uint32{2, 1, 4, 5}) {
		t.Errorf("Squad(547) = %v, want [2 1 4 5]", got)
	}
}

// A row's squad number is his current contract's, so a squad of another team
// (a first-team player at the U21) leaves it out.
func TestSquadLeavesOutNumbersOfPlayersContractedElsewhere(t *testing.T) {
	g := testGame()
	bergvall, axel := g.byUID[4], g.byUID[3]
	bergvall.TeamIndex, bergvall.Contract, bergvall.SquadNumber = 27845, &Contract{TeamIndex: 547, SquadNumber: 15}, 15
	axel.TeamIndex, axel.Contract, axel.SquadNumber = 27845, &Contract{TeamIndex: 547, SquadNumber: 69}, 14
	axel.Loan = &Contract{TeamIndex: 27845, SquadNumber: 14}

	squad, err := g.View(false).Squad(27845)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uint32]int{}
	for _, p := range squad {
		got[p.UID] = p.SquadNumber
	}
	if want := map[uint32]int{4: 0, 3: 14}; !maps.Equal(got, want) {
		t.Errorf("Squad(27845) squad numbers = %v, want %v", got, want)
	}
}

func TestSquadViewsRejectUnknownTeams(t *testing.T) {
	v := testGame().View(false)
	_, squadErr := v.Squad(424242)
	_, loanErr := v.LoanedOut(424242)
	_, depthErr := v.Depth(424242)

	for name, err := range map[string]error{"Squad": squadErr, "LoanedOut": loanErr, "Depth": depthErr} {
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "find_clubs") {
			t.Errorf("%s(424242) err = %v, want an ErrInvalidFilter pointing to find_clubs", name, err)
		}
	}
}

// A national squad row leaves out the club squad number, which would read as
// a national shirt number.
func TestSquadOfANationalTeamListsCalledUpPlayersWithTheirClubs(t *testing.T) {
	g := testGame()
	g.byUID[2].Contract, g.byUID[2].SquadNumber = &Contract{TeamIndex: 547, SquadNumber: 17}, 17

	squad, err := g.View(false).Squad(1233)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(squad); !slices.Equal(got, []uint32{2}) {
		t.Fatalf("Squad(1233) = %v, want [2]", got)
	}
	p := squad[0]
	if p.TeamIndex != 547 || p.Club != "Tottenham Hotspur" || p.NationalTeamIndex == nil || *p.NationalTeamIndex != 1233 || p.NationalTeam != "Argentina" {
		t.Errorf("Romero = team %d %q, national %v %q", p.TeamIndex, p.Club, p.NationalTeamIndex, p.NationalTeam)
	}
	if p.SquadNumber != 0 {
		t.Errorf("Romero's national squad row has his club number %d", p.SquadNumber)
	}
	if club, _ := g.View(false).Squad(547); len(club) == 0 || club[0].UID != 2 || club[0].SquadNumber != 17 {
		t.Errorf("Squad(547) = %+v, want Romero with his club number 17 first", club)
	}
	depth, err := g.View(false).Depth(1233)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(depth[3].Players); !slices.Equal(got, []uint32{2}) {
		t.Errorf("Depth(1233) DC = %v, want [2]", got)
	}
}

func TestNationalTeamIgnoresRefsToMissingOrClubTeams(t *testing.T) {
	club := rawPlayer(1, "Club", "Ref")
	club.NationalTeamRef = 547
	none := rawPlayer(2, "No", "Ref")
	none.NationalTeamRef = -1
	unset := rawPlayer(3, "Zero", "Ref") // 0 is not a team in testWorld
	g := NewGame(Info{Date: gameDate}, []savefile.Player{club, none, unset}, testWorld, PlayerExtras{})
	v := g.View(false)

	for _, uid := range []uint32{1, 2, 3} {
		if p, _ := v.Profile(uid); p.NationalTeamIndex != nil || p.NationalTeam != "" {
			t.Errorf("player %d national team = %v %q, want none", uid, p.NationalTeamIndex, p.NationalTeam)
		}
	}
	if _, err := v.Squad(0); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("Squad(0) err = %v, want ErrInvalidFilter", err)
	}
	if squad, _ := v.Squad(547); !slices.Equal(uids(squad), []uint32{1, 2, 3}) {
		t.Errorf("Squad(547) = %v, want [1 2 3]", uids(squad))
	}
}

func TestSearchFilters(t *testing.T) {
	g := testGame()
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"position", Filter{Position: "DC", Gender: "men"}, []uint32{2, 3}},
		{"women", Filter{Position: "DC", Gender: "women"}, []uint32{6}},
		{"any gender", Filter{Position: "DC"}, []uint32{6, 2, 3}},
		{"age", Filter{MaxAge: 21, Gender: "men"}, []uint32{4, 3}},
		{"potential", Filter{MinPotentialAbility: 165, SortBy: "potential_ability", Gender: "men"}, []uint32{4, 2}},
		{"attribute", Filter{MinAttributes: map[string]int{"Aggression": 18}}, []uint32{2}},
		{"club name", Filter{Club: "tottenham", MaxCurrentAbility: 145}, []uint32{1, 4, 5}},
		{"club short name", Filter{Club: "spurs"}, []uint32{2, 1, 4, 5}},
		{"exclude club", Filter{ExcludeTeamIndex: ptr(int64(547)), Gender: "men"}, []uint32{3}},
		{"national team index", Filter{TeamIndex: ptr(int64(1233))}, []uint32{2}},
		{"exclude national team", Filter{ExcludeTeamIndex: ptr(int64(1233)), Gender: "men"}, []uint32{1, 4, 3, 5}},
		{"age ascending", Filter{Club: "Tottenham", SortBy: "age", Ascending: true, Limit: 2}, []uint32{4, 2}},
		{"nation part", Filter{Nation: "ENGL", Gender: "men"}, []uint32{2, 1, 4, 3, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := g.View(false).Search(tt.f)
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

func TestClubFilterLeavesOutNationalTeams(t *testing.T) {
	w := World{Teams: maps.Clone(testWorld.Teams), Nations: testWorld.Nations}
	w.Teams[77] = &Team{Index: 77, ClubIndex: 30, Name: "Universidad de Chile", Club: "Universidad de Chile", Reputation: 5000}
	w.Teams[78] = &Team{Index: 78, ClubIndex: noTeam, Name: "Chile", National: true, Reputation: 6000}
	w.Teams[79] = &Team{Index: 79, ClubIndex: noTeam, Name: "Chile U21", National: true, Reputation: 7000}
	w.Teams[80] = &Team{Index: 80, ClubIndex: noTeam, Name: "Chile Women", National: true, Women: true, Reputation: 8000}
	local := rawPlayer(1, "Club", "Player")
	local.TeamIndex = 77
	abroad := rawPlayer(2, "Called", "Up")
	abroad.NationalTeamRef = 78
	woman := rawPlayer(3, "Called", "Woman")
	woman.NationalTeamRef = 80
	g := NewGame(Info{Date: gameDate}, []savefile.Player{local, abroad, woman}, w, PlayerExtras{})

	res, err := g.View(false).Search(Filter{Club: "chile"})
	if err != nil || !slices.Equal(uids(res.Players), []uint32{1}) {
		t.Errorf("club chile = %v, %v; want only the club's player 1", uids(res.Players), err)
	}
	if got := teamIndexes(res.NationalTeams); !slices.Equal(got, []int64{80, 78}) {
		t.Errorf("club chile national teams = %v, want [80 78]: most reputable first, Chile U21 has no call-ups", got)
	}
	if res.NationalTeams[0] == w.Teams[80] {
		t.Error("Search returned the game's own Team; want a copy")
	}
	if res, _ := g.View(false).Search(Filter{Club: "chile", Gender: "men"}); !slices.Equal(teamIndexes(res.NationalTeams), []int64{78}) {
		t.Errorf("club chile, men: national teams = %v, want [78]", teamIndexes(res.NationalTeams))
	}
	if res, _ := g.View(false).Search(Filter{Club: "universidad"}); res.NationalTeams != nil {
		t.Errorf("club universidad national teams = %v, want none", teamIndexes(res.NationalTeams))
	}
	if _, err := g.View(false).Search(Filter{Club: "chile women"}); err == nil || !strings.Contains(err.Error(), "Chile Women (team_index 80)") {
		t.Errorf("club chile women: err = %v, want it to name Chile Women", err)
	}
	if _, err := g.View(false).Search(Filter{Club: "chile women", Gender: "men"}); err == nil || strings.Contains(err.Error(), "Chile Women") {
		t.Errorf("club chile women, men: err = %v, want an error without women's teams", err)
	}
	if res, err := g.View(false).Search(Filter{TeamIndex: ptr(int64(78))}); err != nil || !slices.Equal(uids(res.Players), []uint32{2}) {
		t.Errorf("national team index = %v, %v; want the called-up player 2", uids(res.Players), err)
	}
	_, err = testGame().View(false).Search(Filter{Club: "argentina"})
	if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "Argentina (team_index 1233)") {
		t.Errorf("club matching only a national team: err = %v, want an ErrInvalidFilter naming it with its team_index", err)
	}
}

func hintWorld() World {
	w := World{Teams: maps.Clone(testWorld.Teams), Nations: testWorld.Nations}
	w.Teams[91] = &Team{Index: 91, ClubIndex: 41, Name: "Chicago Magic PSG U18", Club: "Chicago Magic PSG", Reputation: 10}
	w.Teams[92] = &Team{Index: 92, ClubIndex: 41, Name: "Chicago Magic PSG U20", Club: "Chicago Magic PSG", Reputation: 20}
	w.Teams[93] = &Team{Index: 93, ClubIndex: 42, Name: "Paris Saint-Germain", Club: "Paris Saint-Germain", Reputation: 9000}
	w.Teams[94] = &Team{Index: 94, ClubIndex: 43, Name: "Chile Women Club", Club: "Chile Women Club", Women: true, Reputation: 500}
	return w
}

func hintGame() *Game {
	psg := rawPlayer(1, "Kylian", "Mbappe")
	psg.TeamIndex = 93
	woman := rawPlayer(2, "Maria", "Silva")
	woman.TeamIndex, woman.Female = 94, true
	return NewGame(Info{Date: gameDate}, []savefile.Player{psg, woman}, hintWorld(), PlayerExtras{})
}

func TestSearchHintsWhenClubMatchesOnlyTeamsWithoutPlayers(t *testing.T) {
	res, err := hintGame().View(false).Search(Filter{Club: "psg"})
	if err != nil || res.Total != 0 {
		t.Fatalf("club psg = %d, %v; want no players", res.Total, err)
	}
	for _, want := range []string{"psg", "no players", "Chicago Magic PSG U20 (team_index 92)", "Chicago Magic PSG U18 (team_index 91)", "find_clubs", "nicknames (PSG, Barça) are not in the save"} {
		if !strings.Contains(res.Hint, want) {
			t.Errorf("hint = %q, want it to contain %q", res.Hint, want)
		}
	}
	if strings.Index(res.Hint, "U20") > strings.Index(res.Hint, "U18") {
		t.Errorf("hint = %q, want the most reputable team first", res.Hint)
	}
}

func TestSearchHintCapsTheTeamsItNames(t *testing.T) {
	w := hintWorld()
	for i := range int64(5) {
		w.Teams[200+i] = &Team{Index: 200 + i, ClubIndex: 50, Name: fmt.Sprintf("Zed %d", i), Reputation: 1}
	}
	res, _ := NewGame(Info{Date: gameDate}, nil, w, PlayerExtras{}).View(false).Search(Filter{Club: "zed"})
	if strings.Count(res.Hint, "team_index") != maxErrorTeams || !strings.Contains(res.Hint, "2 more") {
		t.Errorf("hint = %q, want %d teams and the rest counted", res.Hint, maxErrorTeams)
	}
}

func TestSearchHintsWhenClubTeamsAreOfTheOtherGender(t *testing.T) {
	v := hintGame().View(false)
	res, err := v.Search(Filter{Club: "chile women", Gender: "men"})
	if err != nil || res.Total != 0 {
		t.Fatalf("chile women, men = %d, %v; want no players", res.Total, err)
	}
	for _, want := range []string{"women's teams", "Chile Women Club (team_index 94)", "gender is men"} {
		if !strings.Contains(res.Hint, want) {
			t.Errorf("hint = %q, want it to contain %q", res.Hint, want)
		}
	}
	if strings.Contains(res.Hint, "find_clubs") {
		t.Errorf("hint = %q, want no nickname advice for a gender mismatch", res.Hint)
	}
	res, _ = v.Search(Filter{Club: "paris", Gender: "women"})
	if !strings.Contains(res.Hint, "men's teams") || !strings.Contains(res.Hint, "gender is women") {
		t.Errorf("paris, women: hint = %q, want a men's teams hint", res.Hint)
	}
}

func TestSearchGivesNoHintWhenPlayersExistOrOtherFiltersExplain(t *testing.T) {
	v := hintGame().View(false)
	for name, f := range map[string]Filter{
		"players found":        {Club: "paris"},
		"gender any":           {Club: "chile women", Gender: "any"},
		"filtered out by age":  {Club: "paris", MinAge: 99},
		"no club filter":       {MinAge: 99},
		"matching gender, men": {Club: "paris", Gender: "men"},
	} {
		if res, err := v.Search(f); err != nil || res.Hint != "" {
			t.Errorf("%s: hint = %q, err = %v; want none", name, res.Hint, err)
		}
	}
}

func teamIndexes(teams []*Team) []int64 {
	var out []int64
	for _, t := range teams {
		out = append(out, t.Index)
	}
	return out
}

func TestSearchRejectsUnknownNames(t *testing.T) {
	g := testGame()
	for _, f := range []Filter{
		{Position: "XX"},
		{SortBy: "shoe_size"},
		{Gender: "robots"},
		{MinAttributes: map[string]int{"Speed": 10}},
		{Club: " - "},
		{Club: "totenham"},
		{Nation: "atlantis"},
	} {
		if _, err := g.View(false).Search(f); !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("Search(%+v) err = %v, want ErrInvalidFilter", f, err)
		}
	}
}

func TestClubErrorsNameTheField(t *testing.T) {
	for club, want := range map[string]string{" - ": `club " - "`, "totenham": "find_clubs"} {
		_, err := testGame().View(false).Search(Filter{Club: club})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Search(club %q) err = %v, want it to mention %s", club, err, want)
		}
	}
}

func TestUnknownNationErrorNamesTheField(t *testing.T) {
	_, err := testGame().View(false).Search(Filter{Nation: "atlantis"})

	if err == nil || !strings.Contains(err.Error(), `nation "atlantis"`) {
		t.Errorf("err = %v, want it to name the nation filter", err)
	}
	for _, want := range []string{"nation_index", "United States", "Netherlands", "Türkiye"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to suggest %s", err, want)
		}
	}
}

func TestNationFilterWithoutLettersIsInvalid(t *testing.T) {
	g := NewGame(Info{Date: gameDate}, nil, World{Nations: map[int64]string{0: "United States", 1: "Guinea-Bissau"}}, PlayerExtras{})
	for _, nation := range []string{" ", "-", " - "} {
		_, err := g.View(false).Search(Filter{Nation: nation})
		if !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "nation ") {
			t.Errorf("nation %q: err = %v, want an ErrInvalidFilter naming the nation", nation, err)
		}
	}
}

func TestNationFilterIgnoresPunctuation(t *testing.T) {
	bissau := rawPlayer(1, "Guinea", "Player")
	bissau.NationIndex = 1
	g := NewGame(Info{Date: gameDate}, []savefile.Player{bissau}, World{Nations: map[int64]string{0: "United States", 1: "Guinea-Bissau"}}, PlayerExtras{})
	for _, nation := range []string{"guinea bissau", "Guinea-Bissau", "guinea-bissau "} {
		if res, err := g.View(false).Search(Filter{Nation: nation, Gender: "any"}); err != nil || !slices.Equal(uids(res.Players), []uint32{1}) {
			t.Errorf("nation %q = %v, %v; want player 1", nation, uids(res.Players), err)
		}
	}
}

func TestNationFilterIgnoresSurroundingSpaces(t *testing.T) {
	res, err := testGame().View(false).Search(Filter{Nation: "  england ", Gender: "men"})

	if err != nil || len(res.Players) != 5 {
		t.Errorf("nation \"  england \" = %v, %v; want the 5 English men", uids(res.Players), err)
	}
}

func TestInvalidPositionListsTheCodes(t *testing.T) {
	_, err := testGame().View(false).Search(Filter{Position: "LB"})

	if err == nil || !strings.Contains(err.Error(), "GK SW DL DC DR DM ML MC MR AML AMC AMR ST WBL WBR") {
		t.Errorf("err = %v", err)
	}
}

func TestDepthListsCapablePlayersPerPosition(t *testing.T) {
	g := testGame()

	depth, err := g.View(false).Depth(547)
	if err != nil {
		t.Fatal(err)
	}

	if len(depth) != len(savefile.PositionNames) || depth[0].Position != "GK" || depth[3].Position != "DC" {
		t.Fatalf("depth positions = %+v", depth)
	}
	if got := uids(depth[0].Players); !slices.Equal(got, []uint32{1}) {
		t.Errorf("GK = %v, want [1]", got)
	}
	if got := uids(depth[3].Players); !slices.Equal(got, []uint32{2}) {
		t.Errorf("DC = %v, want [2] (Axel Romero plays elsewhere)", got)
	}
	if got := uids(depth[10].Players); !slices.Equal(got, []uint32{4, 5}) {
		t.Errorf("AMC = %v, want [4 5] by ability", got)
	}
}

func TestFindTeamsByName(t *testing.T) {
	g := testGame()

	for _, q := range []string{"tottenham", "Tottenham-Hotspur!", "spurs"} {
		got := findTeams(t, g.View(false), q, 10)

		if len(got) != 2 || got[0].Index != 547 || got[1].Index != 27845 {
			t.Errorf("FindTeams(%q) = %+v, want first team then U21", q, got)
		}
	}
	// "ham" is inside "Tottenham" but starts no word.
	if got := findTeams(t, g.View(false), "ham", 10); len(got) != 0 {
		t.Errorf("FindTeams(ham) = %+v, want none", got)
	}
	if team, ok := g.View(false).Team(20739); !ok || !team.Women {
		t.Errorf("Team(20739) = %+v, %v", team, ok)
	}
}

func TestSearchMoneyContractAndFitnessFilters(t *testing.T) {
	g := testGame()
	set := func(uid uint32, value, wage int64, endYear int, injured bool) {
		p := g.byUID[uid]
		p.Value = value
		p.Contract = &Contract{WeeklyWage: wage, End: time.Date(endYear, 6, 30, 0, 0, 0, 0, time.UTC)}
		if injured {
			p.Injuries = []InjuryInfo{{ID: 1}}
		}
	}
	set(1, 38_000_000, 68_000, 2028, false)
	set(2, 58_000_000, 200_000, 2026, true)
	set(4, 30_000_000, 40_000, 2026, false)
	axel := g.byUID[3]
	axel.Contract = nil // free agent

	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"max value", Filter{Gender: "men", MaxValue: 40_000_000, Club: "tottenham"}, []uint32{1, 4, 5}},
		{"max wage", Filter{Gender: "men", MaxWage: 70_000, Club: "tottenham"}, []uint32{1, 4}}, // 5 has no contract
		{"contract expiring", Filter{Gender: "men", ContractExpiresBy: "2026-12-31"}, []uint32{2, 4}},
		{"exclude injured", Filter{Gender: "men", Club: "tottenham", ExcludeInjured: true, MinCurrentAbility: 100}, []uint32{1, 4}},
		{"free agents", Filter{Gender: "men", FreeAgentsOnly: true}, []uint32{3}},
		{"sort by value", Filter{Gender: "men", Club: "tottenham", SortBy: "value", Limit: 2}, []uint32{2, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := g.View(false).Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Players
			if !slices.Equal(uids(got), tt.want) {
				t.Errorf("got %v, want %v", uids(got), tt.want)
			}
		})
	}
	if _, err := g.View(false).Search(Filter{ContractExpiresBy: "next summer"}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("bad date err = %v", err)
	}
}

func TestSearchAnyGenderCaseInsensitiveNamesAndTotal(t *testing.T) {
	g := testGame()

	res, err := g.View(false).Search(Filter{Position: "dc", Gender: "any", MinAttributes: map[string]int{"aggression": 1}, SortBy: "AGGRESSION", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	got, total := res.Players, res.Total

	if total != 3 || !slices.Equal(uids(got), []uint32{2, 3}) {
		t.Errorf("got %v (total %d), want [2 3] of 3", uids(got), total)
	}
}

func TestLoanedOutAndContractedTeamFilter(t *testing.T) {
	g := testGame()
	axel := g.byUID[3] // plays elsewhere on loan from Spurs
	axel.Contract = &Contract{TeamIndex: 547, WeeklyWage: 5000, End: time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC), SquadNumber: 69}
	axel.Loan = &Contract{TeamIndex: -1, SquadNumber: 14}
	axel.SquadNumber = 14
	vicario := g.byUID[1]
	vicario.Contract = &Contract{TeamIndex: 547}

	loaned, err := g.View(false).LoanedOut(547)
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(loaned); !slices.Equal(got, []uint32{3}) {
		t.Errorf("LoanedOut(547) = %v, want [3]", got)
	}
	// The parent club's squad-number screen shows the main contract's number.
	if loaned[0].SquadNumber != 69 {
		t.Errorf("LoanedOut(547) squad number %d, want the main contract's 69", loaned[0].SquadNumber)
	}
	if p, _ := g.View(false).Profile(3); p.SquadNumber != 14 {
		t.Errorf("profile squad number %d, want the loan club's 14", p.SquadNumber)
	}
	res, err := g.View(false).Search(Filter{ContractedTeamIndex: ptr(int64(547))})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Players
	if !slices.Equal(uids(got), []uint32{1, 3}) {
		t.Errorf("contracted to 547 = %v, want [1 3]", uids(got))
	}
}

func TestSearchValidatesAttributeMinimums(t *testing.T) {
	g := testGame()

	_, err := g.View(false).Search(Filter{MinAttributes: map[string]int{"Pace": 25}})
	if !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("Pace 25: err = %v, want ErrInvalidFilter", err)
	}
	_, err = g.View(false).Search(Filter{MinAttributes: map[string]int{"Speed": 10}})
	if err == nil || !strings.Contains(err.Error(), "Acceleration") {
		t.Errorf("unknown attribute error does not list valid names: %v", err)
	}
}

func TestExpectedReturnIsTheLatestInjury(t *testing.T) {
	p := PlayerBasics{Injuries: []InjuryInfo{
		{ReturnEarliest: time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)},
		{ReturnEarliest: time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)},
	}}

	if got := p.ExpectedReturn(); !got.Equal(time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ExpectedReturn = %s, want 2025-09-15", got)
	}
}

func TestSearchRejectsUnknownIndicesAndUnnamedNations(t *testing.T) {
	g := testGame()
	for name, f := range map[string]Filter{
		"team":            {TeamIndex: ptr(int64(424242))},
		"contracted team": {ContractedTeamIndex: ptr(int64(424242))},
		"excluded team":   {ExcludeTeamIndex: ptr(int64(424242))},
		"nation index":    {NationIndex: ptr(int64(9999))},
	} {
		if _, err := g.View(false).Search(f); !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("%s: err = %v, want ErrInvalidFilter", name, err)
		}
	}

	unnamed := NewGame(Info{Date: gameDate}, nil, World{}, PlayerExtras{})
	if _, err := unnamed.View(false).Search(Filter{Nation: "england"}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("nation filter without nation names: err = %v, want ErrInvalidFilter", err)
	}
}

func TestFindRejectsQueriesWithoutWords(t *testing.T) {
	v := testGame().View(false)

	for _, q := range []string{"", "   ", " - "} {
		if _, err := v.FindPlayers(q, 10); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "name ") {
			t.Errorf("FindPlayers(%q) error = %v, want an ErrInvalidFilter naming the name", q, err)
		}
		if _, err := v.FindTeams(q, 10); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "name ") {
			t.Errorf("FindTeams(%q) error = %v, want an ErrInvalidFilter naming the name", q, err)
		}
	}
}

func TestSearchDefaultsToTheManagersGender(t *testing.T) {
	g := testGame()
	g.Info.ManagerTeamIndex = 547
	g.byUID[6].Contract = &Contract{TeamIndex: 20739}
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"manager's gender", Filter{Position: "DC"}, []uint32{2, 3}},
		{"any", Filter{Position: "DC", Gender: "any"}, []uint32{6, 2, 3}},
		{"team filter", Filter{Position: "DC", TeamIndex: ptr(int64(20739))}, []uint32{6}},
		{"contracted team filter", Filter{Position: "DC", ContractedTeamIndex: ptr(int64(20739))}, []uint32{6}},
		{"club filter", Filter{Position: "DC", Club: "chelsea"}, []uint32{6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := g.View(false).Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if got := uids(res.Players); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	g.Info.ManagerTeamIndex = 20739
	res, err := g.View(false).Search(Filter{Position: "DC"})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{6}) {
		t.Errorf("women's manager = %v, want [6]", got)
	}

	g.Info.ManagerTeamIndex = -1
	res, err = g.View(false).Search(Filter{Position: "DC"})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{6, 2, 3}) {
		t.Errorf("unknown manager team = %v, want both genders", got)
	}
}

func findPlayers(t *testing.T, v View, query string, limit int) []Profile {
	t.Helper()
	ps, err := v.FindPlayers(query, limit)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func findTeams(t *testing.T, v View, query string, limit int) []*Team {
	t.Helper()
	ts, err := v.FindTeams(query, limit)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestNewGameArrangedMove(t *testing.T) {
	// 8 is arranged for another person, 9 is his but not arranged, 7 is the move.
	linked := savefile.Player{Index: 10, UID: 1, TransferIDs: []uint32{8, 9, 7}}
	none := savefile.Player{Index: 11, UID: 2}
	transfers := []savefile.Transfer{
		{ID: 7, Person: 10, Status: savefile.TransferArranged, Team: 855, Date: savefile.Date{Raw: 0xb7, Year: 2036}},
		{ID: 8, Person: 99, Status: savefile.TransferArranged, Team: 1, Date: savefile.Date{Raw: 1, Year: 2036}},
		{ID: 9, Person: 10, Status: 15, Team: 2, Date: savefile.Date{Raw: 2, Year: 2036}},
	}
	g := NewGame(Info{Date: gameDate}, []savefile.Player{linked, none}, testWorld, PlayerExtras{Transfers: transfers})
	want := time.Date(2036, 7, 1, 0, 0, 0, 0, time.UTC) // day 183
	if a := g.aux[0].interest.arranged; a == nil || a.team != 855 || !a.date.Equal(want) {
		t.Errorf("arranged = %+v, want team 855 on %s", a, want.Format(time.DateOnly))
	}
	if a := g.aux[1].interest.arranged; a != nil {
		t.Errorf("arranged = %+v without transfers, want nil", a)
	}
}

func TestNewGameInterestInputs(t *testing.T) {
	events := []savefile.ContractEvent{{Type: 12, ID: 650}}
	c := &savefile.Contract{
		Full: true, Kind: 2, TeamIndex: 20, End: savefile.Date{Raw: 180, Year: 2038}, Signed: savefile.Date{Raw: 10, Year: 2035},
		PlayingTime: 3, OriginalPlayingTime: 4, PlayingTimeScore: 70, RecentPlayingTimeScore: -5, CoreFlags: 8, Events: events,
		WeeklyWage: 9000, Clauses: []savefile.Clause{{Type: 55, Value: 1000}, {Type: 54}},
		Bonuses: []savefile.CompetitionBonus{{Value: 5}},
	}
	raw := savefile.Player{Index: 3, UID: 3, PreviousClub: 42, Retiring: true, OnlyOneClub: true, Contract: c, Hidden: [8]uint8{0, 0, 17}}
	free := savefile.Player{Index: 4, UID: 4}
	g := NewGame(Info{Date: gameDate}, []savefile.Player{raw, free}, testWorld, PlayerExtras{})
	in := g.aux[0].interest
	if in.previousClub != 42 || !in.retiring || !in.onlyOneClub || in.loyalty != 17 || in.arranged != nil {
		t.Errorf("interest = %+v", in)
	}
	want := interestContract{
		TeamIndex: 20, Kind: 2, End: c.End, Signed: c.Signed, PlayingTime: 3, OriginalPlayingTime: 4,
		PlayingTimeScore: 70, RecentPlayingTimeScore: -5, CoreFlags: 8, Events: events, ClauseTypes: []uint8{55, 54},
	}
	if in.main == nil || !reflect.DeepEqual(*in.main, want) {
		t.Errorf("main = %+v, want %+v", in.main, want)
	}
	if g.aux[1].interest.main != nil {
		t.Errorf("free agent main = %+v, want nil", g.aux[1].interest.main)
	}
}

func TestSearchMinMonthsAtClub(t *testing.T) {
	g := testGame()
	joined := func(uid uint32, d time.Time) { g.byUID[uid].JoinedClub = &d }
	joined(1, time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC)) // exactly 6 months before the game date
	joined(2, time.Date(2025, 1, 8, 0, 0, 0, 0, time.UTC)) // a day short of 6 months
	joined(5, time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC)) // 5 years
	res, err := g.View(false).Search(Filter{Club: "tottenham", MinMonthsAtClub: 6})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{1, 5}) {
		t.Errorf("got %v, want [1 5]: 2 joined too late, 4 has no joined date", got)
	}
	if _, err := g.View(false).Search(Filter{MinMonthsAtClub: -1}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("negative months err = %v", err)
	}
}
