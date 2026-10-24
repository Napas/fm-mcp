package savefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func zstdFrame(t testing.TB, b []byte) []byte {
	t.Helper()
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = enc.Close() }()
	return enc.EncodeAll(b, nil)
}

func innerFile(tag string, version uint16, body string) []byte {
	b := binary.LittleEndian.AppendUint16([]byte{3, 1, tag[0], tag[1], tag[2], tag[3]}, version)
	return append(b, body...)
}

// testInner is an inner file in a synthetic save. A non-zero size
// overrides the uncompressed size written to the directory.
type testInner struct {
	path string
	data []byte
	size uint64
}

// buildSave places inner files back to back.
// Paths are top-level or under "rgman/".
func buildSave(t testing.TB, files ...testInner) []byte {
	t.Helper()
	var data []byte
	root := testDir{name: "test_save"}
	sub := testDir{name: "rgman"}
	for _, in := range files {
		frame := zstdFrame(t, in.data)
		size := in.size
		if size == 0 {
			size = uint64(len(in.data))
		}
		dir, name := &root, in.path
		if rest, ok := strings.CutPrefix(name, "rgman/"); ok {
			dir, name = &sub, rest
		}
		ext := path.Ext(name)
		dir.files = append(dir.files, testFile{
			name: strings.TrimSuffix(name, ext), ext: ext,
			offset: uint64(len(data)), compressed: uint64(len(frame)), size: size, time1: unsetTime,
		})
		data = append(data, frame...)
	}
	if len(sub.files) > 0 {
		root.subdirs = []testDir{sub}
	}
	return assembleSave(t, data, root)
}

func assembleSave(t testing.TB, data []byte, dir testDir) []byte {
	t.Helper()
	const dataOff = HeaderSize - offsetBase
	out := containerHeader(2, 1, "fmf.", 8, uint64(dataOff+len(data)), dataOff, 3)
	out = append(out, data...)
	out = append(out, 2, 1, 'f', 'm', 'f', '.', 8, 0, 0) // directory header
	return append(out, zstdFrame(t, dir.encode(nil))...)
}

func openSave(t *testing.T, b []byte) *Archive {
	t.Helper()
	a, err := NewArchive(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestArchiveListsFilesAndReadsThem(t *testing.T) {
	a := openSave(t, buildSave(t,
		testInner{path: "game_info.dat", data: innerFile("tad.", 46, "26.2.0")},
		testInner{path: "tc_history_dt.cmt", data: innerFile("tmc.", 3727, "history")},
		testInner{path: "rgman/comp_208999.dat", data: innerFile("tad.", 1637, "competition")},
	))

	if got := a.Directory().Name; got != "test_save" {
		t.Errorf("Directory().Name = %q, want test_save", got)
	}
	if got := len(a.Directory().Files); got != 3 {
		t.Errorf("got %d files, want 3", got)
	}

	f, err := a.ReadFile("rgman/comp_208999.dat")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if f.Header != (FileHeader{Extension: ".dat", Version: 1637}) {
		t.Errorf("Header = %+v", f.Header)
	}
	if string(f.Body) != "competition" {
		t.Errorf("Body = %q, want %q", f.Body, "competition")
	}
}

func TestArchiveReadFileReportsMissingFile(t *testing.T) {
	a := openSave(t, buildSave(t, testInner{path: "game_info.dat", data: innerFile("tad.", 46, "x")}))

	_, err := a.ReadFile("game_db.dat")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestArchiveReadFileRejectsSizeMismatch(t *testing.T) {
	a := openSave(t, buildSave(t, testInner{path: "game_info.dat", data: innerFile("tad.", 46, "x"), size: 99}))

	_, err := a.ReadFile("game_info.dat")

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestNewArchiveRejectsEntryOutsideDataArea(t *testing.T) {
	frame := zstdFrame(t, innerFile("tad.", 46, "x"))
	dir := testDir{name: "s", files: []testFile{
		{name: "game_info", ext: ".dat", offset: 1, compressed: uint64(len(frame)), size: 9, time1: unsetTime},
	}}
	b := assembleSave(t, frame, dir)

	_, err := NewArchive(bytes.NewReader(b), int64(len(b)))

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestNewArchiveRejectsNonSave(t *testing.T) {
	b := bytes.Repeat([]byte{0}, 64)

	_, err := NewArchive(bytes.NewReader(b), int64(len(b)))

	if !errors.Is(err, ErrNotSave) {
		t.Errorf("err = %v, want ErrNotSave", err)
	}
}

func TestNewArchiveRejectsOverflowingOffsets(t *testing.T) {
	for _, dirOff := range []uint64{1<<63 - 12, 1<<64 - 1, 1 << 40} {
		b := containerHeader(2, 1, "fmf.", 8, dirOff, HeaderSize-offsetBase, 3)
		b = append(b, make([]byte, 100)...)

		_, err := NewArchive(bytes.NewReader(b), int64(len(b)))

		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("directory offset %#x: err = %v, want ErrCorrupt", dirOff, err)
		}
	}
}

func TestNewArchiveBoundsDirectoryDecompression(t *testing.T) {
	bomb := zstdFrame(t, make([]byte, 200<<20)) // a few KB that expand to 200 MB
	b := containerHeader(2, 1, "fmf.", 8, HeaderSize-offsetBase, HeaderSize-offsetBase, 3)
	b = append(b, 2, 1, 'f', 'm', 'f', '.', 8, 0, 0)
	b = append(b, bomb...)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := NewArchive(bytes.NewReader(b), int64(len(b)))
	runtime.ReadMemStats(&after)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 2*maxDirectorySize {
		t.Errorf("allocated %d MB decoding a %d-byte directory", grew>>20, len(bomb))
	}
}
