package savefile

import "testing"

func FuzzParseTactics(f *testing.F) {
	lineups := map[int32]LastLineup{652: {Opponent: 1545, Players: zazSelection(), Formation: zazFormation()}}
	for _, e := range []tacticsEncoding{enc26, enc27} {
		f.Add(e.tacticsBody(158530, 652, zazSelection(), 0, []Tactic{zazTactic()}, lineups, []int32{652}, zazTactic()))
		f.Add(e.appendTactic(nil, zazTactic(), true))
		f.Add(e.appendHumanTail(nil, 1))
	}
	f.Add(appendSetPieceRoutines(nil, 2))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseTactics(b)
		_, _ = readTactic(newReader(b))
		_ = decodeFormation(b)
		skipSetPieceRoutines(newReader(b))
		skipHumanTail(newReader(b))
	})
}
