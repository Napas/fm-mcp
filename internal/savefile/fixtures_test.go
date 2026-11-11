package savefile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// testFixtureLayouts repeats the parser's layouts so tests check them.
var testFixtureLayouts = map[byte]fixtureLayout{
	0x1c: {size: 63, season: 38, round: 50},
	0x1f: {size: 61, season: 34, round: 46, matchData: true},
}

// fixtureBase encodes a FIXTURE base of the given version.
func fixtureBase(version byte, f Fixture) []byte {
	l := testFixtureLayouts[version]
	b := make([]byte, l.size)
	b[0] = version
	binary.LittleEndian.PutUint32(b[1:], uint32(f.Key))   //nolint:gosec // signed field
	binary.LittleEndian.PutUint32(b[12:], uint32(f.Home)) //nolint:gosec // signed field
	binary.LittleEndian.PutUint32(b[19:], uint32(f.Away)) //nolint:gosec // signed field
	binary.LittleEndian.PutUint16(b[25:], f.Date.Raw)
	binary.LittleEndian.PutUint16(b[27:], f.Date.Year)
	binary.LittleEndian.PutUint16(b[l.season:], f.Season)
	b[l.round] = f.Round
	return b
}

// fixtureItem encodes a list item: i32 index, u8 flag, then a
// FIXTURE_RESULT with the result's match id and events when played, then the
// base. Both records carry version.
func fixtureItem(version byte, index int32, f Fixture) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(index)) //nolint:gosec // signed field
	if f.Result == nil {
		b = append(b, 0)
		return append(b, fixtureBase(version, f)...)
	}
	b = append(b, 1, version)
	b = binary.LittleEndian.AppendUint32(b, uint32(f.Result.MatchID)) //nolint:gosec // signed field
	b = append(b, f.Result.Home[:]...)
	b = append(b, f.Result.Away[:]...)
	b = append(b, make([]byte, 4+4+4)...)                                 // attendance, u32, 3 × u8, u8
	b = binary.LittleEndian.AppendUint32(b, uint32(len(f.Result.Events))) //nolint:gosec // small test data
	for _, e := range f.Result.Events {
		b = append(b, goalEventBytes(e)...)
	}
	return append(b, fixtureBase(version, f)...)
}

// goalEventBytes encodes a 16-byte FIXTURE_RESULT event.
func goalEventBytes(e GoalEvent) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(e.Scorer)) //nolint:gosec // signed field
	b = binary.LittleEndian.AppendUint32(b, uint32(e.Assister))  //nolint:gosec // signed field
	b = binary.LittleEndian.AppendUint32(b, e.Time)
	return append(b, e.Side, byte(e.Kind), e.Minute, e.AddedMinutes)
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
		Result: &FixtureResult{Home: [4]uint8{2, 0xff, 0xff, 0xff}, Away: [4]uint8{1, 0xff, 0xff, 0xff}, MatchID: NoMatchID}}
	upcomingFixture = Fixture{Key: 2887, Home: 532, Away: 547, Date: Date{Raw: 200, Year: 2025}, Round: 255}
	// playedFixture27 is a 27.0.0 result with a match id and two events: a
	// goal with an assist and a penalty in added time.
	playedFixture27 = Fixture{Key: 3543, Home: 456, Away: 16521, Date: Date{Raw: 196, Year: 2026}, Season: 2026, Round: 1,
		Result: &FixtureResult{Home: [4]uint8{1, 0xff, 0xff, 0xff}, Away: [4]uint8{1, 0xff, 0xff, 0xff}, Decoded: true, MatchID: 1022,
			Events: []GoalEvent{
				{Scorer: 45145, Assister: 23569, Time: 2076, Side: 0, Kind: EventGoal, Minute: 9},
				{Scorer: 9397, Assister: NoPerson, Time: 21800, Side: 1, Kind: EventPenalty, Minute: 90, AddedMinutes: 3},
			}}}
)

func TestParseFixtures(t *testing.T) {
	for _, v := range []byte{fixtureVersion26, fixtureVersion27} {
		t.Run(fmt.Sprintf("%#x", v), func(t *testing.T) {
			body := fixtureFile(
				fixtureList(2025, fixtureItem(v, 0, playedFixture), fixtureItem(v, 1, upcomingFixture)),
				fixtureList(2026, fixtureItem(v, 2, upcomingFixture)))

			got, err := parseFixtures(body)
			if err != nil {
				t.Fatalf("parseFixtures: %v", err)
			}
			played := playedFixture
			if v == fixtureVersion27 {
				r := *played.Result
				r.Decoded, r.Events = true, []GoalEvent{}
				played.Result = &r
			}
			if want := []Fixture{played, upcomingFixture, upcomingFixture}; !reflect.DeepEqual(got, want) {
				t.Errorf("fixtures = %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseFixturesGoalEvents(t *testing.T) {
	got, err := parseFixtures(fixtureFile(fixtureList(2026, fixtureItem(fixtureVersion27, 0, playedFixture27))))
	if err != nil {
		t.Fatalf("parseFixtures: %v", err)
	}
	if want := []Fixture{playedFixture27}; !reflect.DeepEqual(got, want) {
		t.Errorf("fixtures = %+v, want %+v", got, want)
	}
}

// TestParseFixturesSkips26Events checks that 26.x results keep no match id
// or events: their layout is not verified.
func TestParseFixturesSkips26Events(t *testing.T) {
	got, err := parseFixtures(fixtureFile(fixtureList(2026, fixtureItem(fixtureVersion26, 0, playedFixture27))))
	if err != nil {
		t.Fatalf("parseFixtures: %v", err)
	}
	r := got[0].Result
	if r == nil || r.Decoded || r.MatchID != NoMatchID || r.Events != nil || r.Home[0] != 1 {
		t.Errorf("26.x result = %+v, want the score only", r)
	}
}

func TestParseFixturesSkipsAbsentList(t *testing.T) {
	absent := []byte{0xe9, 0x07, 0}
	body := fixtureFile(absent, fixtureList(2025, fixtureItem(fixtureVersion26, 0, upcomingFixture)))

	got, err := parseFixtures(body)
	if err != nil || len(got) != 1 {
		t.Fatalf("parseFixtures = %d fixtures, %v; want 1", len(got), err)
	}
}

func TestParseFixturesRejectsWrongCount(t *testing.T) {
	list := fixtureList(2025, fixtureItem(fixtureVersion26, 0, upcomingFixture))
	binary.LittleEndian.PutUint32(list[len(list)-4:], 2)

	if _, err := parseFixtures(fixtureFile(list)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsBadFlag(t *testing.T) {
	item := fixtureItem(fixtureVersion26, 0, upcomingFixture)
	item[4] = 2

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsUnknownVersion(t *testing.T) {
	item := fixtureItem(fixtureVersion26, 0, upcomingFixture)
	item[5] = 0x1d

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseFixturesRejectsTruncatedBase(t *testing.T) {
	item := fixtureItem(fixtureVersion27, 0, upcomingFixture)
	body := fixtureFile(fixtureList(2025, item))
	start := 4 + 4 + 2 + 4 + 2 + 1 + 8 + 4 + 1 // header, list head, index, flag

	for _, n := range []int{start, start + 1} {
		if _, err := parseFixtures(body[:n]); !errors.Is(err, ErrTruncated) {
			t.Errorf("cut at %d: err = %v, want ErrTruncated", n, err)
		}
	}
}

func TestParseFixturesRejectsHugeEventCount(t *testing.T) {
	item := fixtureItem(fixtureVersion26, 0, playedFixture)
	binary.LittleEndian.PutUint32(item[5+1+4+8+12:], maxFixtureEvents+1)

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

// TestTestSave27GoalEvents checks Shamrock Rovers 3–2 Kauno Žalgiris
// (2026-07-15) against the game's match report, and the event counts of
// data/test_save_27_homegrown.fm (game date 2026-07-16).
func TestTestSave27GoalEvents(t *testing.T) {
	fs, err := ReadFixtures(openTestSaveAt(t, testSave27HomegrownPath))
	if err != nil {
		t.Fatal(err)
	}
	var withID, results int
	var shamrock *FixtureResult
	for _, f := range fs {
		if f.Result == nil || f.Result.Home[0] == 0xff {
			continue
		}
		results++
		if f.Result.MatchID != NoMatchID {
			withID++
		}
		if f.Home == 456 && f.Away == 16521 && f.Date.Time().Equal(time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)) {
			shamrock = f.Result
		}
	}
	if results != 3246 || withID != 1027 {
		t.Errorf("%d results, %d with a match id; want 3246 and 1027", results, withID)
	}
	if shamrock == nil {
		t.Fatal("no Shamrock Rovers v Kauno Žalgiris result on 2026-07-15")
	}
	want := []GoalEvent{
		{Scorer: 45145, Assister: 23569, Time: 2076, Side: 0, Kind: EventGoal, Minute: 9},   // O'Reilly-O'Sullivan, Sobowale
		{Scorer: 9397, Assister: 9149, Time: 4768, Side: 1, Kind: EventGoal, Minute: 20},    // Renan Oliveira, Benchaib
		{Scorer: 56834, Assister: 10498, Time: 6200, Side: 0, Kind: EventGoal, Minute: 26},  // Noonan, Byrne
		{Scorer: 9397, Assister: 8922, Time: 11853, Side: 1, Kind: EventGoal, Minute: 46},   // Renan Oliveira, Iyobosa Edokpolor
		{Scorer: 10498, Assister: 23583, Time: 19431, Side: 0, Kind: EventGoal, Minute: 78}, // Byrne, Grant
	}
	if !shamrock.Decoded || shamrock.MatchID != 1022 || !reflect.DeepEqual(shamrock.Events, want) {
		t.Errorf("result = %+v, want match id 1022 and events %+v", shamrock, want)
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
	item := fixtureItem(fixtureVersion26, 0, playedFixture)
	item[5] = 0x1d // the FIXTURE_RESULT version after the flag

	if _, err := parseFixtures(fixtureFile(fixtureList(2025, item))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}
