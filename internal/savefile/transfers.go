package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	transferHeaderSize = 5    // u8 1, u32 count
	transferVersion    = 0x13 // u16 after the class byte
	transferMinSize    = 64   // through the date at offset 60
	// transferBytesPerRecord caps the candidates at the body length / 30.
	// Records are at least 117 bytes (writer 0x3a53bb4).
	transferBytesPerRecord = 30

	transferPersonOff = 3
	transferRef1Off   = 7 // u8 0 before the team
	transferTeamOff   = 8
	transferRef2Off   = 12 // u8 0 before the second team ref
	transferIDOff     = 21
	transferStatusOff = 29
	transferDate1Off  = 56
	transferDateOff   = 60
)

// TransferArranged is the status of an arranged move: the game's
// "Transfer Arranged" check 0x2f54e10 tests it.
const TransferArranged uint8 = 19

// Transfer is a TRANSFER record (layout in docs/format.md).
type Transfer struct {
	ID     uint32 // offset 21 (+0x5c): the value in Player.TransferIDs
	Person uint32 // offset 3, person index
	Class  uint8  // offset 0: 2 or 3
	Team   int32  // offset 8: the team the move is to
	Status uint8  // offset 29 (+0x8d): TransferArranged = arranged
	Date   Date   // offset 60 (+0x7c): the move's date
}

// ReadTransfers decodes the TRANSFER records of the save's transfer_man.dat.
func ReadTransfers(a *Archive) ([]Transfer, error) {
	f, err := a.ReadFile("transfer_man.dat")
	if err != nil {
		return nil, err
	}
	ts, err := parseTransfers(f.Body)
	if err != nil {
		return nil, fmt.Errorf("transfer_man.dat: %w", err)
	}
	return ts, nil
}

// parseTransfers finds the TRANSFER records of class 2 and 3 in the body of
// transfer_man.dat by their checked start, since the rest of the file is not
// decoded. A record's id is kept once, from its first match: the table lists
// ids in ascending order and later copies (status 0 or 3) follow it.
func parseTransfers(body []byte) ([]Transfer, error) {
	if len(body) < transferHeaderSize {
		return nil, fmt.Errorf("%w: body of %d bytes", ErrCorrupt, len(body))
	}
	limit := len(body) / transferBytesPerRecord
	var out []Transfer
	seen := make(map[uint32]struct{})
	candidates := 0
	version := []byte{transferVersion, 0}
	for i := transferHeaderSize + 1; i < len(body); {
		k := bytes.Index(body[i:], version)
		if k < 0 {
			break
		}
		p := i + k - 1
		i += k + 1
		t, ok := transferAt(body, p)
		if !ok {
			continue
		}
		if candidates++; candidates > limit {
			return nil, fmt.Errorf("%w: more than %d transfer records in %d bytes", ErrCorrupt, limit, len(body))
		}
		if _, dup := seen[t.ID]; dup {
			continue
		}
		seen[t.ID] = struct{}{}
		out = append(out, t)
	}
	return out, nil
}

// transferAt reads the record starting at p when its start checks out: class
// 2 or 3, the version, zero ref markers and plausible dates at 56 and 60.
func transferAt(b []byte, p int) (Transfer, bool) {
	if p+transferMinSize > len(b) || (b[p] != 2 && b[p] != 3) || b[p+transferRef1Off] != 0 || b[p+transferRef2Off] != 0 {
		return Transfer{}, false
	}
	if !validDates(b, p+transferDate1Off, p+transferDateOff) {
		return Transfer{}, false
	}
	le := binary.LittleEndian
	return Transfer{
		ID:     le.Uint32(b[p+transferIDOff:]),
		Person: le.Uint32(b[p+transferPersonOff:]),
		Class:  b[p],
		Team:   int32(le.Uint32(b[p+transferTeamOff:])), //nolint:gosec // signed field
		Status: b[p+transferStatusOff],
		Date:   dateAt(b, p+transferDateOff),
	}, true
}
