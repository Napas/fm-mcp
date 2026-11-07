package savefile

// 27.0.0 player instructions ("27.0.0 player instructions" in
// docs/format/tactics.md). Bit n is the C# PlayerInstruction value n; labels
// are the English UI strings for those names.

const (
	playerInstructionsVersion26 = 1 // not decoded
	playerInstructionsVersion27 = 2 // the bitset's version byte
	playerInstructionWords27    = 2
)

// playerInstructions27 is indexed by bit; an empty entry has no value.
var playerInstructions27 = [playerInstructionWords27 * 32]string{
	1:  "Shorter Passing",
	2:  "More Direct Passes",
	3:  "Take More Risks",
	4:  "Take Fewer Risks",
	5:  "Cross More Often",
	6:  "Cross Less Often",
	7:  "Cross From Deep",
	8:  "Cross From Byline",
	9:  "Cross to Near Post",
	10: "Cross to Far Post",
	11: "Cross to Centre",
	12: "Cross to Target Forward",
	13: "Dribble More",
	14: "Dribble Less",
	15: "Run Wide With Ball",
	16: "Cut Inside With Ball",
	17: "Hold Up Ball",
	18: "Shoot More Often",
	19: "Shoot Less Often",
	20: "Be More Expressive",
	21: "Be More Disciplined",
	22: "Get Further Forward",
	23: "Hold Position",
	24: "Roam From Position",
	25: "Stay Wider",
	26: "Sit Narrower",
	27: "Move Into Channels",
	28: "Switch Position",
	29: "Close Down More",
	30: "Close Down Less",
	31: "Tackle Harder",
	32: "Ease Off Tackles",
	33: "Mark Tighter",
	34: "Mark Specific Player",
	35: "Mark Specific Position",
	36: "Attack the Box Centre",
	37: "Attack the Box Left",
	38: "Attack the Box Right",
	39: "Attack the Box Near Post",
	40: "Attack the Box Far Post",
	41: "Attack the Box Front Zone",
	42: "Attack the Box Back Zone",
}

// PlayerInstructions returns the instructions set in a 27.0.0 role option's
// bitset, by bit, and the set bits without a name. A zero bitset (no option
// for the role) or a 26.x one gives none; any other version or word count is
// ErrUnsupported.
func (b Bitset) PlayerInstructions() (set []string, unknown []int, err error) {
	if b == (Bitset{}) || b.Version == playerInstructionsVersion26 {
		return nil, nil, nil
	}
	return namedBits(b, playerInstructionsVersion27, playerInstructionWords27, "player-instruction", playerInstructions27[:])
}
