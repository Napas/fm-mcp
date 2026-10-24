package fm

import (
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func labelWorld() World {
	return World{
		Competitions:    map[int64]string{8: "Sky Bet Championship", 11: "Ligue 1", 12: "Ligue 2"},
		CompetitionUIDs: make([]uint32, 13),
		Stages:          map[int64]string{236: "Playoffs", 17: "Semi Final", 2: "First Leg"},
		Keys: []savefile.CompetitionKey{
			{Comp: 8, Stage: 236, Group: 17, Leg: 2, SecondComp: -1},
			{Comp: 8, Stage: -1, Group: 19, Leg: -1, SecondComp: -1},
			{Comp: 11, Stage: -1, Group: -1, Leg: -1, SecondComp: 12},
			{Comp: -1, Stage: 236, Group: -1, Leg: -1, SecondComp: -1},
		},
	}
}

func TestKeyLabelJoinsStages(t *testing.T) {
	got, ok := labelWorld().keyLabel(0)

	want := competitionLabel{Index: 8, Name: "Sky Bet Championship", Stage: "Playoffs – Semi Final – First Leg", SecondIndex: -1}
	if !ok || got != want {
		t.Errorf("keyLabel(0) = %+v, %v; want %+v", got, ok, want)
	}
}

func TestKeyLabelSkipsUnknownStage(t *testing.T) {
	got, ok := labelWorld().keyLabel(1)

	if !ok || got.Stage != "" || got.Name != "Sky Bet Championship" {
		t.Errorf("keyLabel(1) = %+v, %v; want no stage", got, ok)
	}
}

func TestKeyLabelNamesBothDivisions(t *testing.T) {
	got, ok := labelWorld().keyLabel(2)

	want := competitionLabel{Index: 11, Name: "Ligue 1", SecondIndex: 12, Second: "Ligue 2"}
	if !ok || got != want {
		t.Errorf("keyLabel(2) = %+v, %v; want %+v", got, ok, want)
	}
}

func TestKeyLabelRejectsUnresolvedKeys(t *testing.T) {
	w := labelWorld()
	for _, key := range []int32{-1, 3, 4} {
		if got, ok := w.keyLabel(key); ok {
			t.Errorf("keyLabel(%d) = %+v, want unresolved", key, got)
		}
	}
}

func TestStageName(t *testing.T) {
	w := labelWorld()
	if got := w.stageName(236); got != "Playoffs" {
		t.Errorf("stageName(236) = %q", got)
	}
	if got := w.stageName(-1); got != "" {
		t.Errorf("stageName(-1) = %q, want empty", got)
	}
}
