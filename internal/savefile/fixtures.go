package savefile

import (
	"encoding/binary"
	"fmt"
)

const (
	fixtureManFile = "rgman/fix_man.dat"
	// Version bytes of FIXTURE and FIXTURE_RESULT; each accepts either.
	fixtureVersion26 = 0x1c // 26.2.0 and 26.3.2
	fixtureVersion27 = 0x1f // 27.0.0
	// Base offsets in every version: u8 version, u32 key, u32 stadium, u16,
	// u8, u32 home, 3 × u8, u32 away, 2 × u8, date, date.
	fixtureKeyOff  = 1
	fixtureHomeOff = 12
	fixtureAwayOff = 19
	fixtureDateOff = 25
	// fixtureEventSize is a FIXTURE_RESULT event (not decoded).
	fixtureEventSize = 16

	maxFixtureLists    = 64      // 4 test, 6 KŽ
	maxFixturesPerList = 1 << 20 // 12 575 and about 50 000 seen
	maxFixtureEvents   = 1024    // 28 seen
)

// fixtureLayout is the size of a FIXTURE base and its season and round offsets.
type fixtureLayout struct {
	size, season, round int
}

// fixtureLayouts by version. Its keys are also the accepted FIXTURE_RESULT versions.
var fixtureLayouts = map[byte]fixtureLayout{
	fixtureVersion26: {size: 63, season: 38, round: 50},
	fixtureVersion27: {size: 61, season: 34, round: 46},
}

// Fixture is a FIXTURE record (vtable 0x54e6c20, load 0x3e5c748).
type Fixture struct {
	Key    int32 // ACTUAL_COMP_ID index (base +0x20)
	Home   int32 // TEAM index
	Away   int32 // TEAM index
	Date   Date  // match date (the base holds a second, undecoded date)
	Season uint16
	Round  uint8          // base +0x3b; 255 for friendlies
	Result *FixtureResult // nil until played
}

// FixtureResult holds a played fixture's score bytes per side: [0] after 90
// minutes, [1] after extra time, [2] penalties, [3] the aggregate after a
// second leg; 0xff = none.
type FixtureResult struct {
	Home, Away [4]uint8
}

// ReadFixtures decodes the fixtures of the save's rgman/fix_man.dat.
func ReadFixtures(a *Archive) ([]Fixture, error) {
	f, err := a.ReadFile(fixtureManFile)
	if err != nil {
		return nil, err
	}
	fs, err := parseFixtures(f.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fixtureManFile, err)
	}
	return fs, nil
}

// parseFixtures reads the header (u32, u32, u16, u32), the first list, then
// u32 n more lists. Bytes after the last list are not decoded.
func parseFixtures(body []byte) ([]Fixture, error) {
	r := newReader(body)
	r.take(4 + 4 + 2 + 4)
	var out []Fixture
	out = readFixtureList(r, out)
	n := r.u32()
	if r.err == nil && n > maxFixtureLists {
		r.fail("%d fixture lists", n)
	}
	for i := uint32(0); i < n && r.err == nil; i++ {
		out = readFixtureList(r, out)
	}
	if r.err != nil {
		return nil, r.err
	}
	return out, nil
}

// readFixtureList reads u16 season, u8 present; when present, u32, u32,
// items up to an index of -1, and a u32 equal to the item count. An item is
// i32 index, u8 flag (1 = a FIXTURE_RESULT precedes the base), base.
func readFixtureList(r *reader, out []Fixture) []Fixture {
	r.u16()
	if r.u8() != 1 {
		return out
	}
	r.take(8)
	count := uint32(0)
	for r.err == nil {
		if r.i32() == -1 {
			break
		}
		if count++; count > maxFixturesPerList {
			r.fail("over %d fixtures in a list", maxFixturesPerList)
			break
		}
		var res *FixtureResult
		switch flag := r.u8(); flag {
		case 0:
		case 1:
			res = readFixtureResult(r)
		default:
			r.fail("fixture flag %d", flag)
		}
		if f, ok := readFixtureBase(r); ok {
			f.Result = res
			out = append(out, f)
		}
	}
	if got := r.u32(); r.err == nil && got != count {
		r.fail("fixture list count %d after %d fixtures", got, count)
	}
	return out
}

// readFixtureResult reads u8 version, u32, 4 bytes home, 4 bytes away,
// u32 attendance (guess), u32, 4 × u8, u32 n, n × 16-byte events.
func readFixtureResult(r *reader) *FixtureResult {
	readFixtureVersion(r, "fixture result")
	r.u32()
	var res FixtureResult
	copy(res.Home[:], r.take(4))
	copy(res.Away[:], r.take(4))
	r.take(4 + 4 + 4)
	n := r.u32()
	if r.err == nil && n > maxFixtureEvents {
		r.fail("%d fixture events", n)
		return nil
	}
	r.take(fixtureEventSize * int(n))
	return &res
}

// readFixtureVersion reads the version byte of what (a FIXTURE or
// FIXTURE_RESULT) and returns its base layout.
func readFixtureVersion(r *reader, what string) (fixtureLayout, bool) {
	v := r.u8()
	l, ok := fixtureLayouts[v]
	if !ok {
		r.fail("%s version %#x", what, v)
	}
	return l, ok
}

// readFixtureBase reads a FIXTURE base; its first byte is the version.
func readFixtureBase(r *reader) (Fixture, bool) {
	start := r.off
	l, ok := readFixtureVersion(r, "fixture")
	if !ok {
		return Fixture{}, false
	}
	r.off = start // offsets include the version byte
	b := r.take(l.size)
	if b == nil {
		return Fixture{}, false
	}
	return Fixture{
		Key:    int32(binary.LittleEndian.Uint32(b[fixtureKeyOff:])),  //nolint:gosec // signed field
		Home:   int32(binary.LittleEndian.Uint32(b[fixtureHomeOff:])), //nolint:gosec // signed field
		Away:   int32(binary.LittleEndian.Uint32(b[fixtureAwayOff:])), //nolint:gosec // signed field
		Date:   dateAt(b, fixtureDateOff),
		Season: binary.LittleEndian.Uint16(b[l.season:]),
		Round:  b[l.round],
	}, true
}
