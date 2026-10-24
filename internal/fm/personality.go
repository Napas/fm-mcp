package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// Attribute indexes the personality and media handling rules read.
var (
	attrDetermination   = mustAttribute("Determination")
	attrLeadership      = mustAttribute("Leadership")
	attrAmbition        = mustAttribute("Ambition")
	attrLoyalty         = mustAttribute("Loyalty")
	attrPressure        = mustAttribute("Pressure")
	attrProfessionalism = mustAttribute("Professionalism")
	attrSportsmanship   = mustAttribute("Sportsmanship")
	attrTemperament     = mustAttribute("Temperament")
)

// personalityInput holds the personality rule's inputs, attributes on the
// 1-20 scale.
type personalityInput struct {
	determination, ambition, loyalty, pressure, professionalism, sportsmanship, temperament, leadership int

	age      int // whole years at the game date
	negative bool
	devoted  bool // affinity above 99 for his club (see isDevoted)
}

// label is the game's personality rule (game_plugin 26.3.2, 0x36b906c; names
// from 0x3926560); the first matching case wins. negative is the trailer flag
// the game checks when its restrict argument is set (inference: the profile
// screen sets it).
func (in personalityInput) label() string {
	d, amb, loy, pre, pro, spo, tem := in.determination, in.ambition, in.loyalty, in.pressure, in.professionalism, in.sportsmanship, in.temperament
	neg := in.negative
	switch {
	case d >= 14 && amb >= 12 && pro >= 15 && tem >= 15 && pre >= 14 && loy >= 15 && spo >= 15:
		return "Model Citizen"
	case d >= 14 && amb >= 14 && pro >= 14 && tem <= 9:
		return "Perfectionist"
	case neg && tem <= 4 && pro <= 10:
		return "Temperamental"
	case in.age >= 23 && in.leadership >= 19:
		switch {
		case in.leadership == 20 && d == 20:
			return "Born Leader"
		case tem >= 18 && spo >= 18:
			return "Charismatic Leader"
		}
		return "Leader"
	case pro >= 18 && tem >= 10:
		return pick(in.age >= 23 && pro == 20, "Model Professional", "Professional")
	case d >= 18:
		return pick(amb >= 12, "Driven", "Determined")
	case neg && pro <= 4 && d <= 9:
		return pick(pro == 1, "Slack", "Casual")
	case neg && loy <= 6 && amb >= 16:
		return pick(loy <= 3, "Mercenary", "Fickle")
	case amb >= 16 && loy <= 9:
		return pick(amb == 20, "Very Ambitious", "Ambitious")
	case neg && amb <= 5 && loy >= 11:
		return "Unambitious"
	case spo >= 18 && d <= 9:
		return pick(spo == 20, "Honest", "Sporting")
	case neg && spo <= 4 && d >= 11:
		return pick(spo == 1, "Unsporting", "Realist")
	case neg && d <= 5 && amb <= 9:
		return pick(d == 1, "Easily Discouraged", "Low Determination")
	case loy >= 18 && amb <= 7:
		return pick(loy == 20, pick(in.devoted, "Devoted", "Very Loyal"), "Loyal")
	case pre >= 17 && d >= 15:
		return pick(pre == 20, "Iron Willed", "Resilient")
	case neg && pre <= 3 && d <= 9:
		return pick(pre == 1, "Spineless", "Low Self Belief")
	case pre >= 15 && tem >= 10:
		switch {
		case spo >= 15:
			return "Light-Hearted"
		case pro >= 11:
			return "Spirited"
		}
		return "Jovial"
	case d >= 15:
		return pick(pro <= 14, "Fairly Determined", "Resolute")
	case pro >= 15:
		return "Fairly Professional"
	case amb >= 15:
		return "Fairly Ambitious"
	case loy >= 15:
		return "Fairly Loyal"
	case spo >= 15:
		return "Fairly Sporting"
	}
	return "Balanced"
}

// personalityLabels are the labels personalityInput.label returns.
var personalityLabels = []string{
	"Model Citizen", "Perfectionist", "Temperamental", "Born Leader", "Charismatic Leader", "Leader",
	"Model Professional", "Professional", "Driven", "Determined", "Slack", "Casual", "Mercenary", "Fickle",
	"Very Ambitious", "Ambitious", "Unambitious", "Honest", "Sporting", "Unsporting", "Realist",
	"Easily Discouraged", "Low Determination", "Devoted", "Very Loyal", "Loyal", "Iron Willed", "Resilient",
	"Spineless", "Low Self Belief", "Light-Hearted", "Spirited", "Jovial", "Fairly Determined", "Resolute",
	"Fairly Professional", "Fairly Ambitious", "Fairly Loyal", "Fairly Sporting", "Balanced",
}

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

// personality is computed on demand so players carry no extra string.
func (p *Player) personality() string {
	a := p.Attributes
	return personalityInput{
		determination: int(a[attrDetermination]), ambition: int(a[attrAmbition]), loyalty: int(a[attrLoyalty]),
		pressure: int(a[attrPressure]), professionalism: int(a[attrProfessionalism]), sportsmanship: int(a[attrSportsmanship]),
		temperament: int(a[attrTemperament]), leadership: int(a[attrLeadership]),
		age: p.Age, negative: p.negativeLabels, devoted: p.devoted,
	}.label()
}

// isDevoted reports whether the personality rule gives Devoted (0x36b93b8):
// the first club-affinity item for the club of his current team, else of his
// main contract's team, has a value above 99.
func isDevoted(raw savefile.Player, w World) bool {
	club, ok := w.clubOf(raw.TeamIndex)
	if !ok && raw.Contract != nil {
		club, ok = w.clubOf(raw.Contract.TeamIndex)
	}
	if !ok {
		return false
	}
	for _, c := range raw.ClubAffinities {
		if c.Club == club {
			return c.Value > 99
		}
	}
	return false
}

// traitNames names the bits set in mask, in bit order; unnamed bits are skipped.
func traitNames(mask uint64) []string {
	var out []string
	for bit, name := range savefile.TraitNames {
		if mask&(1<<bit) != 0 && name != "" {
			out = append(out, name)
		}
	}
	return out
}
