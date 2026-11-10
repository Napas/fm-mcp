package savefile

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func indexedReportBody(t *testing.T, frame []byte) []byte {
	t.Helper()
	if len(frame) > maxMatchReportBytes {
		t.Fatal("test frame exceeds the report size cap")
		return nil
	}
	frameSize := uint32(len(frame)) //nolint:gosec // capped at 16 MiB above
	b := binary.LittleEndian.AppendUint32(nil, 401)
	b = append(b, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = binary.LittleEndian.AppendUint32(b, 5)
	b = append(b, []byte("pks_0")...)
	b = binary.LittleEndian.AppendUint32(b, 100000)
	b = append(b, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 395)
	b = binary.LittleEndian.AppendUint32(b, 3)
	b = binary.LittleEndian.AppendUint32(b, frameSize)
	b = binary.LittleEndian.AppendUint32(b, 100000)
	b = binary.LittleEndian.AppendUint32(b, frameSize+3)
	b = append(b, 3)
	b = binary.LittleEndian.AppendUint32(b, frameSize+7)
	b = append(b, 0xaa, 0xbb, 0xcc)
	b = append(b, frame...)
	b = append(b, 0x28, 0xb5, 0x2f, 0xfd) // stale ring bytes, not an indexed report
	return append(b, 1)
}

func TestMatchReportIndexBoundsAndStaleData(t *testing.T) {
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = enc.Close() }()
	body := indexedReportBody(t, enc.EncodeAll(reportBody(), nil))
	r, err := parseMatchReports(body)
	if err != nil || len(r.Reports) != 1 || r.Skipped != 0 || r.Reports[0].ID != 395 {
		t.Fatalf("index: %+v, %v", r, err)
	}
	for n := range len(body) {
		if _, err := parseMatchReports(body[:n]); err == nil {
			t.Fatalf("accepted truncation %d", n)
		}
	}
	for _, offset := range []uint32{0xffffffff, 1000000} {
		bad := append([]byte(nil), body...)
		binary.LittleEndian.PutUint32(bad[37:], offset)
		if _, err := parseMatchReports(bad); err == nil {
			t.Fatal("accepted out-of-bounds frame")
		}
	}
	r, err = parseMatchReports(indexedReportBody(t, enc.EncodeAll([]byte("unknown future format"), nil)))
	if err != nil || r.Skipped != 1 || len(r.Reports) != 0 {
		t.Fatalf("unsupported report: %+v, %v", r, err)
	}
}

func FuzzMatchReportBody(f *testing.F) {
	f.Add(reportBody())
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			t.Skip()
		}
		_, _ = parseMatchReport(1, b)
	})
}

func reportPlayer(person uint32, shirt, side byte) []byte {
	b := make([]byte, 222)
	b[0], b[5], b[6], b[7], b[12] = 2, shirt, side, 2, 2
	binary.LittleEndian.PutUint32(b[1:], person)
	b[86], b[87], b[88], b[92], b[93], b[98] = 33, 28, 2, 1, 1, 1
	binary.LittleEndian.PutUint16(b[136:], 673)
	binary.LittleEndian.PutUint32(b[25:], math.Float32bits(.16))
	binary.LittleEndian.PutUint32(b[191:], math.Float32bits(.20))
	binary.LittleEndian.PutUint32(b[140:], math.Float32bits(11800))
	return b
}

func reportBody() []byte {
	b := make([]byte, 200)
	copy(b, []byte{1, 0, 1, 60, 31})
	binary.LittleEndian.PutUint32(b[11:], 1065339120) // 2026-07-22 20:00
	for side, team := range []uint32{717, 698} {
		i := 100 + side*23
		b[i], b[i+1], b[i+6] = 3, 2, 2
		binary.LittleEndian.PutUint32(b[i+2:], team)
	}
	for side, team := range []uint32{717, 698} {
		block := make([]byte, 452)
		block[0], block[5] = 2, 11
		binary.LittleEndian.PutUint32(block[1:], team)
		b = append(b, block...)
		for j := range 11 {
			b = append(b, reportPlayer(uint32(100+side*100+j), byte(j+1), byte(side))...)
		}
	}
	return b
}

func TestMatchReportBody(t *testing.T) {
	r, err := parseMatchReport(395, reportBody())
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != 395 || r.Home != 717 || r.Away != 698 || r.Kickoff.Format("2006-01-02 15:04") != "2026-07-22 20:00" {
		t.Fatalf("metadata: %+v", r)
	}
	if len(r.Players) != 22 {
		t.Fatalf("players: %d", len(r.Players))
	}
	p := r.Players[0]
	if p.Person != 100 || p.Passes != 33 || p.PassesCompleted != 28 || p.Rating != 6.7 || p.ExpectedGoals != .16 || p.ExpectedAssists != .2 || p.DistanceKM != 11.8 {
		t.Fatalf("stats: %+v", p)
	}
	if r.Players[11].Side != 1 {
		t.Fatal("away side lost")
	}
}

func TestMatchPlayerShotGroupsAndGeneratedReference(t *testing.T) {
	// One group of two shots, versus two groups of one shot. Group headers
	// are 3 bytes; shots are 15 bytes. Grey players have an 11-byte prefix.
	for _, groups := range []byte{1, 2} {
		b := reportPlayer(99, 9, 1)[:216]
		b[41] = 2
		b = append(b, groups)
		for range groups {
			b = append(b, 0, 0, 2/groups)
			b = append(b, make([]byte, 15*2/groups)...)
		}
		b = append(b, make([]byte, 5)...)
		b = append(append([]byte{1}, make([]byte, 10)...), b...)
		p, end, ok := readMatchPlayer(b, 0, 1)
		if !ok || end != len(b) || p.Person != 99 || p.Shots != 2 {
			t.Fatalf("groups %d: %+v, end %d/%d, ok %v", groups, p, end, len(b), ok)
		}
		for n := range len(b) {
			if _, _, ok := readMatchPlayer(b[:n], 0, 1); ok {
				t.Fatalf("accepted truncation %d", n)
			}
		}
	}
}

func TestMatchReportRejectsAmbiguousOrMalformedBody(t *testing.T) {
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:100] },
		func(b []byte) []byte { b[3] = 61; return b },
		func(b []byte) []byte { b[652+87] = 255; return b },
		func(b []byte) []byte { return append(b, b[200:]...) },
	} {
		if _, err := parseMatchReport(1, mutate(reportBody())); err == nil {
			t.Fatal("accepted malformed/ambiguous report")
		}
	}
}

func TestRealCaenMatchReports(t *testing.T) {
	path := os.Getenv("FM_MATCH_REPORT_SAVE")
	if path == "" {
		t.Skip("set FM_MATCH_REPORT_SAVE to a copy of the Caen 2026-07-23 save")
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
	if reports.Skipped != 0 || len(reports.Reports) != 163 {
		t.Fatalf("read %d, skipped %d", len(reports.Reports), reports.Skipped)
	}
	scopes, err := ReadSeasonStats(a)
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	fixtures, err := ReadFixtures(a)
	if err != nil {
		t.Fatal(err)
	}
	known := make(map[string]bool)
	for _, f := range fixtures {
		if f.Result != nil {
			known[fmt.Sprintf("%s/%d/%d", f.Date.Time().Format("2006-01-02"), f.Home, f.Away)] = true
		}
	}
	var linked int
	for _, r := range reports.Reports {
		if known[fmt.Sprintf("%s/%d/%d", r.Kickoff.Format("2006-01-02"), r.Home, r.Away)] {
			linked++
		}
		if r.Home != 717 || r.Away != 698 {
			continue
		}
		for _, p := range r.Players {
			s := scopes[p.Person].Friendlies
			if s == nil || int(s.Starts)+int(s.SubApps) != 1 {
				continue
			}
			if p.Passes != int(s.Detail.Passes) || p.PassesCompleted != int(s.Detail.PassesCompleted) || p.Shots != int(s.Detail.Shots) || p.ShotsOnTarget != int(s.Detail.ShotsOnTarget) || p.Goals != int(s.Goals) || p.Assists != int(s.Assists) || (s.Rated > 0 && p.Rating != float64(s.RatingSum)/10) {
				t.Errorf("person %d report %+v season %+v", p.Person, p, s)
			}
			if p.Person == 15411 || p.Person == 31448 {
				checked++
			}
		}
	}
	if checked != 2 {
		t.Fatalf("Botella/Bomo checked: %d", checked)
	}
	if linked != len(reports.Reports) {
		t.Fatalf("only %d/%d report dates and teams match played fixtures", linked, len(reports.Reports))
	}
}
