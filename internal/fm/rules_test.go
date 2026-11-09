package fm

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

const (
	aLyga         = 691
	lithuanianCup = 700
	womensLeague  = 701
	nationLTU     = 153
	teamKZ        = 16521
)

func rulesWorld() World {
	topLyga := RuleGroup{NationIndex: nationLTU, Name: "TOPLYGA", Squad: []SquadRule{
		{Rule: "max_squad_size", Value: ptr(30), Scope: 255},
		{Rule: "same_nation_players_in_11", Value: ptr(3), Scope: 1, FromYear: 2026, UntilYear: 2030, Undecoded: []string{"GmTy"}},
	}}
	return World{
		Teams: map[int64]*Team{
			teamKZ: {Index: teamKZ, Name: "Kauno Žalgiris", divisionIndex: aLyga},
			900:    {Index: 900, Name: "Leeds United", divisionIndex: noTeam},
		},
		Nations:         map[int64]string{nationLTU: "Lithuania"},
		Competitions:    map[int64]string{aLyga: "A Lyga", lithuanianCup: "Lithuanian Cup", womensLeague: "Moterų A Lyga"},
		CompetitionUIDs: make([]uint32, 800),
		Rules: map[int64]RuleGroup{
			aLyga:         topLyga,
			lithuanianCup: {NationIndex: nationLTU, Name: "Lithuanian Cup"},
		},
	}
}

// rulesView is w seen by KŽ's manager.
func rulesView(w World) View {
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: teamKZ}, nil, w, PlayerExtras{}).View(false)
}

func TestCompetitionRules(t *testing.T) {
	v := rulesView(rulesWorld())
	aLygaRules := CompetitionRules{
		CompetitionIndex: aLyga, Competition: "A Lyga", NationIndex: nationLTU, Nation: "Lithuania", RuleGroup: "TOPLYGA",
		SquadRules: rulesWorld().Rules[aLyga].Squad,
	}
	team, comp, cup := int64(teamKZ), int64(aLyga), int64(lithuanianCup)
	for name, tt := range map[string]struct {
		team, competition *int64
		want              CompetitionRules
	}{
		"the manager's league": {nil, nil, aLygaRules},
		"a team's league":      {&team, nil, aLygaRules},
		"a competition":        {nil, &comp, aLygaRules},
		"a competition over a team": {&team, &cup, CompetitionRules{
			CompetitionIndex: lithuanianCup, Competition: "Lithuanian Cup", NationIndex: nationLTU, Nation: "Lithuania", RuleGroup: "Lithuanian Cup", SquadRules: []SquadRule{},
		}},
		"a group without squad rules": {nil, &cup, CompetitionRules{
			CompetitionIndex: lithuanianCup, Competition: "Lithuanian Cup", NationIndex: nationLTU, Nation: "Lithuania", RuleGroup: "Lithuanian Cup", SquadRules: []SquadRule{},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := v.CompetitionRules(tt.team, tt.competition)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestCompetitionRulesErrors(t *testing.T) {
	leeds, women, unknown, none := int64(900), int64(womensLeague), int64(5000), int64(noTeam)
	withRules := rulesView(rulesWorld())
	w := rulesWorld()
	w.Rules = nil
	withoutRules := rulesView(w)
	for name, tt := range map[string]struct {
		v                 View
		team, competition *int64
		want              error
	}{
		"rules not read":              {withoutRules, nil, nil, ErrRulesUnavailable},
		"a team with no league":       {withRules, &leeds, nil, ErrNoRules},
		"a competition without rules": {withRules, nil, &women, ErrNoRules},
		"no such competition":         {withRules, nil, &unknown, ErrInvalidFilter},
		"no such team":                {withRules, &none, nil, ErrInvalidFilter},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tt.v.CompetitionRules(tt.team, tt.competition); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// The error says why rules can be missing and repeats the load warning
// when some nations' rules did not decode.
func TestCompetitionRulesErrorSaysWhy(t *testing.T) {
	women := int64(womensLeague)
	w := rulesWorld()
	_, err := rulesView(w).CompetitionRules(nil, &women)
	if msg := fmt.Sprint(err); !strings.Contains(msg, "Moterų A Lyga") || !strings.Contains(msg, "continental") || strings.Contains(msg, "unavailable") {
		t.Errorf("error %q", msg)
	}
	w.RulesWarning = "some nations' competition rules unavailable: nation 153 rules: corrupt"
	v := rulesView(w)
	_, err = v.CompetitionRules(nil, &women)
	if msg := fmt.Sprint(err); !strings.Contains(msg, w.RulesWarning) {
		t.Errorf("error %q lacks the warning", msg)
	}
	if !slices.Contains(v.g.Info.Warnings, w.RulesWarning) {
		t.Errorf("load warnings %q lack the rules warning", v.g.Info.Warnings)
	}
}

func TestRuleGroupsFromTheSave(t *testing.T) {
	db := &savefile.GameDB{
		Nations: []savefile.Nation{{}, {RuleGroups: []savefile.RuleGroup{
			{Name: "TOPLYGA", Squad: []savefile.SquadRule{
				{Type: 9, Value: 30, HasValue: true, Scope: 255},
				{Type: 94, Value: 3, HasValue: true, Scope: 1, StartYear: 2026, EndYear: 2030, Other: []string{"GmTy"}},
				{Type: 600, Scope: 1}, // no name, no value
			}},
			{Name: "Lithuanian Cup"},
		}}},
		CompetitionRules: map[int]savefile.CompetitionRules{3: {Nation: 1, Group: 0}, 4: {Nation: 1, Group: 1}},
	}
	want := map[int64]RuleGroup{
		3: {NationIndex: 1, Name: "TOPLYGA", Squad: []SquadRule{
			{Rule: "max_squad_size", Value: ptr(30), Scope: 255},
			{Rule: "same_nation_players_in_11", Value: ptr(3), Scope: 1, FromYear: 2026, UntilYear: 2030, Undecoded: []string{"GmTy"}},
			{Rule: "rule_600", Scope: 1},
		}},
		4: {NationIndex: 1, Name: "Lithuanian Cup", Squad: []SquadRule{}},
	}
	if got := ruleGroups(db); !reflect.DeepEqual(got, want) {
		t.Errorf("ruleGroups = %+v\nwant %+v", got, want)
	}
	if got := ruleGroups(&savefile.GameDB{}); got != nil {
		t.Errorf("ruleGroups without rules = %v, want nil", got)
	}
}
