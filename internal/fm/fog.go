package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// Scouting is how much of a player fog of war shows the manager.
type Scouting string

// Scouting levels. The knowledge thresholds are inferred from game screens:
// at 80 the game shows exact attributes and value but the wage as a range;
// at 30 a few attributes, as ranges, and no value.
const (
	ScoutingFull     Scouting = "full"     // everything
	ScoutingPartial  Scouting = "partial"  // no ability, wage, clauses, playing time, loyalty bonus, hidden attributes, personality, media handling or favoured clubs
	ScoutingRequired Scouting = "required" // also no attributes, traits or value
)

const (
	fullKnowledge    = 100
	partialKnowledge = 80
)

// hiddenAttributes are the attributes shown only with full knowledge: the
// five the game shows only then, and the eight personality ones, which it
// never shows.
var hiddenAttributes = func() (set [len(attributeNames)]bool) {
	shownWhenFull := []string{"Consistency", "Dirtiness", "ImportantMatches", "InjuryProneness", "Versatility"}
	for _, n := range append(shownWhenFull, savefile.HiddenAttributeNames[:]...) {
		set[mustAttribute(n)] = true
	}
	return set
}()

// baseKnowledge is the knowledge reputation alone gives. Thresholds from
// game_plugin 26.3.2 (0x16f2dbc); that it reads the current reputation is a
// guess. Nation and competition knowledge is not decoded, so this can be
// lower than the game's.
func baseKnowledge(reputation int) int {
	switch {
	case reputation >= 8000:
		return 80
	case reputation >= 6500:
		return 50
	case reputation >= 5000:
		return 30
	}
	return 0
}

// knowledgeOf is the manager's knowledge of p: full for players of his club
// by current team or main contract (so loanees count; seen in game), else
// the larger of the stored level and the reputation baseline.
func knowledgeOf(p *Player, clubTeams map[int64]bool, stored uint8) int {
	if clubTeams[p.TeamIndex] || p.Contract != nil && clubTeams[p.Contract.TeamIndex] {
		return fullKnowledge
	}
	return max(int(stored), baseKnowledge(p.Reputation.Current))
}

// sight is a viewer's knowledge of one player, 0..100; always full without
// fog of war.
type sight int

func (s sight) scouting() Scouting {
	switch {
	case s.full():
		return ScoutingFull
	case s.partial():
		return ScoutingPartial
	}
	return ScoutingRequired
}

// full: everything is visible.
func (s sight) full() bool { return s >= fullKnowledge }

// partial: the value and all but the hidden attributes are visible.
func (s sight) partial() bool { return s >= partialKnowledge }

func (s sight) attribute(i int) bool { return s.full() || s.partial() && !hiddenAttributes[i] }

// value reports whether p's value is visible and known.
func (s sight) value(p *Player) bool { return s.partial() && p.hasValue() }

// wage reports whether p's wage is visible and known: players without a
// contract have none.
func (s sight) wage(p *Player) bool { return s.full() && p.Contract != nil }

// wageHidden reports whether fog of war hides p's wage. Whether he has a
// contract is always visible.
func (s sight) wageHidden(p *Player) bool { return !s.full() && p.Contract != nil }

func ptr[T any](v T) *T { return &v }
