package fm

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/fmprefs"
	"github.com/Napas/fm-mcp/internal/savefile"
)

const testSavePath = "../../data/test_save.fm"

func loadTestSave(t *testing.T) *Game {
	t.Helper()
	if _, err := os.Stat(testSavePath); err != nil {
		t.Skipf("test save not available: %v", err)
	}
	g, err := Load(testSavePath, LoadOptions{GameDBDir: gameDBDir()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

// loadRealSave loads the copy of a real save named by env, skipping when env
// is unset and failing when the save's game date is not date. Never point env
// at the game's own save folder.
func loadRealSave(t *testing.T, env string, date time.Time) *Game {
	t.Helper()
	return loadRealSaveWith(t, env, date, LoadOptions{})
}

// loadRealSaveNamed is loadRealSave with the game's names, skipping when the
// game is not installed.
func loadRealSaveNamed(t *testing.T, env string, date time.Time) *Game {
	t.Helper()
	if gameDBDir() == "" {
		t.Skip("game not installed")
	}
	return loadRealSaveWith(t, env, date, LoadOptions{GameDBDir: gameDBDir()})
}

func loadRealSaveWith(t *testing.T, env string, date time.Time, opts LoadOptions) *Game {
	t.Helper()
	path := os.Getenv(env)
	if path == "" {
		t.Skipf("%s not set", env)
	}
	g, err := Load(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Info.Date.Equal(date) {
		t.Fatalf("%s: game date %s, want %s", env, g.Info.Date.Format(time.DateOnly), date.Format(time.DateOnly))
	}
	return g
}

func TestLoadTestSave(t *testing.T) {
	g := loadTestSave(t)

	if !g.Info.Date.Equal(gameDate) || g.Info.SaveName != "test_save" || g.Info.ManagerName != "Napas Mačiulaitis" || g.Info.Build != "26.2.0+2276383" {
		t.Errorf("Info = %+v", g.Info)
	}
	if g.PlayerCount() != 37011 {
		t.Errorf("PlayerCount = %d, want 37011", g.PlayerCount())
	}
	found := findPlayers(t, g.View(false), "james maddison", 1)
	if len(found) != 1 || found[0].Age != 28 || *found[0].CurrentAbility != 151 {
		t.Fatalf("FindPlayers(james maddison) = %+v", found)
	}
	if gameDBDir() != "" && found[0].Nation != "England" {
		t.Errorf("Maddison's nation = %q, want England", found[0].Nation)
	}
	if in := found[0].Injuries; gameDBDir() != "" && (len(in) != 1 || in[0].Name != "torn anterior cruciate ligament") {
		t.Errorf("Maddison's injuries = %+v, want a torn anterior cruciate ligament", in)
	}
	squad, err := g.View(false).Squad(found[0].TeamIndex)
	if err != nil {
		t.Fatal(err)
	}
	if len(squad) != 27 {
		t.Errorf("Maddison's squad has %d players, want 27", len(squad))
	}
}

// gameDBDir is the installed game's database folder, or "" without the game.
func gameDBDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Football Manager 26",
		"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db")
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

func TestLoadWithoutGameWarns(t *testing.T) {
	if _, err := os.Stat(testSavePath); err != nil {
		t.Skip(err)
	}
	g, err := Load(testSavePath, LoadOptions{GameDBDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	names := slices.DeleteFunc(slices.Clone(g.Info.Warnings), func(w string) bool { return !strings.Contains(w, "names unavailable") })
	onlyWarning(t, names, "nation, competition and injury names unavailable")
	if p := findPlayers(t, g.View(false), "james maddison", 1); len(p) != 1 || p[0].Nation != "" {
		t.Errorf("player = %+v", p)
	}
}

func TestLoadResolvesTeamsAndManager(t *testing.T) {
	g := loadTestSave(t)

	if g.Info.ManagerTeamIndex != 547 || g.Info.ManagerClub != "Tottenham Hotspur" {
		t.Errorf("manager = %d %q, want 547 Tottenham Hotspur", g.Info.ManagerTeamIndex, g.Info.ManagerClub)
	}
	m := findPlayers(t, g.View(false), "james maddison", 1)[0]
	if m.Club != "Tottenham Hotspur" || m.Female {
		t.Errorf("Maddison club = %q female = %v", m.Club, m.Female)
	}
	if th := findPlayers(t, g.View(false), "alyssa thompson", 1); len(th) != 1 || th[0].Club != "Chelsea Women" || !th[0].Female {
		t.Errorf("Thompson = %+v", th)
	}
	team, _ := g.View(false).Team(547)
	if gameDBDir() != "" && team.Division != "Premier League" {
		t.Errorf("Spurs division = %q", team.Division)
	}
	if u21 := findTeams(t, g.View(false), "tottenham hotspur u21", 1); len(u21) != 1 || u21[0].Name != "Tottenham Hotspur U21" {
		t.Errorf("U21 = %+v", u21)
	}
}

func TestLoadWarnsAboutUnverifiedBuilds(t *testing.T) {
	if w := buildWarning(3733); w != "" {
		t.Errorf("buildWarning(3733) = %q, want none", w)
	}
	if w := buildWarning(3740); w == "" {
		t.Error("buildWarning(3740) is empty")
	}
}

func TestBuildWorldNamesWomensTeams(t *testing.T) {
	db := &savefile.GameDB{
		NationCount: 251,
		Clubs:       []savefile.Club{{Index: 0, UID: 9, Name: "Chelsea", Women: true}},
		Teams: []savefile.Team{
			{Index: 0, National: true, Container: 139, AgeLimit: 100},
			{Index: 1, National: true, Container: 390, AgeLimit: 100},               // 390 - 251 = 139
			{Index: 2, National: true, Container: 139, AgeLimit: 255, TeamType: 33}, // an unmapped national side
			{Index: 3, UID: 9, Container: 0, AgeLimit: 100},
		},
	}

	w := buildWorld(db, langNames{nations: map[int64]string{139: "England"}})

	if got := w.Teams[1]; got.Name != "England Women" || got.Nation != "England" || !got.Women {
		t.Errorf("women's national team = %+v", got)
	}
	if got := w.Teams[0].Name; got != "England" {
		t.Errorf("men's national team = %q", got)
	}
	if got := w.Teams[2].Name; got == "England" {
		t.Errorf("unmapped national side shares the senior team's name %q", got)
	}
	if got := w.Teams[3].Name; got != "Chelsea Women" {
		t.Errorf("women's club = %q", got)
	}
}

// Favoured clubs show a club's short name (in game: "Osijek" for NK Osijek).
func TestBuildWorldNamesClubsByShortName(t *testing.T) {
	db := &savefile.GameDB{
		Clubs: []savefile.Club{
			{Index: 0, UID: 9, Name: "NK Osijek", ShortName: "Osijek"},
			{Index: 1, UID: 10, Name: "Chelsea", ShortName: "Chelsea", Women: true},
			{Index: 2, UID: 11, Name: "Svendborg forenede Boldklubber", ShortName: "Svendborg fB"},
			{Index: 3, UID: 12, Name: "Long Name Only"},
			{Index: 4, UID: 13},
		},
		Teams: []savefile.Team{
			{Index: 0, UID: 900, Container: 0, TeamType: 12, AgeLimit: 100},
			{Index: 1, UID: 9, Container: 0, AgeLimit: 100},
			{Index: 2, UID: 10, Container: 1, AgeLimit: 100},
		},
	}

	w := buildWorld(db, langNames{})

	want := map[int64]Club{
		0: {Name: "Osijek", FirstTeam: 1},
		1: {Name: "Chelsea Women", FirstTeam: 2},
		2: {Name: "Svendborg fB", FirstTeam: noTeam},
		3: {Name: "Long Name Only", FirstTeam: noTeam},
		4: {Name: "Unnamed club", FirstTeam: noTeam},
	}
	if !maps.Equal(w.Clubs, want) {
		t.Errorf("Clubs = %+v, want %+v", w.Clubs, want)
	}
}

func TestBuildWorldNamesCompetitionsByIndex(t *testing.T) {
	db := &savefile.GameDB{CompetitionUIDs: []uint32{16, 1301394}}

	w := buildWorld(db, langNames{competitions: map[uint32]string{1301394: "UEFA Champions League"}})

	if want := map[int64]string{1: "UEFA Champions League"}; !maps.Equal(w.Competitions, want) {
		t.Errorf("Competitions = %v, want %v", w.Competitions, want)
	}
}

func TestBuildWorldNationalTeamsWithoutLanguageDB(t *testing.T) {
	db := &savefile.GameDB{
		NationCount: 251,
		Teams: []savefile.Team{
			{Index: 0, National: true, Container: 139, AgeLimit: 100},
			{Index: 1, National: true, Container: 300, AgeLimit: 100},
		},
	}

	w := buildWorld(db, langNames{})

	if men := w.Teams[0]; men.Women || men.Name != "National team" {
		t.Errorf("men's national team = %+v", men)
	}
	if women := w.Teams[1]; !women.Women {
		t.Errorf("women's national team = %+v", women)
	}
}

func TestBuildWorldGivesFinancesOnlyToTheFirstTeam(t *testing.T) {
	db := &savefile.GameDB{
		NationCount: 251,
		Clubs: []savefile.Club{{Index: 0, UID: 728, Name: "Tottenham Hotspur", Balance: 242_466_987,
			Budgets: &savefile.Budgets{Transfer: 40_000_000, WeeklyWage: 3_770_160}}},
		Teams: []savefile.Team{
			{Index: 0, UID: 728, Container: 0, AgeLimit: 100},
			{Index: 1, UID: 9001, Container: 0, TeamType: 12, AgeLimit: 100}, // a youth team without an age limit
		},
	}

	w := buildWorld(db, langNames{})

	if w.Teams[0].BankBalance != 242_466_987 || w.Teams[0].Name != "Tottenham Hotspur" {
		t.Errorf("first team = %+v", w.Teams[0])
	}
	if want := (Budgets{TransferBudget: 40_000_000, WageBudget: 3_770_160}); w.Teams[0].Budgets == nil || *w.Teams[0].Budgets != want {
		t.Errorf("first team budgets = %+v, want %+v", w.Teams[0].Budgets, want)
	}
	if w.Teams[1].BankBalance != 0 || w.Teams[1].Budgets != nil {
		t.Errorf("youth team inherited the club's finances: %+v", w.Teams[1])
	}
	if w.Teams[0].ClubIndex != 0 || w.Teams[1].ClubIndex != 0 {
		t.Errorf("club indices = %d %d, want 0 0", w.Teams[0].ClubIndex, w.Teams[1].ClubIndex)
	}
}

func TestBuildWorldReadsSmallTransferBudgetsAsZero(t *testing.T) {
	// The engine's getter reads £1–499 as 0 and keeps negative budgets.
	for stored, want := range map[int32]int64{-250: -250, 0: 0, 1: 0, 499: 0, 500: 500} {
		db := &savefile.GameDB{
			Clubs: []savefile.Club{{Index: 0, UID: 844, Name: "FC Metz", Budgets: &savefile.Budgets{Transfer: stored}}},
			Teams: []savefile.Team{{Index: 0, UID: 844, Container: 0, AgeLimit: 100}},
		}

		b := buildWorld(db, langNames{}).Teams[0].Budgets

		if b.TransferBudget != want {
			t.Errorf("stored %d: transfer budget = %d, want %d", stored, b.TransferBudget, want)
		}
	}
}

// TestRealSave14DecBudgets checks FC Metz's budgets against the Recruitment
// Budgets screen on 2035-12-14 (FM_REAL_SAVE_14DEC): transfer budget €9.63M,
// wage budget €16.43M p/a available plus €47.93M p/a spent. FM appears to truncate amounts in M to two decimals. Never point it
// at the game's save folder.
func TestRealSave14DecBudgets(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	g.Info.Money = NewMoney(euro, fmprefs.PerYear)

	b := g.View(true).Budgets()

	// £8 329 074 and £1 069 817 a week.
	if want := (Budgets{TransferBudget: 9_637_825, WageBudget: 64_371_729}); b == nil || *b != want {
		t.Errorf("budgets = %+v, want %+v", b, want)
	}
}

func TestBuildWorldGivesNationalTeamsNoClub(t *testing.T) {
	db := &savefile.GameDB{NationCount: 251, Teams: []savefile.Team{{Index: 0, National: true, Container: 139, AgeLimit: 100}}}

	if got := buildWorld(db, langNames{}).Teams[0].ClubIndex; got != noTeam {
		t.Errorf("national team ClubIndex = %d, want %d", got, noTeam)
	}
}

func TestStoredKnowledgeIsTheHumansEntry(t *testing.T) {
	fog := savefile.FogOfWar{Humans: []savefile.HumanKnowledge{
		{HumanIndex: 7, Players: map[uint32]uint8{1: 60}},
		{HumanIndex: 88351, Players: map[uint32]uint8{2: 100}},
	}}

	got, err := storedKnowledge(fog, 88351)
	if err != nil || got[2] != 100 || len(got) != 1 {
		t.Errorf("storedKnowledge = %v, %v; want person 2 at 100", got, err)
	}
	if _, err := storedKnowledge(fog, 5); err == nil {
		t.Error("a human without an entry is not an error")
	}
}

func TestLoadReadsScoutingKnowledge(t *testing.T) {
	g := loadTestSave(t)
	v := g.View(true)

	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "scouting") {
			t.Errorf("warning %q", w)
		}
	}
	maddison := findPlayers(t, v, "james maddison", 1)[0]
	if maddison.Scouting != ScoutingFull || maddison.CurrentAbility == nil {
		t.Errorf("own player Maddison = %s, CA %v", maddison.Scouting, maddison.CurrentAbility)
	}
	// The test save stores knowledge of 73 players; some of them are known
	// better than their reputation alone gives.
	scouted := 0
	for _, p := range g.players {
		if k := int(p.knowledge); k > baseKnowledge(p.Reputation.Current) && k < fullKnowledge {
			scouted++
		}
	}
	if scouted == 0 {
		t.Error("no player is known above his reputation baseline")
	}
}

func TestBuildWorldNamesNamelessClubs(t *testing.T) {
	db := &savefile.GameDB{NationCount: 251, Clubs: []savefile.Club{{Index: 0, UID: 1}}, Teams: []savefile.Team{{Index: 0, UID: 2, AgeLimit: 100}}}

	w := buildWorld(db, langNames{})
	if got := w.Teams[0].Name; got != "Unnamed club (other team)" {
		t.Errorf("name = %q", got)
	}
	if got := w.Clubs[0].Name; got != "Unnamed club" {
		t.Errorf("club name = %q", got)
	}
}

func TestLoadReadsSeasonStats(t *testing.T) {
	g := loadTestSave(t)

	c := findPlayers(t, g.View(true), "edinson cavani", 1)
	want := &SeasonStats{Starts: 28, Goals: 16, Assists: 5, AverageRating: ptr(6.84), RedCards: 1, Minutes: 2474, PlayerOfTheMatch: 1}
	if len(c) != 1 || !reflect.DeepEqual(c[0].Season, want) {
		t.Errorf("Cavani season = %+v, want %+v", c, want)
	}
}

type fakeLang struct{ nationsErr, competitionsErr, injuriesErr, stagesErr error }

func (f fakeLang) Nations() ([]savefile.LangName, error) {
	return []savefile.LangName{{UID: 765, Name: "England"}}, f.nationsErr
}

func (f fakeLang) Competitions() ([]savefile.LangName, error) {
	return []savefile.LangName{{UID: 11, Name: "Premier League"}}, f.competitionsErr
}

func (f fakeLang) Injuries() ([]savefile.LangName, error) {
	return []savefile.LangName{{UID: 100, Name: "bruised ankle"}}, f.injuriesErr
}

func (f fakeLang) Stages() ([]savefile.LangName, error) {
	return []savefile.LangName{{UID: 238, Name: "Playoffs"}}, f.stagesErr
}

func TestMissingStageNamesAreAWarning(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{stagesErr: errors.New("no 8_23.dat")}, 1, 1)

	onlyWarning(t, warnings, "stage names unavailable")
	if lang.stages != nil || lang.nations[0] != "England" {
		t.Errorf("lang = %+v, want nations without stages", lang)
	}
}

func TestStageNamesByUID(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{}, 1, 1)

	if len(warnings) != 0 || lang.stages[238] != "Playoffs" {
		t.Errorf("stages = %v, warnings %q", lang.stages, warnings)
	}
}

func TestLoadStageNames(t *testing.T) {
	if gameDBDir() == "" {
		t.Skip("game not installed")
	}
	g := loadTestSave(t)

	if got := g.world.Stages[236]; got != "Playoffs" {
		t.Errorf("stage 236 = %q, want Playoffs", got)
	}
	if len(g.world.Keys) != 3360 {
		t.Errorf("%d competition keys, want 3360", len(g.world.Keys))
	}
	if team := g.world.Teams[547]; team == nil || g.world.Competitions[team.divisionIndex] != "Premier League" {
		t.Errorf("Tottenham's division index does not name the Premier League")
	}
}

// onlyWarning fails unless warnings is one warning containing want.
func onlyWarning(t *testing.T, warnings []string, want string) {
	t.Helper()
	if len(warnings) != 1 || !strings.Contains(warnings[0], want) {
		t.Errorf("warnings = %q, want one about %s", warnings, want)
	}
}

func TestMissingInjuryNamesKeepNationAndCompetitionNames(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{injuriesErr: errors.New("no 8_6.dat")}, 1, 1)

	onlyWarning(t, warnings, "injury names unavailable")
	if lang.nations[0] != "England" || lang.competitions[11] != "Premier League" || lang.injuries != nil {
		t.Errorf("lang = %+v, want nations and competitions without injuries", lang)
	}
}

func TestInjuryCountMismatchKeepsNationNames(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{}, 1, 2)

	onlyWarning(t, warnings, "injury names unavailable")
	if lang.nations[0] != "England" {
		t.Errorf("lang = %+v, want nations kept", lang)
	}
}

func TestMissingNationNamesKeepCompetitionAndInjuryNames(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{nationsErr: errors.New("no 8_9.dat")}, 1, 1)

	onlyWarning(t, warnings, "nation names unavailable")
	if lang.nations != nil || lang.competitions[11] != "Premier League" || lang.injuries[0] != "bruised ankle" {
		t.Errorf("lang = %+v, want competitions and injuries without nations", lang)
	}
}

func TestMissingCompetitionNamesKeepNationAndInjuryNames(t *testing.T) {
	lang, warnings := langNamesFrom(fakeLang{competitionsErr: errors.New("no 8_25.dat")}, 1, 1)

	onlyWarning(t, warnings, "competition names unavailable")
	if lang.nations[0] != "England" || lang.competitions != nil || lang.injuries[0] != "bruised ankle" {
		t.Errorf("lang = %+v, want nations and injuries without competitions", lang)
	}
}

func TestEachMissingNameTableIsItsOwnWarning(t *testing.T) {
	_, warnings := langNamesFrom(fakeLang{nationsErr: errors.New("a"), competitionsErr: errors.New("b"), injuriesErr: errors.New("c"), stagesErr: errors.New("d")}, 1, 1)

	if len(warnings) != 4 {
		t.Errorf("warnings = %q, want 4", warnings)
	}
	for _, w := range warnings {
		if strings.Contains(w, "\n") {
			t.Errorf("warning %q spans lines", w)
		}
	}
}

// realSavePlayers lists players to check in the real save named by env.
type realSavePlayers[W any] struct {
	env     string
	date    time.Time
	players map[uint32]W
}

// checkRealSavePlayers loads each real save (see loadRealSave) and calls
// check on each listed player; a uid missing from the save is an error.
func checkRealSavePlayers[W any](t *testing.T, saves []realSavePlayers[W], check func(*testing.T, *Player, W)) {
	t.Helper()
	for _, s := range saves {
		t.Run(s.env, func(t *testing.T) {
			g := loadRealSave(t, s.env, s.date)
			for uid, w := range s.players {
				p, ok := g.byUID[uid]
				if !ok {
					t.Errorf("uid %d not in the save", uid)
					continue
				}
				check(t, p, w)
			}
		})
	}
}

// TestRealSavesLoyaltyBonusRemaining checks four Metz players' remaining
// loyalty bonus against the contract screens, which show it rounded to the
// nearest €25K: Ashby €350K and Sami €825K on 2035-12-02 (FM_REAL_SAVE),
// Bernardi €725K and Leroux €625K on 2035-12-09 (FM_REAL_SAVE_9DEC). Never
// point these at the game's own save folder.
func TestRealSavesLoyaltyBonusRemaining(t *testing.T) {
	checkRealSavePlayers(t, []realSavePlayers[int64]{ // GBP by uid
		{"FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC), map[uint32]int64{
			2002078397: 308_066, // Chris Ashby: 2 299 a week × 134 weeks
			2002114753: 707_336, // Bruno Sami: 2 972 × 238
		}},
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), map[uint32]int64{
			2002100552: 622_155, // Carlo Bernardi: 3 363 × 185
			2000191040: 534_394, // Louis Leroux: 4 018 × 133
		}},
	}, func(t *testing.T, p *Player, want int64) {
		if p.LoyaltyBonusRemaining != want {
			t.Errorf("%s: remaining loyalty bonus £%d, want £%d", p.Name, p.LoyaltyBonusRemaining, want)
		}
	})
}

// TestRealSavesPersonalityAndTraits checks five players: Ashby, Sami and
// Livramento (FM_REAL_SAVE, 2035-12-02), Bernardi and Leroux
// (FM_REAL_SAVE_9DEC, 2035-12-09). All five personality labels are from their
// profile screens. Ashby's traits are from the game; the others' come from the
// decoded trait masks. Never point these at the game's own save folder.
func TestRealSavesPersonalityAndTraits(t *testing.T) {
	type want struct {
		personality string
		traits      []string
	}
	checkRealSavePlayers(t, []realSavePlayers[want]{
		{"FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002078397: {"Fairly Professional", []string{"Marks Opponent Tightly"}},                                 // Chris Ashby
			2002114753: {"Driven", []string{"Uses Long Throw To Start Counter Attacks"}},                            // Bruno Sami
			28124569:   {"Light-Hearted", []string{"Runs With Ball Down Right", "Refrains From Taking Long Shots"}}, // Tino Livramento
		}},
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002100552: {"Light-Hearted", []string{"Marks Opponent Tightly", "Avoids Using Weaker Foot"}}, // Carlo Bernardi
			2000191040: {"Fairly Determined", []string{"Tries Killer Balls Often", "Plays One-Twos", "Dictates Tempo", // Louis Leroux
				"Looks For Pass Rather Than Attempting To Score", "Likes To Switch Ball To Wide Areas"}},
		}},
	}, func(t *testing.T, p *Player, w want) {
		if got := p.personality(); got != w.personality {
			t.Errorf("%s: personality %q, want %q", p.Name, got, w.personality)
		}
		if got := traitNames(p.traits); !slices.Equal(got, w.traits) || len(got) == 0 {
			t.Errorf("%s: traits %q, want %q", p.Name, got, w.traits)
		}
	})
}

// TestRealSavesDevoted checks the three players the personality rule labels
// Very Loyal or Devoted (rule applied to the bytes; not seen in game):
// Yousefi and Goodwin have affinity 100 for their club, Bondarenko is a free
// agent. Never point these at the game's own save folder.
func TestRealSavesDevoted(t *testing.T) {
	want := map[uint32]string{
		2002168088: "Devoted",    // Yaser Yousefi, Persepolis
		2002181080: "Devoted",    // Shane Goodwin, Columbus Crew
		71101325:   "Very Loyal", // Artem Bondarenko
	}
	checkRealSavePlayers(t, []realSavePlayers[string]{
		{"FM_REAL_SAVE", time.Date(2035, 12, 2, 0, 0, 0, 0, time.UTC), want},
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), want},
		{"FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC), want},
	}, func(t *testing.T, p *Player, want string) {
		if got := p.personality(); got != want {
			t.Errorf("%s: personality %q, want %q", p.Name, got, want)
		}
	})
}

// TestRealSavesPlayingTimeAndSquadNumber checks Metz players' playing time
// (squad list), actual playing time (contract screen headline) and squad
// number. In game: Bernardi's and Assongo's contract screens and the squad
// number screen on 2035-12-14 (FM_REAL_SAVE_14DEC), Bernardi's and Leroux's
// contract screens on 2035-12-09 (FM_REAL_SAVE_9DEC). Rows marked "rule" are
// the rule's predictions, not checked in game. Lobede's is left out: at 25
// the game's youth test may change it. Never point these at the game's own
// save folder.
func TestRealSavesPlayingTimeAndSquadNumber(t *testing.T) {
	type want struct {
		playingTime, original, actual string
		squadNumber                   int
	}
	checkRealSavePlayers(t, []realSavePlayers[want]{
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002100552: {"Regular Starter", "Important Player", "Important Player", 19},     // Carlo Bernardi, score 87
			2000191040: {"Regular Starter", "Important Player", "Important Player", 41},     // Louis Leroux, 88
			2000478936: {"Squad Player", "Important Player", "Surplus to Requirements", 28}, // Ilan Assongo, -12 (rule)
		}},
		{"FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002100552: {"Regular Starter", "Important Player", "Important Player", 19},       // Carlo Bernardi, score 88
			2000478936: {"Squad Player", "Important Player", "Surplus to Requirements", 28},   // Ilan Assongo, -15
			2000191040: {"Regular Starter", "Important Player", "Important Player", 41},       // Louis Leroux, 86 (rule)
			2002078397: {"Surplus to Requirements", "Important Player", "Regular Starter", 4}, // Chris Ashby, 80 (rule)
			2002095003: {"Star Player", "Squad Player", "Regular Starter", 16},                // Luke Reddy, 81 (rule)
			2002078767: {"Squad Player", "Squad Player", "Important Player", 1},               // Stephen Ly, 100 (rule)
			2000245491: {"Squad Player", "Squad Player", "", 24},                              // Jesús Lobede, 25: left out
		}},
	}, func(t *testing.T, p *Player, w want) {
		got := want{playingTimeName(p.playingTime), playingTimeName(p.originalPlayingTime), playingTimeName(p.actualPlayingTime), p.SquadNumber}
		if got != w {
			t.Errorf("%s: %+v, want %+v", p.Name, got, w)
		}
	})
}

// TestRealSaveLoaneesShowTheParentClubsSquadNumber checks Le Ven, on loan
// from Metz on 2035-12-14 (FM_REAL_SAVE_14DEC): Metz's squad-number screen
// shows 69 (his main contract), his profile the loan contract's 14. Never
// point this at the game's own save folder.
func TestRealSaveLoaneesShowTheParentClubsSquadNumber(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	const leVen = 2002275982
	loaned, err := g.View(false).LoanedOut(652)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(loaned, func(p Profile) bool { return p.UID == leVen })
	if i < 0 || loaned[i].SquadNumber != 69 {
		t.Errorf("Metz out on loan: Le Ven at %d, want squad number 69", i)
	}
	if p, _ := g.View(false).Profile(leVen); p.SquadNumber != 14 {
		t.Errorf("Le Ven's profile squad number %d, want 14", p.SquadNumber)
	}
}

// TestRealSaveReserveSquadLeavesOutFirstTeamNumbers checks FC Metz 2 on
// 2035-12-14 (FM_REAL_SAVE_14DEC): its players are contracted to Metz's first
// team, so their squad numbers (19 of 32 have one) are left out of its rows.
// Never point this at the game's own save folder.
func TestRealSaveReserveSquadLeavesOutFirstTeamNumbers(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	squad, err := g.View(false).Squad(19023)
	if err != nil {
		t.Fatal(err)
	}
	numbered := 0
	for _, p := range squad {
		if p.SquadNumber != 0 {
			t.Errorf("FC Metz 2 row %s has squad number %d", p.Name, p.SquadNumber)
		}
		if g.byUID[p.UID].SquadNumber != 0 {
			numbered++
		}
	}
	if len(squad) != 32 || numbered != 19 {
		t.Errorf("FC Metz 2: %d rows, %d with a first-team number; want 32 and 19", len(squad), numbered)
	}
}

// TestRealSaveFavouredClubs checks four profiles seen in game on 2035-12-14
// (FM_REAL_SAVE_14DEC). Matković: Osijek as a player, R. Madrid as a
// supporter. Čuber Potočnik: Maribor as a player (value 50). Leroux and
// Lobede: none (Leroux's values are 40 and 8). Never point this at the game's
// own save folder.
func TestRealSaveFavouredClubs(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC))
	checkFavouredClubs(t, g, map[uint32][]string{
		2000301456: {"R. Madrid supporter", "Osijek player"}, // Anton Matković
		2000196612: {"Maribor player"},                       // Jaka Čuber Potočnik
		2000191040: nil,                                      // Louis Leroux
		2000245491: nil,                                      // Jesús Lobede
	})
}

// TestTestSaveFavouredClubs checks favoured clubs and Devoted in the test
// save (bytes, not seen in game): Matković, then at NK Osijek, supports
// R. Madrid; Čuber Potočnik has none; Necip Uysal's 100 for Beşiktaş, his
// club, makes him Devoted.
func TestTestSaveFavouredClubs(t *testing.T) {
	g := loadTestSave(t)
	checkFavouredClubs(t, g, map[uint32][]string{
		2000301456: {"R. Madrid supporter"}, // Anton Matković
		2000196612: nil,                     // Jaka Čuber Potočnik
	})
	if p := g.byUID[70031727]; p == nil || p.personality() != "Devoted" {
		t.Errorf("Necip Uysal: %+v, want Devoted", p)
	}
}

// checkFavouredClubs checks each player's favoured clubs, written as
// "<club> <as>", and that each club's team index names a team of that club.
func checkFavouredClubs(t *testing.T, g *Game, want map[uint32][]string) {
	t.Helper()
	v := g.View(false)
	for uid, want := range want {
		p, ok := v.Profile(uid)
		if !ok {
			t.Errorf("uid %d not in the save", uid)
			continue
		}
		var got []string
		for _, c := range p.FavouredClubs {
			got = append(got, c.Club+" "+c.As)
			if c.TeamIndex == nil || g.world.Teams[*c.TeamIndex] == nil || g.world.Teams[*c.TeamIndex].ShortName != c.Club {
				t.Errorf("%s: %s has team index %v", p.Name, c.Club, c.TeamIndex)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: favoured clubs %q, want %q", p.Name, got, want)
		}
	}
}

func TestBuildWorldGivesTeamsTheirNationAndNationsTheirScales(t *testing.T) {
	db := &savefile.GameDB{
		NationCount: 251,
		Clubs:       []savefile.Club{{Index: 0, UID: 844, Name: "FC Metz", NationIndex: 143}},
		Teams: []savefile.Team{
			{Index: 0, UID: 844, Container: 0, AgeLimit: 100},
			{Index: 1, National: true, Container: 143, AgeLimit: 100},
			{Index: 2, National: true, Container: 394, AgeLimit: 100}, // 394 - 251 = 143
			{Index: 3, Container: 99, AgeLimit: 100},                  // no such club
		},
		Continents: []savefile.Continent{{Index: 0, UID: 1, Strength: 0.85}, {Index: 1, UID: 2, Strength: 0.99}},
		Nations:    []savefile.Nation{{Index: 0, UID: 5, Continent: 0, MensFactor: 105}, {Index: 1, UID: 6, Continent: -1, MensFactor: 7}},
	}

	w := buildWorld(db, langNames{})

	for team, want := range map[int64]int64{0: 143, 1: 143, 2: 143, 3: noTeam} {
		if got := w.Teams[team].NationIndex; got != want {
			t.Errorf("team %d nation %d, want %d", team, got, want)
		}
	}
	want := map[int64]NationScale{0: {Factor: 105, ContinentStrength: 0.85, HasContinent: true}, 1: {Factor: 7}}
	if !maps.Equal(w.NationScales, want) {
		t.Errorf("NationScales = %+v, want %+v", w.NationScales, want)
	}

	db.NationsErr, db.Nations, db.Continents = errors.New("no NATION table"), nil, nil
	if w := buildWorld(db, langNames{}); w.NationScales != nil {
		t.Errorf("NationScales = %+v without nations, want nil", w.NationScales)
	}
}

func TestLoadWarnsWhenReputationLevelsAreUnavailable(t *testing.T) {
	db := &savefile.GameDB{NationsErr: errors.New("no NATION table")}

	if w := nationWarning(db); !strings.Contains(w, "reputation levels unavailable") || !strings.Contains(w, "no NATION table") {
		t.Errorf("warning %q", w)
	}
	if w := nationWarning(&savefile.GameDB{}); w != "" {
		t.Errorf("warning %q without an error", w)
	}
}

// TestRealSavesReputationLevelAndMediaHandling checks the profile screens of
// 2035-12-14 (FM_REAL_SAVE_14DEC) and 2035-12-09 (FM_REAL_SAVE_9DEC). Cardozo
// is unscouted; the game still shows his reputation. Never point these at the
// game's own save folder.
func TestRealSavesReputationLevelAndMediaHandling(t *testing.T) {
	type want struct {
		reputation string
		media      []string // nil: not seen in game
	}
	saves := []struct {
		env     string
		date    time.Time
		players map[uint32]want
	}{
		{"FM_REAL_SAVE_14DEC", time.Date(2035, 12, 14, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002100552: {"Continental", []string{"Evasive", "Unflappable"}},   // Carlo Bernardi
			2000301456: {"Continental", []string{"Media-friendly"}},           // Anton Matković
			2002078767: {"Continental", nil},                                  // Stephen Ly
			2000191040: {"National", []string{"Level-headed"}},                // Louis Leroux
			2000478936: {"National", []string{"Media-friendly"}},              // Ilan Assongo
			2002099166: {"National", nil},                                     // Pablo Rodríguez
			2002115328: {"National", nil},                                     // Pablo Vilas
			2002098873: {"National", nil},                                     // Asier Castillo
			2002206940: {"National", nil},                                     // Mounir Haddad
			2000280809: {"National", nil},                                     // Rahul Cardozo, Jacksonville Armada U23
			2000196612: {"National", []string{"Media-friendly"}},              // Jaka Čuber Potočnik
			2000245491: {"National", []string{"Volatile", "Confrontational"}}, // Jesús Lobede
		}},
		{"FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC), map[uint32]want{
			2002100552: {"Continental", []string{"Evasive", "Unflappable"}}, // Carlo Bernardi
			2000191040: {"National", []string{"Level-headed"}},              // Louis Leroux
		}},
	}
	for _, s := range saves {
		t.Run(s.env, func(t *testing.T) {
			g := loadRealSave(t, s.env, s.date)
			for uid, w := range s.players {
				p, ok := g.View(false).Profile(uid)
				if !ok {
					t.Errorf("uid %d not in the save", uid)
					continue
				}
				if p.ReputationLevel != w.reputation {
					t.Errorf("%s: reputation %q, want %q", p.Name, p.ReputationLevel, w.reputation)
				}
				if w.media != nil && !slices.Equal(p.MediaHandling, w.media) {
					t.Errorf("%s: media handling %q, want %q", p.Name, p.MediaHandling, w.media)
				}
			}
			cardozo, _ := g.View(true).Profile(2000280809)
			if s.env == "FM_REAL_SAVE_14DEC" && (cardozo.Scouting != ScoutingRequired || cardozo.ReputationLevel != "National") {
				t.Errorf("Cardozo under fog of war: scouting %q, reputation %q", cardozo.Scouting, cardozo.ReputationLevel)
			}
		})
	}
}

// TestRealSave1JanProfiles checks profile fields against the game's screens
// on the KŽ save of 2036-01-01 (FM_REAL_SAVE_1JAN).
func TestRealSave1JanProfiles(t *testing.T) {
	type want struct {
		personality, reputation, contractStart string
		media                                  []string
	}
	g := loadRealSave(t, "FM_REAL_SAVE_1JAN", time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC))
	for uid, w := range map[uint32]want{
		14229525:   {"Ambitious", "Worldwide", "2034-02-02", []string{"Outspoken"}},                // Enzo Fernández; joined Chelsea 2023-01-31
		2000245491: {"Realist", "National", "2033-11-06", []string{"Volatile", "Confrontational"}}, // Jesús Lobede
		2002296691: {"Unambitious", "Local", "2035-04-24", []string{"Level-headed"}},               // Théo Emmanuelli, FC Metz U19
	} {
		p, ok := g.View(false).Profile(uid)
		if !ok {
			t.Errorf("uid %d not in the save", uid)
			continue
		}
		if p.Personality != w.personality || p.ReputationLevel != w.reputation || !slices.Equal(p.MediaHandling, w.media) {
			t.Errorf("%s: personality %q, reputation %q, media %q; want %q, %q, %q", p.Name, p.Personality, p.ReputationLevel, p.MediaHandling, w.personality, w.reputation, w.media)
		}
		if uid == 14229525 && (p.Contract == nil || p.Contract.Joined.Format(time.DateOnly) != "2023-01-31") {
			t.Errorf("%s: contract %+v, want joined 2023-01-31", p.Name, p.Contract)
		}
		if p.Contract == nil || p.Contract.Start.Format(time.DateOnly) != w.contractStart {
			t.Errorf("%s: contract %+v, want start %s", p.Name, p.Contract, w.contractStart)
		}
	}
}

// TestRealSave9DecSeasonDetail checks the keeper split and the internationals
// row on the KŽ save of 2035-12-09 (FM_REAL_SAVE_9DEC; bytes, not checked in
// game). Never point it at the game's own save folder.
func TestRealSave9DecSeasonDetail(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_9DEC", time.Date(2035, 12, 9, 0, 0, 0, 0, time.UTC))
	row := func(uid uint32, competition string) SeasonDetail {
		t.Helper()
		p, ok := g.View(false).Profile(uid)
		if !ok {
			t.Fatalf("uid %d not in the save", uid)
		}
		i := slices.IndexFunc(p.SeasonDetail, func(d SeasonDetail) bool { return d.Competition == competition })
		if i < 0 {
			t.Fatalf("%s has no %s row", p.Name, competition)
		}
		return p.SeasonDetail[i]
	}

	sami := row(2002114753, "league") // Bruno Sami, goalkeeper
	if sami.GoalsConceded != 22 || sami.CleanSheets != 3 || sami.SavesHeld != 27 || sami.SavesParried != 25 ||
		sami.SavesTipped != 6 || !sami.Goalkeeper || sami.PenaltiesFaced != 1 ||
		sami.PenaltiesSaved != 1 || sami.ExpectedGoalsPrevented != -9.62 || sami.MistakesLeadingToGoal != 1 ||
		sami.Headers != 0 || sami.Tackles != 0 || sami.Blocks != 0 {
		t.Errorf("Sami league = %+v", sami)
	}
	if c := row(2002114753, "club"); c.GoalsConceded != 27 || c.CleanSheets != 6 || c.SavesHeld != 40 ||
		c.SavesParried != 31 || c.SavesTipped != 10 || c.ExpectedGoalsPrevented != -11.33 {
		t.Errorf("Sami club = %+v", c)
	}

	m := row(2000301456, "league") // Anton Matković
	if m.TeamGoals != 31 || m.TeamConceded != 16 || m.MatchesLost != 1 || m.FoulsAgainst != 11 ||
		m.OpenPlayCrosses != 16 || m.OpenPlayCrossesCompleted != 10 || m.ShotsOutsideBox != 7 || m.Pressures != 84 ||
		m.PressuresCompleted != 20 || m.OpenPlayKeyPasses != 31 || m.NonPenaltyXG != 4.26 || m.HeadersLost != 38 ||
		m.SavesHeld != 0 || m.GoalsConceded != 0 || m.Goalkeeper {
		t.Errorf("Matković league = %+v", m)
	}

	e := row(2000146883, "internationals") // Francesco Pio Esposito
	if e.Starts != 6 || e.Goals != 7 || e.PenaltiesScored != 2 || e.PlayerOfTheMatch != 2 {
		t.Errorf("Esposito internationals = %+v", e)
	}
	if l := row(2000146883, "league"); l.FreeKickShots != 4 || l.GoalsOutsideBox != 2 || l.ShotsOutsideBox != 19 || l.FoulsAgainst != 19 {
		t.Errorf("Esposito league = %+v", l)
	}
}

func TestLoadWarnsWithoutTransfers(t *testing.T) {
	orig := readTransfers
	readTransfers = func(*savefile.Archive) ([]savefile.Transfer, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { readTransfers = orig })

	g := loadTestSave(t)
	want := "arranged transfers unavailable, so interest cannot tell who has agreed a move: boom"
	if !slices.Contains(g.Info.Warnings, want) {
		t.Errorf("warnings = %q, want %q", g.Info.Warnings, want)
	}
	for i := range g.aux {
		if g.aux[i].interest.arranged != nil {
			t.Fatal("a player has an arranged move without transfers")
		}
	}
}

func TestBuildWorldSetsHasManager(t *testing.T) {
	db := &savefile.GameDB{Teams: []savefile.Team{{Index: 0, Manager: 5, Container: -1}, {Index: 1, Manager: -1, Container: -1}}}
	w := buildWorld(db, langNames{})
	if !w.Teams[0].HasManager || w.Teams[1].HasManager {
		t.Errorf("HasManager = %v, %v; want true, false", w.Teams[0].HasManager, w.Teams[1].HasManager)
	}
}
