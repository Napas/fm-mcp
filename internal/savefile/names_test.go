package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func appendNameTable(b []byte, names ...string) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(names))) //nolint:gosec // small test data
	for i, n := range names {
		b = binary.LittleEndian.AppendUint32(b, uint32(i)) //nolint:gosec // small test data
		b = appendString(b, n)
	}
	return b
}

func TestFindNameTablesSkipsLeadingBytes(t *testing.T) {
	body := []byte{9, 9, 9, 0, 0, 0, 0, 5}
	body = appendNameTable(body, "James", "Guglielmo", "Napas")
	body = appendNameTable(body, "Maddison", "Vicario", "Mačiulaitis")
	body = appendNameTable(body, "Quinn")
	body = append(body, 0xee)

	nt, end, err := findNameTables(body, 3)
	if err != nil {
		t.Fatalf("findNameTables: %v", err)
	}

	if nt.First[2] != "Napas" || nt.Last[2] != "Mačiulaitis" || nt.Common[0] != "Quinn" {
		t.Errorf("tables = %+v", nt)
	}
	if body[end] != 0xee {
		t.Errorf("end = %d points at %#x, want the byte after the tables", end, body[end])
	}
}

func TestFindNameTablesReportsMissingTables(t *testing.T) {
	body := appendNameTable(nil, "only", "one", "table")

	_, _, err := findNameTables(body, 3)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNameTablesResolve(t *testing.T) {
	nt := NameTables{First: []string{"James"}, Last: []string{"Maddison"}, Common: []string{"Quinn"}}

	if got := nt.first(0); got != "James" {
		t.Errorf("first(0) = %q", got)
	}
	if got := nt.last(noName); got != "" {
		t.Errorf("last(none) = %q, want empty", got)
	}
	if got := nt.common(noName); got != "" {
		t.Errorf("common(none) = %q, want empty", got)
	}
	if got := nt.common(7); got != "" {
		t.Errorf("common(out of range) = %q, want empty", got)
	}
}

func TestValidNameRef(t *testing.T) {
	for _, tc := range []struct {
		id   uint32
		want bool
	}{{0, true}, {2, true}, {3, false}, {noName, true}, {noName - 1, false}} {
		if got := validNameRef(tc.id, 3); got != tc.want {
			t.Errorf("validNameRef(%#x, 3) = %v, want %v", tc.id, got, tc.want)
		}
	}
}
