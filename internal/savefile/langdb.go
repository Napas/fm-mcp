package savefile

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// langPrefixSize is the header before the container in a language database
// file: 03 01 "tad." u16 4000 u16 db version u16 7.
const langPrefixSize = 12

// English (UK) text lives in files named "8_<table>.dat".
const (
	langInjuriesFile     = "8_6.dat"
	langNationsFile      = "8_9.dat"
	langCompetitionsFile = "8_25.dat"
	langStagesFile       = "8_23.dat"
)

// LangDB is one of the game's bundled language databases
// (db/<label>/<label>_fm/lang_db.dat or lang_update_db.dat). Saves hold no
// names for nations, competitions or injuries; those are looked up here by
// database uid. Club names are stored in the save.
type LangDB struct {
	a *Archive
}

// Close releases the file.
func (l *LangDB) Close() error { return l.a.Close() }

// LangName is a translated name of a database object.
type LangName struct {
	UID       uint32
	Name      string
	ShortName string
	Code      string // e.g. "ENG"; empty where the table has none
}

// OpenLangDB opens a language database file for reading.
func OpenLangDB(path string) (*LangDB, error) {
	a, err := openFile(path, func(f *os.File, size int64) (*Archive, error) {
		if size < langPrefixSize {
			return nil, fmt.Errorf("%w: %s is %d bytes", ErrNotSave, path, size)
		}
		size -= langPrefixSize
		a, err := NewArchive(io.NewSectionReader(f, langPrefixSize, size), size)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return a, nil
	})
	if err != nil {
		return nil, err
	}
	return &LangDB{a: a}, nil
}

// Nations reads the English nation names. Each record is:
//
//	u32 uid, string name, u8, string short name, u8, string code, string adjective, u8
func (l *LangDB) Nations() ([]LangName, error) {
	return l.readTable(langNationsFile, func(r *reader) LangName {
		v := readLangName(r)
		r.str() // adjective
		r.u8()
		return v
	})
}

// Competitions reads the English competition names. Each record is:
//
//	u32 uid, string name, u8, string short name, u8, string code, u8
func (l *LangDB) Competitions() ([]LangName, error) {
	return l.readTable(langCompetitionsFile, func(r *reader) LangName {
		v := readLangName(r)
		r.u8()
		return v
	})
}

// Stages reads the English stage names ("Group A", "Semi Final"). Each
// record is:
//
//	u32 uid, string name, u8, string short name, u8, string code, u8, u8
func (l *LangDB) Stages() ([]LangName, error) {
	return l.readTable(langStagesFile, func(r *reader) LangName {
		v := readLangName(r)
		r.take(2)
		return v
	})
}

// Injuries reads the English injury names. Each record is:
//
//	u32 uid, string name, u8, string name with article ("a bruised ankle"), u8, u8
func (l *LangDB) Injuries() ([]LangName, error) {
	return l.readTable(langInjuriesFile, func(r *reader) LangName {
		v := LangName{UID: r.u32(), Name: r.str()}
		r.u8()
		r.str() // name with article
		r.take(2)
		return v
	})
}

// readLangName reads the start every English table record shares:
// u32 uid, string name, u8, string short name, u8, string code.
func readLangName(r *reader) LangName {
	v := LangName{UID: r.u32(), Name: r.str()}
	r.u8()
	v.ShortName = r.str()
	r.u8()
	v.Code = r.str()
	return v
}

// readTable reads u32 count + count records from an English table file,
// which must end exactly after the last record.
func (l *LangDB) readTable(name string, record func(*reader) LangName) ([]LangName, error) {
	f, err := l.a.ReadFile(name)
	if err != nil {
		return nil, err
	}
	r := newReader(f.Body)
	n := r.u32()
	if n > maxNames {
		return nil, fmt.Errorf("%w: %d records in %s", ErrCorrupt, n, name)
	}
	out := make([]LangName, 0, capacity(n, r.remaining(), 4)) // a u32 uid at least
	for range n {
		out = append(out, record(r))
	}
	if r.err != nil {
		return nil, fmt.Errorf("%s: %w", name, r.err)
	}
	if r.remaining() != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes in %s", ErrCorrupt, r.remaining(), name)
	}
	return out, nil
}

// NamesByRank maps a save table's record index → name. Nations and injuries
// are stored in ascending uid order (verified on 26.2.0 and 26.3.2), so the
// index is the uid's rank. Without as many save records as language records,
// it returns no names.
func NamesByRank(lang []LangName, saveCount uint32) map[int64]string {
	if uint32(len(lang)) != saveCount { //nolint:gosec // language tables hold a few hundred records
		return nil
	}
	sorted := slices.SortedFunc(slices.Values(lang), func(a, b LangName) int { return cmp.Compare(a.UID, b.UID) })
	out := make(map[int64]string, len(sorted))
	for i, n := range sorted {
		out[int64(i)] = n.Name
	}
	return out
}

// LangDBPath returns the language database for a save's database label
// (e.g. "2620"): the folder's lang_db.dat, else its lang_update_db.dat.
// It returns "" when neither exists.
func LangDBPath(dbDir, label string) string {
	if label == "" || strings.ContainsAny(label, `/\.`) {
		return ""
	}
	for _, name := range []string{"lang_db.dat", "lang_update_db.dat"} {
		p := filepath.Join(dbDir, label, label+"_fm", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
