package fm

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestAssignMaximisesTheTotal(t *testing.T) {
	for _, tt := range []struct {
		name  string
		score [][]int
		want  []int
	}{
		{"greedy would lose", [][]int{{10, 9}, {10, 1}}, []int{1, 0}},
		{"more players than slots", [][]int{{1, 5, 3}, {4, 6, 2}}, []int{1, 0}},
		{"fewer players than slots", [][]int{{3}, {7}}, []int{-1, 0}},
		{"no players", [][]int{{}, {}}, []int{-1, -1}},
		{"no slots", nil, []int{}},
		{"ties keep the earlier player", [][]int{{5, 5}}, []int{0}},
		// Total 5 three ways; [3 1 0] has the smallest player index sum (4).
		{"ties keep the earliest players", [][]int{{1, 0, 1, 2}, {0, 1, 0, 2}, {2, 0, 0, 2}}, []int{3, 1, 0}},
	} {
		if got := assign(tt.score); !slices.Equal(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAssignMatchesBruteForce(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 300 {
		slots, players := 1+r.IntN(5), r.IntN(7)
		score := make([][]int, slots)
		for i := range score {
			score[i] = make([]int, players)
			for j := range score[i] {
				score[i][j] = r.IntN(200)
			}
		}
		got := assign(score)
		used := map[int]bool{}
		total := 0
		for i, j := range got {
			if j < 0 {
				continue
			}
			if used[j] {
				t.Fatalf("%v: player %d assigned twice in %v", score, j, got)
			}
			used[j] = true
			total += score[i][j]
		}
		if filled, want := len(used), min(slots, players); filled != want {
			t.Fatalf("%v: %d slots filled, want %d", score, filled, want)
		}
		if want := bestTotal(score, 0, map[int]bool{}); total != want {
			t.Fatalf("%v: total %d with %v, want %d", score, total, got, want)
		}
	}
}

func bestTotal(score [][]int, slot int, used map[int]bool) int {
	if slot == len(score) {
		return 0
	}
	best := bestTotal(score, slot+1, used) // slot left empty
	for j := range score[slot] {
		if used[j] {
			continue
		}
		used[j] = true
		best = max(best, score[slot][j]+bestTotal(score, slot+1, used))
		used[j] = false
	}
	return best
}
