package savefile

import (
	"errors"
	"reflect"
	"testing"
)

func playerBits27(set ...int) Bitset { return bitsetOf(2, 2, set...) }

func TestPlayerInstructions27(t *testing.T) {
	for _, tt := range []struct {
		name        string
		in          Bitset
		want        []string
		unknown     []int
		unsupported bool
	}{
		// KŽ's tactic (in game 2026-11-07): Švedkauskas (GK) and Krekovič (AML).
		{"GK in possession", playerBits27(2), []string{"More Direct Passes"}, nil, false},
		{"GK out of possession", playerBits27(32), []string{"Ease Off Tackles"}, nil, false},
		{"AML in possession", playerBits27(19), []string{"Shoot Less Often"}, nil, false},
		{"AML out of possession", playerBits27(31), []string{"Tackle Harder"}, nil, false},
		{"none set", playerBits27(), nil, nil, false},
		{"in bit order", playerBits27(42, 1, 33), []string{"Shorter Passing", "Mark Tighter", "Attack the Box Back Zone"}, nil, false},
		{"unnamed bits", playerBits27(0, 43, 63), nil, []int{0, 43, 63}, false},
		{"no role option", Bitset{}, nil, nil, false},
		{"26.x bitset, not decoded", Bitset{Version: 1, Count: 2, Words: [maxBitsetWords]uint32{4}}, nil, nil, false},
		{"other version", Bitset{Version: 3, Count: 2, Words: [maxBitsetWords]uint32{4}}, nil, nil, true},
		{"other word count", Bitset{Version: 2, Count: 3, Words: [maxBitsetWords]uint32{4}}, nil, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, unknown, err := tt.in.PlayerInstructions()
			if !reflect.DeepEqual(got, tt.want) || !reflect.DeepEqual(unknown, tt.unknown) || errors.Is(err, ErrUnsupported) != tt.unsupported {
				t.Errorf("got %q, unknown %v, err %v; want %q, %v, unsupported %v", got, unknown, err, tt.want, tt.unknown, tt.unsupported)
			}
		})
	}
}

// Every PlayerInstruction value from PI_PASS_SHORTER (1) to PI_ATTACK_BACK_ZONE
// (42) has a label, and no label appears twice.
func TestPlayerInstructionTable27(t *testing.T) {
	seen := map[string]int{}
	for i, label := range playerInstructions27 {
		if (label == "") != (i == 0 || i > 42) {
			t.Errorf("bit %d = %q", i, label)
		}
		if label == "" {
			continue
		}
		if j, dup := seen[label]; dup {
			t.Errorf("bits %d and %d are both %q", j, i, label)
		}
		seen[label] = i
	}
}
