package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

type testContinent struct {
	index    uint32
	strength float32
	filler   []byte // stands for the record's undecoded lists
}

func (c testContinent) encode(b []byte) []byte {
	b = append(b, 0, 0xff, 0xff, 0xff)
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(c.strength))
	b = append(b, c.filler...)
	return dbRecord(b, c.index, c.index+1)
}

// continentBody is a CONTINENT table followed by the head of a CURRENCY_RECORD
// table of currencies records.
func continentBody(currencies uint32, cs ...testContinent) []byte {
	var recs []byte
	for _, c := range cs {
		recs = c.encode(recs)
	}
	b := append([]byte{7, 7, 7}, tableBytes(len(cs), recs)...)
	b = binary.LittleEndian.AppendUint32(b, currencies)
	return append(b, 0, tagCurrency, 0xee)
}

func TestReadContinents(t *testing.T) {
	body := continentBody(173,
		testContinent{index: 0, strength: 0.85, filler: []byte{1, 2, 3}},
		testContinent{index: 1, strength: 0.99},
		testContinent{index: 2, strength: 1.0, filler: bytes.Repeat([]byte{9}, 40)},
		testContinent{index: 3, strength: 0},
	)

	got, err := readContinents(body, 173)
	if err != nil {
		t.Fatalf("readContinents: %v", err)
	}

	if want := []Continent{{0, 1, 0.85}, {1, 2, 0.99}, {2, 3, 1}, {3, 4, 0}}; !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadContinentsNeedsTheCurrencyTableAfterIt(t *testing.T) {
	body := continentBody(173, testContinent{index: 0, strength: 0.85})

	if _, err := readContinents(body, 172); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadContinentsRejectsUnusableStrengths(t *testing.T) {
	for _, s := range []float32{-0.1, 1.5, float32(math.NaN()), float32(math.Inf(1))} {
		body := continentBody(1, testContinent{index: 0, strength: 0.85}, testContinent{index: 1, strength: s})

		if _, err := readContinents(body, 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("strength %v: err = %v, want ErrNotFound", s, err)
		}
	}
}

func TestReadContinentsRejectsTooManyContinents(t *testing.T) {
	cs := make([]testContinent, maxContinents+1)
	for i := range cs {
		cs[i] = testContinent{index: uint32(i), strength: 0.85} //nolint:gosec // small test data
	}

	if _, err := readContinents(continentBody(1, cs...), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

type testNation struct {
	index, uid uint32
	continent  int32
	lead       []byte // stands for the record's undecoded lists
}

// nationTail encodes the fixed-grammar end of a NATION record (writer
// 0x35bf574) before its database record.
func nationTail(b []byte, continent int32) []byte {
	b = append(b, 3)
	b = le32(b, continent)
	b = le32(b, 0)
	b = le32(b, 1040) // capital city
	b = le32(b, -1)
	b = append(b, 1, 0)
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(0.5))
	b = append(b, 0)
	b = append(b, 1) // one 5-byte item
	b = append(b, 1, 2, 3, 4, 5)
	b = binary.LittleEndian.AppendUint32(b, 1) // one 28-byte item
	b = append(b, bytes.Repeat([]byte{0x11}, 28)...)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = binary.LittleEndian.AppendUint32(b, 2) // two 12-byte items
	b = append(b, bytes.Repeat([]byte{0x22}, 24)...)
	return append(b, bytes.Repeat([]byte{0x33}, nationTailEndSize)...)
}

func (n testNation) encode(b []byte) []byte {
	b = append(b, tagNation)
	b = append(b, n.lead...)
	b = nationTail(b, n.continent)
	return dbRecord(b, n.index, n.uid)
}

func nationBody(ns ...testNation) []byte {
	var recs []byte
	for _, n := range ns {
		recs = n.encode(recs)
	}
	return append([]byte{5, 5}, tableBytes(len(ns), recs)...)
}

var testNations = []testNation{
	{index: 0, uid: 5, continent: 0, lead: []byte{0xff, 0xff, 0xff, 0x40}},
	{index: 1, uid: 765, continent: 2, lead: bytes.Repeat([]byte{0x80}, 30)},
	{index: 2, uid: 1000, continent: -1},
}

func TestReadNations(t *testing.T) {
	got, err := readNations(nationBody(testNations...), 3, 7)
	if err != nil {
		t.Fatalf("readNations: %v", err)
	}

	if want := []Nation{{Index: 0, UID: 5, Continent: 0}, {Index: 1, UID: 765, Continent: 2}, {Index: 2, UID: 1000, Continent: -1}}; !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadNationsRejectsAContinentOutsideTheTable(t *testing.T) {
	if _, err := readNations(nationBody(testNations...), 3, 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadNationsRejectsARecordWithoutTail(t *testing.T) {
	ns := []byte{}
	ns = testNations[0].encode(ns)
	ns = dbRecord(append(ns, tagNation, 1, 2, 3), 1, 765)
	body := tableBytes(2, ns)

	if _, err := readNations(body, 2, 7); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadNationsRejectsUnorderedUIDs(t *testing.T) {
	ns := []testNation{testNations[1], testNations[0]}
	ns[0].index, ns[1].index = 0, 1

	if _, err := readNations(nationBody(ns...), 2, 7); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// containerRecord encodes a NATIONAL_TEAM_CONTAINER record with factor at
// containerFactorOffset.
func containerRecord(b []byte, index, uid uint32, factor int16) []byte {
	rec := make([]byte, containerFactorOffset+2+7)
	rec[0] = tagContainer
	binary.LittleEndian.PutUint16(rec[containerFactorOffset:], uint16(factor)) //nolint:gosec // two's complement intended
	return dbRecord(append(b, rec...), index, uid)
}

func TestReadNationFactors(t *testing.T) {
	var recs []byte
	recs = containerRecord(recs, 0, 5, 105)
	recs = containerRecord(recs, 1, 765, 187)
	recs = containerRecord(recs, 2, 1000, 0)
	recs = containerRecord(recs, 3, 99, 50) // a women's container
	body := append([]byte{1}, tableBytes(4, recs)...)
	ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 765}, {Index: 2, UID: 1000}}

	if err := readNationFactors(body, 4, ns); err != nil {
		t.Fatalf("readNationFactors: %v", err)
	}

	for i, want := range []int16{105, 187, 0} {
		if ns[i].MensFactor != want {
			t.Errorf("nation %d factor = %d, want %d", i, ns[i].MensFactor, want)
		}
	}
}

func TestReadNationFactorsRejectsShortRecords(t *testing.T) {
	ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 765}}
	var recs []byte
	recs = containerRecord(recs, 0, 5, 105)
	recs = dbRecord(append(recs, tagContainer, 1, 2), 1, 765) // too short for the factor
	recs = containerRecord(recs, 2, 99, 50)

	if err := readNationFactors(tableBytes(3, recs), 3, ns); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadNationFactorsNeedsAContainerPerNation(t *testing.T) {
	ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 765}}
	body := tableBytes(1, containerRecord(nil, 0, 5, 105))

	if err := readNationFactors(body, 1, ns); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadNationsSplitsNationZeroAtItsOwnDatabaseRecord(t *testing.T) {
	ns := []testNation{testNations[0], testNations[1]}
	// The tail ends with a zero byte, so nation 0's database record also
	// matches one byte early.
	body := nationBody(ns...)
	first := bytes.Index(body, binary.LittleEndian.AppendUint32(append(binary.LittleEndian.AppendUint32(nil, 0), 5, 0, 0, 0), 5))
	body[first-1] = 0

	got, err := readNations(body, 2, 7)
	if err != nil {
		t.Fatalf("readNations: %v", err)
	}
	if got[0].UID != 5 || got[1].UID != 765 {
		t.Errorf("got %+v, want uids 5 and 765", got)
	}
}

// repeatedHeads repeats unit, each copy after a table end marker, up to size
// bytes: every copy is a place a nation-data reader tries.
func repeatedHeads(size int, unit []byte) []byte {
	var b []byte
	for len(b) < size {
		b = append(b, tableEnd...)
		b = binary.LittleEndian.AppendUint32(b, 0)
		b = append(b, unit...)
	}
	return b
}

// Crafted bodies full of table heads must not make the readers quadratic.
func TestNationReadersGiveUpOnCraftedBodies(t *testing.T) {
	// Larger than the spans one try may scan.
	const size = 4 << 20
	// A continent head, then a database record with index 3 and a currency
	// head after an end marker: every currency head ends a candidate
	// CONTINENT table, and every continent head before it starts one.
	continentUnit := append(tableHead(4, tagContinent), dbRecord(nil, 3, 4)...)
	continentUnit = append(continentUnit, tableEnd...)
	continentUnit = binary.LittleEndian.AppendUint32(continentUnit, 4)
	continentUnit = append(continentUnit, tableHead(173, tagCurrency)...)
	// Continent heads with no end marker before them, which no try counts,
	// then currency heads.
	denseContinents := bytes.Repeat(tableHead(4, tagContinent), size/6)
	for range maxNationCandidates + 1 {
		denseContinents = append(denseContinents, continentUnit[6:]...)
	}
	tests := []struct {
		name string
		read func() error
	}{
		{"continents", func() error {
			_, err := readContinents(repeatedHeads(size, continentUnit), 173)
			return err
		}},
		{"continents behind dense heads", func() error {
			_, err := readContinents(denseContinents, 173)
			return err
		}},
		{"nations", func() error {
			_, err := readNations(repeatedHeads(size, tableHead(251, tagNation)), 251, 7)
			return err
		}},
		{"container factors", func() error {
			ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 765}}
			return readNationFactors(repeatedHeads(size, tableHead(502, tagContainer)), 502, ns)
		}},
	}
	for _, tt := range tests {
		start := time.Now()
		err := tt.read()
		if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "candidates") {
			t.Errorf("%s: err = %v, want ErrCorrupt naming the candidates", tt.name, err)
		}
		if d := time.Since(start); !raceEnabled && d > time.Second {
			t.Errorf("%s: took %v", tt.name, d)
		}
	}
}

// headsAfterEnds is n copies of unit, each after a table end marker.
func headsAfterEnds(n int, unit []byte) []byte {
	var b []byte
	for range n {
		b = append(b, tableEnd...)
		b = binary.LittleEndian.AppendUint32(b, 0)
		b = append(b, unit...)
	}
	return b
}

// The readers give up only when a candidate beyond the cap exists; failing
// on exactly maxNationCandidates places is "not found".
func TestNationReadersCapCountsCandidatesBeyondTheLimit(t *testing.T) {
	// A currency head whose continent table would end with index 100, which
	// is out of range: each copy is one failed try.
	continentUnit := tableHead(173, tagCurrency)
	continentUnit = binary.LittleEndian.AppendUint32(continentUnit, 100)
	continentUnit = append(continentUnit, make([]byte, 8)...)
	ns := []Nation{{Index: 0, UID: 5}, {Index: 1, UID: 765}}
	tests := []struct {
		name string
		read func(n int) error
	}{
		{"continents", func(n int) error {
			_, err := readContinents(headsAfterEnds(n, continentUnit), 173)
			return err
		}},
		{"nations", func(n int) error {
			_, err := readNations(headsAfterEnds(n, tableHead(251, tagNation)), 251, 7)
			return err
		}},
		{"container factors", func(n int) error {
			return readNationFactors(headsAfterEnds(n, tableHead(502, tagContainer)), 502, ns)
		}},
	}
	for _, tt := range tests {
		if err := tt.read(maxNationCandidates); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s, %d candidates: err = %v, want ErrNotFound", tt.name, maxNationCandidates, err)
		}
		if err := tt.read(maxNationCandidates + 1); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s, %d candidates: err = %v, want ErrCorrupt", tt.name, maxNationCandidates+1, err)
		}
	}
}

// A NATION count far above the game's 251 is rejected before any scan.
func TestReadNationsRejectsCountsAboveTheLimit(t *testing.T) {
	body := headsAfterEnds(1, tableHead(maxNations+1, tagNation))
	if _, err := readNations(body, maxNations+1, 7); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

// The tail also parses five bytes later, with continent 7 from the bytes
// of the second and capital fields; the table has 7 continents, so only
// the real tail fits.
func TestReadNationsSkipsTailsWithAContinentOutsideTheTable(t *testing.T) {
	var rec []byte
	rec = append(rec, tagNation, 0xff, 0xff, 0xff, 0x40)
	tail := len(rec)
	rec = nationTail(rec, 2)
	binary.LittleEndian.PutUint32(rec[tail+5:], 0x0700) // second i32: bytes 00 07 00 00
	binary.LittleEndian.PutUint32(rec[tail+9:], 0x0500) // capital: low byte 0
	rec[tail+nationTailFixed+5] = 0                     // last byte of the 5-byte item
	body := append([]byte{5, 5}, tableBytes(1, dbRecord(rec, 0, 5))...)

	got, err := readNations(body, 1, 7)
	if err != nil {
		t.Fatalf("readNations: %v", err)
	}
	if got[0].Continent != 2 {
		t.Errorf("continent %d, want 2", got[0].Continent)
	}
}
