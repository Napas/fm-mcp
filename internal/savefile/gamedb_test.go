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

// gameDBBody encodes a game_db.dat header; from version 3754 it has a u64
// timestamp after the u32 at +0x7dc.
func gameDBBody(version uint16, label string, gameDate Date) []byte {
	b := binary.LittleEndian.AppendUint16(nil, version)
	b = binary.LittleEndian.AppendUint16(b, 7)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = binary.LittleEndian.AppendUint32(b, 8)
	b = binary.LittleEndian.AppendUint16(b, version)
	b = binary.LittleEndian.AppendUint16(b, 7)
	b = appendString(b, label)
	b = append(b, 0x2c, 0x36, 0xea, 0x07)
	b = binary.LittleEndian.AppendUint32(b, 485714)
	if version >= 3754 {
		b = binary.LittleEndian.AppendUint64(b, 63927573184000)
	}
	b = binary.LittleEndian.AppendUint16(b, 2025)
	b = binary.LittleEndian.AppendUint16(b, 0x21a0)
	b = appendDate(b, gameDate)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = append(b, 1, 1)
	b = binary.LittleEndian.AppendUint32(b, 722795)
	return append(b, 0xee) // first byte of the next section
}

func TestParseGameDBHeaderHandlesDatabaseVersions(t *testing.T) {
	for version, wantTimestamp := range map[uint16]uint64{3727: 0, 3733: 0, 3754: 63927573184000} {
		body := gameDBBody(version, "2620", Date{Raw: 0x1abc, Year: 2025})

		h, err := ParseGameDBHeader(body)
		if err != nil {
			t.Fatalf("version %d: ParseGameDBHeader: %v", version, err)
		}

		if h.DatabaseVersion != version || h.OriginalDatabaseVersion != version {
			t.Errorf("version %d: versions = %d/%d", version, h.DatabaseVersion, h.OriginalDatabaseVersion)
		}
		if h.Label != "2620" {
			t.Errorf("version %d: Label = %q, want 2620", version, h.Label)
		}
		if h.Timestamp != wantTimestamp {
			t.Errorf("version %d: Timestamp = %d, want %d", version, h.Timestamp, wantTimestamp)
		}
		if h.GameDate != (Date{Raw: 0x1abc, Year: 2025}) || h.Unknown808 != 722795 {
			t.Errorf("version %d: GameDate = %+v, Unknown808 = %d", version, h.GameDate, h.Unknown808)
		}
		if h.Size != len(body)-1 || body[h.Size] != 0xee {
			t.Errorf("version %d: Size = %d, want %d", version, h.Size, len(body)-1)
		}
	}
}

func TestParseGameDBHeaderRejectsTruncatedBody(t *testing.T) {
	// 3754 is cut inside its u64 timestamp (body bytes 30-37).
	for version, cut := range map[uint16]int{3727: 30, 3754: 34} {
		body := gameDBBody(version, "2620", Date{Raw: 0x1abc, Year: 2025})

		_, err := ParseGameDBHeader(body[:cut])

		if !errors.Is(err, ErrTruncated) {
			t.Errorf("version %d: err = %v, want ErrTruncated", version, err)
		}
	}
}

func TestVerifiedDatabaseVersion(t *testing.T) {
	for v, want := range map[uint16]bool{3727: true, 3733: true, 3740: false, 3754: false, 0: false} {
		if got := VerifiedDatabaseVersion(v); got != want {
			t.Errorf("VerifiedDatabaseVersion(%d) = %v, want %v", v, got, want)
		}
	}
}
