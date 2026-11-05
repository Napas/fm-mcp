package fm

import "math"

// assign picks a player (column) per slot (row) for the highest total, each
// player once. It returns the player index per slot, -1 when players are
// short. Ties go to the smallest sum of player indices (slot indices when
// players are short).
func assign(score [][]int) []int {
	if len(score) == 0 {
		return nil
	}
	slots, players := len(score), len(score[0])
	if players >= slots {
		return hungarian(slots, players, func(i, j int) int { return score[i][j] })
	}
	out := make([]int, slots)
	for i := range out {
		out[i] = -1
	}
	for player, slot := range hungarian(players, slots, func(i, j int) int { return score[j][i] }) {
		out[slot] = player
	}
	return out
}

// hungarian gives each row a distinct column (cols ≥ rows) for the highest
// total and returns the column per row. A column-index penalty under one score
// point breaks ties towards the smallest column index sum.
func hungarian(rows, cols int, score func(i, j int) int) []int {
	scale := int64(rows*cols + 1)
	cost := func(i, j int) int64 { return -int64(score(i, j))*scale + int64(j) }
	// 1-based; row 0 and column 0 are sentinels.
	u, v := make([]int64, rows+1), make([]int64, cols+1)
	match, way := make([]int, cols+1), make([]int, cols+1)
	for i := 1; i <= rows; i++ {
		match[0] = i
		j0 := 0
		minv := make([]int64, cols+1)
		for j := range minv {
			minv[j] = math.MaxInt64
		}
		used := make([]bool, cols+1)
		for {
			used[j0] = true
			i0, delta, j1 := match[j0], int64(math.MaxInt64), 0
			for j := 1; j <= cols; j++ {
				if used[j] {
					continue
				}
				if c := cost(i0-1, j-1) - u[i0] - v[j]; c < minv[j] {
					minv[j], way[j] = c, j0
				}
				if minv[j] < delta {
					delta, j1 = minv[j], j
				}
			}
			for j := 0; j <= cols; j++ {
				if used[j] {
					u[match[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			if j0 = j1; match[j0] == 0 {
				break
			}
		}
		for j0 != 0 {
			j1 := way[j0]
			match[j0] = match[j1]
			j0 = j1
		}
	}
	out := make([]int, rows)
	for j := 1; j <= cols; j++ {
		if match[j] != 0 {
			out[match[j]-1] = j - 1
		}
	}
	return out
}
