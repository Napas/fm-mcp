package savefile

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestMatchReportStartingXIIndependentOfScoreCounters(t *testing.T) {
	// Offset 145 is not a start flag: it can be 0 for a starter in a 0-0
	// or positive for a substitute. Even entry-clock byte 135 can change
	// for a starter who changes position, so it cannot identify the XI.
	for _, counter := range []byte{0, 1, 2, 4} {
		b := append([]byte(nil), reportBody()[:200]...)
		for side, team := range []uint32{717, 698} {
			block := make([]byte, 452)
			block[0], block[5] = 2, 14
			binary.LittleEndian.PutUint32(block[1:], team)
			b = append(b, block...)
			for slot := range 14 {
				p := reportPlayer(uint32(100+side*100+slot), byte(90-slot), byte(side))
				p[145] = counter
				p[135] = 1
				if slot == 3 || slot == 11 {
					p[135] = 51
				}
				if slot == 12 {
					p[135] = 0
				}
				if slot == 3 || slot == 13 { // Extended references must not shift slot numbering.
					p = append(append([]byte{1}, make([]byte, 10)...), p...)
				}
				b = append(b, p...)
			}
		}
		r, err := parseMatchReport(1128, b)
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range r.Players {
			if want := i%14 < 11; p.Started != want {
				t.Errorf("counter %d, side %d slot %d person %d: started %v, want %v", counter, p.Side, i%14, p.Person, p.Started, want)
			}
		}
	}
}

// Read actual report data, not a substitute from get_last_lineup. LastLineups
// only supplies an independent check when this is the team's latest fixture.
func TestRealMatchReportStartingXI(t *testing.T) {
	path := os.Getenv("FM_MATCH_STARTERS_SAVE")
	if path == "" {
		t.Skip("set FM_MATCH_STARTERS_SAVE to a copy of the Caen 2026-08-03 save")
	}
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	reports, err := ReadMatchReports(a)
	if err != nil {
		t.Fatal(err)
	}
	if reports.Skipped != 0 {
		t.Fatalf("skipped %d reports", reports.Skipped)
	}
	fixtures, err := ReadFixtures(a)
	if err != nil {
		t.Fatal(err)
	}
	tactics, err := ReadTactics(a)
	if err != nil {
		t.Fatal(err)
	}
	type latestMatch struct {
		date      time.Time
		opponent  int32
		ambiguous bool
	}
	latest := make(map[int64]latestMatch)
	lineups := make(map[int64]LastLineup, len(tactics.LastLineups))
	for team, lineup := range tactics.LastLineups {
		lineups[int64(team)] = lineup
	}
	for _, f := range fixtures {
		if f.Result == nil {
			continue
		}
		for _, pair := range [][2]int32{{f.Home, f.Away}, {f.Away, f.Home}} {
			team := int64(pair[0])
			prev, ok := latest[team]
			date := f.Date.Time()
			if !ok || date.After(prev.date) {
				latest[team] = latestMatch{date: date, opponent: pair[1]}
			} else if date.Equal(prev.date) {
				prev.ambiguous = true
				latest[team] = prev
			}
		}
	}
	checked, caenChecked := 0, false
	for _, r := range reports.Reports {
		var counts [2]int
		for _, p := range r.Players {
			if p.Started {
				counts[p.Side]++
			}
		}
		if counts != [2]int{11, 11} {
			t.Errorf("report %d: starts %v, want 11 per side", r.ID, counts)
		}
		for side, team := range []uint32{r.Home, r.Away} {
			l, ok := lineups[int64(team)]
			last, hasLatest := latest[int64(team)]
			date := r.Kickoff.Format(time.DateOnly)
			opponent := r.Away
			if side == 1 {
				opponent = r.Home
			}
			if !ok || !hasLatest || last.ambiguous || int64(last.opponent) != int64(opponent) || date != last.date.Format(time.DateOnly) || int64(l.Opponent) != int64(opponent) {
				continue
			}
			var slots []uint32
			for _, p := range r.Players {
				if p.Side == byte(side) && p.Started {
					slots = append(slots, p.Person)
				}
			}
			for slot, person := range slots {
				if int64(person) != int64(l.Players[slot]) {
					t.Errorf("report %d team %d slot %d: person %d, last lineup %d", r.ID, team, slot, person, l.Players[slot])
				}
			}
			checked++
			if r.ID == 1128 && team == 717 {
				caenChecked = true
			}
		}
		if r.ID == 395 || r.ID == 1128 {
			found := false
			for _, p := range r.Players {
				if p.Person == 31448 {
					found = true
					if !p.Started || !p.Played {
						t.Errorf("Bomo in report %d: %+v", r.ID, p)
					}
				}
			}
			if !found {
				t.Errorf("Bomo missing in report %d", r.ID)
			}
		}
	}
	t.Logf("%d reports; %d teams' starting XIs independently compared with last-lineup records", len(reports.Reports), checked)
	if !caenChecked || checked < 20 {
		t.Fatalf("insufficient independent checks: %d, Caen %v", checked, caenChecked)
	}
}
