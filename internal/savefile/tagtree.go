package savefile

import (
	"encoding/binary"
	"slices"
)

// Tagged trees (see "27.0.0 tagged trees"): the game's competition and rule
// files, embedded in game_db.dat. A value is a type byte then its payload; a
// struct field is a 4-character tag stored reversed, 0x01, then a value; a
// list item is 0x01 then a value.
const (
	treeStruct = 0x0a
	treeList   = 0x0b
	treeString = 0x1a
	treePair   = 0x0f // a (uid, uid) pair
	treeMarker = 0x01

	// treeFieldMin and treeItemMin are the smallest encoded field and list
	// item. With maxTreePrealloc they cap the capacity a count can claim.
	treeFieldMin    = 4 + 1 + 1
	treeItemMin     = 1 + 1
	maxTreePrealloc = 64
	maxTreeDepth    = 32
	// maxTreeNodes caps the values one read decodes, bounding allocation
	// from hostile counts.
	maxTreeNodes = 1 << 16
)

// treeScalarSizes are the payload sizes of the fixed-size types seen in
// 27.0.0 saves: 0x00 is an empty value, 0x02 a tag.
var treeScalarSizes = map[byte]int{
	0x00: 0, 0x01: 4, 0x02: 4, 0x03: 1, treePair: 8, 0x11: 1, 0x12: 2, 0x18: 8, 0x19: 4, 0x20: 4,
}

// treeValue is a decoded tagged-tree value.
type treeValue struct {
	kind   byte
	raw    []byte // a scalar's payload
	str    string
	fields []treeField // a struct's
	items  []treeValue // a list's
}

type treeField struct {
	tag   string // as the game names it, e.g. "sqsr"
	value treeValue
}

// readTree reads the tagged-tree value whose type byte is at p and returns
// it and the offset after it.
func readTree(b []byte, p int) (treeValue, int, error) {
	r := newReader(b)
	r.off = p
	nodes := maxTreeNodes
	v := readTreeValue(r, r.u8(), 0, &nodes)
	return v, r.off, r.err
}

// readTreePayload reads a value of the given kind whose payload starts at p.
func readTreePayload(b []byte, p int, kind byte) (treeValue, int, error) {
	nodes := maxTreeNodes
	return readTreeNodes(b, p, kind, &nodes)
}

// readTreeNodes is readTreePayload decoding at most *nodes values, counted
// down so reads can share a budget.
func readTreeNodes(b []byte, p int, kind byte, nodes *int) (treeValue, int, error) {
	r := newReader(b)
	r.off = p
	v := readTreeValue(r, kind, 0, nodes)
	return v, r.off, r.err
}

func readTreeValue(r *reader, kind byte, depth int, nodes *int) treeValue {
	v := treeValue{kind: kind}
	if r.err != nil {
		return v
	}
	if depth > maxTreeDepth {
		r.fail("tagged tree deeper than %d", maxTreeDepth)
		return v
	}
	if *nodes--; *nodes < 0 {
		r.fail("tagged tree over %d values", maxTreeNodes)
		return v
	}
	switch kind {
	case treeStruct:
		n := r.u32()
		v.fields = make([]treeField, 0, min(maxTreePrealloc, capacity(n, r.remaining(), treeFieldMin)))
		for range n {
			tag := tagName(r.take(4))
			r.expectU8(treeMarker, "tagged-tree field marker")
			kind := r.u8()
			if r.err != nil {
				return v
			}
			v.fields = append(v.fields, treeField{tag: tag, value: readTreeValue(r, kind, depth+1, nodes)})
		}
	case treeList:
		n := r.u32()
		v.items = make([]treeValue, 0, min(maxTreePrealloc, capacity(n, r.remaining(), treeItemMin)))
		for range n {
			r.expectU8(treeMarker, "tagged-tree item marker")
			kind := r.u8()
			if r.err != nil {
				return v
			}
			v.items = append(v.items, readTreeValue(r, kind, depth+1, nodes))
		}
	case treeString:
		v.str = r.str()
	default:
		size, ok := treeScalarSizes[kind]
		if !ok {
			r.fail("tagged-tree type %#x", kind)
			return v
		}
		v.raw = r.take(size)
	}
	return v
}

// tagName returns the tag stored reversed in raw.
func tagName(raw []byte) string {
	tag := slices.Clone(raw)
	slices.Reverse(tag)
	return string(tag)
}

// field returns the struct field tagged tag.
func (v treeValue) field(tag string) (treeValue, bool) {
	i := slices.IndexFunc(v.fields, func(f treeField) bool { return f.tag == tag })
	if i < 0 {
		return treeValue{}, false
	}
	return v.fields[i].value, true
}

// uint returns a 1-, 2- or 4-byte scalar.
func (v treeValue) uint() (uint64, bool) {
	switch len(v.raw) {
	case 1:
		return uint64(v.raw[0]), true
	case 2:
		return uint64(binary.LittleEndian.Uint16(v.raw)), true
	case 4:
		return uint64(binary.LittleEndian.Uint32(v.raw)), true
	}
	return 0, false
}

// ref returns the uid a reference holds: a scalar, the first of a (uid, uid)
// pair, or in a struct the field named by its class in "id  ", e.g. "comp".
func (v treeValue) ref() (uint32, bool) {
	switch {
	case v.kind == treeStruct:
		class, ok := v.field("id  ")
		if !ok || len(class.raw) != 4 {
			return 0, false
		}
		f, ok := v.field(tagName(class.raw))
		if !ok || f.kind == treeStruct {
			return 0, false
		}
		return f.ref()
	case v.kind == treePair && len(v.raw) == 8:
		return binary.LittleEndian.Uint32(v.raw), true
	}
	n, ok := v.uint()
	return uint32(n), ok //nolint:gosec // at most 4 bytes
}
