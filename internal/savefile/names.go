package savefile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

// ErrNotFound means a structure located by scanning was not found.
var ErrNotFound = errors.New("savefile: structure not found")

const (
	// minFirstNames is the smallest first-name table accepted; the test
	// save has 292 706.
	minFirstNames = 100_000
	maxNames      = 10_000_000
	maxNameLen    = 255
	namesToProbe  = 8
	// noName marks an unset name reference (-1 when read as i32).
	noName = ^uint32(0)
)

// NameTables holds the three PERSON_NAME tables of game_db.dat. Person
// records refer to names by index into these slices.
type NameTables struct {
	First  []string
	Last   []string
	Common []string
}

func (nt NameTables) first(id uint32) string  { return lookup(nt.First, id) }
func (nt NameTables) last(id uint32) string   { return lookup(nt.Last, id) }
func (nt NameTables) common(id uint32) string { return lookup(nt.Common, id) }

func lookup(names []string, id uint32) string {
	if int64(id) >= int64(len(names)) {
		return ""
	}
	return names[id]
}

func validNameRef(id uint32, n int) bool {
	return id == noName || int64(id) < int64(n)
}

// findNameTables scans for the first-name table (at least minFirst entries)
// and parses it and the two tables after it. It returns the offset past the
// third. Scanning is needed because the sections before them are not decoded.
func findNameTables(body []byte, minFirst uint32) (NameTables, int, error) {
	for p := 0; p+8 <= len(body); p++ {
		n := binary.LittleEndian.Uint32(body[p:])
		if n < minFirst || n > maxNames || !probeNameTable(body[p+4:], n) {
			continue
		}
		r := newReader(body)
		r.off = p
		var nt NameTables
		if nt.First = readNameTable(r); r.err != nil {
			continue
		}
		if nt.Last = readNameTable(r); r.err != nil {
			continue
		}
		if nt.Common = readNameTable(r); r.err != nil {
			continue
		}
		return nt, r.off, nil
	}
	return NameTables{}, 0, fmt.Errorf("%w: person name tables", ErrNotFound)
}

func probeNameTable(b []byte, n uint32) bool {
	r := newReader(b)
	for i := range min(n, namesToProbe) {
		id := r.u32()
		l := r.u32()
		if r.err != nil || id != i || l > maxNameLen {
			return false
		}
		if s := r.take(int(l)); r.err != nil || !utf8.Valid(s) {
			return false
		}
	}
	return true
}

// readNameTable reads u32 count, then count × (u32 id, string). Ids run 0, 1, 2…
func readNameTable(r *reader) []string {
	n := r.u32()
	if r.err == nil && n > maxNames {
		r.err = fmt.Errorf("%w: %d names", ErrCorrupt, n)
	}
	if r.err != nil {
		return nil
	}
	names := make([]string, 0, capacity(n, r.remaining(), 8)) // u32 id + u32 length
	for i := range n {
		if id := r.u32(); r.err == nil && id != i {
			r.err = fmt.Errorf("%w: name id %d at position %d", ErrCorrupt, id, i)
		}
		names = append(names, r.str())
		if r.err != nil {
			return nil
		}
	}
	return names
}
