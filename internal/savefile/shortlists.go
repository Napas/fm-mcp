package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// scout_man.dat holds player list objects; type 0 is a shortlist (see
// "27.0.0 shortlists" in docs/format/scouting.md). The filter tree between
// an object's name and its list is not decoded: the parser finds each
// object by the marker after its name and its list by the marker before it.
const (
	scoutManVersion  = 31
	shortlistType    = 0
	listEntryVersion = 2
	listEntrySize    = 22
	maxListNameLen   = 255

	// Offsets back from an object's name length field.
	listTypeBack  = 27 // u8 list type
	listZerosBack = 26 // 16 zero bytes
	listZerosLen  = 16
	listDateBack  = 4 // date, meaning unknown
)

var (
	// listFilterMarker follows an object's name: 01 00 01 and the tags plrf, filt.
	listFilterMarker = []byte("\x01\x00\x01frlptlif")
	// listMarker (the tag PrSr and a type byte 0) comes before the u32 count.
	listMarker = []byte("rSrP\x00")
)

// Shortlist is one of the human manager's shortlists.
type Shortlist struct {
	Name    string   // "" for the default list, shown as "(Default)"
	Persons []uint32 // person indexes, in save order
}

// ReadShortlists decodes the shortlists in scout_man.dat.
func ReadShortlists(a *Archive) ([]Shortlist, error) {
	f, err := a.ReadFile("scout_man.dat")
	if err != nil {
		return nil, err
	}
	if f.Header.Version != scoutManVersion {
		return nil, fmt.Errorf("%w: scout_man.dat version %d", ErrUnsupported, f.Header.Version)
	}
	ls, err := parseShortlists(f.Body)
	if err != nil {
		return nil, fmt.Errorf("scout_man.dat shortlists: %w", err)
	}
	return ls, nil
}

// parseShortlists returns the shortlists among body's list objects. Each
// object's list must end before the next object's type byte. Any object whose
// header does not match fails the read, so a shortlist is never dropped
// silently.
func parseShortlists(body []byte) ([]Shortlist, error) {
	type object struct {
		mark, start int // the marker after the name; the type byte
		typ         byte
		name        string
	}
	var objs []object
	for p := 0; ; p++ {
		i := bytes.Index(body[p:], listFilterMarker)
		if i < 0 {
			break
		}
		p += i
		start, typ, name, ok := listHeader(body, p)
		if !ok {
			return nil, fmt.Errorf("%w: no list header before byte %d", ErrCorrupt, p)
		}
		objs = append(objs, object{p, start, typ, name})
	}
	var out []Shortlist
	for i, o := range objs {
		if o.typ != shortlistType {
			continue
		}
		end := len(body)
		if i+1 < len(objs) {
			end = max(o.mark, objs[i+1].start)
		}
		persons, err := listPersons(body[o.mark:end])
		if err != nil {
			return nil, fmt.Errorf("list %q at byte %d: %w", o.name, o.mark, err)
		}
		out = append(out, Shortlist{Name: o.name, Persons: persons})
	}
	if out == nil {
		return nil, fmt.Errorf("%w: no shortlist", ErrNotFound)
	}
	return out, nil
}

// listHeader finds the name ending at mark: a u32 length L at mark-4-L, with
// the zero run, a plausible date and valid UTF-8 around it. It returns the
// type byte's offset, the list type and the name.
func listHeader(body []byte, mark int) (int, byte, string, bool) {
	for l := 0; l <= maxListNameLen; l++ {
		n := mark - l - 4
		if n-listTypeBack < 0 {
			return 0, 0, "", false
		}
		if binary.LittleEndian.Uint32(body[n:]) != uint32(l) { //nolint:gosec // l ≤ maxListNameLen
			continue
		}
		name := body[n+4 : mark]
		zeros := body[n-listZerosBack : n-listZerosBack+listZerosLen]
		if !utf8.Valid(name) || !dateAt(body, n-listDateBack).plausible(minContractYear, maxContractYear) ||
			bytes.Count(zeros, []byte{0}) != listZerosLen {
			continue
		}
		return n - listTypeBack, body[n-listTypeBack], string(name), true
	}
	return 0, 0, "", false
}

// listPersons reads the list after the first listMarker in obj: a u32 count
// and that many entries (u8 version, u32 person, date added, u32, date, i32,
// u8 0).
func listPersons(obj []byte) ([]uint32, error) {
	i := bytes.Index(obj, listMarker)
	if i < 0 {
		return nil, fmt.Errorf("%w: no list", ErrCorrupt)
	}
	b := obj[i+len(listMarker):]
	if len(b) < 4 {
		return nil, fmt.Errorf("%w: list count cut off", ErrCorrupt)
	}
	n := binary.LittleEndian.Uint32(b)
	b = b[4:]
	if uint64(n)*listEntrySize > uint64(len(b)) {
		return nil, fmt.Errorf("%w: %d entries need more than the %d bytes left", ErrCorrupt, n, len(b))
	}
	persons := make([]uint32, n)
	for k := range persons {
		e := b[k*listEntrySize:]
		if e[0] != listEntryVersion || e[listEntrySize-1] != 0 || !dateAt(e, 5).plausible(minContractYear, maxContractYear) {
			return nil, fmt.Errorf("%w: entry %d", ErrCorrupt, k)
		}
		persons[k] = binary.LittleEndian.Uint32(e[1:])
	}
	return persons, nil
}
