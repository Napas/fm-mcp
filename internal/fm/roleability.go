package fm

import (
	"cmp"
	"iter"
	"math/bits"
	"slices"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Role ability follows game_plugin 26.3.2: the role rating 0x48c0a50, the
// best role rating 0x48ba160 and the ability on the current-ability scale
// 0x2fbcb20. Only in-possession roles are covered.

// place is a position in a tactic: its index in savefile.PositionNames and
// its position mask, with a side bit for one of a pair.
type place struct {
	index int
	mask  uint32
}

// placeAt is position i without a side.
func placeAt(i int) place { return place{index: i, mask: savefile.PositionMasks[i]} }

// placeOf is a tactic slot's position; false when it has none.
func placeOf(s savefile.SlotRole) (place, bool) {
	code, ok := s.PositionCode()
	if !ok {
		return place{}, false
	}
	return place{index: slices.Index(savefile.PositionNames[:], code), mask: s.Position}, true
}

// Position masks the rating blends a neighbouring position for: the left and
// right positions, with their side bits.
const (
	leftPositions  = 0x111248
	rightPositions = 0x208924
)

// Role bits with a role-specific position rule or threshold.
const (
	wideOverlappingCentreBacks = 0x48000000000000 // Wide and Overlapping Centre-Back
	insideFullBack             = 0x10000000000000
	insideWideForwards         = 0x2080008000000 // Inside Forward, Playmaking Winger, Inside Winger
	falseNine                  = 0x10000000000
	advancedCentreBack         = 0x4000
	completeForward            = 0x100000
	threshold100Roles          = 0x600100000000 // Free Role, Wide Forward, Box To Box Playmaker
	threshold80Roles           = 0x9a000004000  // ACB, Enganche, Regista, False Nine, Playmaking Winger
	threshold80BestRoles       = 0x90000004000  // ACB, False Nine, Playmaking Winger
	threshold60Roles           = 0x4800000000   // Half Back, Advanced Wing-Back
	positionIndexDL            = 2
	positionIndexDC            = 3
	positionIndexDR            = 4
	positionIndexDM            = 5
	positionIndexAMC           = 10
)

// positionRoles are the in-possession roles the game offers per position, in
// PositionNames order (0x48b6c98); none at SW.
var positionRoles = [len(savefile.PositionNames)]uint64{
	0x0020000000001001, 0, // GK, SW
	0x009010000000000c, 0x0048000021004002, 0x009010000000000c, // DL, DC, DR
	0x0000400800018010,                                         // DM
	0x00020800000000c0, 0x0301000000020220, 0x00020800000000c0, // ML, MC, MR
	0x0002280008000080, 0x0100020100020200, 0x0002280008000080, // AML, AMC, AMR
	0x00000100800c0c00,                     // ST
	0x0080104000000008, 0x0080104000000008, // WBL, WBR
}

// bestRolePosition is the lowest position rating the best role is looked for at.
const bestRolePosition = 10

// offeredRoles yields each position rated minRating or more with each role
// offered there, in position then role bit order.
func (in *roleInputs) offeredRoles(minRating uint8) iter.Seq2[place, uint64] {
	return func(yield func(place, uint64) bool) {
		for i, roles := range positionRoles {
			if in.positions[i] < minRating {
				continue
			}
			for r := roles; r != 0; r &= r - 1 {
				if !yield(placeAt(i), uint64(1)<<bits.TrailingZeros64(r)) {
					return
				}
			}
		}
	}
}

// roleInputs are what a player's role ability is computed from.
type roleInputs struct {
	attributes     [len(savefile.AttributeNames)]uint8 // 1..100 as stored
	positions      [len(savefile.PositionNames)]uint8  // 1..20
	currentAbility int
	best           int // bestRating
}

func newRoleInputs(p savefile.Player) roleInputs {
	in := roleInputs{attributes: p.Attributes, positions: p.Positions, currentAbility: int(p.CurrentAbility)}
	in.best = in.bestRating()
	return in
}

// rating is the role rating at a position, 1..200; 0 for a role without
// attributes. Attributes give 2 × (16 × key mean + 4 × preferable mean +
// 3 × mean of (100 − negative)) / the weights of the lists with a non-zero
// sum; a position rating under 20 then takes off a share (positionPenalty).
func (in *roleInputs) rating(role uint64, at place) int {
	a, ok := roleAttributesByRole[role]
	if !ok {
		return 0
	}
	var key, preferable, negative float64
	weight := 0
	if s := in.sum(a.key, false); s > 0 {
		key, weight = float64(s)/float64(len(a.key)), 16
	}
	if s := in.sum(a.preferable, false); s > 0 {
		preferable = 4 * float64(s) / float64(len(a.preferable))
		weight += 4
	}
	if s := in.sum(a.negative, true); s > 0 {
		negative = float64(s) / float64(len(a.negative))
		weight += 3
	}
	if weight == 0 {
		return 0
	}
	// The game computes in float32.
	v := float32(key*16 + preferable)
	v = float32(float64(v) + negative*3)
	v = float32(float64(v) / float64(weight))
	r := int(float64(v) * 2)
	r -= r * positionPenalty(in.positionRating(role, at)) / 100
	return max(r, 1)
}

// sum adds the attributes at idx; negative ones count as 100 − value, at
// least 1.
func (in *roleInputs) sum(idx []int, negative bool) int {
	s := 0
	for _, i := range idx {
		v := int(in.attributes[i])
		if negative {
			v = max(100-v, 1)
		}
		s += v
	}
	return s
}

// positionRating is the rating at the position, for some roles from 12 up
// blended with a neighbouring position when that one is higher. False Nine
// always averages with AMC, halved under 15, unless that is 0.
func (in *roleInputs) positionRating(role uint64, at place) int {
	p := int(in.positions[at.index])
	pos := func(i int) int { return int(in.positions[i]) }
	blend := func(other int) int {
		if other > p {
			return (p + other) / 2
		}
		return p
	}
	switch {
	case role&wideOverlappingCentreBacks != 0 && p >= 12:
		switch {
		case at.mask&leftPositions != 0:
			return blend(pos(positionIndexDL))
		case at.mask&rightPositions != 0:
			return blend(pos(positionIndexDR))
		}
		return blend(max(pos(positionIndexDL), pos(positionIndexDR)))
	case role&insideFullBack != 0 && p >= 12:
		return blend(pos(positionIndexDC))
	case role&insideWideForwards != 0 && p >= 12:
		return blend(pos(positionIndexAMC))
	case role&falseNine != 0 && p >= 12:
		amc := pos(positionIndexAMC)
		if amc < naturalRating {
			amc /= 2
		}
		if amc == 0 {
			return p
		}
		return (p + amc) / 2
	case role&advancedCentreBack != 0 && p >= 12:
		return blend(pos(positionIndexDM))
	}
	return p
}

// positionPenalty is the share, in percent, a position rating takes off a
// role rating: 20 − p from 15 up, 2.5 × (20 − p) from 12, else 4 × (20 − p).
func positionPenalty(p int) int {
	switch {
	case p >= naturalRating:
		return 20 - p
	case p >= 12:
		return (20 - p) * 5 / 2
	}
	return (20 - p) * 4
}

// bestRating is the highest role rating over the positions rated 10 or more
// and the roles offered there; a rating under bestThreshold counts as 0.
func (in *roleInputs) bestRating() int {
	best := 0
	for at, role := range in.offeredRoles(bestRolePosition) {
		if v := in.rating(role, at); v >= bestThreshold(role) {
			best = max(best, v)
		}
	}
	return best
}

// ability is the role ability at a position on the current-ability scale:
// the best role gives the current ability, others their rating's share of
// the best rating. At a position rated 14 or less the rating first moves
// part of the way to the best. False for a role without attributes.
func (in *roleInputs) ability(role uint64, at place) (int, bool) {
	if _, ok := roleAttributesByRole[role]; !ok {
		return 0, false
	}
	r, b := in.rating(role, at), in.best
	switch {
	case r == 0:
		return in.currentAbility, true
	case b == 0:
		return r, true
	}
	if in.positions[at.index] <= 14 && b > r {
		d := max(float64(float32(1-float64(r)/float64(b))), 0)
		r += (b - r) / (int(float64(float32(d*800))/100) + 2)
	}
	v := in.currentAbility
	if r != b {
		v = int(min(float64(r)/float64(b), 1) * float64(in.currentAbility))
	}
	if v < abilityThreshold(role) {
		v /= 2
	}
	return v, true
}

// abilityThreshold is the role ability under which the game halves it.
func abilityThreshold(role uint64) int {
	if role&completeForward != 0 {
		return 120
	}
	return roleThreshold(role, threshold80Roles)
}

// bestThreshold is the role rating under which a role is not the best.
func bestThreshold(role uint64) int { return roleThreshold(role, threshold80BestRoles) }

func roleThreshold(role, roles80 uint64) int {
	switch {
	case role&threshold100Roles != 0:
		return 100
	case role&roles80 != 0:
		return 80
	case role&threshold60Roles != 0:
		return 60
	}
	return 0
}

// inPossessionRoleNamed returns the in-possession role bit with this name,
// ignoring case.
func inPossessionRoleNamed(name string) (uint64, bool) {
	name = strings.TrimSpace(name)
	for role := range roleAttributesByRole {
		if n, _ := savefile.InPossessionRoleName(role); strings.EqualFold(n, name) {
			return role, true
		}
	}
	return 0, false
}

// offeredAnywhere are the in-possession roles offered at some position.
var offeredAnywhere = func() uint64 {
	var all uint64
	for _, roles := range positionRoles {
		all |= roles
	}
	return all
}()

// inPossessionRoleNames lists the names of the role bits in roles, sorted.
func inPossessionRoleNames(roles uint64) string {
	var names []string
	for r := roles; r != 0; r &= r - 1 {
		n, _ := savefile.InPossessionRoleName(uint64(1) << bits.TrailingZeros64(r))
		names = append(names, n)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// roleAbility is p's ability in role at a position; role must have attributes.
func (g *Game) roleAbility(p *Player, role uint64, at place) int {
	v, _ := g.auxOf(p).roles.ability(role, at)
	return v
}

// RoleFit is a player's role ability in an in-possession role at a position.
type RoleFit struct {
	Role        string `json:"role"`
	Position    string `json:"position"`
	RoleAbility int    `json:"role_ability"`
}

const bestRolesShown = 3

// bestRoles returns the highest role abilities, one per role, over the roles
// offered at positions rated 15 or more. Ties keep position, then role bit,
// order.
func (in *roleInputs) bestRoles() []RoleFit {
	var fits []RoleFit
	for at, role := range in.offeredRoles(naturalRating) {
		v, _ := in.ability(role, at)
		name, _ := savefile.InPossessionRoleName(role)
		fits = append(fits, RoleFit{Role: name, Position: savefile.PositionNames[at.index], RoleAbility: v})
	}
	slices.SortStableFunc(fits, func(a, b RoleFit) int { return cmp.Compare(b.RoleAbility, a.RoleAbility) })
	var out []RoleFit
	for _, f := range fits {
		if len(out) == bestRolesShown {
			break
		}
		if !slices.ContainsFunc(out, func(o RoleFit) bool { return o.Role == f.Role }) {
			out = append(out, f)
		}
	}
	return out
}
