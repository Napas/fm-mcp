package savefile

import "encoding/binary"

// Current injuries are a list "u8 n + n items" in the player part (written
// by 0x2f9c868). Each item is:
//
//	u8 2, u16 injury index, u8 m, m × 26-byte match record,
//	date start, u16 days min, u16 days max, u8, u16, i32, u8, i32 team index,
//	u8, i32 (-1 or 0), 3 × u8, u16 0xffff
//
// A match record (u8 1, a date, rest not decoded) is carried by knocks picked
// up in a match. The list's position is not decoded, so it is found by
// validating items. Every item matches an injury_manager.dat entry with the
// same return window (8 603 items in a 26.3.2 save, 1 321 in a 26.2.0 save).
const (
	// Item head: type, index, match record count.
	injuryItemType      = 2
	injuryIDOff         = 1
	injuryMatchCountOff = 3
	injuryHeadSize      = 4

	matchRecordType = 1
	matchRecordSize = 26
	maxMatchRecords = 4

	// Item tail, after the match records.
	injuryMinOff    = 4
	injuryMaxOff    = 6
	injuryRefOff    = 21
	injuryEndOff    = 28
	injuryTailSize  = 30
	injuryUnsetWord = 0xffff

	maxInjuries   = 9
	maxInjuryDays = 3000
	minInjuryYear = 1990
	maxInjuryYear = 2200
)

// Injury is a current injury.
type Injury struct {
	ID      uint16 // index into the save's INJURY table, which is in ascending uid order
	Start   Date
	DaysMin uint16 // expected return: Start + DaysMin … Start + DaysMax
	DaysMax uint16
}

func findInjuries(b []byte, from, to int) []Injury {
	for p := from + 1; p < to; p++ {
		n := int(b[p-1])
		if b[p] != injuryItemType || n < 1 || n > maxInjuries {
			continue
		}
		if out, ok := readInjuries(b, p, n, to); ok {
			return out
		}
	}
	return nil
}

func readInjuries(b []byte, p, n, to int) ([]Injury, bool) {
	out := make([]Injury, 0, n)
	for range n {
		if p+injuryHeadSize > to || b[p] != injuryItemType {
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
		if p+injuryTailSize > to || binary.LittleEndian.Uint16(b[p+injuryEndOff:]) != injuryUnsetWord {
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
		p += injuryTailSize
	}
	return out, true
}
