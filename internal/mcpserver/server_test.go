package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Napas/fm-mcp/internal/fm"
	"github.com/Napas/fm-mcp/internal/fmprefs"
	"github.com/Napas/fm-mcp/internal/savefile"
)

var gameDate = time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC)

var world = fm.World{
	Teams: map[int64]*fm.Team{
		547: {Index: 547, ClubIndex: 416, NationIndex: 139, Name: "Tottenham Hotspur", Club: "Tottenham Hotspur", Division: "Premier League", Reputation: 7750, BankBalance: 29_636_499,
			Budgets: &fm.Budgets{TransferBudget: 8_329_074, WageBudget: 1_069_817}},
		20739: {Index: 20739, ClubIndex: 9, Name: "Chelsea", Club: "Chelsea", Reputation: 9000, Women: true, BankBalance: 5_000_000},
	},
	Clubs:    map[int64]fm.Club{416: {Name: "Spurs", FirstTeam: 547}},
	Injuries: map[int64]string{27: "torn anterior cruciate ligament"},
	// England: national cut 4422, regional 2327, local 465.
	NationScales: map[int64]fm.NationScale{139: {Factor: 187, ContinentStrength: 0.99, HasContinent: true}},
}

func raw(uid uint32, first, last string, club int64, ca int16, pos int) savefile.Player {
	p := savefile.Player{
		UID: uid, FirstName: first, LastName: last, TeamIndex: club,
		BirthDate:      savefile.Date{Raw: 100, Year: 2000},
		CurrentAbility: ca, PotentialAbility: ca + 10, CurrentReputation: uint16(ca) * 10, //nolint:gosec // small test values
	}
	p.Positions[pos] = 20
	for i := range p.Attributes {
		p.Attributes[i] = 60
	}
	return p
}

func fakeGame(path string, extra ...savefile.Player) *fm.Game {
	return fakeGameWith(path, fm.PlayerExtras{}, extra...)
}

// fakeGameWith is fakeGame with extras.
func fakeGameWith(path string, extras fm.PlayerExtras, extra ...savefile.Player) *fm.Game {
	info := fm.Info{SaveName: filepath.Base(path), Date: gameDate, ManagerName: "Napas Mačiulaitis", ManagerTeamIndex: 547, ManagerClub: "Tottenham Hotspur"}
	thompson := raw(4, "Alyssa", "Thompson", 20739, 172, 3)
	thompson.Female = true
	romero := raw(2, "Cristian", "Romero", 547, 156, 3)
	romero.Value = 58_400_000
	romero.BirthDate = savefile.Date{Raw: 117, Year: 1998} // 27: the game's youth test cannot apply
	romero.Contract = &savefile.Contract{TeamIndex: 547, WeeklyWage: 200_000, End: savefile.Date{Raw: 181, Year: 2029}, PlayingTime: 4, OriginalPlayingTime: 2, PlayingTimeScore: -15, SquadNumber: 17, WeeklyLoyalty: 1000}
	romero.Hidden = [8]uint8{10, 10, 10, 10, 15, 10, 10, 10} // professionalism 15
	romero.Traits = 1<<17 | 1<<37                            // Argues With Officials, Dives Into Tackles
	romero.ClubAffinities = []savefile.ClubAffinity{{Club: 416, Value: 100, Reason: 1}}
	romero.Injuries = []savefile.Injury{{ID: 27, Start: savefile.Date{Raw: 188, Year: 2025}, DaysMin: 10, DaysMax: 20}}
	axel := raw(3, "Axel", "Romero", -1, 65, 3) // a free agent
	return fm.NewGame(info, append([]savefile.Player{
		raw(1, "Guglielmo", "Vicario", 547, 144, 0),
		romero,
		axel,
		thompson,
	}, extra...), world, extras)
}

type harness struct {
	t       *testing.T
	session *mcp.ClientSession
	loaded  []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return harnessWith(t)
}

// harnessWith serves fakeGame plus extra players.
func harnessWith(t *testing.T, extra ...savefile.Player) *harness {
	t.Helper()
	dir := t.TempDir()
	for i, name := range []string{"old.fm", "new.fm", "notes.txt"} {
		writeFile(t, filepath.Join(dir, name), "x", gameDate.Add(time.Duration(i)*time.Hour))
	}
	var h *harness
	srv := New(Options{
		SaveDirs: []string{dir, filepath.Join(dir, "missing")},
		Load: func(path string) (*fm.Game, error) {
			h.loaded = append(h.loaded, filepath.Base(path))
			return fakeGame(path, extra...), nil
		},
	})
	h = connect(t, srv)
	return h
}

func writeFile(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// connect starts srv and returns a harness with a client session to it.
func connect(t *testing.T, srv *mcp.Server) *harness {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return &harness{t: t, session: cs}
}

func (h *harness) call(name string, args map[string]any, out any) *mcp.CallToolResult {
	h.t.Helper()
	res, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("%s: %v", name, err)
	}
	if out != nil && !res.IsError {
		b, err := json.Marshal(res.StructuredContent)
		if err != nil {
			h.t.Fatal(err)
		}
		if err := json.Unmarshal(b, out); err != nil {
			h.t.Fatalf("%s: decoding %s: %v", name, b, err)
		}
	}
	return res
}

func TestListSavesNewestFirst(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Saves []struct {
			Name string `json:"name"`
		} `json:"saves"`
	}
	h.call("list_saves", nil, &out)

	if len(out.Saves) != 2 || out.Saves[0].Name != "new.fm" || out.Saves[1].Name != "old.fm" {
		t.Errorf("saves = %+v, want new.fm then old.fm", out.Saves)
	}
}

func TestToolsLoadNewestSaveOnFirstUse(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Info        fm.Info `json:"info"`
		PlayerCount int     `json:"player_count"`
		LoadedFrom  string  `json:"loaded_from"`
	}
	h.call("game_overview", nil, &out)
	h.call("game_overview", nil, nil)

	if out.PlayerCount != 4 || out.Info.ManagerClub != "Tottenham Hotspur" || filepath.Base(out.LoadedFrom) != "new.fm" {
		t.Errorf("overview = %+v", out)
	}
	if len(h.loaded) != 1 {
		t.Errorf("loaded %v, want one load", h.loaded)
	}
}

func TestLoadSaveByPath(t *testing.T) {
	h := newHarness(t)
	var saves struct {
		Saves []struct {
			Path string `json:"path"`
		} `json:"saves"`
	}
	h.call("list_saves", nil, &saves)

	h.call("load_save", map[string]any{"path": saves.Saves[1].Path}, nil)

	if len(h.loaded) != 1 || h.loaded[0] != "old.fm" {
		t.Errorf("loaded %v, want [old.fm]", h.loaded)
	}
	if res := h.call("load_save", map[string]any{"path": "/etc/passwd"}, nil); !res.IsError {
		t.Error("load_save accepted a non-.fm path")
	}
}

func TestFindAndGetPlayer(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("find_players", map[string]any{"name": "romero"}, &found)
	if len(found.Players) != 2 || found.Players[0].UID != 2 || found.Players[0].Club != "Tottenham Hotspur" {
		t.Fatalf("find_players = %+v", found.Players)
	}
	r := found.Players[0]
	if r.Value == nil || *r.Value != 58_400_000 || r.Wage == nil || *r.Wage != 200_000 || r.ContractEnd != "2029-06-30" || r.InjuredUntil != "2025-07-17" || r.Injury != "torn anterior cruciate ligament" {
		t.Errorf("summary money/contract/injury = %+v", r)
	}

	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 2}, &p)
	if p.Name != "Cristian Romero" || p.Attributes.Get("Tackling") != 12 {
		t.Errorf("get_player = %+v", p)
	}
	// All attributes 60 at DC 20: the roles without negative attributes tie at his current ability.
	if want := []fm.RoleFit{{Role: "Advanced Centre-Back", Position: "DC", RoleAbility: 156}, {Role: "Ball-Playing Centre-Back", Position: "DC", RoleAbility: 156}, {Role: "Overlapping Centre-Back", Position: "DC", RoleAbility: 156}}; !slices.Equal(p.BestRoles, want) {
		t.Errorf("best_roles = %+v, want %+v", p.BestRoles, want)
	}
	if res := h.call("get_player", map[string]any{"uid": 99}, nil); !res.IsError {
		t.Error("get_player(99) did not report an error")
	}
}

func TestSquadDefaultsToManagersClub(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Team    fm.Team         `json:"team"`
		Players []PlayerSummary `json:"players"`
	}
	h.call("get_squad", nil, &out)

	if out.Team.Name != "Tottenham Hotspur" || out.Team.BankBalance != 29_636_499 || len(out.Players) != 2 || out.Players[0].UID != 2 {
		t.Errorf("squad = %+v", out)
	}
}

func TestSearchAndCompare(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("search_players", map[string]any{"position": "DC", "exclude_team_index": 547}, &found)
	if len(found.Players) != 1 || found.Players[0].UID != 3 {
		t.Errorf("search_players defaulting to the manager's men's team = %+v", found.Players)
	}
	h.call("search_players", map[string]any{"position": "DC", "gender": "women"}, &found)
	if len(found.Players) != 1 || found.Players[0].UID != 4 {
		t.Errorf("search_players for women = %+v", found.Players)
	}
	if res := h.call("search_players", map[string]any{"position": "XX"}, nil); !res.IsError {
		t.Error("search_players accepted an unknown position")
	}

	var cmp struct {
		Players []fm.Profile `json:"players"`
	}
	h.call("compare_players", map[string]any{"uids": []int{2, 3}}, &cmp)
	if len(cmp.Players) != 2 || cmp.Players[1].Name != "Axel Romero" {
		t.Errorf("compare_players = %+v", cmp.Players)
	}
}

func TestSearchByRole(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	if res := h.call("search_players", map[string]any{"position": "DC", "role": "Overlapping Centre-Back", "sort_by": "age"}, &found); res.IsError {
		t.Fatalf("search_players: %s", allText(res))
	}
	// Romero's best role; Axel, a free agent, is unscouted.
	if len(found.Players) != 2 || found.Players[0].RoleAbility == nil || *found.Players[0].RoleAbility != 156 || found.Players[1].RoleAbility != nil {
		t.Errorf("rows = %+v", found.Players)
	}
	if res := h.call("search_players", map[string]any{"role": "Overlapping Centre-Back"}, nil); !res.IsError || !strings.Contains(allText(res), "needs a position") {
		t.Errorf("a role without a position = %s, want an error", allText(res))
	}
	if res := h.call("search_players", map[string]any{"position": "ST", "role": "Goalkeeper"}, nil); !res.IsError || !strings.Contains(allText(res), "not offered at ST") {
		t.Errorf("a role not offered at the position = %s, want an error", allText(res))
	}
}

func TestRoleAbilityIsExplained(t *testing.T) {
	h := newHarness(t)

	var paragraph string // the instructions' line on role_ability
	for line := range strings.SplitSeq(h.session.InitializeResult().Instructions, "\n") {
		if strings.HasPrefix(line, "role_ability") {
			paragraph = line
		}
	}
	for _, want := range []string{"role_ability", "compare_players", "current-ability scale (0-200)", "best role usually equals his current ability", "halves", "stars", "cannot reproduce", "rank", "3 times in 4", "ties", "better of left and right", "fog of war"} {
		if !strings.Contains(paragraph, want) {
			t.Errorf("instructions' role_ability paragraph lacks %q", want)
		}
	}
	if strings.Contains(paragraph, "as the game does") {
		t.Errorf("instructions claim the game's role order: %s", paragraph)
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, wants := range map[string][]string{
		"get_tactic":      {"role_ability"},
		"get_player":      {"best_roles"},
		"compare_players": {"best_roles"},
		"search_players":  {"role", "role_ability", "interest"},
	} {
		i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == name })
		if i < 0 {
			t.Fatalf("no tool %s", name)
		}
		for _, want := range wants {
			if !strings.Contains(tools.Tools[i].Description, want) {
				t.Errorf("%s description lacks %q", name, want)
			}
		}
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "search_players" })
	if b, _ := json.Marshal(tools.Tools[i].InputSchema); !strings.Contains(string(b), "in-possession role offered at that position") || !strings.Contains(string(b), "role_ability") {
		t.Errorf("search_players schema does not explain role: %s", b)
	}
}

func ptr[T any](v T) *T { return &v }

func TestEndToEndOnTestSave(t *testing.T) {
	const save = "../../data/test_save.fm"
	if _, err := os.Stat(save); err != nil {
		t.Skipf("test save not available: %v", err)
	}
	srv := New(Options{DefaultSave: save})
	h := connect(t, srv)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("find_players", map[string]any{"name": "James Maddison", "limit": 1}, &found)
	if len(found.Players) != 1 || *found.Players[0].CurrentAbility != 151 || found.Players[0].Age != 28 {
		t.Fatalf("find_players = %+v", found.Players)
	}
	var squad struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("get_squad", map[string]any{"team_index": found.Players[0].TeamIndex}, &squad)
	if len(squad.Players) != 27 {
		t.Errorf("squad has %d players, want 27", len(squad.Players))
	}

	// Argentina's senior squad (team 1233) holds Cristian Romero of Tottenham.
	h.call("get_squad", map[string]any{"team_index": 1233}, &squad)
	romero := slices.IndexFunc(squad.Players, func(p PlayerSummary) bool { return p.Name == "Cristian Romero" })
	if len(squad.Players) != 23 || romero < 0 || squad.Players[romero].TeamIndex != 547 || squad.Players[romero].Club != "Tottenham Hotspur" {
		t.Errorf("Argentina squad = %d players, Romero at %d: %+v", len(squad.Players), romero, squad.Players)
	}
}

func TestSquadDepth(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Positions []struct {
			Position string `json:"position"`
			Players  []struct {
				UID    uint32 `json:"uid"`
				Rating int    `json:"rating"`
			} `json:"players"`
		} `json:"positions"`
	}
	h.call("squad_depth", nil, &out)

	if len(out.Positions) != 15 || out.Positions[3].Position != "DC" || len(out.Positions[3].Players) != 1 {
		t.Fatalf("squad_depth = %+v", out)
	}
	if p := out.Positions[3].Players[0]; p.UID != 2 || p.Rating != 20 {
		t.Errorf("DC = %+v, want Romero rated 20", p)
	}
}

func TestSquadNeedsClubWhenManagersClubIsUnknown(t *testing.T) {
	srv := New(Options{DefaultSave: "x.fm", Load: func(string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: -1}, []savefile.Player{raw(3, "Axel", "Romero", -1, 65, 3)}, fm.World{}, fm.PlayerExtras{}), nil
	}})
	h := connect(t, srv)

	for _, tool := range []string{"get_squad", "squad_depth"} {
		if res := h.call(tool, nil, nil); !res.IsError {
			t.Errorf("%s without team_index listed free agents instead of failing", tool)
		}
	}
}

func TestFindClubs(t *testing.T) {
	h := newHarness(t)

	var out struct {
		Teams []fm.Team `json:"teams"`
	}
	h.call("find_clubs", map[string]any{"name": "chelsea"}, &out)

	if len(out.Teams) != 1 || out.Teams[0].Index != 20739 || !out.Teams[0].Women {
		t.Errorf("find_clubs = %+v", out.Teams)
	}
}

func TestFogOfWarHidesOtherClubsBankBalances(t *testing.T) {
	h := newHarness(t)

	for tool, args := range map[string]map[string]any{
		"find_clubs": {"name": "chelsea"},
		"get_squad":  {"team_index": 20739},
	} {
		if got := h.rawJSON(tool, args); strings.Contains(got, "bank_balance") {
			t.Errorf("%s under fog of war = %s, want no bank balance", tool, got)
		}
	}
	h.call("set_fog_of_war", map[string]any{"enabled": false}, nil)
	if got := h.rawJSON("find_clubs", map[string]any{"name": "chelsea"}); !strings.Contains(got, `"bank_balance":5000000`) {
		t.Errorf("find_clubs without fog of war = %s", got)
	}
}

func TestPrompts(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	list, err := h.session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range list.Prompts {
		names = append(names, p.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"contract_review", "scout_position", "squad_review"}) {
		t.Errorf("prompts = %v", names)
	}

	res, err := h.session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "scout_position", Arguments: map[string]string{"position": "DL", "max_value": "20000000"}})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "DL") || !strings.Contains(text, "20000000") || !strings.Contains(text, "search_players") {
		t.Errorf("scout_position text = %q", text)
	}

	if _, err := h.session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "scout_position"}); err == nil {
		t.Error("scout_position without a position did not fail")
	}
}

func TestSearchGenderAnyAndSchema(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
		Total   int             `json:"total"`
	}
	h.call("search_players", map[string]any{"position": "DC", "gender": "any"}, &found)
	if len(found.Players) != 3 || found.Total != 3 {
		t.Errorf("gender any = %d players (total %d), want both genders (3)", len(found.Players), found.Total)
	}

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "search_players" {
			continue
		}
		b, _ := json.Marshal(tool.InputSchema)
		if strings.Contains(string(b), "both when empty") || !strings.Contains(string(b), "manager") {
			t.Errorf("gender schema does not describe the default: %s", b)
		}
	}
}

func TestUnknownTeamIsAnError(t *testing.T) {
	h := newHarness(t)

	for _, tool := range []string{"get_squad", "squad_depth"} {
		if res := h.call(tool, map[string]any{"team_index": 99999}, nil); !res.IsError || !strings.Contains(allText(res), "find_clubs") {
			t.Errorf("%s with an unknown team_index = %s, want an error pointing to find_clubs", tool, allText(res))
		}
	}
}

func TestSquadListsPlayersOutOnLoanAndContractedFilter(t *testing.T) {
	loanee := raw(5, "Mikey", "Moore", 900, 120, 9)
	loanee.Contract = &savefile.Contract{TeamIndex: 547, WeeklyWage: 20000, End: savefile.Date{Raw: 181, Year: 2027}}
	loanee.Loan = &savefile.Contract{TeamIndex: 900, End: savefile.Date{Raw: 181, Year: 2026}}
	h := harnessWith(t, loanee)

	var out struct {
		Players   []PlayerSummary `json:"players"`
		OutOnLoan []PlayerSummary `json:"out_on_loan"`
	}
	h.call("get_squad", nil, &out)
	if len(out.OutOnLoan) != 1 || out.OutOnLoan[0].UID != 5 {
		t.Errorf("out_on_loan = %+v", out.OutOnLoan)
	}

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("search_players", map[string]any{"contracted_team_index": 547}, &found)
	if len(found.Players) != 2 || found.Players[1].UID != 5 {
		t.Errorf("contracted to 547 = %+v", found.Players)
	}
}

func TestFindWithoutANameIsAnError(t *testing.T) {
	h := newHarness(t)

	for _, tool := range []string{"find_players", "find_clubs"} {
		if res := h.call(tool, map[string]any{"name": "  "}, nil); !res.IsError {
			t.Errorf("%s with a blank name did not fail", tool)
		}
	}
}

func TestGenderDefaultSkippedForTeamFilters(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("search_players", map[string]any{"team_index": 20739}, &found)

	if len(found.Players) != 1 || found.Players[0].UID != 4 {
		t.Errorf("women's team searched by a men's manager = %+v", found.Players)
	}
}

func TestContractReviewPromptUsesContractedTeam(t *testing.T) {
	h := newHarness(t)

	res, err := h.session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "contract_review", Arguments: map[string]string{"expires_by": "2026-06-30"}})
	if err != nil {
		t.Fatal(err)
	}

	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "contracted_team_index") || !strings.Contains(text, "2026-06-30") {
		t.Errorf("contract_review = %q", text)
	}
}

func TestCompareLimitAndLoadDefault(t *testing.T) {
	h := newHarness(t)

	uids := make([]int, maxCompare+1)
	for i := range uids {
		uids[i] = 1
	}
	if res := h.call("compare_players", map[string]any{"uids": uids}, nil); !res.IsError {
		t.Errorf("compare_players accepted %d players", len(uids))
	}
}

func TestLoadSaveWithoutPathUsesDefaultSave(t *testing.T) {
	var loaded []string
	srv := New(Options{DefaultSave: "/x/chosen.fm", SaveDirs: []string{t.TempDir()}, Load: func(path string) (*fm.Game, error) {
		loaded = append(loaded, path)
		return fakeGame(path), nil
	}})
	h := connect(t, srv)

	var out struct {
		LoadedFrom string `json:"loaded_from"`
	}
	h.call("load_save", nil, &out)

	if out.LoadedFrom != "/x/chosen.fm" || len(loaded) != 1 {
		t.Errorf("loaded %v, reported %q; want /x/chosen.fm once", loaded, out.LoadedFrom)
	}
}

func TestLoadLogsTimingAndWarnings(t *testing.T) {
	var logs []string
	srv := New(Options{DefaultSave: "/x/a.fm", Logf: func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
		Load: func(path string) (*fm.Game, error) {
			return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: -1, Warnings: []string{"names unavailable"}}, nil, fm.World{}, fm.PlayerExtras{}), nil
		}})

	connect(t, srv).call("game_overview", nil, nil)

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "/x/a.fm") || !strings.Contains(joined, "names unavailable") {
		t.Errorf("logs = %q", joined)
	}
}

func TestFirstCallReportsLoadWarnings(t *testing.T) {
	srv := New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: -1, Warnings: []string{"nation names unavailable"}},
			[]savefile.Player{raw(1, "Guglielmo", "Vicario", 547, 144, 0)}, world, fm.PlayerExtras{}), nil
	}})
	h := connect(t, srv)

	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	first := h.call("find_players", map[string]any{"name": "vicario"}, &found)
	second := h.call("find_players", map[string]any{"name": "vicario"}, nil)

	if len(found.Players) != 1 || !strings.Contains(allText(first), "nation names unavailable") {
		t.Errorf("first call: players %d, text %q", len(found.Players), allText(first))
	}
	if strings.Contains(allText(second), "nation names unavailable") {
		t.Errorf("warnings repeated on a later call: %q", allText(second))
	}
}

func TestScoutPromptOmitsUnsetFilters(t *testing.T) {
	h := newHarness(t)

	res, err := h.session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "scout_position", Arguments: map[string]string{"position": "ST"}})
	if err != nil {
		t.Fatal(err)
	}

	text := res.Messages[0].Content.(*mcp.TextContent).Text
	if strings.Contains(text, "max_age") || strings.Contains(text, "max_value") || strings.Contains(text, " any") {
		t.Errorf("prompt mentions unset filters: %q", text)
	}
}

// TestOutputSchemasAreObjects guards the MCP rule (enforced by TypeScript
// clients) that every tool's output schema has top-level type "object".
func TestOutputSchemasAreObjects(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		b, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var s struct {
			Type any `json:"type"`
		}
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		if s.Type != "object" {
			t.Errorf("%s: output schema type = %v, want \"object\"", tool.Name, s.Type)
		}
	}
}

func TestSearchPlayersHintIsDescribed(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "search_players" })
	if i < 0 {
		t.Fatal("no search_players tool")
	}
	b, err := json.Marshal(tools.Tools[i].OutputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if d := s.Properties["hint"].Description; !strings.Contains(d, "club filter") {
		t.Errorf("hint description = %q, want one on the club filter", d)
	}
}

// rawJSON calls a tool and returns its structured result as JSON text.
func (h *harness) rawJSON(name string, args map[string]any) string {
	h.t.Helper()
	res := h.call(name, args, nil)
	if res.IsError {
		h.t.Fatalf("%s: %v", name, res.Content)
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

func allText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func TestFogOfWarIsOnByDefaultAndHidesUnscoutedPlayers(t *testing.T) {
	h := newHarness(t)

	var overview struct {
		FogOfWar bool `json:"fog_of_war"`
	}
	h.call("game_overview", nil, &overview)
	if !overview.FogOfWar {
		t.Error("game_overview: fog of war is off by default")
	}
	axel := h.rawJSON("get_player", map[string]any{"uid": 3}) // free agent, reputation 650
	for _, key := range []string{`"attributes"`, `"current_ability"`, `"potential_ability"`, `"value"`} {
		if strings.Contains(axel, key) {
			t.Errorf("unscouted player shows %s: %s", key, axel)
		}
	}
	if !strings.Contains(axel, `"scouting":"required"`) || !strings.Contains(axel, `"knowledge":0`) {
		t.Errorf("unscouted player = %s", axel)
	}
	romero := h.rawJSON("get_player", map[string]any{"uid": 2})
	if !strings.Contains(romero, `"scouting":"full"`) || !strings.Contains(romero, `"current_ability":156`) {
		t.Errorf("own player = %s", romero)
	}
	summary := h.rawJSON("find_players", map[string]any{"name": "axel"})
	if strings.Contains(summary, `"current_ability"`) || !strings.Contains(summary, `"scouting":"required"`) {
		t.Errorf("summary = %s", summary)
	}
}

func TestSetFogOfWar(t *testing.T) {
	h := newHarness(t)

	var out struct {
		FogOfWar bool `json:"fog_of_war"`
	}
	h.call("set_fog_of_war", map[string]any{"enabled": false}, &out)
	if out.FogOfWar {
		t.Error("set_fog_of_war(false) reported fog of war on")
	}
	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 3}, &p)
	if p.CurrentAbility == nil || *p.CurrentAbility != 65 || p.Scouting != "" || p.Knowledge != nil {
		t.Errorf("with fog of war off: %+v", p)
	}
	h.call("load_save", nil, nil) // the setting survives a reload
	var overview struct {
		FogOfWar bool `json:"fog_of_war"`
	}
	h.call("game_overview", nil, &overview)
	if overview.FogOfWar {
		t.Error("fog of war came back on after load_save")
	}
	h.call("set_fog_of_war", map[string]any{"enabled": true}, &out)
	if !out.FogOfWar {
		t.Error("set_fog_of_war(true) reported fog of war off")
	}
	if res := h.call("set_fog_of_war", nil, nil); !res.IsError {
		t.Error("set_fog_of_war without enabled did not fail")
	}
}

func TestFogOfWarOffOption(t *testing.T) {
	srv := New(Options{DefaultSave: "/x/a.fm", FogOfWarOff: true, Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) { return fakeGame(path), nil }})
	h := connect(t, srv)

	if got := h.rawJSON("get_player", map[string]any{"uid": 3}); !strings.Contains(got, `"current_ability":65`) || strings.Contains(got, "scouting") {
		t.Errorf("get_player with fog of war off = %s", got)
	}
}

func TestSearchAndDepthWithFogOfWar(t *testing.T) {
	h := newHarness(t)

	var found struct {
		Players []PlayerSummary `json:"players"`
		Total   int             `json:"total"`
	}
	h.call("search_players", map[string]any{"min_current_ability": 60, "gender": "any"}, &found)
	if found.Total != 2 || found.Players[0].UID != 2 || found.Players[1].UID != 1 {
		t.Errorf("min ability with fog of war = %+v (total %d), want only own players", found.Players, found.Total)
	}
	depth := h.rawJSON("squad_depth", map[string]any{"team_index": 20739})
	if strings.Contains(depth, `"current_ability"`) || !strings.Contains(depth, `"scouting":"required"`) || !strings.Contains(depth, `"knowledge":0`) {
		t.Errorf("squad_depth of an unscouted team = %s", depth)
	}
}

func TestNotForSaleSummary(t *testing.T) {
	star := raw(7, "Tino", "Livramento", 547, 161, 4)
	star.Value = 300_000_000
	h := harnessWith(t, star)

	got := h.rawJSON("find_players", map[string]any{"name": "livramento"})
	if !strings.Contains(got, `"not_for_sale":true`) || strings.Contains(got, `"value"`) {
		t.Errorf("summary = %s", got)
	}
}

func TestTeamRowsLeaveOutBudgets(t *testing.T) {
	h := newHarness(t)

	if got := h.rawJSON("get_squad", nil); strings.Contains(got, "_budget") || !strings.Contains(got, `"bank_balance":29636499`) {
		t.Errorf("get_squad = %s", got)
	}
}

func TestInstructionsCoverFogOfWarAndColours(t *testing.T) {
	h := newHarness(t)

	text := h.session.InitializeResult().Instructions
	for _, want := range []string{"set_fog_of_war", "explicitly", "scouting", "other clubs' bank balances", "🟡", "🟢"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	for _, want := range []string{"transfer_budget", "wage_budget", "current wage bill (probably players and staff)", "spent this season", "No budgets means"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if strings.Contains(text, "original_transfer_budget") {
		t.Error("instructions still mention original_transfer_budget")
	}
	if strings.Contains(guessedFields(text), "joined_club") {
		t.Error("instructions still list joined_club as a guess")
	}
}

func TestScoutPromptCoversFogOfWar(t *testing.T) {
	h := newHarness(t)

	res, err := h.session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "scout_position", Arguments: map[string]string{"position": "ST"}})
	if err != nil {
		t.Fatal(err)
	}

	if text := res.Messages[0].Content.(*mcp.TextContent).Text; !strings.Contains(text, "fog of war") || !strings.Contains(text, "scouting") {
		t.Errorf("scout_position ignores fog of war: %q", text)
	}
}

// euroHarness serves fakeGame shown in euros per year.
func euroHarness(t *testing.T) *harness {
	t.Helper()
	euro := savefile.Currency{UID: 28, Name: "European Euro", ShortName: "Euro", Symbol: "€", PerGBP: 1.1571305}
	keeper := raw(6, "Fraser", "Forster", 547, 120, 0)
	keeper.Contract = &savefile.Contract{TeamIndex: 547, WeeklyWage: 50_000}
	srv := New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		g := fakeGame(path, keeper)
		g.Info.Money = fm.NewMoney(euro, fmprefs.PerYear)
		return g, nil
	}})
	return connect(t, srv)
}

func TestOverviewShowsTheManagersClubBudgets(t *testing.T) {
	h := euroHarness(t)

	// £8 329 074 and £1 069 817 a week, in euros per year.
	overview := h.rawJSON("game_overview", nil)
	if !strings.Contains(overview, `"budgets":{"transfer_budget":9637825,"wage_budget":64371729}`) {
		t.Errorf("game_overview = %s", overview)
	}
}

func TestOverviewWithoutTheManagersClubHasNoBudgets(t *testing.T) {
	srv := New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		g := fakeGame(path)
		g.Info.ManagerTeamIndex = -1
		return g, nil
	}})

	if overview := connect(t, srv).rawJSON("game_overview", nil); strings.Contains(overview, `"budgets":`) {
		t.Errorf("game_overview = %s", overview)
	}
}

func TestMoneyIsInTheUsersCurrencyAndPeriod(t *testing.T) {
	h := euroHarness(t)

	overview := h.rawJSON("game_overview", nil)
	if !strings.Contains(overview, `"currency":{"name":"European Euro","short_name":"Euro","symbol":"€"}`) || !strings.Contains(overview, `"wage_period":"p/a"`) {
		t.Errorf("game_overview = %s", overview)
	}
	if squad := h.rawJSON("get_squad", nil); !strings.Contains(squad, `"bank_balance":34293296`) {
		t.Errorf("get_squad = %s", squad)
	}
	// Romero: £58.4M, £200 000 a week.
	var found struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("find_players", map[string]any{"name": "cristian romero"}, &found)
	if r := found.Players[0]; *r.Value != 67_576_420 || r.Wage == nil || *r.Wage != 12_034_157 {
		t.Errorf("Romero value %d wage %v", *r.Value, r.Wage)
	}
	// Forster's £50 000 a week is €3.0M a year; Vicario has no contract, so no wage.
	h.call("search_players", map[string]any{"max_wage": 12_000_000, "max_value": 70_000_000, "gender": "any", "team_index": 547}, &found)
	if len(found.Players) != 1 || found.Players[0].UID != 6 {
		t.Errorf("search in euros per year = %+v", found.Players)
	}
	if res := h.call("search_players", map[string]any{"sort_by": "wage"}, nil); res.IsError {
		t.Errorf("sort_by wage failed: %v", res.Content)
	}
}

func TestNoToolMentionsWeeklyWage(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "weekly_wage") || !strings.Contains(string(b), "max_wage") {
		t.Errorf("tools still use weekly_wage or lack max_wage")
	}
	text := h.session.InitializeResult().Instructions
	for _, want := range []string{"currency", "wage_period", "€922K"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if strings.Contains(text, "Money is in GBP") {
		t.Error("instructions still say money is in GBP")
	}
	prompts, err := h.session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range prompts.Prompts {
		for _, a := range p.Arguments {
			if a.Name == "max_value" && (strings.Contains(a.Description, "GBP") || !strings.Contains(a.Description, "currency")) {
				t.Errorf("%s max_value = %q", p.Name, a.Description)
			}
		}
	}
}

func TestSearchAlwaysReportsTotalAndNeedsScouting(t *testing.T) {
	h := newHarness(t)

	// Vicario and Romero fall short; fog of war hides free agent Axel's potential.
	got := h.rawJSON("search_players", map[string]any{"min_potential_ability": 200})
	if !strings.Contains(got, `"players":[]`) || !strings.Contains(got, `"total":0`) || !strings.Contains(got, `"needs_scouting":1`) {
		t.Errorf("search with fog of war = %s", got)
	}
	h.call("set_fog_of_war", map[string]any{"enabled": false}, nil)
	if got := h.rawJSON("search_players", map[string]any{"min_potential_ability": 200}); !strings.Contains(got, `"total":0`) || !strings.Contains(got, `"needs_scouting":0`) {
		t.Errorf("search without fog of war = %s", got)
	}
	if got := h.rawJSON("find_players", map[string]any{"name": "romero"}); strings.Contains(got, "total") || strings.Contains(got, "needs_scouting") {
		t.Errorf("find_players = %s", got)
	}
}

func TestNeedsScoutingIsExplained(t *testing.T) {
	h := newHarness(t)

	if text := h.session.InitializeResult().Instructions; !strings.Contains(text, "needs_scouting") {
		t.Error("instructions do not explain needs_scouting")
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "search_players" && !strings.Contains(tool.Description, "needs_scouting") {
			t.Errorf("search_players description = %q", tool.Description)
		}
	}
}

func TestProfileToolsMentionSeasonStats(t *testing.T) {
	h := newHarness(t)
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, tool := range tools.Tools {
		if tool.Name != "get_player" && tool.Name != "compare_players" {
			continue
		}
		found++
		if !strings.Contains(tool.Description, "season stats per competition") {
			t.Errorf("%s description = %q", tool.Name, tool.Description)
		}
	}
	if found != 2 {
		t.Errorf("found %d of get_player and compare_players", found)
	}
}

func TestSummaryWageTellsZeroFromUnknown(t *testing.T) {
	unpaid := raw(8, "Unpaid", "Trialist", 547, 90, 5)
	unpaid.Contract = &savefile.Contract{TeamIndex: 547}
	h := harnessWith(t, unpaid)
	h.call("set_fog_of_war", map[string]any{"enabled": false}, nil)

	if got := h.rawJSON("find_players", map[string]any{"name": "unpaid"}); !strings.Contains(got, `"wage":0`) {
		t.Errorf("a £0 wage is left out: %s", got)
	}
	if got := h.rawJSON("find_players", map[string]any{"name": "axel"}); strings.Contains(got, `"wage"`) {
		t.Errorf("a free agent shows a wage: %s", got)
	}
}

func TestLoadReturnsFreedMemoryToTheOS(t *testing.T) {
	var events []string
	dir := t.TempDir()
	for _, name := range []string{"good.fm", "bad.fm"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h := connect(t, New(Options{SaveDirs: []string{dir}, releaseMemory: func() { events = append(events, "release") }, Load: func(path string) (*fm.Game, error) {
		events = append(events, "load "+filepath.Base(path))
		if filepath.Base(path) == "bad.fm" {
			return nil, errors.New("corrupt")
		}
		return fakeGame(path), nil
	}}))

	h.call("load_save", map[string]any{"path": filepath.Join(dir, "good.fm")}, nil)
	h.call("load_save", map[string]any{"path": filepath.Join(dir, "bad.fm")}, nil)

	want := []string{"load good.fm", "release", "load bad.fm", "release"}
	if !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

// writingSettle leaves room for a slow machine: a stalled writer goroutine
// cannot let a constantly written save settle, and timing bounds hold.
const (
	testSettle    = 50 * time.Millisecond
	writingSettle = 500 * time.Millisecond
)

// reloadHarness serves one save file; Load returns results in order (nil is success).
type reloadHarness struct {
	*harness
	path     string
	loads    int
	warnings []string // of every loaded game
}

func newReloadHarness(t *testing.T, results ...error) *reloadHarness {
	t.Helper()
	return newReloadHarnessSettling(t, testSettle, results...)
}

func newReloadHarnessSettling(t *testing.T, settle time.Duration, results ...error) *reloadHarness {
	t.Helper()
	r := &reloadHarness{path: filepath.Join(t.TempDir(), "career.fm")}
	writeFile(t, r.path, "v1", time.Now().Add(-time.Hour))
	r.harness = connect(t, New(Options{DefaultSave: r.path, settleTime: settle, Load: func(path string) (*fm.Game, error) {
		r.loads++
		if r.loads <= len(results) && results[r.loads-1] != nil {
			return nil, results[r.loads-1]
		}
		g := fakeGame(path)
		g.Info.Warnings = r.warnings
		return g, nil
	}}))
	return r
}

func (r *reloadHarness) touch(mod time.Time, content string) {
	r.t.Helper()
	writeFile(r.t, r.path, content, mod)
}

// keepWriting touches the save every 5 ms until stop is called.
func (r *reloadHarness) keepWriting() (stop func()) {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			case <-time.After(5 * time.Millisecond):
				now := time.Now()
				_ = os.Chtimes(r.path, now, now)
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// callWhileWriting fails unless the server gives up waiting for a constantly changing save.
func (r *reloadHarness) callWhileWriting() {
	r.t.Helper()
	r.touch(time.Now(), "being written")
	stop := r.keepWriting()
	defer stop()
	loads := r.loads
	if res := r.call("game_overview", nil, nil); r.loads != loads || !strings.Contains(r.notesOf(res), "still being written") {
		r.t.Fatalf("the save settled while being written (loads %d → %d, notes %q); the writer goroutine stalled", loads, r.loads, r.notesOf(res))
	}
}

func TestSaveIsReloadedWhenItChangesOnDisk(t *testing.T) {
	r := newReloadHarness(t)

	r.call("game_overview", nil, nil)
	r.call("game_overview", nil, nil)
	if r.loads != 1 {
		t.Fatalf("loads = %d before any change, want 1", r.loads)
	}

	r.touch(time.Now().Add(-time.Minute), "v2")
	r.call("find_players", map[string]any{"name": "romero"}, nil)
	if r.loads != 2 {
		t.Errorf("loads = %d after the save changed, want 2", r.loads)
	}
}

func TestReloadWaitsUntilTheSaveStopsChanging(t *testing.T) {
	r := newReloadHarness(t)
	r.call("game_overview", nil, nil)

	written := time.Now()
	r.touch(written, "being written")
	r.call("game_overview", nil, nil)

	// The wait counts from the file's modification time.
	if r.loads != 2 || time.Since(written) < testSettle {
		t.Errorf("loads = %d %s after the write, want 2 after at least %s", r.loads, time.Since(written), testSettle)
	}
}

func TestFailedReloadKeepsTheLoadedSave(t *testing.T) {
	r := newReloadHarness(t, nil, errors.New("truncated"))
	r.call("game_overview", nil, nil)

	r.touch(time.Now().Add(-time.Minute), "v2")
	res := r.call("find_players", map[string]any{"name": "romero"}, nil)

	if res.IsError {
		t.Fatalf("find_players failed: %+v", res.Content)
	}
	if notes := r.notesOf(res); !strings.Contains(notes, "could not be reloaded") || !strings.Contains(notes, "truncated") {
		t.Errorf("no reload warning in %q", notes)
	}
	r.call("game_overview", nil, nil)
	if r.loads != 2 {
		t.Errorf("loads = %d, want 2: the same broken file is not retried", r.loads)
	}
}

func TestGameOverviewReportsAFailedReload(t *testing.T) {
	r := newReloadHarness(t, nil, errors.New("truncated"))
	r.call("game_overview", nil, nil)

	r.touch(time.Now().Add(-time.Minute), "v2")
	res := r.call("game_overview", nil, nil)

	if notes := r.notesOf(res); !strings.Contains(notes, "could not be reloaded") {
		t.Errorf("no reload warning in %q", notes)
	}
}

// Some clients show the model only the structured result.
func TestReloadIsAnnouncedInEveryToolsStructuredResult(t *testing.T) {
	r := newReloadHarness(t)
	r.call("game_overview", nil, nil)

	for i, c := range []struct {
		tool string
		args map[string]any
	}{
		{"game_overview", nil},
		{"find_players", map[string]any{"name": "romero"}},
		{"get_player", map[string]any{"uid": 2}},
		{"compare_players", map[string]any{"uids": []any{1, 2}}},
		{"find_clubs", map[string]any{"name": "tottenham"}},
		{"get_squad", nil},
		{"squad_depth", nil},
		{"search_players", map[string]any{"position": "DC"}},
	} {
		r.touch(time.Now().Add(-time.Duration(i+1)*time.Minute), fmt.Sprintf("v%d", i+2))
		res := r.call(c.tool, c.args, nil)
		if res.IsError {
			t.Fatalf("%s: %s", c.tool, allText(res))
		}
		if notes := r.notesOf(res); !strings.Contains(notes, "reloaded") || !strings.Contains(notes, "2025-07-07") {
			t.Errorf("%s: save notes = %q, want the reload with the game date", c.tool, notes)
		}
	}
	if notes := r.notesOf(r.call("find_players", map[string]any{"name": "romero"}, nil)); notes != "" {
		t.Errorf("reload announced twice: %q", notes)
	}
}

func TestSaveThatKeepsChangingIsLoadedOnceItSettles(t *testing.T) {
	r := newReloadHarnessSettling(t, writingSettle)
	r.call("game_overview", nil, nil)

	r.callWhileWriting()

	r.touch(time.Now().Add(-time.Minute), "v2")
	r.call("game_overview", nil, nil)
	if r.loads != 2 {
		t.Errorf("loads = %d after the save settled, want 2", r.loads)
	}
}

func TestFutureModificationTimeDoesNotStall(t *testing.T) {
	r := newReloadHarnessSettling(t, writingSettle)
	r.call("game_overview", nil, nil)

	r.touch(time.Now().Add(time.Hour), "v2")
	start := time.Now()
	r.call("game_overview", nil, nil)

	if r.loads != 2 || time.Since(start) >= writingSettle {
		t.Errorf("loads = %d after %s, want 2 without waiting out the cap", r.loads, time.Since(start))
	}
}

func TestMissingSaveWarnsOnceAndReloadsWhenBack(t *testing.T) {
	r := newReloadHarness(t)
	r.call("game_overview", nil, nil)

	if err := os.Remove(r.path); err != nil {
		t.Fatal(err)
	}
	if notes := r.notesOf(r.call("game_overview", nil, nil)); !strings.Contains(notes, "no longer on disk") {
		t.Errorf("no missing-save note in %q", notes)
	}
	if notes := r.notesOf(r.call("game_overview", nil, nil)); strings.Contains(notes, "no longer on disk") {
		t.Errorf("missing save reported twice: %q", notes)
	}
	r.touch(time.Now().Add(-time.Minute), "v2")
	r.call("game_overview", nil, nil)
	if r.loads != 2 {
		t.Errorf("loads = %d after the save came back, want 2", r.loads)
	}
}

func TestWaitForSettledStopsWhenCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.fm")
	writeFile(t, path, "x", time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := waitForSettled(ctx, path, time.Hour)

	if !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
		t.Errorf("err = %v after %s, want context.Canceled at once", err, time.Since(start))
	}
}

func TestSaveRestoredUnchangedIsReportedWhenGoneAgain(t *testing.T) {
	r := newReloadHarness(t)
	r.call("game_overview", nil, nil)
	fi, err := os.Stat(r.path)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 2 {
		if err := os.Remove(r.path); err != nil {
			t.Fatal(err)
		}
		if notes := r.notesOf(r.call("game_overview", nil, nil)); !strings.Contains(notes, "no longer on disk") {
			t.Errorf("removal %d: no missing-save note in %q", i+1, notes)
		}
		r.touch(fi.ModTime(), "v1")
		r.call("game_overview", nil, nil)
	}
	if r.loads != 1 {
		t.Errorf("loads = %d, want 1: the restored save is unchanged", r.loads)
	}
}

func TestWaitForSettledReportsAMissingFile(t *testing.T) {
	_, err := waitForSettled(context.Background(), filepath.Join(t.TempDir(), "gone.fm"), testSettle)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestOverviewKeepsLoadWarningsOutOfNotes(t *testing.T) {
	r := newReloadHarness(t)
	r.warnings = []string{"names missing"}

	if notes := r.notesOf(r.call("game_overview", nil, nil)); notes != "" {
		t.Errorf("first overview repeats load warnings as notes: %q", notes)
	}
	r.touch(time.Now().Add(-time.Minute), "v2")
	if notes := r.notesOf(r.call("game_overview", nil, nil)); !strings.Contains(notes, "reloaded") || strings.Contains(notes, "names missing") {
		t.Errorf("notes after reload = %q; want the reload note without load warnings", notes)
	}
}

func TestParallelCallsShareOneSettleWait(t *testing.T) {
	r := newReloadHarnessSettling(t, writingSettle)
	r.call("game_overview", nil, nil)

	r.touch(time.Now(), "being written")
	stop := r.keepWriting()
	start := time.Now()
	results := make([]*mcp.CallToolResult, 4)
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			results[i], errs[i] = r.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "find_players", Arguments: map[string]any{"name": "romero"}})
		})
	}
	wg.Wait()
	stop()

	if capped := (maxSettleWaits + 1) * writingSettle; time.Since(start) > 2*capped {
		t.Errorf("4 parallel calls took %s, want under %s (one settle wait)", time.Since(start), 2*capped)
	}
	for i, res := range results {
		if errs[i] != nil || res.IsError || !strings.Contains(r.notesOf(res), "still being written") {
			t.Errorf("call %d: err %v, result %+v; want the earlier copy with a note", i, errs[i], res)
		}
	}
	if r.loads != 1 {
		t.Errorf("loads = %d, want 1", r.loads)
	}
}

func TestNotesReachTheUserWhenACallFails(t *testing.T) {
	r := newReloadHarness(t)
	r.call("game_overview", nil, nil)

	r.touch(time.Now().Add(-time.Minute), "v2")
	res := r.call("get_player", map[string]any{"uid": 99}, nil)

	if !res.IsError || !strings.Contains(allText(res), "reloaded") {
		t.Errorf("error result = %q, want it to carry the reload note", allText(res))
	}
}

// notesOf fails the test on an error or undecodable result, so "no notes" checks cannot pass by accident.
func (h *harness) notesOf(res *mcp.CallToolResult) string {
	h.t.Helper()
	if res.IsError || res.StructuredContent == nil {
		h.t.Fatalf("notesOf: no structured result: %s", allText(res))
	}
	var out struct {
		SaveNotes []string `json:"save_notes"`
	}
	b, err := json.Marshal(res.StructuredContent)
	if err == nil {
		err = json.Unmarshal(b, &out)
	}
	if err != nil {
		h.t.Fatalf("notesOf: %v", err)
	}
	return strings.Join(out.SaveNotes, "; ")
}

func TestLoadSaveWarningsAreNotRepeatedAsNotes(t *testing.T) {
	r := newReloadHarness(t)
	r.warnings = []string{"names missing"}
	r.call("load_save", nil, nil)

	if notes := r.notesOf(r.call("find_players", map[string]any{"name": "romero"}, nil)); notes != "" {
		t.Errorf("notes after load_save = %q; its overview already showed the warnings", notes)
	}
}

func TestFutureModificationTimeIsLoadedRightAfterAGiveUp(t *testing.T) {
	r := newReloadHarnessSettling(t, writingSettle)
	r.call("game_overview", nil, nil)
	r.callWhileWriting()

	r.touch(time.Now().Add(time.Hour), "v2")
	r.call("game_overview", nil, nil)

	if r.loads != 2 {
		t.Errorf("loads = %d, want 2: a future mtime counts as settled even right after a give-up", r.loads)
	}
}

func TestPlayerRowsAndProfilesCarrySeasonStats(t *testing.T) {
	ashby := raw(5, "Chris", "Ashby", 547, 135, 3)
	ashby.Index = 146891
	loanee := raw(6, "Mikey", "Moore", 900, 120, 9)
	loanee.Index = 6
	loanee.Contract = &savefile.Contract{TeamIndex: 547}
	loanee.Loan = &savefile.Contract{TeamIndex: 900}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "s.fm"), "x", gameDate)
	h := connect(t, New(Options{SaveDirs: []string{dir}, Load: func(path string) (*fm.Game, error) {
		return fakeGameWith(path, fm.PlayerExtras{Season: map[uint32]savefile.SeasonRecord{
			146891: {Club: &savefile.MatchStats{RatingSum: 1185, Minutes: 1400, Starts: 16, SubApps: 2, Rated: 18, Assists: 1, PlayerOfTheMatch: 2, RedCards: 1, YellowCards: 7, Detail: savefile.MatchDetail{Passes: 700}}},
			6:      {Club: &savefile.MatchStats{RatingSum: 70, Minutes: 90, Starts: 1, Rated: 1, Goals: 1}},
		}}, ashby, loanee), nil
	}}))

	want := &fm.SeasonStats{Minutes: 1400, PlayerOfTheMatch: 2, Starts: 16, SubApps: 2, Assists: 1, AverageRating: new(6.58), YellowCards: 7, RedCards: 1}
	var squad struct {
		Players   []PlayerSummary `json:"players"`
		OutOnLoan []PlayerSummary `json:"out_on_loan"`
	}
	h.call("get_squad", nil, &squad)
	seasons := map[uint32]*fm.SeasonStats{}
	for _, p := range append(squad.Players, squad.OutOnLoan...) {
		seasons[p.UID] = p.Season
	}
	if !reflect.DeepEqual(seasons, map[uint32]*fm.SeasonStats{1: nil, 2: nil, 5: want, 6: {Starts: 1, Goals: 1, Minutes: 90, AverageRating: new(7.0)}}) {
		t.Errorf("get_squad seasons = %+v", seasons)
	}
	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 5}, &p)
	if !reflect.DeepEqual(p.Season, want) {
		t.Errorf("get_player season = %+v", p.Season)
	}
	if i := slices.IndexFunc(p.SeasonDetail, func(d fm.SeasonDetail) bool { return d.Competition == "club" }); i < 0 {
		t.Errorf("get_player season_detail = %+v, want a club row", p.SeasonDetail)
	}
	var rows struct {
		Players []PlayerSummary `json:"players"`
	}
	var found json.RawMessage
	h.call("find_players", map[string]any{"name": "ashby"}, &found)
	if bytes.Contains(found, []byte("season_detail")) {
		t.Errorf("find_players rows carry season_detail: %s", found)
	}
	if err := json.Unmarshal(found, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows.Players) != 1 || !reflect.DeepEqual(rows.Players[0].Season, want) {
		t.Errorf("find_players rows = %+v", rows.Players)
	}
	h.call("search_players", map[string]any{"team_index": 547, "position": "DC"}, &rows)
	if i := slices.IndexFunc(rows.Players, func(p PlayerSummary) bool { return p.UID == 5 }); i < 0 || !reflect.DeepEqual(rows.Players[i].Season, want) {
		t.Errorf("search_players rows = %+v", rows.Players)
	}
	h.call("search_players", map[string]any{"gender": "any", "sort_by": "season_goals", "min_season_minutes": 1}, &rows)
	if len(rows.Players) != 2 || rows.Players[0].UID != 6 || rows.Players[1].UID != 5 {
		t.Errorf("search_players by season goals with minutes = %+v, want Moore then Ashby", rows.Players)
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "search_players" })
	if b, _ := json.Marshal(tools.Tools[i].InputSchema); !strings.Contains(string(b), "season_average_rating") || !strings.Contains(string(b), "min_season_minutes") {
		t.Errorf("search_players schema lacks the season keys: %s", b)
	}
	if text := h.session.InitializeResult().Instructions; !strings.Contains(text, "season") {
		t.Error("instructions do not explain season stats")
	}
}

func TestPlayerRowsCarryTheNationalTeam(t *testing.T) {
	w := fm.World{Teams: maps.Clone(world.Teams)}
	w.Teams[1233] = &fm.Team{Index: 1233, ClubIndex: -1, Name: "Argentina", National: true}
	romero := raw(2, "Cristian", "Romero", 547, 156, 3)
	romero.NationalTeamRef = 1233
	vicario := raw(1, "Guglielmo", "Vicario", 547, 144, 0)
	vicario.NationalTeamRef = -1
	h := connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{romero, vicario}, w, fm.PlayerExtras{}), nil
	}}))

	var squad struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("get_squad", map[string]any{"team_index": 1233}, &squad)
	if len(squad.Players) != 1 || squad.Players[0].NationalTeam != "Argentina" || squad.Players[0].NationalTeamIndex == nil || *squad.Players[0].NationalTeamIndex != 1233 || squad.Players[0].TeamIndex != 547 {
		t.Errorf("Argentina squad = %+v", squad.Players)
	}
	got := h.rawJSON("find_players", map[string]any{"name": "vicario"})
	if !strings.Contains(got, `"name":"Guglielmo Vicario"`) || strings.Contains(got, "national_team") {
		t.Errorf("a player without a call-up = %s", got)
	}
}

func TestClubSearchListsNationalTeamsLeftOut(t *testing.T) {
	w := fm.World{Teams: maps.Clone(world.Teams)}
	var players []savefile.Player
	for i := range 4 * maxNationalTeams {
		index := int64(1233 + i)
		w.Teams[index] = &fm.Team{Index: index, ClubIndex: -1, Name: fmt.Sprintf("Tottenham Hotspur Nationals %d", i), National: true, Reputation: 1000 - i}
		p := raw(uint32(100+i), "Called", "Up", 547, 100, 3) //nolint:gosec // small test values
		p.NationalTeamRef = int32(index)                     //nolint:gosec // small test values
		players = append(players, p)
	}
	h := connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: 547}, players, w, fm.PlayerExtras{}), nil
	}}))

	var out searchOut
	h.call("search_players", map[string]any{"club": "tottenham"}, &out)
	var got []int64
	for _, team := range out.NationalTeams {
		got = append(got, team.Index)
	}
	if !slices.Equal(got, []int64{1233, 1234, 1235, 1236, 1237}) || out.NationalTeams[0].Name != "Tottenham Hotspur Nationals 0" {
		t.Errorf("national_teams = %v, want the %d most reputable in order", got, maxNationalTeams)
	}
}

func TestClubSearchWithoutPlayersCarriesAHint(t *testing.T) {
	w := fm.World{Teams: maps.Clone(world.Teams)}
	w.Teams[91] = &fm.Team{Index: 91, ClubIndex: 41, Name: "Chicago Magic PSG U18", Reputation: 10}
	w.Teams[92] = &fm.Team{Index: 92, ClubIndex: 42, Name: "Chile Women", Women: true, Reputation: 10}
	h := connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(string) (*fm.Game, error) {
		return fm.NewGame(fm.Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{raw(1, "Some", "Spur", 547, 100, 3)}, w, fm.PlayerExtras{}), nil
	}}))

	var out searchOut
	h.call("search_players", map[string]any{"club": "psg"}, &out)
	if !strings.Contains(out.Hint, "Chicago Magic PSG U18 (team_index 91)") || !strings.Contains(out.Hint, "find_clubs") {
		t.Errorf("hint = %q, want the matched team and the find_clubs advice", out.Hint)
	}
	out = searchOut{}
	h.call("search_players", map[string]any{"club": "chile women", "gender": "men"}, &out)
	if !strings.Contains(out.Hint, "women's teams") {
		t.Errorf("hint = %q, want the gender mismatch", out.Hint)
	}
	if got := h.rawJSON("search_players", map[string]any{"club": "tottenham"}); strings.Contains(got, `"hint"`) {
		t.Errorf("a search with players = %s, want no hint", got)
	}
}

func TestInstructionsCoverSeasonStatsAndNationalTeams(t *testing.T) {
	text := newHarness(t).session.InitializeResult().Instructions

	for _, want := range []string{
		"season (find_players, get_squad and search_players rows, get_player and compare_players)",
		"unless game_overview or save_notes warn that season stats are unavailable: then a missing season means unknown",
		"The server never hides season stats under fog of war",
		"season_detail",
		"count × 90 / minutes",
		"continental counts all continental cups together",
		"internationals (the senior national team's matches, probably those of this international season; club rows exclude them)",
		"drawn = starts + sub_appearances - matches_won - matches_lost",
		`Goalkeepers (GK rating 15 or more) have "goalkeeper": true`,
		"instead of tackles/tackles_won, headers/headers_won, blocks and open_play_crosses_completed",
		"headers_lost is not recorded for goalkeepers",
		"non_penalty_expected_goals",
		"expected_goals_prevented",
		"get_squad, squad_depth and search_players by team_index treat a called-up player as a player of both teams",
		"club filter matches club teams only",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if strings.Contains(text, "international matches are not included") {
		t.Error("instructions say season_detail has no internationals")
	}
	if strings.Contains(text, "every player row") || strings.Contains(text, "Fog of war never hides season stats") {
		t.Error("instructions claim season on every row (squad_depth has none) or speak for the game's fog of war")
	}
	guesses := guessedFields(text)
	if guesses == "" {
		t.Error("instructions lack the list of guessed fields")
	} else if strings.Contains(guesses, "goal") || strings.Contains(guesses, "assist") {
		t.Errorf("guessed fields %q include season goals or assists; they are verified in game", guesses)
	}
	for _, gone := range []string{"shots_on_target_faced", "npxg", "xg_prevented", "saves (", "matches this international season"} {
		if strings.Contains(text, gone) {
			t.Errorf("instructions still mention %q", gone)
		}
	}
}

// guessedFields returns the instructions' sentence listing guessed fields,
// without its lead-in; "" when there is none.
func guessedFields(instructions string) string {
	_, guesses, _ := strings.Cut(instructions, "These fields are guesses")
	guesses, _, _ = strings.Cut(guesses, ". ")
	return guesses
}

func TestPlayingTimeOnProfilesAndSquadRowsOnly(t *testing.T) {
	h := newHarness(t)

	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 2}),
		h.rawJSON("compare_players", map[string]any{"uids": []uint32{1, 2}}),
		h.rawJSON("get_squad", nil),
	} {
		for _, field := range []string{`"playing_time":"Squad Player"`, `"original_playing_time":"Important Player"`, `"actual_playing_time":"Surplus to Requirements"`, `"squad_number":17`} {
			if !strings.Contains(got, field) {
				t.Errorf("no %s: %s", field, got)
			}
		}
	}
	for _, got := range []string{
		h.rawJSON("find_players", map[string]any{"name": "romero"}),
		h.rawJSON("search_players", map[string]any{"team_index": 547}),
	} {
		if strings.Contains(got, "playing_time") || strings.Contains(got, "squad_number") {
			t.Errorf("row carries playing time or squad number: %s", got)
		}
	}
	text := h.session.InitializeResult().Instructions
	for _, want := range []string{
		`playing_time (get_player, compare_players, get_squad) is FM's Playing Time: the squad status the club has agreed with the player, as squad lists show it and the contract screen's "Agreed" line.`,
		`actual_playing_time is the contract screen's headline: how his game time currently rates`,
		`original_playing_time is the status agreed when the contract was signed; FM does not show it.`,
		`For a player on loan, all three come from his loan contract.`,
		`actual_playing_time is left out for players away from their current contract's team or at a team that is not a club's first team`,
		`The game's adjustments for youth contracts, playing-time promises and team settings are not applied, so a served actual_playing_time can still differ from FM's.`,
		`squad_number (get_player, compare_players, get_squad) is the number registered with his current contract's team (his loan club while on loan). get_squad leaves it out for players contracted to another team (e.g. first-team players in a reserve squad) and in national-team squads; in its out_on_loan rows it is the number at the club whose squad it is. Fog of war does not hide it.`,
		`shirt_number (get_last_lineup) is the number worn in that match and can differ from squad_number.`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if guesses := guessedFields(text); strings.Contains(guesses, "agreed playing time") || !strings.Contains(guesses, "actual playing time (checked in game for 3 first-team outfield players; the others are predictions of the decoded rule)") {
		t.Errorf("guessed fields = %q, want actual playing time with its in-game evidence and not agreed playing time", guesses)
	}
	if !strings.Contains(text, `"partial" hides current and potential ability, wage, clauses, playing time, loyalty bonus, hidden attributes, personality, media handling and favoured clubs; "required" also hides attributes, traits and value`) {
		t.Error("instructions do not say fog of war hides playing time")
	}
}

func TestPersonalityOnProfilesAndSquadRowsAndTraitsOnProfilesOnly(t *testing.T) {
	h := newHarness(t)

	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 2}),
		h.rawJSON("compare_players", map[string]any{"uids": []uint32{1, 2}}),
	} {
		if !strings.Contains(got, `"personality":"Fairly Professional"`) || !strings.Contains(got, `"traits":["Argues With Officials","Dives Into Tackles"]`) ||
			!strings.Contains(got, `"Professionalism":15`) {
			t.Errorf("no personality, traits or hidden attributes: %s", got)
		}
	}
	squad := h.rawJSON("get_squad", nil)
	if !strings.Contains(squad, `"personality":"Fairly Professional"`) || strings.Contains(squad, `"traits"`) {
		t.Errorf("get_squad rows want personality, no traits: %s", squad)
	}
	for _, got := range []string{
		h.rawJSON("find_players", map[string]any{"name": "romero"}),
		h.rawJSON("search_players", map[string]any{"team_index": 547}),
	} {
		if strings.Contains(got, `"personality"`) || strings.Contains(got, `"traits"`) {
			t.Errorf("row carries personality or traits: %s", got)
		}
	}
	text := h.session.InitializeResult().Instructions
	if !strings.Contains(text, "A contract's start is when its current terms were signed (the contract screen's Start); joined is when the player joined that team. A player's joined_club is when he joined his current club, as his profile shows it; it can be later than his contract's joined.") {
		t.Error("instructions do not define a contract's start and joined dates")
	}
	if !strings.Contains(text, "personality (get_player, compare_players, get_squad) is FM's personality label") || !strings.Contains(text, "traits (get_player, compare_players) lists the player's traits") {
		t.Error("instructions do not explain personality and traits")
	}
	if g := guessedFields(text); strings.Contains(g, "personality") {
		t.Errorf("guessed fields %q list personality, which is checked in game", g)
	}
}

func TestReputationLevelAndMediaHandlingOnProfilesAndSquadRows(t *testing.T) {
	h := newHarness(t)

	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 2}),
		h.rawJSON("compare_players", map[string]any{"uids": []uint32{1, 2}}),
		h.rawJSON("get_squad", nil),
	} {
		if !strings.Contains(got, `"reputation_level":"Local"`) || !strings.Contains(got, `"media_handling":["Media-friendly"]`) {
			t.Errorf("no reputation level or media handling: %s", got)
		}
	}
	for _, got := range []string{
		h.rawJSON("find_players", map[string]any{"name": "romero"}),
		h.rawJSON("search_players", map[string]any{"team_index": 547}),
	} {
		if strings.Contains(got, `"reputation_level"`) || strings.Contains(got, `"media_handling"`) {
			t.Errorf("row carries reputation level or media handling: %s", got)
		}
	}
	text := h.session.InitializeResult().Instructions
	for _, want := range []string{
		"reputation_level (get_player, compare_players, get_squad) is the reputation word on FM's player profile",
		"and for everyone when game_overview warns that reputation levels are unavailable",
		"media_handling (get_player, compare_players, get_squad) lists FM's media handling styles",
		"hidden attributes, personality, media handling and favoured clubs;",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	if g := guessedFields(text); !strings.Contains(g, "reputation_level words Regional and Obscure") || !strings.Contains(g, "reputation_level of players without a team") || !strings.Contains(g, "media_handling (for some players the game may show") {
		t.Errorf("guessed fields %q lack reputation_level and media_handling", g)
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "get_player" && (!strings.Contains(tool.Description, "media handling") || !strings.Contains(tool.Description, "reputation level")) {
			t.Errorf("get_player description %q", tool.Description)
		}
	}
}

func TestFavouredClubsOnProfilesOnly(t *testing.T) {
	h := newHarness(t)

	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 2}),
		h.rawJSON("compare_players", map[string]any{"uids": []uint32{1, 2}}),
	} {
		if !strings.Contains(got, `"favoured_clubs":[{"as":"supporter","club":"Spurs","team_index":547}]`) {
			t.Errorf("no favoured clubs: %s", got)
		}
	}
	for _, got := range []string{
		h.rawJSON("get_squad", nil),
		h.rawJSON("find_players", map[string]any{"name": "romero"}),
		h.rawJSON("search_players", map[string]any{"team_index": 547}),
	} {
		if strings.Contains(got, "favoured_clubs") {
			t.Errorf("row carries favoured clubs: %s", got)
		}
	}
	text := h.session.InitializeResult().Instructions
	if !strings.Contains(text, `favoured_clubs (get_player, compare_players) lists the clubs the player favours (FM's Favoured Clubs) by short name, with team_index (the club's first team, when it has one) and as "supporter" or "player" when known`) {
		t.Error("instructions do not explain favoured_clubs")
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "get_player" && !strings.Contains(tool.Description, "favoured clubs") {
			t.Errorf("get_player description: %s", tool.Description)
		}
	}
}

func TestLoyaltyBonusOnProfilesOnly(t *testing.T) {
	h := newHarness(t)

	// Romero: £1 000 a week for the 207 whole weeks from 2025-07-07 to 2029-06-30.
	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 2}),
		h.rawJSON("compare_players", map[string]any{"uids": []uint32{1, 2}}),
	} {
		if !strings.Contains(got, `"loyalty_bonus_remaining":207000`) {
			t.Errorf("no loyalty bonus: %s", got)
		}
	}
	for _, got := range []string{
		h.rawJSON("get_player", map[string]any{"uid": 1}),
		h.rawJSON("find_players", map[string]any{"name": "romero"}),
		h.rawJSON("search_players", map[string]any{"team_index": 547}),
		h.rawJSON("get_squad", nil),
	} {
		if strings.Contains(got, "loyalty_bonus") {
			t.Errorf("carries a loyalty bonus: %s", got)
		}
	}
	text := h.session.InitializeResult().Instructions
	if !strings.Contains(text, "loyalty_bonus_remaining (get_player, compare_players) is the main contract's loyalty bonus still to be paid") {
		t.Error("instructions do not explain loyalty_bonus_remaining")
	}
	if strings.Contains(strings.ReplaceAll(guessedFields(text), "reputation_level", ""), "reputation") {
		t.Error("instructions list the home, current and world reputation labels as a guess; they are verified by code")
	}
}

// tacticsHarness serves fakeGame with tactics and three Spurs players: Sami
// (person 7), Ly (person 8) and Forster (person 9).
func tacticsHarness(t *testing.T, tactics *savefile.Tactics) *harness {
	t.Helper()
	sami := raw(7, "Bruno", "Sami", 547, 140, 0)
	sami.Index = 7
	ly := raw(8, "Stephen", "Ly", 547, 130, 12)
	ly.Index = 8
	forster := raw(9, "Fraser", "Forster", 547, 120, 0)
	forster.Index = 9
	return connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		return fakeGameWith(path, fm.PlayerExtras{Tactics: tactics}, sami, ly, forster), nil
	}}))
}

// spursLineup holds Sami in goal, Ly up front and Forster first on the bench.
func spursLineup() [26]int32 {
	var l [26]int32
	for i := range l {
		l[i] = -1
	}
	l[0], l[10], l[11] = 7, 8, 9
	return l
}

// tacticHarness serves a tactic with spursLineup selected.
func tacticHarness(t *testing.T) *harness {
	t.Helper()
	var slots [11]savefile.TacticSlot
	slots[0] = savefile.TacticSlot{
		InPossession:    savefile.SlotRole{Position: 0x1, Role: 0x401000},
		OutOfPossession: savefile.SlotRole{Position: 0x1, Role: 0x400001},
	}
	slots[10] = savefile.TacticSlot{
		InPossession:    savefile.SlotRole{Position: 0x4000, Role: 0x80800000},
		OutOfPossession: savefile.SlotRole{Position: 0x2000, Role: 0x40400000},
	}
	return tacticsHarness(t, &savefile.Tactics{Team: 547, Selection: spursLineup(), Tactics: []savefile.Tactic{{Name: "ZaZ - Autumn 3.39 ST", Mentality: 6, Slots: slots}}})
}

func TestGetTactic(t *testing.T) {
	h := tacticHarness(t)

	var got fm.Tactic
	if res := h.call("get_tactic", nil, &got); res.IsError {
		t.Fatalf("get_tactic: %s", allText(res))
	}

	if got.TeamIndex != 547 || got.Team != "Tottenham Hotspur" || got.Name != "ZaZ - Autumn 3.39 ST" || got.Mentality != "Attacking" || got.LineupSource != "selected" || len(got.Slots) != 11 {
		t.Fatalf("get_tactic = %+v", got)
	}
	want := []fm.TacticSlot{
		{Slot: 0,
			InPossession:    fm.SlotRole{Position: "GK", Role: "Ball-Playing Goalkeeper"},
			OutOfPossession: fm.SlotRole{Position: "GK", Role: "Goalkeeper"},
			Player:          &fm.TacticPlayer{UID: 7, Name: "Bruno Sami"},
			RoleAbility:     ptr(140)},
		{Slot: 10,
			InPossession:    fm.SlotRole{Position: "ST", Role: "Channel Forward"},
			OutOfPossession: fm.SlotRole{Position: "AMC", Role: "Attacking Midfielder"},
			Player:          &fm.TacticPlayer{UID: 8, Name: "Stephen Ly"},
			RoleAbility:     ptr(130)},
	}
	if gotSlots := []fm.TacticSlot{got.Slots[0], got.Slots[10]}; !reflect.DeepEqual(gotSlots, want) {
		t.Errorf("slots 0 and 10 = %+v, want %+v", gotSlots, want)
	}
	if !reflect.DeepEqual(got.Bench, []fm.TacticPlayer{{UID: 9, Name: "Fraser Forster"}}) {
		t.Errorf("bench = %+v", got.Bench)
	}
	if text := h.rawJSON("get_tactic", nil); strings.Contains(text, "knowledge") {
		t.Errorf("get_tactic carries fog of war: %s", text)
	}
}

func TestGetTacticWithoutATacticIsAnError(t *testing.T) {
	h := newHarness(t)

	if res := h.call("get_tactic", nil, nil); !res.IsError || !strings.Contains(allText(res), "no tactic") {
		t.Errorf("get_tactic = %s, want a no-tactic error", allText(res))
	}
}

func TestGetTacticIsExplained(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "get_tactic" })
	if i < 0 {
		t.Fatal("no get_tactic tool")
	}
	for _, want := range []string{"lineup_source", "search_players", "squad_depth", "guesses", lastMatchCaveat, `"none"`, "may belong to another of his teams", "roles FM26 renamed", "unnamed_player"} {
		if !strings.Contains(tools.Tools[i].Description, want) {
			t.Errorf("get_tactic description lacks %q", want)
		}
	}
	text := h.session.InitializeResult().Instructions
	for _, want := range []string{"get_tactic", `"last_match"`, "in_possession", "out_of_possession", lastMatchCaveat, `"none"`, "may belong to another of his teams"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
	guessed := guessedFields(text)
	for _, renamed := range []string{"Inside Wing-Back", "Ball-Playing Centre-Back", "Free Role", "Second Striker", "Line-Holding Goalkeeper", "Inside Outlet Winger"} {
		if !strings.Contains(guessed, renamed) {
			t.Errorf("instructions do not list the renamed role %q as a guess", renamed)
		}
	}
	for _, verified := range []string{"mentality", "dut"} {
		if strings.Contains(guessed, verified) {
			t.Errorf("instructions list %q as a guess", verified)
		}
	}
}

// lastMatchCaveat is the get_tactic description's and instructions' caveat on
// last-match slots.
const lastMatchCaveat = "placing the last match's players in this tactic's slots assumes that match used this tactic"

// lastLineupHarness serves Spurs' last match against Chelsea with
// spursLineup in a 4-4-2, shirt numbers Sami 88, Ly 1 and Forster 30, and
// person 50 (not a player) on the bench. The manager's tactic is for
// another team.
func lastLineupHarness(t *testing.T) *harness {
	t.Helper()
	f := &savefile.Formation{Mentality: 4}
	ip := []uint32{0x1, 0x4, 0x10, 0x10, 0x8, 0x100, 0x400, 0x400, 0x200, 0x4000, 0x4000}
	for i := range f.Slots {
		f.Slots[i] = savefile.TacticSlot{
			InPossession:    savefile.SlotRole{Position: ip[i], Role: 0x2},
			OutOfPossession: savefile.SlotRole{Position: ip[i], Role: 0x8},
		}
	}
	f.Slots[10].InPossession.Role = 0x80000 // Poacher
	players := spursLineup()
	players[12] = 50 // not a player
	var shirts [26]uint8
	shirts[0], shirts[10], shirts[11], shirts[12] = 88, 1, 30, 255
	return tacticsHarness(t, &savefile.Tactics{Team: 20739, LastLineups: map[int32]savefile.LastLineup{
		547: {Opponent: 20739, Players: players, ShirtNumbers: shirts, Formation: f},
	}})
}

func TestGetLastLineup(t *testing.T) {
	h := lastLineupHarness(t)

	for _, args := range []map[string]any{{"team_index": 547}, nil} { // nil: the manager's first team
		var got fm.LastLineup
		if res := h.call("get_last_lineup", args, &got); res.IsError {
			t.Fatalf("get_last_lineup %v: %s", args, allText(res))
		}
		if got.TeamIndex != 547 || got.OpponentIndex != 20739 || got.Opponent != "Chelsea" || got.Mentality != "Balanced" ||
			got.InPossessionShape != "4-4-2" || got.OutOfPossessionShape != "4-4-2" || len(got.Slots) != 11 {
			t.Fatalf("get_last_lineup %v = %+v", args, got)
		}
		st := got.Slots[10]
		if st.InPossession != (fm.SlotRole{Position: "ST", Role: "Poacher"}) || st.Player == nil || st.Player.Name != "Stephen Ly" || st.Player.ShirtNumber != 1 {
			t.Errorf("slot 10 = %+v, player %+v", st, st.Player)
		}
		if !reflect.DeepEqual(got.Bench, []fm.TacticPlayer{{UID: 9, Name: "Fraser Forster", ShirtNumber: 30}, {Unnamed: true}}) {
			t.Errorf("bench = %+v", got.Bench)
		}
	}
	if text := h.rawJSON("get_last_lineup", map[string]any{"team_index": 547}); strings.Contains(text, "knowledge") || strings.Contains(text, "duty") {
		t.Errorf("get_last_lineup carries fog of war or duties: %s", text)
	}
}

func TestGetLastLineupErrors(t *testing.T) {
	h := lastLineupHarness(t)

	for _, tt := range []struct {
		team int
		want string
	}{
		{99999, "no team with team_index 99999"},
		{20739, "no last match for team_index 20739"},
	} {
		if res := h.call("get_last_lineup", map[string]any{"team_index": tt.team}, nil); !res.IsError || !strings.Contains(allText(res), tt.want) || strings.Contains(allText(res), "game_overview") {
			t.Errorf("team %d: %s, want an error containing %q and no game_overview hint", tt.team, allText(res), tt.want)
		}
	}

	unread := tacticsHarness(t, nil)
	if res := unread.call("get_last_lineup", map[string]any{"team_index": 547}, nil); !res.IsError ||
		!strings.Contains(allText(res), "could not be read") || !strings.Contains(allText(res), "game_overview") {
		t.Errorf("unreadable last lineups: %s, want an error pointing at game_overview's warnings", allText(res))
	}
}

func TestGetLastLineupIsExplained(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "get_last_lineup" })
	if i < 0 {
		t.Fatal("no get_last_lineup tool")
	}
	for _, want := range []string{"lineup, formation and roles", "most recent match", "who it played", "no duty", "date is not known", "Fog of war does not hide it",
		"shirt_number", "unnamed_player", "Names of roles FM26 renamed are guesses"} {
		if !strings.Contains(tools.Tools[i].Description, want) {
			t.Errorf("get_last_lineup description lacks %q", want)
		}
	}
	text := h.session.InitializeResult().Instructions
	for _, want := range []string{"get_last_lineup", "opponent_team_index", "in_possession_shape", "no duty", "date is not known", "shirt_number", "unnamed_player"} {
		if !strings.Contains(text, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
}

func TestSearchPlayersInterest(t *testing.T) {
	const save = "../../data/test_save.fm"
	if _, err := os.Stat(save); err != nil {
		t.Skipf("test save not available: %v", err)
	}
	h := connect(t, New(Options{DefaultSave: save}))

	var out struct {
		Players []struct {
			Interest *fm.Interest `json:"interest"`
		} `json:"players"`
		Total           int `json:"total"`
		InterestDropped *struct {
			Total    int            `json:"total"`
			ByReason map[string]int `json:"by_reason"`
		} `json:"interest_dropped"`
	}
	args := map[string]any{"interest": "transfer", "min_current_ability": 100, "limit": 50}
	raw := h.rawJSON("search_players", args)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out.InterestDropped == nil {
		t.Fatalf("no interest_dropped in %s", raw)
	}
	if len(out.Players) == 0 {
		t.Fatal("no players")
	}
	for i, p := range out.Players {
		if p.Interest != nil && p.Interest.Transfer == fm.VerdictOut {
			t.Errorf("row %d is out for a transfer: %+v", i, p.Interest)
		}
	}
	if !slices.ContainsFunc(out.Players, func(p struct {
		Interest *fm.Interest `json:"interest"`
	}) bool {
		return p.Interest != nil
	}) {
		t.Error("no row carries interest")
	}
	if got := h.rawJSON("search_players", map[string]any{"limit": 1}); strings.Contains(got, "interest") {
		t.Errorf("a search without the option mentions interest: %s", got)
	}
	res := h.call("search_players", map[string]any{"interest": "bogus"}, nil)
	if !res.IsError {
		t.Error("interest bogus was accepted")
	}
}

func TestGetPlayerCarriesInterest(t *testing.T) {
	path := os.Getenv("FM_REAL_SAVE_16JAN")
	if path == "" {
		t.Skip("FM_REAL_SAVE_16JAN not set")
	}
	g, err := fm.Load(path, fm.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "real.fm"), "x", time.Now())
	h := connect(t, New(Options{SaveDirs: []string{dir}, Load: func(string) (*fm.Game, error) { return g, nil }}))
	var p fm.Profile
	if res := h.call("get_player", map[string]any{"uid": 2002115699}, &p); res.IsError { // Marco
		t.Fatalf("get_player: %+v", res.Content)
	}
	if p.Interest == nil || p.Interest.TransferReason == nil || p.Interest.LoanReason == nil {
		t.Fatalf("interest = %+v", p.Interest)
	}
	for _, r := range []*fm.InterestFactor{p.Interest.TransferReason, p.Interest.LoanReason} {
		if r.Code != "transfer_arranged" || !strings.Contains(r.Text, "Bari") {
			t.Errorf("reason = %+v, want transfer_arranged naming Bari", r)
		}
	}
}

func TestInterestIsExplained(t *testing.T) {
	h := newHarness(t)
	var paragraph string
	for line := range strings.SplitSeq(h.session.InitializeResult().Instructions, "\n") {
		if strings.HasPrefix(line, "interest") {
			paragraph = line
		}
	}
	for _, want := range []string{
		"only_just_joined", "transfer_arranged", "retiring", "only_one_club", "recently_left_your_club", "club_reputation_gap",
		"his own reputation", "never likely", "game_overview", "warnings", "own players", "other gender", "club is unknown",
		"transfer_reason", "loan_reason", "transfer_factors", "loan_factors", "estimate",
	} {
		if !strings.Contains(paragraph, want) {
			t.Errorf("instructions' interest paragraph lacks %q", want)
		}
	}
	if strings.Contains(paragraph, "always carry interest") {
		t.Errorf("instructions say interest is always present: %s", paragraph)
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"get_player", "compare_players"} {
		i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == name })
		for _, want := range []string{"transfer_reason", "loan_reason", "transfer_factors", "loan_factors"} {
			if !strings.Contains(tools.Tools[i].Description, want) {
				t.Errorf("%s description lacks %q", name, want)
			}
		}
	}
}

func TestSearchRowsCarryJoinedClub(t *testing.T) {
	recent := raw(5, "Chris", "Ashby", 547, 135, 3)
	recent.JoinedClub = savefile.Date{Raw: 1, Year: 2025} // 2025-01-01
	h := harnessWith(t, recent)
	var rows struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("search_players", map[string]any{"team_index": 547, "min_months_at_club": 6}, &rows)
	if len(rows.Players) != 1 || rows.Players[0].UID != 5 || rows.Players[0].JoinedClub != "2025-01-01" {
		t.Errorf("rows = %+v, want only Ashby with joined_club 2025-01-01", rows.Players)
	}
}

func TestSearchSortsByPer90AndRowsCarryIt(t *testing.T) {
	ashby := raw(5, "Chris", "Ashby", 547, 135, 3)
	ashby.Index = 5
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "s.fm"), "x", gameDate)
	h := connect(t, New(Options{SaveDirs: []string{dir}, Load: func(path string) (*fm.Game, error) {
		return fakeGameWith(path, fm.PlayerExtras{Season: map[uint32]savefile.SeasonRecord{
			5: {Club: &savefile.MatchStats{Starts: 10, Minutes: 900, Detail: savefile.MatchDetail{ProgressivePasses: 45}}},
		}}, ashby), nil
	}}))
	var rows struct {
		Players []PlayerSummary `json:"players"`
	}
	h.call("search_players", map[string]any{"sort_by": "per90_progressive_passes", "min_per90": map[string]any{"progressive_passes": 4}}, &rows)
	if len(rows.Players) != 1 || rows.Players[0].UID != 5 || !reflect.DeepEqual(rows.Players[0].Per90, map[string]float64{"progressive_passes": 4.5}) {
		t.Errorf("rows = %+v, want Ashby with progressive_passes 4.5 per 90", rows.Players)
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "search_players" })
	if b, _ := json.Marshal(tools.Tools[i].InputSchema); !strings.Contains(string(b), "per90_") || !strings.Contains(string(b), "min_per90") {
		t.Errorf("search_players schema lacks the per-90 options: %s", b)
	}
}

func detailHarness(t *testing.T) *harness {
	t.Helper()
	ashby := raw(5, "Chris", "Ashby", 900, 135, 3)
	ashby.Index = 5
	ashby.Positions = [15]uint8{1, 9, 10, 20}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "s.fm"), "x", gameDate)
	return connect(t, New(Options{SaveDirs: []string{dir}, FogOfWarOff: true, Load: func(path string) (*fm.Game, error) {
		return fakeGameWith(path, fm.PlayerExtras{Season: map[uint32]savefile.SeasonRecord{
			5: {Club: &savefile.MatchStats{Starts: 10, Minutes: 900}, League: &savefile.MatchStats{Starts: 8, Minutes: 720}},
		}}, ashby), nil
	}}))
}

func competitions(p fm.Profile) []string {
	var out []string
	for _, d := range p.SeasonDetail {
		out = append(out, d.Competition)
	}
	return out
}

func TestProfilesLeaveOutPositionRatingsBelowTen(t *testing.T) {
	h := detailHarness(t)
	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 5}, &p)
	want := []fm.PositionRating{{Position: "DL", Rating: 10}, {Position: "DC", Rating: 20}}
	if !reflect.DeepEqual(p.Positions, want) {
		t.Errorf("positions = %+v, want %+v", p.Positions, want)
	}
}

func TestProfileDetailLevels(t *testing.T) {
	h := detailHarness(t)
	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 5}, &p)
	if got := competitions(p); !slices.Equal(got, []string{"club", "league"}) {
		t.Errorf("get_player season_detail = %v, want club and league by default", got)
	}
	h.call("get_player", map[string]any{"uid": 5, "detail": "summary"}, &p)
	if got := competitions(p); !slices.Equal(got, []string{"club"}) {
		t.Errorf("summary season_detail = %v, want club only", got)
	}

	var cmp struct {
		Players []fm.Profile `json:"players"`
	}
	h.call("compare_players", map[string]any{"uids": []int{5}}, &cmp)
	if got := competitions(cmp.Players[0]); !slices.Equal(got, []string{"club"}) {
		t.Errorf("compare_players season_detail = %v, want club only by default", got)
	}
	h.call("compare_players", map[string]any{"uids": []int{5}, "detail": "full"}, &cmp)
	if got := competitions(cmp.Players[0]); !slices.Equal(got, []string{"club", "league"}) {
		t.Errorf("compare_players full season_detail = %v", got)
	}
	if res := h.call("get_player", map[string]any{"uid": 5, "detail": "everything"}, nil); !res.IsError {
		t.Error("get_player accepted detail \"everything\"")
	}
}

func TestSummaryInterestKeepsVerdictsAndReasons(t *testing.T) {
	factor := fm.InterestFactor{Code: "club_reputation_gap", Text: "x"}
	in := fm.Profile{Interest: &fm.Interest{Transfer: fm.VerdictOut, TransferReason: &factor, TransferFactors: []fm.InterestFactor{factor}, LoanFactors: []fm.InterestFactor{factor}}}
	got, err := detailSummary.apply(in)
	if err != nil {
		t.Fatal(err)
	}
	want := &fm.Interest{Transfer: fm.VerdictOut, TransferReason: &factor}
	if !reflect.DeepEqual(got.Interest, want) {
		t.Errorf("summary interest = %+v, want %+v", got.Interest, want)
	}
	if in.Interest.TransferFactors == nil {
		t.Error("summary changed the profile it was given")
	}
}

func TestSearchByHeightTraitsPersonalityReleaseClauseAndRoleAbility(t *testing.T) {
	tall := raw(5, "Chris", "Ashby", 547, 135, 3)
	tall.Height = 194
	tall.Traits = 1 << 17 // Argues With Officials
	tall.Contract = &savefile.Contract{TeamIndex: 547, Clauses: []savefile.Clause{{Type: 0, Value: 20_000_000, Extra: 0xffff}}}
	h := harnessWith(t, tall)
	h.call("set_fog_of_war", map[string]any{"enabled": false}, nil)
	var rows struct {
		Players []PlayerSummary `json:"players"`
	}
	for _, args := range []map[string]any{
		{"min_height_cm": 190},
		{"traits": []string{"Argues With Officials"}, "max_release_clause": 30_000_000},
		{"personalities": []string{"Balanced"}, "min_height_cm": 190},
		{"position": "DC", "role": "Overlapping Centre-Back", "min_role_ability": 1, "min_height_cm": 190},
	} {
		args["team_index"] = 547
		h.call("search_players", args, &rows)
		if len(rows.Players) != 1 || rows.Players[0].UID != 5 {
			t.Errorf("search_players %v = %+v, want only Ashby", args, rows.Players)
		}
	}
	if res := h.call("search_players", map[string]any{"traits": []string{"Shoots A Lot"}}, nil); !res.IsError {
		t.Error("search_players accepted an unknown trait")
	}
}

func TestTrimmingProfilesLeavesTheGameUnchanged(t *testing.T) {
	h := detailHarness(t)
	for range 2 {
		var p fm.Profile
		h.call("get_player", map[string]any{"uid": 2, "detail": "summary"}, &p) // Romero: only DC rated
		if len(p.Positions) != 1 {
			t.Fatalf("summary positions = %+v", p.Positions)
		}
	}
	var depth depthOut
	if res := h.call("squad_depth", map[string]any{"team_index": 547}, &depth); res.IsError {
		t.Fatalf("squad_depth: %+v", res.Content)
	}
	i := slices.IndexFunc(depth.Positions, func(d DepthPosition) bool { return d.Position == "DC" })
	if i < 0 || !slices.ContainsFunc(depth.Positions[i].Players, func(p DepthPlayer) bool { return p.UID == 2 && p.Rating == 20 }) {
		t.Errorf("squad_depth after trimmed profiles = %+v, want Romero at DC rated 20", depth.Positions)
	}
	for range 2 {
		var p fm.Profile
		h.call("get_player", map[string]any{"uid": 5, "detail": "summary"}, &p)
	}
	var p fm.Profile
	h.call("get_player", map[string]any{"uid": 5, "detail": "full"}, &p)
	if len(p.SeasonDetail) != 2 {
		t.Errorf("full season_detail after summaries = %+v, want club and league", p.SeasonDetail)
	}
}

func TestDetailLevelsAreExplained(t *testing.T) {
	h := newHarness(t)
	text := h.session.InitializeResult().Instructions
	if !strings.Contains(text, `compare_players defaults to detail "summary"`) {
		t.Error("instructions do not say compare_players defaults to summary")
	}
	if !strings.Contains(text, `compare_players with detail "full"`) {
		t.Error("instructions' interest paragraph does not say compare_players needs full for the factors")
	}
	if !strings.Contains(text, "a stat his side does not record") {
		t.Error("instructions do not say per-90 of the other side's stats is missing")
	}
	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"get_player", "compare_players"} {
		i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == name })
		if i < 0 {
			t.Fatalf("no %s tool", name)
		}
		if !strings.Contains(tools.Tools[i].Description, "below 10") {
			t.Errorf("%s description does not say ratings below 10 are left out", name)
		}
	}
}

// fixturesHarness serves fakeGame with a Premier League (COMP 1, key 0)
// result and fixture for Tottenham.
func fixturesHarness(t *testing.T) *harness {
	t.Helper()
	w := world
	w.Competitions = map[int64]string{1: "Premier League", 2: "Scottish Premier League"}
	w.CompetitionUIDs = []uint32{10, 11, 12}
	w.Keys = []savefile.CompetitionKey{{Comp: 1, Stage: -1, Group: -1, Leg: -1, SecondComp: -1}}
	fixtures := []savefile.Fixture{
		{Key: 0, Home: 547, Away: 20739, Date: savefile.Date{Raw: 182, Year: 2025}, Season: 2024,
			Result: &savefile.FixtureResult{Home: [4]uint8{2, 0xff, 0xff, 0xff}, Away: [4]uint8{1, 0xff, 0xff, 0xff}}},
		{Key: 0, Home: 20739, Away: 547, Date: savefile.Date{Raw: 228, Year: 2025}, Season: 2025},
	}
	return connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		info := fm.Info{SaveName: filepath.Base(path), Date: gameDate, ManagerTeamIndex: 547}
		return fm.NewGame(info, nil, w, fm.PlayerExtras{Fixtures: fixtures}), nil
	}}))
}

func TestGetFixturesDefaultsToManager(t *testing.T) {
	h := fixturesHarness(t)

	var got fm.FixtureList
	if res := h.call("get_fixtures", nil, &got); res.IsError {
		t.Fatalf("get_fixtures: %s", allText(res))
	}
	if len(got.Results) != 1 || got.Results[0].Score == nil || got.Results[0].Score.Home != 2 || got.Results[0].Competition != "Premier League" {
		t.Errorf("results = %+v", got.Results)
	}
	if len(got.Upcoming) != 1 || got.Upcoming[0].HomeTeam != "Chelsea" || got.Upcoming[0].Date.Format(time.DateOnly) != "2025-08-16" {
		t.Errorf("upcoming = %+v", got.Upcoming)
	}
}

func TestGetFixturesFilters(t *testing.T) {
	h := fixturesHarness(t)

	var got fm.FixtureList
	if res := h.call("get_fixtures", map[string]any{"competition": "premier league", "from": "2025-08-01", "status": "upcoming"}, &got); res.IsError {
		t.Fatalf("get_fixtures: %s", allText(res))
	}
	if len(got.Upcoming) != 1 || len(got.Results) != 0 {
		t.Errorf("got %+v, want the one upcoming fixture", got)
	}
}

func TestGetFixturesErrors(t *testing.T) {
	h := fixturesHarness(t)

	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"from": "16/08/2025"}, "from"},
		{map[string]any{"to": "2025-13-01"}, "to"},
		{map[string]any{"competition": "league"}, "competition_index"},
		{map[string]any{"status": "soon"}, "status"},
	} {
		if res := h.call("get_fixtures", tt.args, nil); !res.IsError || !strings.Contains(allText(res), tt.want) {
			t.Errorf("get_fixtures %v: %s, want an error about %q", tt.args, allText(res), tt.want)
		}
	}
}

func TestFixturesAndLineupsAreExplained(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	desc := map[string]string{}
	for _, tool := range tools.Tools {
		desc[tool.Name] = tool.Description
	}
	for _, want := range []string{"results", "upcoming", "stage", "after_extra_time", "penalties", "aggregate", "competition_index", "Fog of war does not hide fixtures"} {
		if !strings.Contains(desc["get_fixtures"], want) {
			t.Errorf("get_fixtures description lacks %q", want)
		}
	}
	if strings.Contains(desc["get_last_lineup"], "next fixture is not known") || !strings.Contains(desc["get_last_lineup"], "get_fixtures") {
		t.Errorf("get_last_lineup should point to get_fixtures: %s", desc["get_last_lineup"])
	}
	text := h.session.InitializeResult().Instructions
	if !strings.Contains(text, "get_fixtures") || strings.Contains(text, "next fixture is not known") {
		t.Errorf("instructions should describe get_fixtures")
	}
}

// tablesHarness serves fakeGame with a two-row Premier League table (COMP 1,
// uid 11) holding Tottenham.
func tablesHarness(t *testing.T) *harness {
	t.Helper()
	w := world
	w.Competitions = map[int64]string{1: "Premier League"}
	w.CompetitionUIDs = []uint32{10, 11}
	raw := savefile.LeagueTables{Tables: []savefile.LeagueTable{{CompetitionUID: 11, Stage: -1, Rows: []savefile.TableRow{
		{Team: 547, Position: 0, Overall: savefile.TableStats{Played: 1, Won: 1, GoalsFor: 2, GoalsAgainst: 1, Points: 3}, Form: [10]uint8{3}},
		{Team: 20739, Position: 1, Overall: savefile.TableStats{Played: 1, Lost: 1, GoalsFor: 1, GoalsAgainst: 2}, Form: [10]uint8{1}},
	}}}}
	return connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		info := fm.Info{SaveName: filepath.Base(path), Date: gameDate, ManagerTeamIndex: 547}
		return fm.NewGame(info, nil, w, fm.PlayerExtras{Tables: &raw, LeagueStages: 1}), nil
	}}))
}

func TestGetTableDefaultsToManager(t *testing.T) {
	h := tablesHarness(t)

	var got struct {
		Tables []fm.Table `json:"tables"`
	}
	if res := h.call("get_table", nil, &got); res.IsError {
		t.Fatalf("get_table: %s", allText(res))
	}
	if len(got.Tables) != 1 || got.Tables[0].Competition != "Premier League" || got.Tables[0].LabelSource != "file" ||
		got.Tables[0].Rows[0].Team != "Tottenham Hotspur" || got.Tables[0].Rows[0].Points != 3 || got.Tables[0].Rows[0].Form != "W" {
		t.Errorf("tables = %+v", got.Tables)
	}
	if res := h.call("get_table", map[string]any{"competition": "premier league"}, &got); res.IsError || len(got.Tables) != 1 {
		t.Errorf("by name: %s", allText(res))
	}
}

func TestGetTableNoTable(t *testing.T) {
	h := tablesHarness(t)

	if res := h.call("get_table", map[string]any{"competition_index": 0}, nil); !res.IsError || !strings.Contains(allText(res), "no table") || !strings.Contains(allText(res), "get_fixtures") {
		t.Errorf("no table: %s", allText(res))
	}
}

func TestGetTableIsExplained(t *testing.T) {
	h := newHarness(t)

	tools, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tools.Tools, func(tool *mcp.Tool) bool { return tool.Name == "get_table" })
	if i < 0 {
		t.Fatal("no get_table tool")
	}
	for _, want := range []string{"never computed", "shootout_losses", "label_source", "form", "points as the game stores them", "get_fixtures", "share a label"} {
		if !strings.Contains(tools.Tools[i].Description, want) {
			t.Errorf("get_table description lacks %q", want)
		}
	}
	if !strings.Contains(h.session.InitializeResult().Instructions, "get_table") {
		t.Error("instructions lack get_table")
	}
}

func TestFixturesAndTablesUnreadable(t *testing.T) {
	h := connect(t, New(Options{DefaultSave: "/x/a.fm", Logf: func(string, ...any) {}, Load: func(path string) (*fm.Game, error) {
		return fakeGame(path), nil
	}}))

	for _, tool := range []string{"get_fixtures", "get_table"} {
		if res := h.call(tool, nil, nil); !res.IsError || !strings.Contains(allText(res), "could not be read") || !strings.Contains(allText(res), "game_overview") {
			t.Errorf("%s: %s, want an error pointing at game_overview's warnings", tool, allText(res))
		}
	}
}

func TestGetFixturesRejectsReversedRange(t *testing.T) {
	h := fixturesHarness(t)

	if res := h.call("get_fixtures", map[string]any{"from": "2025-09-01", "to": "2025-08-01"}, nil); !res.IsError || !strings.Contains(allText(res), "after") {
		t.Errorf("reversed range: %s", allText(res))
	}
	if res := h.call("get_fixtures", map[string]any{"from": "16/08/2025"}, nil); !res.IsError || !strings.Contains(allText(res), "YYYY-MM-DD") {
		t.Errorf("bad date: %s", allText(res))
	}
}
