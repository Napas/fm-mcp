package savefile

import "testing"

func TestClauseSemantics(t *testing.T) {
	release := Clause{Type: 0, Value: 60_000_000, Extra: 0xffff}
	sellOn := Clause{Type: 8, Value: -1, Extra: 15}
	extension := Clause{Type: 22, Value: -1, Extra: 1}

	if release.Name() != "minimum fee release clause" || sellOn.Name() != "sell-on percentage" || (Clause{Type: 99}).Name() != "unknown clause" {
		t.Errorf("names = %q %q", release.Name(), sellOn.Name())
	}
	if v, ok := release.Amount(); !ok || v != 60_000_000 {
		t.Errorf("release amount = %d, %v", v, ok)
	}
	if _, ok := sellOn.Amount(); ok {
		t.Error("sell-on has an amount")
	}
	if v, ok := sellOn.Percent(); !ok || v != 15 {
		t.Errorf("sell-on percent = %d, %v", v, ok)
	}
	if v, ok := extension.Count(); !ok || v != 1 {
		t.Errorf("extension count = %d, %v", v, ok)
	}
	if _, ok := release.Count(); ok {
		t.Error("release clause has a count")
	}
	// Release clauses sometimes carry an Extra of unknown meaning.
	if v, ok := (Clause{Type: 0, Value: 50_000_000, Extra: 1011}).Count(); ok {
		t.Errorf("release clause with Extra 1011 reports count %d", v)
	}
}

func TestCompetitionBonusSemantics(t *testing.T) {
	qualify := CompetitionBonus{Type: 10, Competition: 11, Target: 637, Value: 60964}
	if name, ok := qualify.Name(); !ok || name != "qualification bonus" {
		t.Errorf("type 10 name = %q, %v", name, ok)
	}
	if v, ok := qualify.Amount(); !ok || v != 60964 {
		t.Errorf("amount = %d, %v", v, ok)
	}
	if name, ok := (CompetitionBonus{Type: 6}).Name(); ok {
		t.Errorf("type 6 is named %q", name)
	}
	if _, ok := (CompetitionBonus{Type: 10, Value: -1}).Amount(); ok {
		t.Error("negative value reported as an amount")
	}
}

func TestPlayingTimeNames(t *testing.T) {
	want := map[PlayingTime]string{
		1: "Star Player", 2: "Important Player", 3: "Regular Starter", 4: "Squad Player",
		5: "Impact Sub", 6: "Squad Player", 7: "Fringe Player", 8: "Fringe Player",
		9: "Emergency Backup", 10: "Breakthrough Prospect", 11: "Future Prospect", 12: "Hot Prospect",
		13: "Youngster", 14: "B Team Regular", 15: "First-Choice Goalkeeper", 16: "Cup Goalkeeper",
		17: "Domestic Cup Goalkeeper", 18: "Continental Cup Goalkeeper", 19: "Cup Goalkeeper", 20: "Backup",
		21: "Emergency Backup", 22: "Surplus to Requirements",
	}
	for v := range PlayingTime(255) {
		name, ok := v.Name()
		if w, known := want[v]; ok != known || name != w {
			t.Errorf("PlayingTime(%d).Name() = %q, %v; want %q, %v", v, name, ok, w, known)
		}
	}
}
