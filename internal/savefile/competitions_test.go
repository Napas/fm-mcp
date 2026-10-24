package savefile

import (
	"encoding/binary"
	"errors"
	"slices"
	"testing"
)

// competitionKeyRecord encodes an ACTUAL_COMP_ID record: u8 tag 0, five
// i32 fields, then the database record (index, id, id with id = index+1).
func competitionKeyRecord(index uint32, k CompetitionKey) []byte {
	b := []byte{0}
	for _, v := range []int32{k.Comp, k.Stage, k.Group, k.Leg, k.SecondComp} {
		b = binary.LittleEndian.AppendUint32(b, uint32(v)) //nolint:gosec // signed field
	}
	b = binary.LittleEndian.AppendUint32(b, index)
	b = binary.LittleEndian.AppendUint32(b, index+1)
	return binary.LittleEndian.AppendUint32(b, index+1)
}

func TestReadCompetitionKeys(t *testing.T) {
	want := []CompetitionKey{{267, -1, -1, -1, -1}, {8, 236, 17, 2, -1}}
	body := tableBytes(2, append(competitionKeyRecord(0, want[0]), competitionKeyRecord(1, want[1])...))

	got, err := readCompetitionKeys(body)
	if err != nil {
		t.Fatalf("readCompetitionKeys: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestReadCompetitionKeysRejectsWrongID(t *testing.T) {
	second := competitionKeyRecord(1, CompetitionKey{8, 236, 17, 2, -1})
	binary.LittleEndian.PutUint32(second[len(second)-4:], 7)
	body := tableBytes(2, append(competitionKeyRecord(0, CompetitionKey{267, -1, -1, -1, -1}), second...))

	if _, err := readCompetitionKeys(body); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadCompetitionKeysRejectsHugeCount(t *testing.T) {
	body := tableBytes(maxCompetitionKeys+1, competitionKeyRecord(0, CompetitionKey{267, -1, -1, -1, -1}))

	if _, err := readCompetitionKeys(body); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// stageNameRecord encodes a STAGE_NAME record: u8 tag 0, u8 0, then the
// database record.
func stageNameRecord(index, uid uint32) []byte {
	return dbRecord([]byte{0, 0}, index, uid)
}

func TestReadStageNames(t *testing.T) {
	var recs []byte
	for i, uid := range []uint32{1, 3, 238} {
		recs = append(recs, stageNameRecord(uint32(i), uid)...) //nolint:gosec // small test data
	}

	got, err := readStageNames(tableBytes(3, recs), 3)
	if err != nil {
		t.Fatalf("readStageNames: %v", err)
	}
	if want := []uint32{1, 3, 238}; !slices.Equal(got, want) {
		t.Errorf("uids = %v, want %v", got, want)
	}
}

func TestTestSaveCompetitionKeys(t *testing.T) {
	a := openTestSave(t)
	db, err := ReadGameDB(a)
	if err != nil {
		t.Fatal(err)
	}
	checkCompetitionKeys(t, db, 3360, 240)
}

// checkCompetitionKeys checks the key and stage tables' sizes and that every
// key refers to existing competitions and stages. Some keys have no
// competition (24 test, 214 KŽ; no fixture uses them).
func checkCompetitionKeys(t *testing.T, db *GameDB, keys int, leagueStages uint32) {
	t.Helper()
	if db.CompetitionKeysErr != nil || db.StagesErr != nil {
		t.Fatalf("keys: %v; stages: %v", db.CompetitionKeysErr, db.StagesErr)
	}
	if len(db.CompetitionKeys) != keys || len(db.StageUIDs) != 285 || db.LeagueStageCount != leagueStages {
		t.Errorf("%d keys, %d stages, %d league stages; want %d, 285, %d", len(db.CompetitionKeys), len(db.StageUIDs), db.LeagueStageCount, keys, leagueStages)
	}
	comps, stages := int32(len(db.CompetitionUIDs)), int32(len(db.StageUIDs)) //nolint:gosec // table sizes fit
	for i, k := range db.CompetitionKeys {
		if k.Comp < -1 || k.Comp >= comps || k.SecondComp >= comps || k.SecondComp < -1 {
			t.Fatalf("key %d = %v: competition out of range", i, k)
		}
		for _, s := range []int32{k.Stage, k.Group, k.Leg} {
			if s < -1 || s >= stages {
				t.Fatalf("key %d = %v: stage out of range", i, k)
			}
		}
	}
}

// checkRealSaveCompetitionKeys checks keys and stage uids of the KŽ save of
// 2035-12-02 (Championship playoff keys, decoded from bytes).
func checkRealSaveCompetitionKeys(t *testing.T, db *GameDB) {
	t.Helper()
	checkCompetitionKeys(t, db, 7753, 645)
	if len(db.CompetitionKeys) <= 5724 || len(db.StageUIDs) <= 275 {
		return
	}
	if k := db.CompetitionKeys[5618]; k != (CompetitionKey{8, 236, 17, 2, -1}) {
		t.Errorf("key 5618 = %v", k)
	}
	if k := db.CompetitionKeys[5724]; k != (CompetitionKey{8, 236, 19, -1, -1}) {
		t.Errorf("key 5724 = %v", k)
	}
	for i, uid := range map[int]uint32{236: 238, 17: 18, 2: 3, 275: 2000223075} {
		if db.StageUIDs[i] != uid {
			t.Errorf("stage %d uid = %d, want %d", i, db.StageUIDs[i], uid)
		}
	}
}
