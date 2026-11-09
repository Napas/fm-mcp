package savefile

import (
	"encoding/binary"
	"errors"
	"runtime"
	"slices"
	"testing"
)

// Tagged-tree encoders for tests. A value is its type byte then its payload.

func tScalar(kind byte, payload ...byte) []byte { return append([]byte{kind}, payload...) }

func tU8(v uint8) []byte   { return tScalar(0x11, v) }
func tU16(v uint16) []byte { return tScalar(0x12, binary.LittleEndian.AppendUint16(nil, v)...) }
func tU32(v uint32) []byte { return tScalar(0x01, binary.LittleEndian.AppendUint32(nil, v)...) }

// tPair is the 8-byte type the game uses for (uid, uid) references.
func tPair(a, b uint32) []byte {
	return tScalar(0x0f, binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, a), b)...)
}

// tTag is the 4-byte type holding a tag, e.g. a struct's class in "id  ".
func tTag(tag string) []byte { return tScalar(0x02, reversed(tag)...) }

func tString(s string) []byte {
	return append(binary.LittleEndian.AppendUint32([]byte{0x1a}, uint32(len(s))), s...) //nolint:gosec // test strings are short
}

// tField is a struct field: the tag stored reversed, 0x01, then the value.
func tField(tag string, value []byte) []byte {
	return append(append(reversed(tag), 0x01), value...)
}

func tStruct(fields ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{treeStruct}, uint32(len(fields))) //nolint:gosec // test sizes are small
	return append(b, slices.Concat(fields...)...)
}

// tList is a list; each item is 0x01 then the value.
func tList(items ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{treeList}, uint32(len(items))) //nolint:gosec // test sizes are small
	for _, it := range items {
		b = append(append(b, 0x01), it...)
	}
	return b
}

func reversed(tag string) []byte {
	b := []byte(tag)
	slices.Reverse(b)
	return b
}

func TestReadTreeDecodesEveryType(t *testing.T) {
	tree := tStruct(
		tField("name", tString("TOPLYGA")),
		tField("valu", tU8(30)),
		tField("year", tU16(2000)),
		tField("comp", tU32(2300046)),
		tField("dyow", tScalar(0x00)),
		tField("flag", tScalar(0x03, 1)),
		tField("EdDt", tScalar(0x20, 1, 2, 3, 4)),
		tField("xfgl", tScalar(0x18, 1, 2, 3, 4, 5, 6, 7, 8)),
		tField("Clat", tScalar(0x19, 0, 0, 0x80, 0x3f)),
		tField("sqsr", tList(tStruct(tField("type", tU8(9))), tStruct(tField("type", tU8(94))))),
	)
	b := append(append([]byte{0xee}, tree...), 0xee)
	v, end, err := readTree(b, 1)
	if err != nil {
		t.Fatalf("readTree: %v", err)
	}
	if end != len(b)-1 {
		t.Errorf("end = %d, want %d", end, len(b)-1)
	}
	if name, _ := v.field("name"); name.str != "TOPLYGA" {
		t.Errorf("name = %q", name.str)
	}
	for tag, want := range map[string]uint64{"valu": 30, "year": 2000, "comp": 2300046, "flag": 1} {
		f, _ := v.field(tag)
		if got, ok := f.uint(); !ok || got != want {
			t.Errorf("%s = %d, %v; want %d", tag, got, ok, want)
		}
	}
	rules, _ := v.field("sqsr")
	var types []uint64
	for _, it := range rules.items {
		ty, _ := it.field("type")
		n, _ := ty.uint()
		types = append(types, n)
	}
	if !slices.Equal(types, []uint64{9, 94}) {
		t.Errorf("sqsr types = %v", types)
	}
	if _, ok := v.field("none"); ok {
		t.Error("field(none) found")
	}
	if s, _ := v.field("name"); func() bool { _, ok := s.uint(); return ok }() {
		t.Error("a string has a uint")
	}
}

func TestTreeRef(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value []byte
		want  uint32
	}{
		{"u32", tU32(2300046), 2300046},
		{"u8", tU8(7), 7},
		{"pair", tPair(2300046, 2300046), 2300046},
		{"struct", tStruct(tField("id  ", tTag("comp")), tField("comp", tPair(11, 11)), tField("DBID", tU32(11))), 11},
		{"struct without DBID", tStruct(tField("id  ", tTag("comp")), tField("comp", tU8(7))), 7},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v, _, err := readTree(tStruct(tField("comp", tt.value)), 0)
			if err != nil {
				t.Fatal(err)
			}
			f, _ := v.field("comp")
			if got, ok := f.ref(); !ok || got != tt.want {
				t.Errorf("ref = %d, %v; want %d", got, ok, tt.want)
			}
		})
	}
	v, _, _ := readTree(tStruct(tField("name", tString("x")), tField("s", tStruct(tField("id  ", tTag("comp"))))), 0)
	for _, tag := range []string{"name", "s"} {
		f, _ := v.field(tag)
		if _, ok := f.ref(); ok {
			t.Errorf("%s has a ref", tag)
		}
	}
}

func TestReadTreeRejectsBadInput(t *testing.T) {
	deep := tStruct()
	for range maxTreeDepth + 1 {
		deep = tStruct(tField("deep", deep))
	}
	for _, tt := range []struct {
		name string
		b    []byte
		want error
	}{
		{"unknown type", tStruct(tField("what", tScalar(0x42, 0))), ErrCorrupt},
		{"field marker", append(tStruct(tField("name", tU8(1)))[:9], append([]byte{0x02}, tU8(1)...)...), ErrCorrupt},
		{"item marker", append(tStruct(tField("sqsr", tList(tU8(1))))[:15], 0x02, 0x11, 1), ErrCorrupt},
		{"truncated", tStruct(tField("name", tString("TOPLYGA")))[:12], ErrTruncated},
		{"huge count", binary.LittleEndian.AppendUint32([]byte{treeStruct}, 1<<31), ErrTruncated},
		{"huge string", tStruct(tField("name", binary.LittleEndian.AppendUint32([]byte{treeString}, 1<<31))), ErrTruncated},
		{"too deep", deep, ErrCorrupt},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := readTree(tt.b, 0); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// Nested values claiming huge counts allocate little: each level's capacity
// is capped.
func TestReadTreeBoundsNestedAllocations(t *testing.T) {
	var b []byte
	for range maxTreeDepth {
		b = append(binary.LittleEndian.AppendUint32(b, 0xffffffff), treeMarker, treeList)
	}
	b = append(append([]byte{treeList}, b...), make([]byte, 1<<20)...)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, _, err := readTree(b, 0); err == nil {
		t.Fatal("readTree accepted a truncated list")
	}
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 1<<20 {
		t.Errorf("readTree allocated %d bytes for a %d-byte input", got, len(b))
	}
}

func TestReadTreeCapsItsValues(t *testing.T) {
	items := make([][]byte, maxTreeNodes)
	for i := range items {
		items[i] = tScalar(0x00)
	}
	if _, _, err := readTree(tList(items...), 0); !errors.Is(err, ErrCorrupt) {
		t.Errorf("a list of %d values: err = %v, want ErrCorrupt", maxTreeNodes, err)
	}
	if _, _, err := readTree(tList(items[1:]...), 0); err != nil {
		t.Errorf("a list of %d values: %v", maxTreeNodes-1, err)
	}
}
