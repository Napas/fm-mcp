package savefile

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
	"time"
)

// fixtureBase encodes a 63-byte FIXTURE base.
func fixtureBase(f Fixture) []byte {
	b := make([]byte, fixtureBaseSize)
	b[0] = fixtureVersion
	binary.LittleEndian.PutUint32(b[1:], uint32(f.Key))   //nolint:gosec // signed field
	binary.LittleEndian.PutUint32(b[12:], uint32(f.Home)) //nolint:gosec // signed field
	binary.LittleEndian.PutUint32(b[19:], uint32(f.Away)) //nolint:gosec // signed field
	binary.LittleEndian.PutUint16(b[25:], f.Date.Raw)
	binary.LittleEndian.PutUint16(b[27:], f.Date.Year)
	binary.LittleEndian.PutUint16(b[38:], f.Season)
	b[50] = f.Round
	return b
}

// fixtureItem encodes a list item: i32 index, u8 flag, then a
// FIXTURE_RESULT with events when played, then the base.
func fixtureItem(index int32, f Fixture, events int) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(index)) //nolint:gosec // signed field
	if f.Result == nil {
		b = append(b, 0)
		return append(b, fixtureBase(f)...)
	}
	b = append(b, 1, fixtureVersion, 0, 0, 0, 0)
	b = append(b, f.Result.Home[:]...)
	b = append(b, f.Result.Away[:]...)
	b = append(b, make([]byte, 4+4+4)...)                   // attendance, u32, 3 × u8, u8
	b = binary.LittleEndian.AppendUint32(b, uint32(events)) //nolint:gosec // small test data
	b = append(b, make([]byte, 16*events)...)
	return append(b, fixtureBase(f)...)
}

// fixtureList encodes a present list with its items and item count.
func fixtureList(season uint16, items ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint16(nil, season)
	b = append(b, 1, 0, 0, 0, 0, 0, 0, 0, 0)
	for _, it := range items {
		b = append(b, it...)
	}
	b = append(b, 0xff, 0xff, 0xff, 0xff)
	return binary.LittleEndian.AppendUint32(b, uint32(len(items))) //nolint:gosec // small test data
}

// fixtureFile encodes fix_man.dat: header, the first list, u32 n, n lists,
// then trailing bytes.
func fixtureFile(first []byte, more ...[]byte) []byte {
	b := make([]byte, 4+4+2+4)
	b = append(b, first...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(more))) //nolint:gosec // small test data
	for _, l := range more {
		b = append(b, l...)
	}
	return append(b, 0xde, 0xad)
}

var (
	playedFixture = Fixture{Key: 232, Home: 547, Away: 442, Date: Date{Raw: 228, Year: 2025}, Season: 2025, Round: 0,
		Result: &FixtureResult{Home: [4]uint8{2, 0xff, 0xff, 0xff}, Away: [4]uint8{1, 0xff, 0xff, 0xff}}}
	upcomingFixture = Fixture{Key: 2887, Home: 532, Away: 547, Date: Date{Raw: 200, Year: 2025}, Round: 255}
)

func TestParseFixtures(t *testing.T) {
	body := fixtureFile(fixtureList(2025, fixtureItem(0, playedFixture, 1)), fixtureList(2026, fixtureItem(1, upcomingFixture, 0)))

	got, err := parseFixtures(body)
	if err != nil {
		t.Fatalf("parseFixtures: %v", err)
	}
	if want := []Fixture{playedFixture, upcomingFixture}; !reflect.DeepEqual(got, want) {
		t.Errorf("fixtures = %+v, want %+v", got, want)
	}
}

func TestParseFixturesSkipsAbsentList(t *testing.T) {
	absent := []byte{0xe9, 0x07, 0}
	body := fixtureFile(absent, fixtureList(2025, fixtureItem(0, upcomingFixture, 0)))

	got, err := parseFixtures(body)
	if err != nil || len(got) != 1 {
		t.Fatalf("parseFixtures = %d fixtures, %v; want 1", len(got), err)
	}
}

func TestParseFixturesRejectsWrongCount(t *testing.T) {
	list := fixtureList(2025, fixtureItem(0, upcomingFixture, 0))
	binary.LittleEndian.PutUint32(list[len(list)-4:], 2)

	if _, err := parseFixtures(fixtureFile(list)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsBadFlag(t *testing.T) {
	item := fixtureItem(0, upcomingFixture, 0)
	item[4] = 2

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsUnknownVersion(t *testing.T) {
	item := fixtureItem(0, upcomingFixture, 0)
	item[5] = 0x1d

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsHugeEventCount(t *testing.T) {
	item := fixtureItem(0, playedFixture, 0)
	binary.LittleEndian.PutUint32(item[5+1+4+8+12:], maxFixtureEvents+1)

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestTestSaveFixtures(t *testing.T) {
	fs, err := ReadFixtures(openTestSave(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 19239 {
		t.Errorf("%d fixtures, want 19239", len(fs))
	}
	// Tottenham v Burnley on 2025-08-16 opens the 2025/26 Premier League
	// (key 232) for team 547.
	for _, f := range fs {
		if f.Key == 232 && f.Season == 2025 && f.Round == 0 && (f.Home == 547 || f.Away == 547) {
			if f.Home != 547 || f.Away != 442 || !f.Date.Time().Equal(time.Date(2025, 8, 16, 0, 0, 0, 0, time.UTC)) || f.Result != nil {
				t.Errorf("Tottenham's first league fixture = %+v", f)
			}
			return
		}
	}
	t.Error("no first-round league fixture for Tottenham")
}

// checkRealSaveFixtures checks the KŽ save of 2035-12-02: fixture count,
// key range and a second leg won on penalties (bytes).
func checkRealSaveFixtures(t *testing.T, a *Archive, keys int) {
	t.Helper()
	fs, err := ReadFixtures(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 138714 {
		t.Errorf("%d fixtures, want 138714", len(fs))
	}
	found := false
	for _, f := range fs {
		if f.Key < 0 || int(f.Key) >= keys {
			t.Fatalf("fixture key %d out of range", f.Key)
		}
		if f.Key == 5619 && f.Home == 520 && f.Season == 2033 {
			found = true
			if f.Result == nil || f.Result.Home != [4]uint8{0, 0, 4, 1} || f.Result.Away != [4]uint8{0, 0, 1, 1} {
				t.Errorf("2033 playoff second leg = %+v", f.Result)
			}
		}
	}
	if !found {
		t.Error("no 2033 playoff second leg at home to team 520")
	}
}

func TestParseFixturesRejectsUnknownResultVersion(t *testing.T) {
	item := fixtureItem(0, playedFixture, 0)
	item[5] = 0x1d // the FIXTURE_RESULT version after the flag

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}
