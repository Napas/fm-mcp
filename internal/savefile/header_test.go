package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func containerHeader(kind, order byte, magic string, version uint16, dirOff, dataOff uint64, b25 byte) []byte {
	b := []byte{kind, order}
	b = append(b, magic...)
	b = binary.LittleEndian.AppendUint16(b, version)
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint64(b, dirOff)
	b = binary.LittleEndian.AppendUint64(b, dataOff)
	return append(b, b25)
}

func TestParseHeaderResolvesAbsoluteOffsets(t *testing.T) {
	h, err := ParseHeader(containerHeader(2, 1, "fmf.", 8, 46481093, 17, 3))
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	want := Header{Magic: "fmf.", Version: 8, DirectoryOffset: 46481102, DataOffset: 26, Byte25: 3}
	if h != want {
		t.Errorf("got %+v, want %+v", h, want)
	}
}

func TestParseHeaderRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"wrong kind", containerHeader(3, 1, "fmf.", 8, 100, 17, 3), ErrNotSave},
		{"wrong magic", containerHeader(2, 1, "zip.", 8, 100, 17, 3), ErrNotSave},
		{"big endian", containerHeader(2, 0, "fmf.", 8, 100, 17, 3), ErrUnsupported},
		{"version 7", containerHeader(2, 1, "fmf.", 7, 100, 17, 3), ErrUnsupported},
		{"truncated", containerHeader(2, 1, "fmf.", 8, 100, 17, 3)[:20], ErrTruncated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseHeader(tt.in)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
