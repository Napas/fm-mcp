package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"runtime"
	"testing"
)

func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

const smallAlloc = 4 << 20

func TestHugeCountsOnTinyInputsAllocateLittle(t *testing.T) {
	huge := uint32(20_000_000)
	head := binary.LittleEndian.AppendUint32(nil, huge)
	table := append([]byte{0, 0, 0, 0, 1, 0, 0, 0, 0}, append(head, 0, tagClub, 1, 2, 3)...)
	currencies := append([]byte{0, 0, 0, 0, 1, 0, 0, 0, 0}, append(head, 0, tagCurrency, 1, 2, 3)...)
	underCap := binary.LittleEndian.AppendUint32(nil, 9_000_000)
	names := append(underCap, 0, 0, 0, 0, 1, 0, 0, 0, 'a')
	fog := binary.LittleEndian.AppendUint32([]byte{1}, 1)
	fog = binary.LittleEndian.AppendUint32(fog, 7)
	fog = binary.LittleEndian.AppendUint32(fog, 0xffffffff)
	fog = append(fog, make([]byte, 40<<20)...)

	tests := map[string]func(){
		"club table":     func() { _, _, _ = readClubTable(table, huge) },
		"currency table": func() { _, _, _ = readCurrencyTable(currencies, huge) },
		"split table":    func() { _, _, _ = splitTable(append(underCap, 0, 1, 2), 0, personTrailer) },
		"name table":     func() { readNameTable(newReader(names)) },
		"fog of war":     func() { _, _ = parseFogOfWar(fog) },
	}
	for name, f := range tests {
		if n := allocated(f); n > smallAlloc {
			t.Errorf("%s: allocated %d MB for a tiny input", name, n>>20)
		}
	}
}

// lyingFrame is a zstd frame whose header claims a 3 GB content size but
// holds one 10-byte raw block.
func lyingFrame() []byte {
	b := []byte{0x28, 0xb5, 0x2f, 0xfd, 0xc0, 0x00} // magic, 8-byte content size, window descriptor
	b = binary.LittleEndian.AppendUint64(b, 3<<30)
	b = append(b, 0x51, 0x00, 0x00) // last raw block of 10 bytes
	return append(b, make([]byte, 10)...)
}

func TestReadFileIgnoresClaimedContentSize(t *testing.T) {
	frame := lyingFrame()
	dir := testDir{name: "s", files: []testFile{{name: "game_db", ext: ".dat", offset: 0, compressed: uint64(len(frame)), size: 10, time1: unsetTime}}}
	a := openSave(t, assembleSave(t, frame, dir))

	var err error
	n := allocated(func() { _, err = a.ReadFile("game_db.dat") })

	if n > smallAlloc {
		t.Errorf("allocated %d MB for an 18-byte frame", n>>20)
	}
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestNewArchiveBoundsDirectoryRead(t *testing.T) {
	// The header puts the directory at the start of a 100 MB data area.
	b := containerHeader(2, 1, "fmf.", 8, HeaderSize-offsetBase, HeaderSize-offsetBase, 3)
	b = append(b, make([]byte, 100<<20)...)

	n := allocated(func() { _, _ = NewArchive(bytes.NewReader(b), int64(len(b))) })

	if n > maxDirectorySize+smallAlloc {
		t.Errorf("read %d MB looking for the directory", n>>20)
	}
}
