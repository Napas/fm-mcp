package savefile

import (
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"
)

var unsetPermitDate = Date{Raw: 1, Year: UnsetYear}

// permitRecord is an 18-byte work permit record; a set date gets flag 1.
func permitRecord(person uint32, counts [2]uint16, year, nation uint16, d Date) []byte {
	b := []byte{permitVersion}
	b = binary.LittleEndian.AppendUint32(b, person)
	b = binary.LittleEndian.AppendUint16(b, counts[0])
	b = binary.LittleEndian.AppendUint16(b, counts[1])
	b = binary.LittleEndian.AppendUint16(b, year)
	b = binary.LittleEndian.AppendUint16(b, nation)
	b = binary.LittleEndian.AppendUint16(b, d.Raw)
	b = binary.LittleEndian.AppendUint16(b, d.Year)
	if d.Year != UnsetYear {
		return append(b, 1)
	}
	return append(b, 0)
}

// permitList is the u32 count and the records.
func permitList(recs ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(recs))) //nolint:gosec // test sizes
	for _, r := range recs {
		b = append(b, r...)
	}
	return b
}

var (
	// Sallinger (Austria, Hibernian) and Sichenje (Kenya, Charlton) in test_save_27.
	sallingerPermit = permitRecord(1442, [2]uint16{}, 2025, 167, unsetPermitDate)
	sichenjePermit  = permitRecord(8286, [2]uint16{}, 2026, 139, Date{Raw: 33, Year: 2026})
	// Jeannot Esua's Irish permit in the homegrown save has a count of 19.
	esuaPermit = permitRecord(9478, [2]uint16{19, 0}, 2025, 163, unsetPermitDate)
)

func TestParseWorkPermits(t *testing.T) {
	junk := []byte{0xff, 0xff, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00}
	after := []byte{0xa2, 0x04, 0x00, 0x00, 0x03, 0xb0, 0x28, 0x00, 0x00}
	body := append(append(append([]byte{}, junk...), permitList(sallingerPermit, sichenjePermit, sichenjePermit, esuaPermit)...), after...)
	got, err := parseWorkPermits(body, 200)
	if err != nil {
		t.Fatal(err)
	}
	want := []WorkPermit{
		{Person: 1442, Nation: 167, Year: 2025, Date: unsetPermitDate},
		{Person: 8286, Nation: 139, Year: 2026, Date: Date{Raw: 33, Year: 2026}},
		{Person: 8286, Nation: 139, Year: 2026, Date: Date{Raw: 33, Year: 2026}},
		{Person: 9478, Nation: 163, Year: 2025, Date: unsetPermitDate},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// withByte is a copy of r with byte off set to v.
func withByte(r []byte, off int, v byte) []byte {
	c := append([]byte{}, r...)
	c[off] = v
	return c
}

func TestParseWorkPermitsRejects(t *testing.T) {
	for name, tt := range map[string]struct {
		body []byte
		want error
	}{
		"empty":                {[]byte{}, ErrNotFound},
		"no list":              {make([]byte, 64), ErrNotFound},
		"unsorted persons":     {permitList(sichenjePermit, sallingerPermit), ErrNotFound},
		"nation out of range":  {permitList(permitRecord(1442, [2]uint16{}, 2025, 200, unsetPermitDate)), ErrNotFound},
		"date without flag":    {permitList(withByte(sichenjePermit, permitFlagsOff, 0)), ErrNotFound},
		"flag without date":    {permitList(withByte(sallingerPermit, permitFlagsOff, 1)), ErrNotFound},
		"flag other than 0, 1": {permitList(withByte(sallingerPermit, permitFlagsOff, 2)), ErrNotFound},
		"implausible year":     {permitList(permitRecord(1442, [2]uint16{}, 1800, 167, unsetPermitDate)), ErrNotFound},
		"implausible date":     {permitList(permitRecord(1442, [2]uint16{}, 2025, 167, Date{Raw: 400, Year: 2026})), ErrNotFound},
		"other record version": {permitList(withByte(sallingerPermit, 0, 3)), ErrNotFound},
		"truncated list":       {permitList(sallingerPermit, sichenjePermit)[:4+permitSize+5], ErrNotFound},
		"two lists":            {append(permitList(sallingerPermit), append([]byte{0xee}, permitList(sichenjePermit)...)...), ErrCorrupt},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := parseWorkPermits(tt.body, 200); !errors.Is(err, tt.want) {
				t.Errorf("got %+v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

// Overlapping candidate lists cannot make the scan check more records than
// its budget.
func TestParseWorkPermitsBudget(t *testing.T) {
	// A record's last 4 bytes read as a count of 486 400 (date year 1900 in
	// bytes 1-2), so every record starts a candidate list covering the
	// records after it. An unsorted record near the end fails each late.
	const n = 500_000
	rec := permitRecord(1, [2]uint16{}, 2025, 1, unsetPermitDate)
	body := make([]byte, 0, n*permitSize)
	for i := range n {
		if i == 486_400 {
			body = append(body, permitRecord(0, [2]uint16{}, 2025, 1, unsetPermitDate)...)
			continue
		}
		body = append(body, rec...)
	}
	_, err := parseWorkPermits(body, 200)
	if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "records checked") {
		t.Errorf("err = %v, want ErrCorrupt over the budget", err)
	}
}
