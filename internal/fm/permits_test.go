package fm

import (
	"reflect"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func TestWorkPermits(t *testing.T) {
	w := World{Teams: testWorld.Teams, Clubs: testWorld.Clubs, Nations: map[int64]string{139: "England", 172: "Switzerland"}}
	kone := rawPlayer(12095306, "Sékou", "Koné")
	kone.Index = 8334
	none := rawPlayer(2, "Dejan", "Kulusevski")
	none.Index = 9
	permits := []savefile.WorkPermit{
		{Person: 8334, Nation: 139, Year: 2024, Date: savefile.Date{Raw: 242, Year: 2025}, ESC: true},
		{Person: 8334, Nation: 172, Year: 2024, Date: savefile.Date{Raw: 1, Year: savefile.UnsetYear}},
		{Person: 77, Nation: 139, Year: 2020}, // not a player
	}
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, []savefile.Player{kone, none}, w, PlayerExtras{WorkPermits: permits})
	v := g.View(true) // fog of war never hides work permits

	p, _ := v.Profile(12095306)
	want := []WorkPermit{
		{NationRef: NationRef{NationIndex: 139, Nation: "England"}, Year: 2024, ESCPlace: true},
		{NationRef: NationRef{NationIndex: 172, Nation: "Switzerland"}, Year: 2024},
	}
	if !reflect.DeepEqual(p.WorkPermits, want) {
		t.Errorf("WorkPermits = %+v, want %+v", p.WorkPermits, want)
	}
	if p, _ := v.Profile(2); p.WorkPermits != nil {
		t.Errorf("Kulusevski's WorkPermits = %+v, want none", p.WorkPermits)
	}
	rows, err := v.FindPlayers("Kone", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].WorkPermits != nil {
		t.Errorf("search rows = %+v, want Koné without work permits", rows)
	}
}
