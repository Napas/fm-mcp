package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// maxCompetitionKeys caps the ACTUAL_COMP_ID count (3 360 and 7 753 seen).
	maxCompetitionKeys = 1 << 20
	// competitionKeySize is an ACTUAL_COMP_ID record: u8 tag, 5 × i32,
	// database record.
	competitionKeySize = 1 + 5*4 + databaseRecordSize
	// competitionKeysHead is from a table's u32 count to its first record's
	// database record: u32 count, u8 0, u8 tag, 5 × i32.
	competitionKeysHead = 4 + 1 + 1 + 5*4
	stageNamePool       = "STAGE_NAME"
	leagueStagePool     = "LEAGUE_STAGE"
)

// firstKeyRecord is the database record of a key table's first record:
// index 0, id 1, id 1.
var firstKeyRecord = []byte{0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}

// CompetitionKey is an ACTUAL_COMP_ID record (writer 0x33618e8): the
// competition and stage a fixture belongs to. A fixture's competition ref
// is a key index.
type CompetitionKey struct {
	Comp       int32 // COMP index; -1 in a few keys no fixture uses
	Stage      int32 // STAGE_NAME index; -1 = none
	Group      int32 // STAGE_NAME index of the sub-stage or group; -1 = none
	Leg        int32 // STAGE_NAME index of the leg or replay; -1 = none
	SecondComp int32 // COMP index of the other division in a two-division playoff; -1 = none
}

// readCompetitionKeys reads the ACTUAL_COMP_ID table. It has no memory pool,
// so it is found by its first record's database record and must be the only
// table that parses.
func readCompetitionKeys(body []byte) ([]CompetitionKey, error) {
	var found []CompetitionKey
	tries := 0
	for from := 0; ; {
		i := bytes.Index(body[from:], firstKeyRecord)
		if i < 0 {
			break
		}
		hit := from + i
		from = hit + 1
		p := hit - competitionKeysHead
		if p < 0 || !afterTableEnd(body, p) || body[p+4] != 0 || body[p+5] != 0 {
			continue
		}
		if tries++; tries > maxNationCandidates {
			return nil, tooManyCandidates("ACTUAL_COMP_ID")
		}
		keys, ok := competitionKeysAt(body, p)
		if !ok {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("%w: two ACTUAL_COMP_ID tables", ErrCorrupt)
		}
		found = keys
	}
	if found == nil {
		return nil, fmt.Errorf("%w: ACTUAL_COMP_ID table", ErrNotFound)
	}
	return found, nil
}

// competitionKeysAt parses the key table whose count is at p: every record
// has tag 0, index i and ids i+1, and an end marker follows the last.
func competitionKeysAt(body []byte, p int) ([]CompetitionKey, bool) {
	n := binary.LittleEndian.Uint32(body[p:])
	if n == 0 || n > maxCompetitionKeys || len(body)-p-5 < int(n)*competitionKeySize {
		return nil, false
	}
	r := newReader(body)
	r.off = p + 5
	out := make([]CompetitionKey, 0, n)
	for i := range n {
		if r.u8() != 0 {
			return nil, false
		}
		k := CompetitionKey{Comp: r.i32(), Stage: r.i32(), Group: r.i32(), Leg: r.i32(), SecondComp: r.i32()}
		if index, id, id2 := r.u32(), r.u32(), r.u32(); r.err != nil || index != i || id != i+1 || id2 != id {
			return nil, false
		}
		out = append(out, k)
	}
	return out, bytes.HasPrefix(body[r.off:], tableEnd)
}

// readStageNames reads the STAGE_NAME table of count records (u8 tag 0,
// u8 0, database record) and returns the uids by stage index. Names are in
// the language database by uid.
func readStageNames(body []byte, count uint32) ([]uint32, error) {
	uids, _, err := readTable(body, count, []byte{0}, stageNamePool, func(r *reader) (uint32, uint32) {
		r.u8()
		r.expectU8(0, "STAGE_NAME byte")
		index, uid := readDatabaseRecord(r)
		return uid, index
	})
	return uids, err
}
