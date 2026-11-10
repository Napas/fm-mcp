package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

const (
	maxMatchReportBytes    = 16 << 20
	maxAllMatchReportBytes = 256 << 20
	maxMatchReports        = 20000
)

// MatchReports contains only reports whose supported structure validated.
// Skipped counts indexed reports that could not be decoded, never zero stats.
type MatchReports struct {
	Reports []MatchReport
	Skipped int
}

// MatchReport is an indexed match_stats_cfg.dat PKS record, format 60/FM27.
// IDs belong to this save. The game's ring buffers can discard old reports.
type MatchReport struct {
	ID         uint32
	Kickoff    time.Time
	Home, Away uint32
	Players    []MatchPlayer
}

// MatchPlayer contains verified counters, not reconstructed season deltas.
// Minutes and goalkeeper-specific counters are deliberately not interpreted.
type MatchPlayer struct {
	Person                                              uint32
	Side, Shirt                                         uint8
	Played, Started                                     bool
	Rating                                              float64
	Goals, Assists, Shots, ShotsOnTarget                int
	Passes, PassesCompleted, KeyPasses                  int
	Tackles, TacklesWon, Dribbles, Fouls, Interceptions int
	ExpectedGoals, ExpectedAssists, DistanceKM          float64
}

// ReadMatchReports reads live PKS indexes, not a magic-byte scan: unused
// ring-buffer space contains stale/partial compressed frames.
func ReadMatchReports(a *Archive) (MatchReports, error) {
	f, err := a.ReadFile("match_stats_cfg.dat")
	if err != nil {
		return MatchReports{}, err
	}
	if f.Header.Version != 6 {
		return MatchReports{}, fmt.Errorf("%w: match_stats_cfg.dat version %d", ErrUnsupported, f.Header.Version)
	}
	return parseMatchReports(f.Body)
}

func parseMatchReports(body []byte) (MatchReports, error) {
	r := newReader(body)
	r.u32() // next report ID
	flags := [2]byte{r.u8(), r.u8()}
	if flags != [2]byte{1, 1} {
		return MatchReports{}, fmt.Errorf("%w: PKS header", ErrUnsupported)
	}
	groups := r.u32()
	if groups > 16 {
		return MatchReports{}, fmt.Errorf("%w: %d PKS groups", ErrCorrupt, groups)
	}
	dec, err := newDecoder(maxMatchReportBytes)
	if err != nil {
		return MatchReports{}, err
	}
	defer dec.Close()
	var out MatchReports
	seen := make(map[uint32]bool)
	totalBytes := 0
	for range groups {
		r.u32() // group ID
		name := r.str()
		r.u32() // configured capacity
		flags := [2]byte{r.u8(), r.u8()}
		if flags != [2]byte{1, 1} {
			r.fail("unsupported PKS group flags")
		}
		n := r.u32()
		if n > maxMatchReports || len(seen)+int(n) > maxMatchReports {
			r.fail("too many match reports")
			return out, r.err
		}
		type entry struct{ id, offset, size uint32 }
		entries := make([]entry, 0, int(n))
		for range n {
			entries = append(entries, entry{r.u32(), r.u32(), r.u32()})
		}
		r.u32() // capacity
		r.u32() // ring write position, NOT payload length
		if r.u8() != 3 {
			r.fail("unsupported PKS storage kind")
		}
		payload := r.take(int(r.u32()))
		r.u8() // group trailer
		if r.err != nil {
			return MatchReports{}, r.err
		}
		for _, e := range entries {
			if seen[e.id] {
				return MatchReports{}, fmt.Errorf("%w: duplicate report %d", ErrCorrupt, e.id)
			}
			seen[e.id] = true
			end := uint64(e.offset) + uint64(e.size)
			if e.size == 0 || end > uint64(len(payload)) {
				return MatchReports{}, fmt.Errorf("%w: %s report %d outside PKS buffer", ErrCorrupt, name, e.id)
			}
			if err := dec.Reset(bytes.NewReader(payload[e.offset:end])); err != nil {
				out.Skipped++
				continue
			}
			// A streaming cap also handles frames without a declared content size.
			buf, err := io.ReadAll(io.LimitReader(dec, maxMatchReportBytes+1))
			if err != nil || len(buf) > maxMatchReportBytes {
				out.Skipped++
				continue
			}
			totalBytes += len(buf)
			if totalBytes > maxAllMatchReportBytes {
				return MatchReports{}, fmt.Errorf("%w: match report decompression budget exceeded", ErrCorrupt)
			}
			report, err := parseMatchReport(e.id, buf)
			if err != nil {
				out.Skipped++
				continue
			}
			out.Reports = append(out.Reports, report)
		}
	}
	if r.err != nil {
		return MatchReports{}, r.err
	}
	if r.remaining() != 0 {
		return MatchReports{}, fmt.Errorf("%w: trailing PKS bytes", ErrUnsupported)
	}
	return out, nil
}

func parseMatchReport(id uint32, b []byte) (MatchReport, error) {
	if len(b) < 64 || !bytes.Equal(b[:5], []byte{1, 0, 1, 60, 31}) {
		return MatchReport{}, fmt.Errorf("%w: match report format", ErrUnsupported)
	}
	minutes := binary.LittleEndian.Uint32(b[11:])
	date := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(minutes/1440)).Add(time.Duration(minutes%1440) * time.Minute)
	if date.Year() < 1900 || date.Year() > 4000 {
		return MatchReport{}, fmt.Errorf("%w: match date", ErrCorrupt)
	}
	// The two serialized team references in the header are 23 bytes each.
	// Require a unique pair and independently validate each team's full list.
	var pairs []int
	for i := 0; i+46 <= min(len(b), 1024); i++ {
		if b[i] == 3 && b[i+1] == 2 && b[i+6] == 2 && b[i+23] == 3 && b[i+24] == 2 && b[i+29] == 2 {
			pairs = append(pairs, i)
		}
	}
	if len(pairs) != 1 {
		return MatchReport{}, fmt.Errorf("%w: ambiguous report teams", ErrUnsupported)
	}
	i := pairs[0]
	out := MatchReport{ID: id, Kickoff: date, Home: binary.LittleEndian.Uint32(b[i+2:]), Away: binary.LittleEndian.Uint32(b[i+25:])}
	if out.Home == out.Away {
		return MatchReport{}, fmt.Errorf("%w: identical report teams", ErrCorrupt)
	}
	for side, team := range []uint32{out.Home, out.Away} {
		players, ok := readMatchTeam(b, team, byte(side))
		if !ok {
			return MatchReport{}, fmt.Errorf("%w: report %d team %d player list", ErrUnsupported, id, team)
		}
		out.Players = append(out.Players, players...)
	}
	return out, nil
}

// Team tactics make the header variable-length. Search a bounded region
// after a matching team reference, then require its entire declared roster
// to parse (including shots, side flags and unique person references).
func readMatchTeam(b []byte, team uint32, side byte) ([]MatchPlayer, bool) {
	q := binary.LittleEndian.AppendUint32([]byte{2}, team)
	var found []MatchPlayer
	foundStart := -1
	for pos := 0; pos < len(b); {
		rel := bytes.Index(b[pos:], q)
		if rel < 0 {
			break
		}
		pos += rel
		base := pos
		pos++
		if base+6 > len(b) {
			continue
		}
		n := int(b[base+5])
		if n < 11 || n > 64 {
			continue
		}
		for start := base + 400; start < min(base+600, len(b)-221); start++ {
			j := start
			var players []MatchPlayer
			seen := make(map[uint32]bool, n)
			for slot := range n {
				p, end, ok := readMatchPlayer(b, j, side)
				if !ok || seen[p.Person] {
					break
				}
				seen[p.Person] = true
				// This roster stores the initial XI in tactic-slot order,
				// followed by the bench. Player byte 145 is a match counter,
				// not a start flag (it is zero throughout a 0-0 report).
				p.Started = slot < tacticSlots
				players = append(players, p)
				j = end
			}
			if len(players) != n {
				continue
			}
			canonical := start
			if b[start] == 1 {
				canonical += 11
			}
			if foundStart >= 0 && canonical != foundStart {
				return nil, false
			}
			foundStart = canonical
			found = players
		}
	}
	return found, foundStart >= 0
}

func readMatchPlayer(b []byte, start int, side byte) (MatchPlayer, int, bool) {
	var p MatchPlayer
	i := start
	if i < 0 || i >= len(b) {
		return p, 0, false
	}
	if b[i] == 1 {
		i += 11
	} // generated/grey player's extended reference
	if i+222 > len(b) {
		return p, 0, false
	}
	s := b[i:]
	if s[0] != 2 || s[5] > 99 || s[6] != side || s[7] != 2 || s[12] != 2 || s[87] > s[86] || s[42] > s[41] {
		return p, 0, false
	}
	rating := binary.LittleEndian.Uint16(s[136:])
	if rating > 1000 {
		return p, 0, false
	}
	j := i + 216
	groups := int(b[j])
	j++
	if groups > 32 {
		return p, 0, false
	}
	shots := 0
	for range groups {
		if j+3 > len(b) {
			return p, 0, false
		}
		n := int(b[j+2])
		shots += n
		j += 3 + n*15
		if j > len(b) {
			return p, 0, false
		}
	}
	if j+5 > len(b) || (groups > 0 && shots != int(s[41])) {
		return p, 0, false
	}
	f := func(off int) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(s[off:]))) }
	xg, xa, metres := f(25), f(191), f(140)
	for _, v := range []float64{xg, xa, metres} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return p, 0, false
		}
	}
	if xg > 100 || xa > 100 || metres > 100000 {
		return p, 0, false
	}
	p = MatchPlayer{Person: binary.LittleEndian.Uint32(s[1:]), Side: side, Shirt: s[5], Played: s[135] > 0, Rating: math.Round(float64(rating)/10) / 10,
		Goals: int(s[20]), Assists: int(s[56]), Shots: int(s[41]), ShotsOnTarget: int(s[42]),
		Passes: int(s[86]), PassesCompleted: int(s[87]), KeyPasses: int(s[88]),
		Tackles: int(s[92]), TacklesWon: int(s[93]), Dribbles: int(s[58]), Fouls: int(s[60]), Interceptions: int(s[98]),
		ExpectedGoals: math.Round(xg*100) / 100, ExpectedAssists: math.Round(xa*100) / 100, DistanceKM: math.Round(metres/100) / 10}
	return p, j + 5, true
}
