package savefile

import (
	"encoding/binary"
	"fmt"
)

// Work permit records (TRANSFER_MANAGER +0x720; writer 0x2e0f8e0, 27.0.0):
// u8 version, i32 person, u16 ×2 (unknown), u16 year, u16 nation, date,
// u8 flags (bit 0: set exactly when the date is; probably an ESC place).
const (
	permitSize    = 18
	permitVersion = 2

	permitPersonOff = 1
	permitYearOff   = 9
	permitNationOff = 11
	permitDateOff   = 13
	permitFlagsOff  = 17
)

// WorkPermit is a work permit a nation granted a person. The game matches it
// to a club by the club's nation; England, Scotland, Wales and Northern
// Ireland count as one (0x2ddd12c).
type WorkPermit struct {
	Person uint32 // person index
	Nation int64  // NATION index of the granting nation
	Year   uint16 // probably the year it was granted (inference)
	Date   Date   // set exactly when ESC is; see docs/format/transfers.md
	// ESC (flag bit 0): probably an ESC work permit place (inference: one
	// player checked; England or Wales only).
	ESC bool
}

// ReadWorkPermits decodes transfer_man.dat's work permits. nations is the
// save's NATION count.
func ReadWorkPermits(a *Archive, nations uint32) ([]WorkPermit, error) {
	f, err := a.ReadFile("transfer_man.dat")
	if err != nil {
		return nil, err
	}
	ps, err := parseWorkPermits(f.Body, nations)
	if err != nil {
		return nil, fmt.Errorf("transfer_man.dat work permits: %w", err)
	}
	return ps, nil
}

// parseWorkPermits finds the list by shape, as the bytes before it are not
// decoded: a u32 count, then that many valid records sorted by person. The
// body must hold exactly one. Records checked past each candidate's first
// are capped at what the body can hold, keeping the scan linear.
func parseWorkPermits(body []byte, nations uint32) ([]WorkPermit, error) {
	limit := len(body) / permitSize
	budget := limit
	start, count := -1, 0
	for p := 0; p+4 <= len(body); p++ {
		n := int(binary.LittleEndian.Uint32(body[p:]))
		if n == 0 || n > (len(body)-p-4)/permitSize {
			continue
		}
		checked, ok := permitListAt(body[p+4:], n, nations)
		if budget -= checked; budget < 0 {
			return nil, fmt.Errorf("%w: more than %d work permit records checked in %d bytes", ErrCorrupt, limit, len(body))
		}
		if !ok {
			continue
		}
		if start >= 0 {
			return nil, fmt.Errorf("%w: work permit lists at bytes %d and %d", ErrCorrupt, start-4, p)
		}
		start, count = p+4, n
	}
	if start < 0 {
		return nil, fmt.Errorf("%w: no work permit list", ErrNotFound)
	}
	out := make([]WorkPermit, count)
	for i := range out {
		out[i] = permitAt(body[start+i*permitSize:])
	}
	return out, nil
}

// permitListAt reports whether b starts with n permit records sorted by
// person, and how many records after the first it checked.
func permitListAt(b []byte, n int, nations uint32) (int, bool) {
	prev := int64(-1)
	for i := range n {
		r := b[i*permitSize:]
		if !permitOK(r, nations) {
			return i, false
		}
		person := int64(binary.LittleEndian.Uint32(r[permitPersonOff:]))
		if person < prev {
			return i, false
		}
		prev = person
	}
	return n - 1, true
}

// permitOK checks the version, year, date, nation range, and that flag bit 0
// is set exactly when the date is.
func permitOK(r []byte, nations uint32) bool {
	le := binary.LittleEndian
	if r[0] != permitVersion || r[permitFlagsOff] > 1 || uint32(le.Uint16(r[permitNationOff:])) >= nations {
		return false
	}
	if year := le.Uint16(r[permitYearOff:]); year < minContractYear || year > maxContractYear {
		return false
	}
	d := dateAt(r, permitDateOff)
	return d.plausible(minContractYear, maxContractYear) && permitESC(r) == (d.Year != UnsetYear)
}

// permitESC reports flag bit 0; permitOK rejects other bits.
func permitESC(r []byte) bool { return r[permitFlagsOff] == 1 }

func permitAt(r []byte) WorkPermit {
	le := binary.LittleEndian
	return WorkPermit{
		Person: le.Uint32(r[permitPersonOff:]),
		Nation: int64(le.Uint16(r[permitNationOff:])),
		Year:   le.Uint16(r[permitYearOff:]),
		Date:   dateAt(r, permitDateOff),
		ESC:    permitESC(r),
	}
}
