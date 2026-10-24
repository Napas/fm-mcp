package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func humansFile(count uint16, index uint32) []byte {
	b := binary.LittleEndian.AppendUint16(innerFile("tad.", 21, ""), count)
	b = binary.LittleEndian.AppendUint32(b, index)
	return append(b, 0x0c, 0, 1, 3) // rest not decoded
}

func TestReadHumanIndex(t *testing.T) {
	a := openSave(t, buildSave(t, testInner{path: "humans.dat", data: humansFile(1, 158530)}))

	got, err := ReadHumanIndex(a)
	if err != nil || got != 158530 {
		t.Errorf("ReadHumanIndex = %d, %v; want 158530", got, err)
	}
}

func TestReadHumanIndexRejectsOtherHumanCounts(t *testing.T) {
	noHumans := binary.LittleEndian.AppendUint16(innerFile("tad.", 21, ""), 0)
	for count, data := range map[uint16][]byte{0: noHumans, 2: humansFile(2, 7)} {
		a := openSave(t, buildSave(t, testInner{path: "humans.dat", data: data}))
		if _, err := ReadHumanIndex(a); !errors.Is(err, ErrUnsupported) {
			t.Errorf("count %d: err = %v, want ErrUnsupported", count, err)
		}
	}
}
