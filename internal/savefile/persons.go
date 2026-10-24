package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// databaseRecordSize is the index/uid block that ends every table record.
const databaseRecordSize = 12

const (
	maxTableRecords   = 10_000_000
	personTrailerSize = 20

	// Trailer searches in splitTable may scan scanBudgetFactor × the bytes
	// from the table's start to the end of the body, plus scanBudgetSlack; a
	// crafted body can otherwise force a rescan of the rest of the body per
	// record.
	scanBudgetFactor = 8
	scanBudgetSlack  = 1 << 16
)

// Record locates one record of a game_db.dat table (persons,
// competitions…) inside the file's body. Start..End includes the record's
// trailer.
type Record struct {
	Tag   uint8 // class tag; for persons 2 = player, 3 = player-coach
	Index uint32
	UID   uint32 // database uid
	Flags uint64 // trailer flags; for persons bit 12 marks a woman
	Start int
	End   int
}

// splitTable splits the table whose u32 count is at start and returns the
// offset past the last record. It serves tables whose record bodies are not
// fully decoded (persons, competitions).
//
// Record bodies are not decoded, so boundaries come from the trailer each
// record ends with:
//
//	u64 flags | u32 index | u32 uid | u32 extra (uid again when unset)
//
// The flags' upper three bytes and uid are never zero. Attribute bytes can
// still mimic a trailer, so a trailer whose last two words match is
// preferred; one whose words differ is used only when no matching one
// appears before the next record's trailer.
func splitTable(body []byte, start int, valid trailerCheck) ([]Record, int, error) {
	budget := scanBudgetFactor*max(len(body)-start, 0) + scanBudgetSlack
	var scanned int
	r := newReader(body)
	r.off = start
	n := r.u32()
	r.u8()
	if r.err != nil {
		return nil, 0, r.err
	}
	if n > maxTableRecords {
		return nil, 0, fmt.Errorf("%w: %d records", ErrCorrupt, n)
	}
	recs := make([]Record, 0, capacity(n, len(body)-r.off, databaseRecordSize))
	pos := r.off
	for k := range n {
		limit := len(body)
		if k+1 < n {
			if j, ok := findTrailer(body, pos, limit, k+1, true, valid, &scanned); ok {
				limit = j
			}
		}
		j, ok := findTrailer(body, pos, limit, k, true, valid, &scanned)
		if !ok {
			j, ok = findTrailer(body, pos, limit, k, false, valid, &scanned)
		}
		if scanned > budget {
			return nil, 0, fmt.Errorf("%w: trailer search for record %d exceeds its scan budget", ErrCorrupt, k)
		}
		if !ok {
			return nil, 0, fmt.Errorf("%w: no trailer for record %d after offset %d", ErrCorrupt, k, pos)
		}
		uid := binary.LittleEndian.Uint32(body[j+4:])
		end := j + 12
		var flags uint64
		if j >= 8 {
			flags = binary.LittleEndian.Uint64(body[j-8:])
		}
		recs = append(recs, Record{Tag: body[pos], Index: k, UID: uid, Flags: flags, Start: pos, End: end})
		pos = end
	}
	return recs, pos, nil
}

// trailerCheck tells whether the index word at j can start the trailer of
// the record that begins at pos.
type trailerCheck func(body []byte, pos, j int) bool

// personTrailer requires the flags u64 before the index word to have
// zero upper three bytes.
func personTrailer(body []byte, pos, j int) bool {
	return j-8 >= pos && body[j-3] == 0 && body[j-2] == 0 && body[j-1] == 0
}

// competitionTrailer: competition records end with the bare database record,
// and the next record (or the table's end marker) starts with a zero byte.
func competitionTrailer(body []byte, _, j int) bool {
	return j+12 < len(body) && body[j+12] == 0
}

// findTrailer returns the offset of the index word of a trailer for index k,
// at or after pos, with the index word before limit. strict requires the uid
// word to repeat. The bytes searched are added to *scanned.
func findTrailer(body []byte, pos, limit int, k uint32, strict bool, valid trailerCheck, scanned *int) (int, bool) {
	key := binary.LittleEndian.AppendUint32(nil, k)
	for q := pos; q < limit; {
		i := bytes.Index(body[q:limit], key)
		if i < 0 {
			*scanned += limit - q
			return 0, false
		}
		*scanned += i + len(key)
		j := q + i
		q = j + 1
		if j <= pos || j+12 > len(body) || !valid(body, pos, j) {
			continue
		}
		uid := binary.LittleEndian.Uint32(body[j+4:])
		extra := binary.LittleEndian.Uint32(body[j+8:])
		if uid != 0 && (uid == extra || !strict) {
			return j, true
		}
	}
	return 0, false
}
