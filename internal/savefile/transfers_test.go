package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

// marcoTransfer is the start of Unai Marco's arranged move to Bari, id 10183
// (KŽ save 2036-01-16, bytes).
var marcoTransfer = []byte{
	0x02, 0x13, 0x00, 0x81, 0xcf, 0x02, 0x00, // class 2, version 0x13, person 184193
	0x00, 0x57, 0x03, 0x00, 0x00, // team 855
	0x00, 0x57, 0x03, 0x00, 0x00, // team 855
	0x00, 0x00, 0x00, 0x00, // +0x58
	0xc7, 0x27, 0x00, 0x00, // id 10183
	0xce, 0x72, 0x05, 0x00, // +0x60
	0x13,                   // status 19
	0xff, 0xff, 0xff, 0xff, // +0x64
}

// transferRecord pads head to a 64-byte record with dates at offsets 56 and 60.
func transferRecord(head []byte, d56, d60 Date) []byte {
	rec := make([]byte, transferMinSize)
	copy(rec, head)
	binary.LittleEndian.PutUint16(rec[56:], d56.Raw)
	binary.LittleEndian.PutUint16(rec[58:], d56.Year)
	binary.LittleEndian.PutUint16(rec[60:], d60.Raw)
	binary.LittleEndian.PutUint16(rec[62:], d60.Year)
	return rec
}

// transferHead is a record start of the given class, person, team, id and status.
func transferHead(class uint8, person uint32, team int32, id uint32, status uint8) []byte {
	b := []byte{class, 0x13, 0x00}
	b = binary.LittleEndian.AppendUint32(b, person)
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint32(b, uint32(team)) //nolint:gosec // test value
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint32(b, uint32(team)) //nolint:gosec // test value
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = binary.LittleEndian.AppendUint32(b, id)
	b = binary.LittleEndian.AppendUint32(b, 0)
	return append(b, status)
}

func transferBody(records ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{1}, uint32(len(records))) //nolint:gosec // test sizes
	for _, r := range records {
		b = append(b, r...)
	}
	return b
}

var (
	feb4  = Date{Raw: 0x7223, Year: 2036} // 2036-02-04, upper bits as saved
	jul1  = Date{Raw: 0xb7, Year: 2036}   // 2036-07-01
	unset = Date{Raw: 1, Year: 1900}
)

func TestParseTransfers(t *testing.T) {
	marco := Transfer{ID: 10183, Person: 184193, Class: 2, Team: 855, Status: 19, Date: jul1}
	guingamp := Transfer{ID: 10810, Person: 184193, Class: 3, Team: 649, Status: 15, Date: jul1}
	tests := []struct {
		name string
		body []byte
		want []Transfer
	}{
		{
			name: "two records",
			body: transferBody(transferRecord(marcoTransfer, feb4, jul1), transferRecord(transferHead(3, 184193, 649, 10810, 15), feb4, jul1)),
			want: []Transfer{marco, guingamp},
		},
		{
			name: "unset first date",
			body: transferBody(transferRecord(marcoTransfer, unset, jul1)),
			want: []Transfer{{ID: 10183, Person: 184193, Class: 2, Team: 855, Status: 19, Date: jul1}},
		},
		{
			name: "a later copy of an id is dropped",
			body: transferBody(transferRecord(marcoTransfer, feb4, jul1), transferRecord(transferHead(2, 184193, 855, 10183, 0), feb4, jul1)),
			want: []Transfer{marco},
		},
		{
			name: "other classes are skipped",
			body: transferBody(transferRecord(transferHead(4, 1, 2, 3, 19), feb4, jul1), transferRecord(transferHead(10, 1, 2, 4, 19), feb4, jul1)),
		},
		{
			name: "bad dates",
			body: transferBody(transferRecord(marcoTransfer, Date{Raw: 0, Year: 2036}, jul1), transferRecord(marcoTransfer, feb4, Date{Raw: 183, Year: 1899})),
		},
		{
			name: "ref marker not zero",
			body: transferBody(func() []byte {
				rec := transferRecord(marcoTransfer, feb4, jul1)
				rec[7] = 1
				return rec
			}()),
		},
		{
			name: "record cut short",
			body: transferBody(transferRecord(marcoTransfer, feb4, jul1)[:transferMinSize-1]),
		},
		{
			name: "junk",
			body: transferBody(bytes.Repeat([]byte{0x02, 0x13, 0x00, 0xff, 0x7f, 0x13}, 50)),
		},
		{
			name: "header only",
			body: transferBody(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTransfers(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 || len(tt.want) != 0 {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("got %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

func TestParseTransfersRejects(t *testing.T) {
	// Every 16 bytes a candidate: class 2, version 0x13, zero ref markers at
	// 7 and 12, a date at 8 (day 1 of 2028) and one at 12 (day 256 of 2028).
	period := []byte{0x02, 0x13, 0x00, 0, 0, 0, 0, 0x00, 0x01, 0x00, 0xec, 0x07, 0x00, 0x01, 0xec, 0x07}
	tests := []struct {
		name string
		body []byte
	}{
		{"shorter than its header", []byte{1, 0, 0, 0}},
		{"empty", nil},
		{"more candidates than the body can hold", transferBody(bytes.Repeat(period, 200))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := parseTransfers(tt.body); !errors.Is(err, ErrCorrupt) {
				t.Errorf("got %d records, %v; want ErrCorrupt", len(got), err)
			}
		})
	}
}

// TestRealSave16JanTransfers checks the TRANSFER records of the KŽ save of
// 2036-01-16, named by FM_REAL_SAVE_16JAN, against the player-interest
// research (docs/superpowers/specs/2026-10-23-player-interest).
func TestRealSave16JanTransfers(t *testing.T) {
	a, db := openRealSave(t, "FM_REAL_SAVE_16JAN", time.Date(2036, 1, 16, 0, 0, 0, 0, time.UTC))
	ts, err := ReadTransfers(a)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[uint32]Transfer, len(ts))
	statuses := make(map[uint8]int)
	for _, tr := range ts {
		byID[tr.ID] = tr
		statuses[tr.Status]++
	}

	if got, want := byID[10183], (Transfer{ID: 10183, Person: 184193, Class: 2, Team: 855, Status: TransferArranged, Date: Date{Raw: 0xb7, Year: 2036}}); got != want {
		t.Errorf("id 10183 (Marco to Bari) = %+v, want %+v", got, want)
	}
	if got := byID[10810]; got.Team != 649 || got.Status != 15 {
		t.Errorf("id 10810 (Marco to Guingamp) = %+v, want team 649, status 15", got)
	}

	// arranged returns the player's linked records of status 19: same person
	// and an id in his TransferIDs.
	arranged := func(p *Player) []Transfer {
		var out []Transfer
		for _, id := range p.TransferIDs {
			if tr, ok := byID[id]; ok && tr.Person == p.Index && tr.Status == TransferArranged {
				out = append(out, tr)
			}
		}
		return out
	}
	linked := 0
	for i := range db.Players {
		linked += len(arranged(&db.Players[i]))
	}
	if linked < 410 || linked > 425 {
		t.Errorf("%d linked status-19 records, want 410..425 (research: 418)", linked)
	}
	for _, uid := range []uint32{
		2002115333, // Eneko Gamboa
		2002125884, // Marko Tomašević
		2002123791, // Aleksei Korshun
	} {
		i := slices.IndexFunc(db.Players, func(p Player) bool { return p.UID == uid })
		if i < 0 {
			t.Fatalf("no player with uid %d", uid)
		}
		if got := arranged(&db.Players[i]); len(got) != 0 {
			t.Errorf("uid %d: arranged transfers %+v, want none", uid, got)
		}
	}
	t.Logf("%d records, %d linked arranged; by status %v", len(ts), linked, statuses)
}
