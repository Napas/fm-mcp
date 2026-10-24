package savefile

import (
	"errors"
	"testing"
)

func TestReaderReadsLittleEndianValues(t *testing.T) {
	r := newReader([]byte{
		0x01,
		0x02, 0x01,
		0x04, 0x03, 0x02, 0x01,
		0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	})

	if got := r.u8(); got != 0x01 {
		t.Errorf("u8 = %#x, want 0x01", got)
	}
	if got := r.u16(); got != 0x0102 {
		t.Errorf("u16 = %#x, want 0x0102", got)
	}
	if got := r.u32(); got != 0x01020304 {
		t.Errorf("u32 = %#x, want 0x01020304", got)
	}
	if got := r.u64(); got != 0x0102030405060708 {
		t.Errorf("u64 = %#x, want 0x0102030405060708", got)
	}
	if got := r.i64(); got != -1 {
		t.Errorf("i64 = %d, want -1", got)
	}
	if r.err != nil {
		t.Fatalf("err = %v", r.err)
	}
	if r.remaining() != 0 {
		t.Errorf("remaining = %d, want 0", r.remaining())
	}
}

func TestReaderReadsLengthPrefixedString(t *testing.T) {
	r := newReader([]byte{4, 0, 0, 0, 'S', 'p', 'u', 'r', 's'})

	if got := r.str(); got != "Spur" {
		t.Errorf("str = %q, want %q", got, "Spur")
	}
	if r.err != nil {
		t.Fatalf("err = %v", r.err)
	}
	if r.remaining() != 1 {
		t.Errorf("remaining = %d, want 1", r.remaining())
	}
}

func TestReaderErrorsOnShortInputAndStaysFailed(t *testing.T) {
	r := newReader([]byte{1, 2, 3})

	if got := r.u32(); got != 0 {
		t.Errorf("u32 on short input = %d, want 0", got)
	}
	if !errors.Is(r.err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", r.err)
	}
	if got := r.u8(); got != 0 {
		t.Errorf("u8 after failure = %d, want 0", got)
	}
}

func TestReaderRejectsStringLongerThanCap(t *testing.T) {
	r := newReader([]byte{0xff, 0xff, 0xff, 0x7f, 'x'})

	r.str()

	if !errors.Is(r.err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", r.err)
	}
}

func TestReaderExpect(t *testing.T) {
	r := newReader([]byte{9, 0x42, 0})
	r.expectU8(9, "tag")
	r.expectU16(0x42, "version")
	if r.err != nil || r.remaining() != 0 {
		t.Fatalf("matching values: err %v, %d bytes left", r.err, r.remaining())
	}
	r = newReader([]byte{8, 0x42, 0})
	r.expectU8(9, "tag")
	r.expectU16(0x43, "version")
	if !errors.Is(r.err, ErrCorrupt) || r.err.Error() != "savefile: corrupt data: tag 0x8 at offset 1" {
		t.Errorf("err = %v, want the first mismatch as ErrCorrupt", r.err)
	}
}
