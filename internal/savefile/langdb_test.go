package savefile

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func langNationRecord(uid uint32, name, short, code, adjective string) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uid)
	b = appendString(b, name)
	b = append(b, 0xff)
	b = appendString(b, short)
	b = append(b, 0xff)
	b = appendString(b, code)
	b = appendString(b, adjective)
	return append(b, 0)
}

func langTable(records ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(records))) //nolint:gosec // small test data
	for _, r := range records {
		b = append(b, r...)
	}
	return b
}

// writeLangDB writes a language database file: a 12-byte prefix, then a
// container holding the given inner files.
func writeLangDB(t *testing.T, files ...testInner) string {
	t.Helper()
	prefix := []byte{3, 1, 't', 'a', 'd', '.', 0xa0, 0x0f, 0x95, 0x0e, 7, 0}
	path := filepath.Join(t.TempDir(), "lang_db.dat")
	if err := os.WriteFile(path, append(prefix, buildSave(t, files...)...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLangDBReadsEnglishNations(t *testing.T) {
	body := langTable(
		langNationRecord(765, "England", "England", "ENG", "English"),
		langNationRecord(5, "Algeria", "Algeria", "ALG", "Algerian"),
	)
	ldb, err := OpenLangDB(writeLangDB(t, testInner{path: "8_9.dat", data: append(innerFile("tad.", 3728, ""), body...)}))
	if err != nil {
		t.Fatalf("OpenLangDB: %v", err)
	}
	defer func() { _ = ldb.Close() }()

	nations, err := ldb.Nations()
	if err != nil {
		t.Fatalf("Nations: %v", err)
	}

	want := []LangName{{UID: 765, Name: "England", ShortName: "England", Code: "ENG"}, {UID: 5, Name: "Algeria", ShortName: "Algeria", Code: "ALG"}}
	if len(nations) != len(want) || nations[0] != want[0] || nations[1] != want[1] {
		t.Errorf("Nations = %+v, want %+v", nations, want)
	}
}

func TestLangDBRejectsTrailingBytes(t *testing.T) {
	body := append(langTable(langNationRecord(765, "England", "England", "ENG", "English")), 0xee)
	ldb, err := OpenLangDB(writeLangDB(t, testInner{path: "8_9.dat", data: append(innerFile("tad.", 3728, ""), body...)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()

	if _, err := ldb.Nations(); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestOpenLangDBRejectsShortFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lang_db.dat")
	if err := os.WriteFile(path, []byte{3, 1}, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := OpenLangDB(path); err == nil {
		t.Error("OpenLangDB accepted a 2-byte file")
	}
}

func langCompRecord(uid uint32, name, short, code string) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uid)
	b = appendString(b, name)
	b = append(b, 0)
	b = appendString(b, short)
	b = append(b, 0)
	b = appendString(b, code)
	return append(b, 0)
}

func TestLangDBReadsEnglishCompetitions(t *testing.T) {
	body := langTable(langCompRecord(11, "Premier League", "Premier League", "PL"), langCompRecord(1301394, "UEFA Champions League", "Champions League", "UCL"))
	ldb, err := OpenLangDB(writeLangDB(t, testInner{path: "8_25.dat", data: append(innerFile("tad.", 3728, ""), body...)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()

	comps, err := ldb.Competitions()
	if err != nil {
		t.Fatalf("Competitions: %v", err)
	}

	if len(comps) != 2 || comps[1] != (LangName{UID: 1301394, Name: "UEFA Champions League", ShortName: "Champions League", Code: "UCL"}) {
		t.Errorf("Competitions = %+v", comps)
	}
}

func langInjuryRecord(uid uint32, name, withArticle string) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uid)
	b = appendString(b, name)
	b = append(b, 0xff)
	b = appendString(b, withArticle)
	return append(b, 0xff, 0)
}

func TestLangDBReadsEnglishInjuries(t *testing.T) {
	body := langTable(langInjuryRecord(1, "cold", "a cold"), langInjuryRecord(100, "bruised ankle", "a bruised ankle"))
	ldb, err := OpenLangDB(writeLangDB(t, testInner{path: "8_6.dat", data: append(innerFile("tad.", 3725, ""), body...)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()

	injuries, err := ldb.Injuries()
	if err != nil {
		t.Fatalf("Injuries: %v", err)
	}

	want := []LangName{{UID: 1, Name: "cold"}, {UID: 100, Name: "bruised ankle"}}
	if !slices.Equal(injuries, want) {
		t.Errorf("Injuries = %+v, want %+v", injuries, want)
	}
}

func TestLangDBReadsEnglishStages(t *testing.T) {
	stage := func(uid uint32, name string) []byte {
		return append(langCompRecord(uid, name, name, ""), 0)
	}
	body := langTable(stage(1, "Replay"), stage(238, "Playoffs"))
	ldb, err := OpenLangDB(writeLangDB(t, testInner{path: "8_23.dat", data: append(innerFile("tad.", 3728, ""), body...)}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()

	stages, err := ldb.Stages()
	if err != nil {
		t.Fatalf("Stages: %v", err)
	}
	want := []LangName{{UID: 1, Name: "Replay", ShortName: "Replay"}, {UID: 238, Name: "Playoffs", ShortName: "Playoffs"}}
	if !slices.Equal(stages, want) {
		t.Errorf("Stages = %+v, want %+v", stages, want)
	}
}
