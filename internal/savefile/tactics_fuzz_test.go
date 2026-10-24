package savefile

import "testing"

func FuzzParseTactics(f *testing.F) {
	lineups := map[int32]LastLineup{652: {Opponent: 1545, Players: zazSelection(), Formation: zazFormation()}}
	f.Add(tacticsBody(158530, 652, zazSelection(), 0, []Tactic{zazTactic()}, lineups, []int32{652}, zazTactic()))
	f.Add(appendTactic(nil, zazTactic(), true))
	f.Add(appendSetPieceRoutines(nil, 2))
	f.Add(appendHumanTail(nil, 1))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseTactics(b)
		_, _ = readTactic(newReader(b))
		_ = decodeFormation(b)
		skipSetPieceRoutines(newReader(b))
		skipHumanTail(newReader(b))
	})
}
