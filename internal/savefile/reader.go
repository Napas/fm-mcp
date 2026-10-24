package savefile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrTruncated means the input ended before a value could be read.
var ErrTruncated = errors.New("savefile: truncated input")

// maxStringLen caps a length-prefixed string so a corrupt length cannot
// force a huge allocation.
const maxStringLen = 1 << 20

// reader decodes little-endian values from a byte slice. The first failure
// is kept in err; later reads return zero values.
type reader struct {
	b   []byte
	off int
	err error
}

func newReader(b []byte) *reader {
	return &reader{b: b}
}

func (r *reader) remaining() int {
	return len(r.b) - r.off
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > r.remaining() {
		r.err = fmt.Errorf("%w: need %d bytes at offset %d, have %d", ErrTruncated, n, r.off, r.remaining())
		return nil
	}
	v := r.b[r.off : r.off+n]
	r.off += n
	return v
}

func (r *reader) u8() uint8 {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *reader) u16() uint16 {
	if b := r.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (r *reader) i32() int32 {
	return int32(r.u32()) //nolint:gosec // two's-complement reinterpretation is intended
}

func (r *reader) f32() float32 {
	return math.Float32frombits(r.u32())
}

func (r *reader) u64() uint64 {
	if b := r.take(8); b != nil {
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

func (r *reader) i64() int64 {
	return int64(r.u64()) //nolint:gosec // two's-complement reinterpretation is intended
}

func (r *reader) str() string {
	n := r.u32()
	if r.err == nil && n > maxStringLen {
		r.err = fmt.Errorf("%w: string length %d at offset %d exceeds %d", ErrTruncated, n, r.off-4, maxStringLen)
		return ""
	}
	return string(r.take(int(n)))
}

// refs8 reads a u8 count followed by that many i32 object references.
func (r *reader) refs8() []int32 {
	n := int(r.u8())
	if r.err != nil || n == 0 {
		return nil
	}
	refs := make([]int32, n)
	for i := range refs {
		refs[i] = r.i32()
	}
	return refs
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s at offset %d", ErrCorrupt, fmt.Sprintf(format, args...), r.off)
	}
}

// expectU8 reads a u8 and fails the reader unless it is want; what names the
// value in the error.
func (r *reader) expectU8(want uint8, what string) {
	if v := r.u8(); v != want {
		r.fail("%s %#x", what, v)
	}
}

// expectU16 reads a u16 and fails the reader unless it is want.
func (r *reader) expectU16(want uint16, what string) {
	if v := r.u16(); v != want {
		r.fail("%s %#x", what, v)
	}
}

// capacity is the starting capacity for count records read from avail
// bytes: never more than the bytes can hold, so a corrupt count cannot force
// a large allocation before any record is read.
func capacity(count uint32, avail, minRecordSize int) int {
	return max(0, min(int(count), avail/minRecordSize))
}
