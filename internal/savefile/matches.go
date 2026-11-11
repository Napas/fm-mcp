package savefile

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	matchPackFile = "match_stats_cfg.dat"
	// matchPackVersion is the inner file version of every 27.0.0 save checked.
	matchPackVersion = 6

	maxMatchPacks      = 16      // 4 seen
	maxMatchEntries    = 1 << 16 // 544 seen in one pack
	maxMatchObjectSize = 4 << 20 // 244 153 bytes seen
	// maxMatchPackDecoded caps all objects' decompressed bytes together:
	// 30.9 MB seen.
	maxMatchPackDecoded = 256 << 20

	// matchPlayerSize is a player record's fixed part; records are this long
	// or longer, and their length is not decoded.
	matchPlayerSize = 222
	matchPlayerTag  = 2
	// matchPositionBits are the position bits seen in records (bits 0–14)
	// and the two side flags (0x100000, 0x200000).
	matchPositionBits = 0x00307fff
	minMatchRating    = 300
	maxMatchRating    = 1000
	maxMatchXG        = 10
	startersPerSide   = 11
	// maxMatchPerson bounds person indices: real ones are below 200 000;
	// float bytes read as a person are about 1e9.
	maxMatchPerson = 1 << 24
)

// Team stats sit at fixed offsets before each side's first player record
// (docs/format/matches.md "Team stats").
const (
	teamBlockSize       = 296
	teamShotsBlocked    = -296 // u8
	teamClearCutChances = -295 // u8
	teamCorners         = -283 // u8
	teamFouls           = -280 // u8
	teamYellowCards     = -278 // u8
	teamPasses          = -264 // u16, attempted
	teamPassesCompleted = -262 // u16
	teamTackles         = -258 // u16, attempted
	teamTacklesWon      = -256 // u16
	teamHeaders         = -254 // u16, attempted
	teamHeadersWon      = -252 // u16
	teamThirdsCompleted = -166 // u16 own third; middle +4, final +8

	// Plausibility caps: 07-16's 1 642 sides stay within 961 passes, 66
	// headers, 20 tackles and 20 of any count.
	maxTeamPasses = 2000
	maxTeamDuels  = 200
	maxTeamCount  = 40
)

// TeamStats is a side's totals in a match.
type TeamStats struct {
	Passes, PassesCompleted uint16
	// PassesCompletedByThird is own, middle and final third; nil unless it
	// adds up to PassesCompleted (objects without a setup section hold 0s).
	PassesCompletedByThird *[3]uint16
	Tackles, TacklesWon    uint16
	Headers, HeadersWon    uint16
	ShotsBlocked           uint8 // the side's own shots that were blocked
	ClearCutChances        uint8
	Corners                uint8
	Fouls                  uint8
	YellowCards            uint8
}

// MatchObject is a match stored in match_stats_cfg.dat: its match id (the
// FIXTURE_RESULT's) and its player records, home side first. Players is nil
// when the object does not decompress or its records do not read as two
// sides of 11 starters.
type MatchObject struct {
	MatchID int32
	Players []MatchPlayer
	// TeamStats are the home and away totals; nil when the players or the
	// totals do not read.
	TeamStats *[2]TeamStats
}

// MatchPlayer is a player record's fixed fields. Every squad member has one;
// both position fields are 0 for those who did not play.
type MatchPlayer struct {
	Person int32
	Shirt  uint8
	Side   uint8 // 0 home, 1 away
	// StartPositions are position bits at kick-off; 0 for a player who did
	// not start (inference: the bit meanings).
	StartPositions uint32
	// EndPositions are position bits at the end; non-zero for a substitute
	// who came on. 0 for a starter taken off only in some of the human's
	// matches, so it does not show who was taken off elsewhere.
	EndPositions uint32
	XG           float32 // expected goals
	Rating       uint16  // match rating × 100; a placeholder for players who did not play
}

// Started reports whether p was on the pitch at kick-off.
func (p MatchPlayer) Started() bool { return p.StartPositions != 0 }

// Played reports whether p was on the pitch.
func (p MatchPlayer) Played() bool { return p.Started() || p.EndPositions != 0 }

// ReadMatchObjects decodes match_stats_cfg.dat. A file version other than 6
// gives ErrUnsupported.
func ReadMatchObjects(a *Archive) ([]MatchObject, error) {
	f, err := a.ReadFile(matchPackFile)
	if err != nil {
		return nil, err
	}
	if f.Header.Version != matchPackVersion {
		return nil, fmt.Errorf("%w: %s version %d", ErrUnsupported, matchPackFile, f.Header.Version)
	}
	objs, err := parseMatchPack(f.Body, maxMatchPackDecoded)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", matchPackFile, err)
	}
	return objs, nil
}

// parseMatchPack reads u32 (inference: the next match id), u8, u8, u32 n,
// then n packs: u32 index, string name, u32, u8, u8, u32 count, count ×
// (u32 match id, u32 offset, u32 size), u32, u32, u8 3, u32 data length, the
// data (one zstd frame per entry, at its offset) and u8 1. Objects
// decompress to at most budget bytes together.
func parseMatchPack(body []byte, budget int) ([]MatchObject, error) {
	dec, err := newDecoder(maxMatchObjectSize)
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	r := newReader(body)
	r.take(4 + 1 + 1)
	n := r.u32()
	if r.err == nil && n > maxMatchPacks {
		r.fail("%d match packs", n)
	}
	out := []MatchObject{} // read but empty is not unread
	left := budget
	for range n {
		if r.err != nil {
			break
		}
		r.u32()
		r.str()
		r.take(4 + 1 + 1)
		count := r.u32()
		if r.err == nil && count > maxMatchEntries {
			r.fail("%d match pack entries", count)
			break
		}
		table := r.take(12 * int(count))
		r.take(4 + 4)
		r.expectU8(3, "match pack byte")
		data := r.take(int(r.u32()))
		r.expectU8(1, "match pack end")
		if r.err != nil {
			break
		}
		for i := range int(count) {
			e := table[12*i:]
			id := int32(binary.LittleEndian.Uint32(e)) //nolint:gosec // signed field
			off, size := uint64(binary.LittleEndian.Uint32(e[4:])), uint64(binary.LittleEndian.Uint32(e[8:]))
			if off+size > uint64(len(data)) {
				r.fail("match %d at %d+%d is past the pack's %d bytes", id, off, size, len(data))
				break
			}
			obj, err := dec.DecodeAll(data[off:off+size], nil)
			if left -= len(obj); left < 0 {
				r.fail("match objects decompress to more than %d bytes", budget)
				break
			}
			o := MatchObject{MatchID: id}
			if err == nil {
				o.Players, o.TeamStats, _ = readMatchObject(obj)
			}
			out = append(out, o)
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return out, nil
}

// scanMatchPlayers finds the player records in a decompressed object: tag
// bytes 2 at offsets 0, 7 and 12, side 0 or 1 at 6, a person from 0 to
// maxMatchPerson, position bits within matchPositionBits at 8 and 13, a
// finite xG at 25 and 29 and a rating of 300–1000 at 136. After a record it
// resumes past the record's fixed part: bytes inside longer records can look
// like a record. Each side must have 11 starters and no person may repeat.
func scanMatchPlayers(obj []byte) ([]MatchPlayer, [2]recordSpan, error) {
	var out []MatchPlayer
	seen := make(map[int32]bool)
	var starters [2]int
	spans := [2]recordSpan{{first: -1}, {first: -1}}
	for i := 0; i+matchPlayerSize <= len(obj); {
		p, ok := matchRecord(obj[i:])
		if !ok {
			i++
			continue
		}
		if seen[p.Person] {
			return nil, spans, fmt.Errorf("%w: person %d appears twice in a match", ErrCorrupt, p.Person)
		}
		seen[p.Person] = true
		if spans[p.Side].first < 0 {
			spans[p.Side].first = i
		}
		spans[p.Side].end = i + matchPlayerSize
		if p.Started() {
			starters[p.Side]++
		}
		out = append(out, p)
		i += matchPlayerSize
	}
	if starters != [2]int{startersPerSide, startersPerSide} {
		return nil, spans, fmt.Errorf("%w: %d and %d starters in a match", ErrCorrupt, starters[0], starters[1])
	}
	return out, spans, nil
}

// recordSpan is where a side's player records start and where the fixed
// part of its last one ends.
type recordSpan struct{ first, end int }

// readMatchObject reads an object's player records and, when they read,
// its team stats. The away block must start after the home records.
func readMatchObject(obj []byte) ([]MatchPlayer, *[2]TeamStats, error) {
	players, spans, err := scanMatchPlayers(obj)
	if err != nil {
		return nil, nil, err
	}
	if spans[1].first-teamBlockSize < spans[0].end {
		return players, nil, nil
	}
	var teams [2]TeamStats
	for side, s := range spans {
		var ok bool
		if teams[side], ok = readTeamStats(obj, s.first); !ok {
			return players, nil, nil
		}
	}
	return players, &teams, nil
}

// readTeamStats reads the team stats before the player record at first;
// false when they do not fit, more were completed than attempted or a
// value is above its plausibility cap.
func readTeamStats(obj []byte, first int) (TeamStats, bool) {
	if first < teamBlockSize {
		return TeamStats{}, false
	}
	u8 := func(rel int) uint8 { return obj[first+rel] }
	u16 := func(rel int) uint16 { return binary.LittleEndian.Uint16(obj[first+rel:]) }
	s := TeamStats{
		Passes: u16(teamPasses), PassesCompleted: u16(teamPassesCompleted),
		Tackles: u16(teamTackles), TacklesWon: u16(teamTacklesWon),
		Headers: u16(teamHeaders), HeadersWon: u16(teamHeadersWon),
		ShotsBlocked: u8(teamShotsBlocked), ClearCutChances: u8(teamClearCutChances), Corners: u8(teamCorners),
		Fouls: u8(teamFouls), YellowCards: u8(teamYellowCards),
	}
	if th := [3]uint16{u16(teamThirdsCompleted), u16(teamThirdsCompleted + 4), u16(teamThirdsCompleted + 8)}; s.PassesCompleted > 0 && int(th[0])+int(th[1])+int(th[2]) == int(s.PassesCompleted) {
		s.PassesCompletedByThird = &th
	}
	ok := s.PassesCompleted <= s.Passes && s.TacklesWon <= s.Tackles && s.HeadersWon <= s.Headers &&
		s.Passes <= maxTeamPasses && s.Tackles <= maxTeamDuels && s.Headers <= maxTeamDuels &&
		max(s.ShotsBlocked, s.ClearCutChances, s.Corners, s.Fouls, s.YellowCards) <= maxTeamCount
	return s, ok
}

// matchRecord reads a player record at the start of b; false when b does not
// hold one.
func matchRecord(b []byte) (MatchPlayer, bool) {
	if b[0] != matchPlayerTag || b[7] != matchPlayerTag || b[12] != matchPlayerTag || b[6] > 1 {
		return MatchPlayer{}, false
	}
	p := MatchPlayer{
		Person: int32(binary.LittleEndian.Uint32(b[1:])), //nolint:gosec // signed field
		Shirt:  b[5], Side: b[6],
		StartPositions: binary.LittleEndian.Uint32(b[8:]),
		EndPositions:   binary.LittleEndian.Uint32(b[13:]),
		XG:             math.Float32frombits(binary.LittleEndian.Uint32(b[25:])),
		Rating:         binary.LittleEndian.Uint16(b[136:]),
	}
	faced := math.Float32frombits(binary.LittleEndian.Uint32(b[29:]))
	ok := p.Person >= 0 && p.Person < maxMatchPerson && p.StartPositions&^matchPositionBits == 0 && p.EndPositions&^matchPositionBits == 0 &&
		p.Rating >= minMatchRating && p.Rating <= maxMatchRating && finiteIn(p.XG, maxMatchXG) && finiteIn(faced, 2*maxMatchXG)
	return p, ok
}

// finiteIn reports whether v is a number in [0, maxV].
func finiteIn(v, maxV float32) bool {
	return v >= 0 && v <= maxV // false for NaN
}
