package fm

import "strings"

// stageSeparator joins a key's stage, group and leg names.
const stageSeparator = " – "

// competitionLabel names a fixture's competition key.
type competitionLabel struct {
	Index       int64  // COMP index
	Name        string // "" when the language database has no name
	Stage       string // stage, group and leg names joined, e.g. "Playoffs – Semi Final – First Leg"
	SecondIndex int64  // the other division of a two-division playoff; -1 = none
	Second      string
}

// keyLabel resolves an ACTUAL_COMP_ID key index. It is false for an index
// outside the table or a key whose competition is not a COMP index.
func (w World) keyLabel(key int32) (competitionLabel, bool) {
	if key < 0 || int(key) >= len(w.Keys) {
		return competitionLabel{}, false
	}
	k := w.Keys[key]
	if !w.isCompetition(int64(k.Comp)) {
		return competitionLabel{}, false
	}
	l := competitionLabel{Index: int64(k.Comp), Name: w.Competitions[int64(k.Comp)], SecondIndex: -1}
	if w.isCompetition(int64(k.SecondComp)) {
		l.SecondIndex, l.Second = int64(k.SecondComp), w.Competitions[int64(k.SecondComp)]
	}
	var stages []string
	for _, s := range []int32{k.Stage, k.Group, k.Leg} {
		if name := w.stageName(int64(s)); name != "" {
			stages = append(stages, name)
		}
	}
	l.Stage = strings.Join(stages, stageSeparator)
	return l, true
}

func (w World) isCompetition(i int64) bool { return i >= 0 && i < int64(len(w.CompetitionUIDs)) }

// stageName returns a STAGE_NAME index's name; "" for -1 or an unknown stage.
func (w World) stageName(i int64) string { return w.Stages[i] }
