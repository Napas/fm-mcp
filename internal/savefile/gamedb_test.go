package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func TestDateTimeUsesLowNineBitsAsDayOfYear(t *testing.T) {
	tests := []struct {
		d    Date
		want time.Time
	}{
		{Date{Raw: 0x1abc, Year: 2025}, time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC)},
		{Date{Raw: 0x7047, Year: 2026}, time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)},
		{Date{Raw: 0x0001, Year: 2024}, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		if got := tt.d.Time(); !got.Equal(tt.want) {
			t.Errorf("%+v.Time() = %s, want %s", tt.d, got, tt.want)
		}
	}
}

func appendDate(b []byte, d Date) []byte {
	b = binary.LittleEndian.AppendUint16(b, d.Raw)
	return binary.LittleEndian.AppendUint16(b, d.Year)
}

func gameDBBody(label string, gameDate Date) []byte {
	b := binary.LittleEndian.AppendUint16(nil, 3727)
	b = binary.LittleEndian.AppendUint16(b, 7)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint32(b, 8)
	b = binary.LittleEndian.AppendUint16(b, 3727)
	b = binary.LittleEndian.AppendUint16(b, 7)
	b = appendString(b, label)
	b = append(b, 0x2c, 0x36, 0xea, 0x07)
	b = binary.LittleEndian.AppendUint32(b, 485714)
	b = binary.LittleEndian.AppendUint16(b, 2025)
	b = binary.LittleEndian.AppendUint16(b, 0x21a0)
	b = appendDate(b, gameDate)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = append(b, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, 722795)
	return append(b, 0xee) // first byte of the next section
}

func TestParseGameDBHeader(t *testing.T) {
	body := gameDBBody("2620", Date{Raw: 0x1abc, Year: 2025})

	h, err := ParseGameDBHeader(body)
	if err != nil {
		t.Fatalf("ParseGameDBHeader: %v", err)
	}

	if h.DatabaseVersion != 3727 || h.OriginalDatabaseVersion != 3727 {
		t.Errorf("versions = %d/%d, want 3727/3727", h.DatabaseVersion, h.OriginalDatabaseVersion)
	}
	if h.Label != "2620" {
		t.Errorf("Label = %q, want 2620", h.Label)
	}
	if h.GameDate != (Date{Raw: 0x1abc, Year: 2025}) {
		t.Errorf("GameDate = %+v", h.GameDate)
	}
	if h.Size != len(body)-1 || body[h.Size] != 0xee {
		t.Errorf("Size = %d, want %d", h.Size, len(body)-1)
	}
}

func TestParseGameDBHeaderRejectsTruncatedBody(t *testing.T) {
	body := gameDBBody("2620", Date{Raw: 0x1abc, Year: 2025})

	_, err := ParseGameDBHeader(body[:30])

	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
}

func TestVerifiedDatabaseVersion(t *testing.T) {
	for v, want := range map[uint16]bool{3727: true, 3733: true, 3740: false, 0: false} {
		if got := VerifiedDatabaseVersion(v); got != want {
			t.Errorf("VerifiedDatabaseVersion(%d) = %v, want %v", v, got, want)
		}
	}
}
