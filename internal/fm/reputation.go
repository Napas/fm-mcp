package fm

import "math"

// NationScale holds what the reputation rule reads from a nation.
type NationScale struct {
	Factor            int16   // its men's national-team container's factor (+0x572)
	ContinentStrength float32 // its continent's strength (+0xe4)
	HasContinent      bool    // false for defunct nations
}

// nationScale is s0 of the reputation rule: max(1, round(factor / 10)) / 20,
// with the rounded value cut to int8 as the game does (0x2fbe778).
func (n NationScale) nationScale() float32 {
	k := int8(int64(math.Round(float64(float32(n.Factor) / 10)))) //nolint:gosec // the game truncates to int8
	return float32(max(1, k)) / 20
}

// hasContinentScale reports whether the nation gives s1: a continent of
// non-zero strength.
func (n NationScale) hasContinentScale() bool {
	return n.HasContinent && n.ContinentStrength != 0
}

// continentScale is s1 of the reputation rule: 2·strength − 1.
func (n NationScale) continentScale() float32 {
	f := n.ContinentStrength
	return f + (f - 1)
}

// reputationWords names reputationLevel's levels: the language group of
// 227312…227317 in order. Continental and National are checked in game; the
// others follow the order (inference).
var reputationWords = [...]string{"", "Worldwide", "Continental", "National", "Regional", "Local", "Obscure"}

// reputationThresholds are the rule's cuts for continental, national,
// regional and local reputation, in float32 as the game computes them.
func reputationThresholds(s0, s1 float32) (tc, tn, tr, tl int) {
	scaled := func(base float32) int {
		x := float32(s1 * base)
		return int(float32(s0 * x))
	}
	tn = min(max(scaled(4750), 2150), 5250)
	tr = scaled(2500)
	if tn < tr {
		tr = tn - 1250
	}
	tl = scaled(500)
	if tr < tl {
		tl = max(tr-1250, 1)
	}
	tc = int(float32(s1 * 5250))
	if s1 > 0.95 {
		tc += 1500
	}
	if tc > 7250 {
		tc = 6500
	}
	return tc, tn, tr, tl
}

// reputationLevel is the game's player reputation level (game_plugin
// 26.3.2, 0x2fbe6a0) from the current and world reputation, 1 Worldwide …
// 6 Obscure.
func reputationLevel(cr, wr int, s0, s1 float32) int {
	tc, tn, tr, tl := reputationThresholds(s0, s1)
	switch {
	case wr > 7249:
		return 1
	case wr >= 3626 && cr >= tc:
		return 2
	case cr >= tn:
		return 3
	case cr >= tr:
		return 4
	case cr < tl:
		return 6
	}
	return 5
}

// reputationLevel names b's reputation level. The scales come from the
// nation of his team (its club's or the national team's), else from his
// nationality (inference for players without a team); s1 also falls back to
// his nationality when the team's nation has no continent or one of strength
// 0, and is 1 when neither gives one. Women's players are left out: the game reads their nation's women's container, whose factor is
// not read. Player-coaches are left out: the game takes the better of this
// rule and the staff rule, whose inputs are not decoded.
func (w World) reputationLevel(b PlayerBasics) string {
	if w.NationScales == nil || b.Female || b.PlayerCoach {
		return ""
	}
	nation := int64(noTeam)
	if t := w.Teams[b.TeamIndex]; t != nil {
		nation = t.NationIndex
	}
	home, okHome := w.NationScales[nation]
	own, okOwn := w.NationScales[b.NationIndex]
	s0, s1 := float32(1), float32(1)
	switch {
	case okHome:
		s0 = home.nationScale()
	case okOwn:
		s0 = own.nationScale()
	}
	switch {
	case okHome && home.hasContinentScale():
		s1 = home.continentScale()
	case okOwn && own.hasContinentScale():
		s1 = own.continentScale()
	}
	return reputationWords[reputationLevel(b.Reputation.Current, b.Reputation.World, s0, s1)]
}
