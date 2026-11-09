package fm

import (
	"errors"
	"fmt"

	"github.com/Napas/fm-mcp/internal/savefile"
)

var (
	// ErrRulesUnavailable means the save's competition rules were not read:
	// a save before FM 27.0.0, or one whose nations do not decode.
	ErrRulesUnavailable = errors.New("fm: competition rules unavailable")
	// ErrNoRules means a competition has no rule group in the save.
	ErrNoRules = errors.New("fm: no competition rules")
)

// RuleGroup is a competition's group of rules in its nation's rules file.
type RuleGroup struct {
	NationIndex int64
	Name        string // the game's internal name, e.g. "TOPLYGA"
	Squad       []SquadRule
}

// SquadRule is a squad-selection rule. Whether Value is a minimum or a
// maximum depends on the rule and is not stored.
type SquadRule struct {
	Rule      string   `json:"rule"`            // the game's name, e.g. "max_squad_size"; "rule_<id>" when the table has none
	Value     *int     `json:"value,omitempty"` // absent on fee rules
	Scope     int      `json:"scope"`           // 255 or 1; meaning unknown
	FromYear  int      `json:"from_year,omitempty"`
	UntilYear int      `json:"until_year,omitempty"`
	Undecoded []string `json:"undecoded_conditions,omitempty"` // tags of further conditions, e.g. "GmTy"
}

// CompetitionRules are a competition's squad rules.
type CompetitionRules struct {
	CompetitionIndex int64       `json:"competition_index"`
	Competition      string      `json:"competition,omitempty"`
	NationIndex      int64       `json:"nation_index"`
	Nation           string      `json:"nation,omitempty"`
	RuleGroup        string      `json:"rule_group"`
	SquadRules       []SquadRule `json:"squad_rules"`
}

// ruleGroups returns rule groups by COMP index; nil when rules were not read.
func ruleGroups(db *savefile.GameDB) map[int64]RuleGroup {
	if db.CompetitionRules == nil {
		return nil
	}
	out := make(map[int64]RuleGroup, len(db.CompetitionRules))
	for comp, r := range db.CompetitionRules {
		g := db.Nations[r.Nation].RuleGroups[r.Group]
		squad := make([]SquadRule, 0, len(g.Squad))
		for _, s := range g.Squad {
			squad = append(squad, newSquadRule(s))
		}
		out[int64(comp)] = RuleGroup{NationIndex: int64(r.Nation), Name: g.Name, Squad: squad}
	}
	return out
}

func newSquadRule(s savefile.SquadRule) SquadRule {
	r := SquadRule{Rule: squadRuleNames[s.Type], Scope: int(s.Scope), FromYear: int(s.StartYear), UntilYear: int(s.EndYear), Undecoded: s.Other}
	if r.Rule == "" {
		r.Rule = fmt.Sprintf("rule_%d", s.Type)
	}
	if s.HasValue {
		r.Value = ptr(int(s.Value))
	}
	return r
}

// CompetitionRules returns the rules of a competition, else of a team's
// league, else of the manager's league.
func (v View) CompetitionRules(team, competition *int64) (CompetitionRules, error) {
	team, err := v.g.queryTeam(team, competition)
	if err != nil {
		return CompetitionRules{}, err
	}
	w := v.g.world
	if w.Rules == nil {
		return CompetitionRules{}, ErrRulesUnavailable
	}
	if competition == nil {
		div := w.Teams[*team].divisionIndex
		if div == noTeam {
			return CompetitionRules{}, fmt.Errorf("%w: %s has no league", ErrNoRules, named("team_index", *team, w.teamName(*team)))
		}
		competition = &div
	}
	g, ok := w.Rules[*competition]
	if !ok {
		err := fmt.Errorf("%w: no rules read for %s; rules are not read for women's or continental competitions, competitions whose formats name different rule groups, or single stages", ErrNoRules, named("competition_index", *competition, w.Competitions[*competition]))
		if w.RulesWarning != "" {
			err = fmt.Errorf("%w; %s", err, w.RulesWarning)
		}
		return CompetitionRules{}, err
	}
	return CompetitionRules{
		CompetitionIndex: *competition, Competition: w.Competitions[*competition],
		NationIndex: g.NationIndex, Nation: w.Nations[g.NationIndex],
		RuleGroup: g.Name, SquadRules: append([]SquadRule{}, g.Squad...),
	}, nil
}
