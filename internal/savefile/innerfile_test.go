package savefile

import (
	"errors"
	"testing"
)

func TestParseFileHeaderReversesTagIntoExtension(t *testing.T) {
	tests := []struct {
		in   []byte
		want FileHeader
	}{
		{[]byte{3, 1, 't', 'a', 'd', '.', 0xa0, 0x0f}, FileHeader{Extension: ".dat", Version: 4000}},
		{[]byte{3, 1, 't', 'm', 'c', '.', 0x8f, 0x0e, 0xff}, FileHeader{Extension: ".cmt", Version: 3727}},
	}
	for _, tt := range tests {
		got, err := ParseFileHeader(tt.in)
		if err != nil {
			t.Fatalf("ParseFileHeader(% x): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
		}
	}
}

func TestParseFileHeaderRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"container kind", []byte{2, 1, 't', 'a', 'd', '.', 0, 0}, ErrCorrupt},
		{"big endian", []byte{3, 0, 't', 'a', 'd', '.', 0, 0}, ErrUnsupported},
		{"truncated", []byte{3, 1, 't', 'a'}, ErrTruncated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFileHeader(tt.in)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
