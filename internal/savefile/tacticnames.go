package savefile

// Values are the game's C# PositionMask, InPossessionRole and
// OutOfPossessionRole enums (IL2CPP metadata). Roles FM26 renamed ("inferred")
// are labelled from the English language file. A stored role also carries a
// duty bit (dutyMask), which the Tactics Planner does not show.

const (
	positionLeft  = 0x100000
	positionRight = 0x200000
	dutyMask      = 0x200000 | 0x400000 | 0x800000 | 0x2000000 | 0x4000000 | 0x400000000
)

// PositionMasks are the PositionMask values of PositionNames, in that order.
var PositionMasks = [len(PositionNames)]uint32{
	0x1, 0x2, 0x8, 0x10, 0x4, 0x80, 0x200, 0x400, 0x100, 0x1000, 0x2000, 0x800, 0x4000, 0x40, 0x20,
}

var positionCodes = func() map[uint32]string {
	out := make(map[uint32]string, len(PositionMasks))
	for i, m := range PositionMasks {
		out[m] = PositionNames[i]
	}
	return out
}()

var inPossessionRoles = map[uint64]string{
	0x1:               "Goalkeeper",
	0x1000:            "Ball-Playing Goalkeeper",
	0x20000000000000:  "No Nonsense Goalkeeper",
	0x4:               "Full-Back",
	0x8:               "Wing-Back",
	0x1000000000:      "No-Nonsense Full-Back",
	0x4000000000:      "Advanced Wing-Back",
	0x100000000000:    "Inside Wing-Back", // inferred: InvertedWingBack
	0x10000000000000:  "Inside Full-Back", // inferred: InvertedFullBack
	0x80000000000000:  "Playmaking Wing-Back",
	0x2:               "Centre-Back",
	0x4000:            "Advanced Centre-Back",     // inferred: Libero
	0x1000000:         "Ball-Playing Centre-Back", // inferred: BallPlayingDefender; the game shows "BCB"
	0x20000000:        "No-Nonsense Centre-Back",
	0x8000000000000:   "Wide Centre-Back",
	0x40000000000000:  "Overlapping Centre-Back",
	0x200000000000000: "Midfield Playmaker",
	0x10:              "Defensive Midfielder",
	0x20:              "Central Midfielder",
	0x8000:            "Deep Lying Playmaker",
	0x10000:           "Box To Box Midfielder",
	0x10000000:        "Ball Winning Midfielder",
	0x200000000:       "Anchor", // inferred: AnchorMan
	0x800000000:       "Half Back",
	0x2000000000:      "Enganche",
	0x8000000000:      "Regista",
	0x400000000000:    "Box To Box Playmaker",
	0x800000000000:    "Mezzala",
	0x4000000000000:   "Segundo Volante",
	0x40:              "Wide Midfielder",
	0x80:              "Winger",
	0x200:             "Attacking Midfielder",
	0x100000000000000: "Channel Midfielder",
	0x20000:           "Advanced Playmaker",
	0x8000000:         "Inside Forward",
	0x2000000000000:   "Inside Winger", // inferred: InvertedWinger
	0x40000000:        "Defensive Winger",
	0x100000000:       "Free Role",           // inferred: Trequartista
	0x40000000000:     "Wide Target Forward", // inferred: WideTargetMan
	0x80000000000:     "Playmaking Winger",   // inferred: WidePlaymaker
	0x200000000000:    "Wide Forward",
	0x1000000000000:   "Wide Central Midfielder",
	0x400:             "Deep Lying Forward",
	0x800:             "Centre-Forward",
	0x40000:           "Target Forward", // inferred: TargetMan
	0x80000:           "Poacher",
	0x100000:          "Complete Forward",
	0x80000000:        "Channel Forward",
	0x10000000000:     "False Nine",     // inferred: FalseNumberNine
	0x20000000000:     "Second Striker", // inferred: ShadowStriker
}

var outOfPossessionRoles = map[uint64]string{
	0x1:            "Goalkeeper",
	0x2:            "Sweeper Keeper",
	0x4:            "Line-Holding Goalkeeper", // inferred: LineKeeper
	0x8:            "Centre-Back",
	0x10:           "Stopping Centre-Back",
	0x20:           "Covering Centre-Back",
	0x40:           "Wide Centre-Back",
	0x80:           "Stopping Wide Centre-Back",
	0x100:          "Covering Wide Centre-Back",
	0x200:          "Full-Back",
	0x400:          "Pressing Full-Back",
	0x800:          "Holding Full-Back",
	0x1000:         "Wing-Back",
	0x2000:         "Pressing Wing-Back",
	0x4000:         "Holding Wing-Back",
	0x8000:         "Defensive Midfielder",
	0x10000:        "Dropping Defensive Midfielder",
	0x20000:        "Pressing Defensive Midfielder",
	0x40000:        "Screening Defensive Midfielder",
	0x80000:        "Wide Covering Defensive Midfielder",
	0x100000:       "Central Midfielder",
	0x8000000:      "Pressing Central Midfielder",
	0x10000000:     "Screening Central Midfielder",
	0x20000000:     "Wide Covering Central Midfielder",
	0x40000000:     "Attacking Midfielder",
	0x80000000:     "Tracking Attacking Midfielder",
	0x100000000:    "Central Outlet Midfielder",
	0x200000000:    "Splitting Outlet Attacking Midfielder",
	0x800000000:    "Wide Midfielder",
	0x1000000000:   "Tracking Wide Midfielder",
	0x2000000000:   "Wide Outlet Wide Midfielder",
	0x4000000000:   "Winger",
	0x8000000000:   "Tracking Winger",
	0x10000000000:  "Inside Outlet Winger", // inferred: InvertingOutletWinger
	0x20000000000:  "Wide Outlet Winger",
	0x40000000000:  "Centre-Forward",
	0x80000000000:  "Tracking Centre-Forward",
	0x100000000000: "Central Outlet Centre-Forward",
	0x200000000000: "Splitting Outlet Centre-Forward",
}

// lineupCells maps a formation cell (inverse of 0x48cea4c) to a position mask.
// Rows run from goal to attack, columns right to left. 7 and 11 are the
// unnamed attacker right and left bits.
var lineupCells = map[int8]uint32{
	-15: 0x1,
	-13: 0x4, -12: 0x10 | positionRight, -11: 0x10, -10: 0x10 | positionLeft, -9: 0x8,
	-8: 0x20, -7: 0x80 | positionRight, -6: 0x80, -5: 0x80 | positionLeft, -4: 0x40,
	-3: 0x100, -2: 0x400 | positionRight, -1: 0x400, 0: 0x400 | positionLeft, 1: 0x200,
	2: 0x800, 3: 0x2000 | positionRight, 4: 0x2000, 5: 0x2000 | positionLeft, 6: 0x1000,
	7: 0x8000, 8: 0x4000 | positionRight, 9: 0x4000, 10: 0x4000 | positionLeft, 11: 0x10000,
}

// lineupInPossessionRoles maps a role index (0x48cecd0) to an in-possession
// role bit; 50 means no role.
var lineupInPossessionRoles = map[int8]uint64{
	-31: 0x1, -30: 0x2, -29: 0x4, -28: 0x8, -27: 0x10, -26: 0x20, -25: 0x40, -24: 0x80,
	-23: 0x200, -22: 0x400, -21: 0x800, -20: 0x1000, -18: 0x4000, -17: 0x8000, -16: 0x10000,
	-15: 0x20000, -14: 0x40000, -13: 0x80000, -12: 0x100000, -11: 0x1000000, -10: 0x8000000,
	-9: 0x10000000, -8: 0x20000000, -7: 0x40000000, -6: 0x80000000, -5: 0x100000000,
	-4: 0x200000000, -3: 0x800000000, -2: 0x1000000000, -1: 0x2000000000, 0: 0x4000000000,
	1: 0x8000000000, 2: 0x10000000000, 3: 0x20000000000, 4: 0x40000000000, 5: 0x80000000000,
	6: 0x100000000000, 7: 0x200000000000, 8: 0x400000000000, 9: 0x800000000000,
	10: 0x2000000000000, 11: 0x1000000000000, 12: 0x4000000000000, 13: 0x8000000000000,
	14: 0x10000000000000, 15: 0x20000000000000, 16: 0x40000000000000, 17: 0x80000000000000,
	18: 0x100000000000000, 19: 0x200000000000000,
	50: 0,
}

// lineupOutOfPossessionRoles maps a role index (0x48cf070) to an
// out-of-possession role bit: bits 0..20, 27..33 and 35..45 in order,
// numbered from -31.
var lineupOutOfPossessionRoles = func() map[int8]uint64 {
	out := make(map[int8]uint64)
	i := int8(-31)
	for _, bits := range [][2]int{{0, 20}, {27, 33}, {35, 45}} {
		for b := bits[0]; b <= bits[1]; b++ {
			out[i] = 1 << b
			i++
		}
	}
	return out
}()

// PositionCode returns the position as a code such as "DC" or "WBL"; false
// for an empty, combined or unknown mask.
func (s SlotRole) PositionCode() (string, bool) {
	c, ok := positionCodes[s.Position&^(positionLeft|positionRight)]
	return c, ok
}

// Side returns "left" or "right" for one of a doubled position (two DC,
// two MC…), else "".
func (s SlotRole) Side() string {
	switch s.Position & (positionLeft | positionRight) {
	case positionLeft:
		return "left"
	case positionRight:
		return "right"
	}
	return ""
}

// RoleBit returns Role without its duty bit.
func (s SlotRole) RoleBit() uint64 { return s.Role &^ dutyMask }

// InPossessionRoleName names an in-possession role; false for unknown values.
func (s SlotRole) InPossessionRoleName() (string, bool) { return InPossessionRoleName(s.Role) }

// InPossessionRoleName names an in-possession role value, ignoring its duty
// bit; false for unknown values.
func InPossessionRoleName(role uint64) (string, bool) {
	n, ok := inPossessionRoles[role&^dutyMask]
	return n, ok
}

// OutOfPossessionRoleName names an out-of-possession role; false for unknown
// values.
func (s SlotRole) OutOfPossessionRoleName() (string, bool) {
	n, ok := outOfPossessionRoles[s.RoleBit()]
	return n, ok
}

// mentalityNames is the game's C# Mentality enum (IL2CPP metadata), 1..7.
var mentalityNames = [...]string{1: "Very Defensive", 2: "Defensive", 3: "Cautious", 4: "Balanced", 5: "Positive", 6: "Attacking", 7: "Very Attacking"}

// MentalityName names t.Mentality; false when it is off the scale.
func (t Tactic) MentalityName() (string, bool) { return mentalityName(t.Mentality) }

// MentalityName names f.Mentality; false when it is off the scale.
func (f Formation) MentalityName() (string, bool) { return mentalityName(f.Mentality) }

func mentalityName(v uint8) (string, bool) {
	n := lookup(mentalityNames[:], uint32(v))
	return n, n != ""
}
