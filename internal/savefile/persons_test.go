package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func encodePersonTrailer(flags uint64, index, uid uint32) []byte {
	b := binary.LittleEndian.AppendUint64(nil, flags)
	b = binary.LittleEndian.AppendUint32(b, index)
	b = binary.LittleEndian.AppendUint32(b, uid)
	return binary.LittleEndian.AppendUint32(b, uid)
}

func personTable(records ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(records))) //nolint:gosec // small test data
	b = append(b, 0)
	for _, r := range records {
		b = append(b, r...)
	}
	return b
}

func TestSplitPersonTableUsesTrailers(t *testing.T) {
	rec0 := append([]byte{2, 0xaa, 0xbb}, encodePersonTrailer(0x400400000, 0, 28093567)...)
	// Record 1 holds a decoy: its index after three zero bytes, with a zero uid.
	decoy := []byte{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rec1 := append(append([]byte{1}, decoy...), encodePersonTrailer(0x8, 1, 45)...)
	body := append([]byte{0xff, 0xff}, personTable(rec0, rec1)...)
	body = append(body, 0xee)

	recs, end, err := splitTable(body, 2, personTrailer)
	if err != nil {
		t.Fatalf("splitTable: %v", err)
	}

	start0 := 2 + 5
	want := []Record{
		{Tag: 2, Index: 0, UID: 28093567, Flags: 0x400400000, Start: start0, End: start0 + len(rec0)},
		{Tag: 1, Index: 1, UID: 45, Flags: 0x8, Start: start0 + len(rec0), End: start0 + len(rec0) + len(rec1)},
	}
	if len(recs) != len(want) {
		t.Fatalf("got %d records, want %d: %+v", len(recs), len(want), recs)
	}
	for i := range want {
		if recs[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, recs[i], want[i])
		}
	}
	if body[end] != 0xee {
		t.Errorf("end = %d, want the byte after the last record", end)
	}
}

func TestSplitPersonTablePrefersTrailerWithRepeatedUID(t *testing.T) {
	// Record 1 holds a loose match (differing uid words, as in attribute
	// data) before its real trailer. Record 2's trailer is loose only, which
	// the game writes when the third word holds a separate non-zero field.
	rec0 := append([]byte{1}, encodePersonTrailer(0x8a04000701, 0, 45)...) // flags of the real record 0
	loose := encodePersonTrailer(0, 1, 0x14140100)
	binary.LittleEndian.PutUint32(loose[16:], 0x01010114)
	rec1 := append(append([]byte{1}, loose...), encodePersonTrailer(0x400000, 1, 47)...)
	last := encodePersonTrailer(0x8, 2, 2002042480)
	binary.LittleEndian.PutUint32(last[16:], 939947854)
	rec2 := append([]byte{5}, last...)

	recs, _, err := splitTable(personTable(rec0, rec1, rec2), 0, personTrailer)
	if err != nil {
		t.Fatalf("splitTable: %v", err)
	}

	if recs[1].UID != 47 || recs[1].End-recs[1].Start != len(rec1) {
		t.Errorf("record 1 = %+v, want uid 47 spanning %d bytes", recs[1], len(rec1))
	}
	if recs[2].UID != 2002042480 {
		t.Errorf("record 2 = %+v, want uid 2002042480", recs[2])
	}
}

func TestSplitPersonTableReportsMissingTrailer(t *testing.T) {
	body := personTable([]byte{2, 1, 2, 3})

	_, _, err := splitTable(body, 0, personTrailer)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestSplitTableWithCompetitionTrailers(t *testing.T) {
	// Competition records end with (index, uid, uid) right after record data
	// and the next record starts with tag 0.
	rec0 := append([]byte{0, 0x07, 0x01, 0x00, 0x6c, 0x07, 0, 0, 0xff}, dbRecord(nil, 0, 1)...)
	rec1 := append([]byte{0, 0x07, 0x58, 0x00, 0xea, 0x07, 0, 1, 0xff}, dbRecord(nil, 1, 11)...)
	body := append(personTable(rec0, rec1), tableEnd...)

	recs, end, err := splitTable(body, 0, competitionTrailer)
	if err != nil {
		t.Fatalf("splitTable: %v", err)
	}

	if len(recs) != 2 || recs[0].UID != 1 || recs[1].UID != 11 || end != len(body)-len(tableEnd) {
		t.Errorf("records = %+v, end = %d", recs, end)
	}
}

// Loose trailers make every look-ahead scan to the end of the body.
func TestSplitTableBoundsScanWork(t *testing.T) {
	const n = 40_000
	rec := func(k uint32) []byte {
		tr := encodePersonTrailer(0, k, 0x14140100)
		binary.LittleEndian.PutUint32(tr[16:], 0x01010114)
		return append([]byte{1}, tr...)
	}
	recs := make([][]byte, n)
	for k := range recs {
		recs[k] = rec(uint32(k)) //nolint:gosec // small test data
	}
	body := personTable(recs...)

	_, _, err := splitTable(body, 0, personTrailer)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func looseRecord(k uint32) []byte {
	tr := encodePersonTrailer(0, k, 0x14140100)
	binary.LittleEndian.PutUint32(tr[16:], 0x01010114)
	return append([]byte{1}, tr...)
}

func strictRecord(k uint32) []byte {
	return append([]byte{1}, encodePersonTrailer(0, k, 1<<20)...)
}

// The budget covers the bytes after the table's start, so a large prefix
// does not raise it. Thirty loose-only records at the front of a 100 KB table
// cost about 60 times the table; the prefix would admit them at 10 times its
// size.
func TestSplitTableBudgetIgnoresPrefix(t *testing.T) {
	const loose, total = 30, 5000
	recs := make([][]byte, total)
	for k := range recs {
		if k < loose {
			recs[k] = looseRecord(uint32(k)) //nolint:gosec // small test data
		} else {
			recs[k] = strictRecord(uint32(k)) //nolint:gosec // small test data
		}
	}
	prefix := make([]byte, 16<<20)
	body := append(prefix, personTable(recs...)...)

	_, _, err := splitTable(body, len(prefix), personTrailer)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestFindTrailerCountsMissedBytes(t *testing.T) {
	body := make([]byte, 1000)
	var scanned int

	_, ok := findTrailer(body, 100, 900, 7, true, personTrailer, &scanned)

	if ok || scanned != 800 {
		t.Errorf("ok = %v, scanned = %d, want a miss over 800 bytes", ok, scanned)
	}
}

func TestFindTrailerCountsHits(t *testing.T) {
	// Index 0 matches at every byte of a zero run; the zero uid rejects each.
	body := make([]byte, 100)
	var scanned int

	_, ok := findTrailer(body, 10, 60, 0, true, personTrailer, &scanned)

	if ok || scanned < 4*(60-10-3) {
		t.Errorf("ok = %v, scanned = %d, want each hit counted", ok, scanned)
	}
}
