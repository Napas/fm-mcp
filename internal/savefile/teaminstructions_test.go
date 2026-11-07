package savefile

import (
	"errors"
	"reflect"
	"testing"
)

// KŽ's tactic as the Team Instructions screen showed it (in game, 2026-11-07,
// game date 2026-05-04): every option that is not the default.
func TestTeamInstructions27KZ(t *testing.T) {
	got, unknown, err := kzInstructions27.TeamInstructions()
	if err != nil || len(unknown) != 0 {
		t.Fatalf("err %v, unknown bits %v; want none", err, unknown)
	}
	want := []TeamInstruction{
		{InPossessionInstructions, "Passing Directness", "Shorter"},
		{InPossessionInstructions, "Build-Up Strategy", "Play Through Press"},
		{InPossessionInstructions, "Shots from Distance", "Discourage"},
		{InPossessionInstructions, "Patience", "Work Ball Into Box"},
		{InPossessionInstructions, "Play for Set Pieces", "Keep Ball in Play"},
		{InPossessionInstructions, "GK Distribution", "Centre-Backs"},
		{InPossessionInstructions, "Goal Kicks", "Short"},
		{InPossessionInstructions, "Attacking Width", "Wider"},
		{OutOfPossessionInstructions, "Defensive Line", "Higher"},
		{OutOfPossessionInstructions, "Line of Engagement", "High Press"},
		{OutOfPossessionInstructions, "Trigger Press", "More Often"},
		{OutOfPossessionInstructions, "Short Goalkeeper Distribution", "Yes"},
		{OutOfPossessionInstructions, "Defensive Transition", "Counter-Press"},
		{InPossessionInstructions, "Time Wasting", "Standard"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestTeamInstructions27(t *testing.T) {
	bits := func(set ...int) Bitset { return bitsetOf(teamInstructionsVersion27, teamInstructionWords27, set...) }
	for _, tt := range []struct {
		name        string
		in          Bitset
		want        []TeamInstruction
		unknown     []int
		unsupported bool
	}{
		{"preset", bits(62), []TeamInstruction{{OutOfPossessionInstructions, "Line of Engagement", "Mid Block"}}, nil, false},
		{"none set", bits(), nil, nil, false},
		{"unnamed bits", bits(0, 41, 83, 95), []TeamInstruction{{OutOfPossessionInstructions, "Cross Engagement", "Invite Crosses"}}, []int{0, 41, 95}, false},
		{"26.x bitset, not decoded", Bitset{Version: 6, Count: 3, Words: [maxBitsetWords]uint32{0x008330a4}}, nil, nil, false},
		{"other version", Bitset{Version: 8, Count: 3, Words: [maxBitsetWords]uint32{1 << 3}}, nil, nil, true},
		{"other word count", Bitset{Version: teamInstructionsVersion27, Count: 4, Words: [maxBitsetWords]uint32{1 << 3}}, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, unknown, err := tt.in.TeamInstructions()
			if !reflect.DeepEqual(got, tt.want) || !reflect.DeepEqual(unknown, tt.unknown) || errors.Is(err, ErrUnsupported) != tt.unsupported {
				t.Errorf("got %+v, unknown %v, err %v; want %+v, %v, unsupported %v", got, unknown, err, tt.want, tt.unknown, tt.unsupported)
			}
		})
	}
}

// Every named bit has an instruction, an option and a phase, and no option
// appears twice.
func TestTeamInstructionTable27(t *testing.T) {
	seen := map[[2]string]int{}
	for i, o := range teamInstructions27 {
		if o == (TeamInstruction{}) {
			continue
		}
		if o.Instruction == "" || o.Option == "" || o.Phase > SetPieceInstructions {
			t.Errorf("bit %d = %+v", i, o)
		}
		k := [2]string{o.Instruction, o.Option}
		if j, dup := seen[k]; dup {
			t.Errorf("bits %d and %d are both %v", j, i, k)
		}
		seen[k] = i
	}
}
