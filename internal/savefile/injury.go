package savefile

import "encoding/binary"

// Current injuries are a list "u8 n + n items" in the player part (written
// by 0x2f9c868). Each item is:
//
//	u8 type, u16 injury index, u8 m, m × 26-byte match record,
//	date start, u16 days min, u16 days max, u8, u16, i32, u8, i32 team index,
//	u8, i32 (-1 or 0), 3 × u8, end
//
// The end is u16 0xffff for type 2 (below 27.0.0) and u8 0xff for type 3.
//
// A match record (u8 1, a date, rest not decoded) is carried by knocks picked
// up in a match. The list's position is not decoded, so it is found by
// validating items. Every item matches an injury_manager.dat entry with the
// same return window (8 603 items in a 26.3.2 save, 1 321 in a 26.2.0 save,
// 1 632 in a 27.0.0 save).
const (
	// Item head: type, index, match record count.
	injuryIDOff         = 1
	injuryMatchCountOff = 3
	injuryHeadSize      = 4

	matchRecordType = 1
	matchRecordSize = 26
	maxMatchRecords = 4

	// Item tail, after the match records.
	injuryMinOff = 4
	injuryMaxOff = 6
	injuryRefOff = 21
	injuryEndOff = 28 // 0xff bytes up to the tail size

	maxInjuries   = 9
	maxInjuryDays = 3000
	minInjuryYear = 1990
	maxInjuryYear = 2200
)

// injuryItemFormat is an item's type byte and tail size.
type injuryItemFormat struct {
	typ  byte
	tail int
}

// injuryFormatGate is the first database version seen with type 3 items; the
// game's own gate may be lower.
const injuryFormatGate = dbVersion27_0

// injuryFormat returns the item format for dataVersion.
func injuryFormat(dataVersion uint16) injuryItemFormat {
	if dataVersion >= injuryFormatGate {
		return injuryItemFormat{typ: 3, tail: 29}
	}
	return injuryItemFormat{typ: 2, tail: 30}
}

// Injury is a current injury.
type Injury struct {
	ID      uint16 // index into the save's INJURY table, which is in ascending uid order
	Start   Date
	DaysMin uint16 // expected return: Start + DaysMin … Start + DaysMax
	DaysMax uint16
}

func findInjuries(b []byte, from, to int, f injuryItemFormat) []Injury {
	for p := from + 1; p < to; p++ {
		n := int(b[p-1])
		if b[p] != f.typ || n < 1 || n > maxInjuries {
			continue
		}
		if out, ok := readInjuries(b, p, n, to, f); ok {
			return out
		}
	}
	return nil
}

func readInjuries(b []byte, p, n, to int, f injuryItemFormat) ([]Injury, bool) {
	out := make([]Injury, 0, n)
	for range n {
		if p+injuryHeadSize > to || b[p] != f.typ {
			return nil, false
		}
		id := binary.LittleEndian.Uint16(b[p+injuryIDOff:])
		matches := int(b[p+injuryMatchCountOff])
		if matches > maxMatchRecords {
			return nil, false
		}
		p += injuryHeadSize
		for range matches {
			if p+matchRecordSize > to || b[p] != matchRecordType || !dateAt(b, p+1).plausible(minInjuryYear, maxInjuryYear) {
				return nil, false
			}
			p += matchRecordSize
		}
		if p+f.tail > to || !allFF(b[p+injuryEndOff:p+f.tail]) {
			return nil, false
		}
		if ref := binary.LittleEndian.Uint32(b[p+injuryRefOff:]); ref != ^uint32(0) && ref != 0 {
			return nil, false
		}
		in := Injury{
			ID:      id,
			Start:   dateAt(b, p),
			DaysMin: binary.LittleEndian.Uint16(b[p+injuryMinOff:]),
			DaysMax: binary.LittleEndian.Uint16(b[p+injuryMaxOff:]),
		}
		if !in.Start.plausible(minInjuryYear, maxInjuryYear) || in.DaysMin > in.DaysMax || in.DaysMax > maxInjuryDays {
			return nil, false
		}
		out = append(out, in)
		p += f.tail
	}
	return out, true
}

func allFF(b []byte) bool {
	for _, c := range b {
		if c != 0xff {
			return false
		}
	}
	return true
}
