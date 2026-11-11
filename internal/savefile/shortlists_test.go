package savefile

import (
	"encoding/binary"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// listEntry is a 22-byte player list entry (version 2) for person, added on
// 2026-11-25.
func listEntry(person uint32) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{2}, person)
	b = append(b, 0x49, 0x6b, 0xea, 0x07) // added 329/2026
	b = append(b, 0, 0, 0, 0)
	b = append(b, 0x01, 0x00, 0x6c, 0x07) // 1/1900: unset
	return append(b, 0xff, 0xff, 0xff, 0xff, 0)
}

// listObject is a scout_man.dat player list object of type typ (0: a
// shortlist), with a filter tree cut down to its first tags.
func listObject(typ byte, name string, persons ...uint32) []byte {
	b := []byte{0x06, 0x81, 0xc4, 0x01, 0x00, 0x01, typ}
	b = append(b, make([]byte, 16)...)
	b = append(b, 0, 0xff, 0xff, 0xff, 0xff, 0x01, 0x01, 0x00, 0x6c, 0x07)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(name))) //nolint:gosec // test sizes
	b = append(b, name...)
	b = append(b, "\x01\x00\x01frlptlif\x00\x12\x00\x00\x00roip\x00rfsc\x00\x01\x01\x00rSrP\x00"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(persons))) //nolint:gosec // test sizes
	for _, p := range persons {
		b = append(b, listEntry(p)...)
	}
	// The bytes after the list: zeros, a u32 count of u32s, then the next
	// object's u16.
	b = append(b, make([]byte, 14)...)
	return append(b, 0x02, 0, 0, 0, 0x01, 0, 0, 0, 0x02, 0, 0, 0, 0, 0, 0, 0, 0x1e, 0x00)
}

func scoutBody(objects ...[]byte) []byte {
	b := []byte{0x1f, 0x00, 0x04, 0x00, 0x00, 0x00}
	for _, o := range objects {
		b = append(b, o...)
	}
	return b
}

func TestParseShortlists(t *testing.T) {
	body := scoutBody(
		listObject(0, "interested", 8213, 9553),
		listObject(0, "", 20570),
		listObject(3, "", 18982, 11815), // a scouting list
		listObject(0, "empty"),
	)
	got, err := parseShortlists(body)
	if err != nil {
		t.Fatal(err)
	}
	want := []Shortlist{
		{Name: "interested", Persons: []uint32{8213, 9553}},
		{Name: "", Persons: []uint32{20570}},
		{Name: "empty", Persons: []uint32{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// nestedShortlist is a type-0 header with an id of 0 (valid UTF-8) and its
// marker, used as another object's name.
var nestedShortlist = string(append(append([]byte{0}, make([]byte, 16)...), 0, 0, 0, 0, 0, 1, 0x01, 0x00, 0x6c, 0x07, 0, 0, 0, 0)) + "\x01\x00\x01frlptlif"

func TestParseShortlistsRejects(t *testing.T) {
	good := listObject(0, "", 20570)
	badEntry := listObject(0, "", 20570)
	badEntry[len(badEntry)-len(listEntry(0))-32] = 9 // the entry's version byte
	long := listObject(0, "", 20570)
	long[len(long)-len(listEntry(0))-36] = 50 // count: more entries than bytes
	noList := listObject(0, "", 20570)
	copy(noList[len(noList)-len(listEntry(0))-41:], "xxxx") // the list's tag
	notHeader := listObject(0, "ab", 1)
	notHeader[10] = 1 // inside the zero run before the name length field
	for _, tt := range []struct {
		name string
		body []byte
		want error
	}{
		{"no shortlist", scoutBody(listObject(3, "", 1)), ErrNotFound},
		{"empty body", nil, ErrNotFound},
		{"bad entry", scoutBody(badEntry), ErrCorrupt},
		{"count past end", scoutBody(long), ErrCorrupt},
		{"header cut off", listObject(0, "", 20570)[10:], ErrCorrupt},
		{"next header starts before the list", scoutBody(listObject(3, nestedShortlist)), ErrCorrupt},
		{"no list", scoutBody(noList, good), ErrCorrupt},
		{"zero run broken", scoutBody(notHeader), ErrCorrupt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := parseShortlists(tt.body); !errors.Is(err, tt.want) {
				t.Errorf("got %+v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

// The list ends before the next object's header, even with body left after it.
func TestParseShortlistsCountPastNextObject(t *testing.T) {
	long := listObject(0, "", 20570)
	long[len(long)-len(listEntry(0))-36] = 50 // 1100 bytes of entries; 20 objects follow
	_, err := parseShortlists(scoutBody(append([][]byte{long}, slices.Repeat([][]byte{listObject(0, "", 1)}, 20)...)...))
	if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "50 entries need more than") {
		t.Errorf("err = %v, want the count rejected against the next object", err)
	}
}
