package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// Follows game_plugin 27.0.0: rating 0x491ab5c, best rating 0x491af40,
// ability 0x491aa68. FM26 saves get the same rules; 26.3.2 was not decoded.

// oopPositionRoles are the out-of-possession roles the game offers per
// position without a side context (0x491d7c4), in PositionNames order.
var oopPositionRoles = [len(savefile.PositionNames)]uint64{
	0x7, 0, // GK, SW
	0xe00, 0x1f8, 0xe00, // DL, DC, DR
	0xf8000,                                // DM
	0x3800000000, 0x38100000, 0x3800000000, // ML, MC, MR
	0x3c000000000, 0x3c0000000, 0x3c000000000, // AML, AMC, AMR
	0x3c0000000000, // ST
	0x7000, 0x7000, // WBL, WBR
}

// Out-of-possession role bits by the position their rating blends with.
const (
	oopWideCentreBacks = 0x1c0          // Wide, Stopping Wide, Covering Wide Centre-Back: DL or DR
	oopToWingBack      = 0x1000080400   // Pressing Full-Back, Wide Covering DM, Tracking Wide Midfielder
	oopToWideMidfield  = 0x8020002000   // Pressing Wing-Back, Wide Covering CM, Tracking Winger
	oopToCentreBack    = 0x10000        // Dropping Defensive Midfielder
	oopToCentralMid    = 0x80020000     // Pressing DM, Tracking Attacking Midfielder
	oopToAttackingMid  = 0x90008000000  // Pressing CM, Inside Outlet Winger, Tracking Centre-Forward
	oopToWideAttacking = 0x200200000000 // Splitting Outlet Attacking Midfielder and Centre-Forward
)

// oopRating is the out-of-possession role rating at a position; 0 for a
// role without attributes.
func (in *roleInputs) oopRating(role uint64, at place) int {
	a, ok := oopRoleAttributesByRole[role]
	if !ok {
		return 0
	}
	return in.weighted(a, in.oopPositionRating(role, at))
}

// oopPositionRating is the position rating, averaged from 12 up with a
// higher neighbouring position. Side-dependent neighbours need the slot's
// side; without one only the wide centre-backs blend (the higher of DL, DR).
func (in *roleInputs) oopPositionRating(role uint64, at place) int {
	p := int(in.positions[at.index])
	if p < 12 {
		return p
	}
	other := 0
	switch {
	case role&oopWideCentreBacks != 0:
		other = in.fullBackRating(at)
	case role&oopToWingBack != 0:
		other, _ = in.sideRating(at, positionIndexWBL, positionIndexWBR)
	case role&oopToWideMidfield != 0:
		other, _ = in.sideRating(at, positionIndexML, positionIndexMR)
	case role&oopToCentreBack != 0:
		other = int(in.positions[positionIndexDC])
	case role&oopToCentralMid != 0:
		other = int(in.positions[positionIndexMC])
	case role&oopToAttackingMid != 0:
		other = int(in.positions[positionIndexAMC])
	case role&oopToWideAttacking != 0:
		other, _ = in.sideRating(at, positionIndexAML, positionIndexAMR)
	}
	return blend(p, other)
}

// oopBestRating is the highest out-of-possession rating over the positions
// rated 10 or more and the roles offered there; no threshold applies.
func (in *roleInputs) oopBestRating() int {
	best := 0
	for at, role := range in.offeredRoles(oopPositionRoles, bestRolePosition) {
		best = max(best, in.oopRating(role, at))
	}
	return best
}

// oopAbility is the out-of-possession role ability at a position, on the
// current-ability scale. It is the in-possession ability under
// rules.outOfPossession() against the best out-of-possession rating, never
// halved. False for a role without attributes.
func (in *roleInputs) oopAbility(role uint64, at place, rules roleRules) (int, bool) {
	if _, ok := oopRoleAttributesByRole[role]; !ok {
		return 0, false
	}
	r, b := in.oopRating(role, at), int(in.oopBest)
	switch {
	case r == 0:
		return in.currentAbility, true
	case b == 0:
		return r, true
	}
	oop := rules.outOfPossession()
	return in.shareOfBest(r, b, oop.pullsUp(in.positions[at.index]), oop), true
}

// outOfPossession returns 27.0.0's rules for every build.
func (roleRules) outOfPossession() roleRules { return roleRules27 }
