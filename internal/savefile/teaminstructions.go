package savefile

import "fmt"

// 27.0.0 team instructions ("27.0.0 team instructions" in
// docs/format/tactics.md). Bit n is entry n of the option-id table at
// 0x52f34d8 in game_plugin (ids are TacticTeamInstructionOptions values).
// Titles and the options KŽ's tactic uses come from the Team Instructions
// screen; the other labels are from the C# names.

const (
	teamInstructionsVersion26 = 6 // not decoded
	teamInstructionsVersion27 = 7 // the bitset's version byte
	teamInstructionWords27    = 3
)

// InstructionPhase groups team instructions as the Team Instructions screen
// does, in its order.
type InstructionPhase uint8

const (
	InPossessionInstructions InstructionPhase = iota
	OutOfPossessionInstructions
	SetPieceInstructions
)

// TeamInstruction is one option a tactic sets.
type TeamInstruction struct {
	Phase       InstructionPhase
	Instruction string // e.g. "Passing Directness"
	Option      string // e.g. "Shorter"
}

// teamInstructions27 is indexed by bit; a zero entry has no option id.
var teamInstructions27 = [teamInstructionWords27 * 32]TeamInstruction{
	1:  {InPossessionInstructions, "Passing Directness", "Much More Direct"},
	2:  {InPossessionInstructions, "Passing Directness", "More Direct"},
	3:  {InPossessionInstructions, "Passing Directness", "Shorter"},
	4:  {InPossessionInstructions, "Passing Directness", "Much Shorter"},
	5:  {InPossessionInstructions, "Pass Reception", "Into Space"},
	6:  {InPossessionInstructions, "Pass Reception", "Into Feet"},
	7:  {InPossessionInstructions, "Build-Up Strategy", "Play Through Press"},
	8:  {InPossessionInstructions, "Build-Up Strategy", "Bypass Press"},
	9:  {InPossessionInstructions, "Patience", "Hit Early Crosses"},
	10: {InPossessionInstructions, "Crossing Style", "Floated Crosses"},
	11: {InPossessionInstructions, "Crossing Style", "Whipped Crosses"},
	12: {InPossessionInstructions, "Crossing Style", "Low Crosses"},
	13: {InPossessionInstructions, "Dribbling", "Run at Defence"},
	14: {InPossessionInstructions, "Dribbling", "Dribble Less"},
	15: {InPossessionInstructions, "Shots from Distance", "Encourage"},
	16: {InPossessionInstructions, "Shots from Distance", "Discourage"},
	17: {InPossessionInstructions, "Patience", "Work Ball Into Box"},
	18: {InPossessionInstructions, "Play for Set Pieces", "Play for Set Pieces"},
	19: {InPossessionInstructions, "Creative Freedom", "More Expressive"},
	20: {InPossessionInstructions, "Creative Freedom", "More Disciplined"},
	21: {InPossessionInstructions, "Progress Through", "Left Flank"},
	22: {InPossessionInstructions, "Progress Through", "Right Flank"},
	23: {InPossessionInstructions, "Progress Through", "Middle"},
	24: {InPossessionInstructions, "Progress Through", "Both Flanks"},
	25: {InPossessionInstructions, "Supporting Runs", "Overlap Both Flanks"},
	26: {InPossessionInstructions, "Supporting Runs", "Overlap Left"},
	27: {InPossessionInstructions, "Supporting Runs", "Overlap Right"},
	28: {InPossessionInstructions, "Supporting Runs", "Underlap Both Flanks"},
	29: {InPossessionInstructions, "Supporting Runs", "Underlap Left"},
	30: {InPossessionInstructions, "Supporting Runs", "Underlap Right"},
	// Inference: bit 33 has no option id; KŽ's tactic sets it and Keep Ball in
	// Play is the one setting on screen no other bit explains.
	33: {InPossessionInstructions, "Play for Set Pieces", "Keep Ball in Play"},
	35: {InPossessionInstructions, "GK Distribution", "Centre-Backs"},
	36: {InPossessionInstructions, "GK Distribution", "Full-Backs"},
	37: {InPossessionInstructions, "GK Distribution", "Flanks"},
	38: {InPossessionInstructions, "GK Distribution", "Over Opposition Defence"},
	39: {InPossessionInstructions, "GK Distribution", "Playmaker"},
	40: {InPossessionInstructions, "GK Distribution", "Target Forward"},
	42: {InPossessionInstructions, "GK Distribution (Speed)", "Quick"},
	43: {InPossessionInstructions, "GK Distribution (Speed)", "Slow"},
	44: {InPossessionInstructions, "Goal Kicks", "Short"},
	45: {InPossessionInstructions, "Goal Kicks", "Long"},
	46: {InPossessionInstructions, "Tempo", "Much Higher"},
	47: {InPossessionInstructions, "Tempo", "Higher"},
	48: {InPossessionInstructions, "Tempo", "Lower"},
	49: {InPossessionInstructions, "Tempo", "Much Lower"},
	50: {InPossessionInstructions, "Attacking Width", "Much Wider"},
	51: {InPossessionInstructions, "Attacking Width", "Wider"},
	52: {InPossessionInstructions, "Attacking Width", "Narrower"},
	53: {InPossessionInstructions, "Attacking Width", "Much Narrower"},
	54: {InPossessionInstructions, "Attacking Transition", "Counter"},
	55: {InPossessionInstructions, "Attacking Transition", "Hold Shape"},
	56: {OutOfPossessionInstructions, "Defensive Line", "Much Higher"},
	57: {OutOfPossessionInstructions, "Defensive Line", "Higher"},
	58: {OutOfPossessionInstructions, "Defensive Line", "Deeper"},
	59: {OutOfPossessionInstructions, "Defensive Line", "Much Deeper"},
	60: {SetPieceInstructions, "Goalkeeper Up for Attacking Set Pieces", "Yes"},
	61: {OutOfPossessionInstructions, "Line of Engagement", "High Press"},
	62: {OutOfPossessionInstructions, "Line of Engagement", "Mid Block"},
	63: {OutOfPossessionInstructions, "Line of Engagement", "Low Block"},
	65: {OutOfPossessionInstructions, "Trigger Press", "Much More Often"},
	66: {OutOfPossessionInstructions, "Trigger Press", "More Often"},
	67: {OutOfPossessionInstructions, "Trigger Press", "Less Often"},
	68: {OutOfPossessionInstructions, "Trigger Press", "Much Less Often"},
	// The game's name is Prevent Short GK Distribution.
	69: {OutOfPossessionInstructions, "Short Goalkeeper Distribution", "Yes"},
	70: {OutOfPossessionInstructions, "Defensive Transition", "Counter-Press"},
	71: {OutOfPossessionInstructions, "Defensive Transition", "Regroup"},
	72: {OutOfPossessionInstructions, "Tackling", "Get Stuck In"},
	73: {OutOfPossessionInstructions, "Tackling", "Stay on Feet"},
	76: {OutOfPossessionInstructions, "Defensive Line Behaviour", "Hold Line"},
	77: {OutOfPossessionInstructions, "Defensive Line Behaviour", "Drop Off"},
	78: {InPossessionInstructions, "Time Wasting", "More Often"},
	79: {InPossessionInstructions, "Time Wasting", "Standard"},
	80: {OutOfPossessionInstructions, "Pressing Trap", "Trap Inside"},
	81: {OutOfPossessionInstructions, "Pressing Trap", "Trap Outside"},
	82: {OutOfPossessionInstructions, "Cross Engagement", "Stop Crosses"},
	83: {OutOfPossessionInstructions, "Cross Engagement", "Invite Crosses"},
}

// TeamInstructions returns the options set in a 27.0.0 bitset, by bit, and the
// set bits without a name. A 26.x bitset gives none; any other version or
// word count is ErrUnsupported.
func (b Bitset) TeamInstructions() (set []TeamInstruction, unknown []int, err error) {
	if b.Version == teamInstructionsVersion26 {
		return nil, nil, nil
	}
	if b.Version != teamInstructionsVersion27 || b.Count != teamInstructionWords27 {
		return nil, nil, fmt.Errorf("%w: team-instruction bitset version %d with %d words", ErrUnsupported, b.Version, b.Count)
	}
	for i, o := range teamInstructions27 {
		if !b.Has(i) {
			continue
		}
		if o == (TeamInstruction{}) {
			unknown = append(unknown, i)
			continue
		}
		set = append(set, o)
	}
	return set, unknown, nil
}
