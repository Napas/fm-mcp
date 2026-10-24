package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

type testFile struct {
	name, ext                string
	offset, compressed, size uint64
	time1                    int64
}

type testDir struct {
	name    string
	files   []testFile
	subdirs []testDir
}

func appendString(b []byte, s string) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(s))) //nolint:gosec // test strings are short
	return append(b, s...)
}

func (d testDir) encode(b []byte) []byte {
	b = appendString(b, d.name)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(d.files))) //nolint:gosec // small test data
	for _, f := range d.files {
		b = appendString(b, f.name)
		b = appendString(b, f.ext)
		b = binary.LittleEndian.AppendUint64(b, f.offset)
		b = binary.LittleEndian.AppendUint64(b, f.compressed)
		b = binary.LittleEndian.AppendUint64(b, f.size)
		b = binary.LittleEndian.AppendUint64(b, uint64(f.time1)) //nolint:gosec // two's complement intended
		b = binary.LittleEndian.AppendUint64(b, uint64(f.time1)) //nolint:gosec // two's complement intended
	}
	b = binary.LittleEndian.AppendUint32(b, uint32(len(d.subdirs))) //nolint:gosec // small test data
	for _, s := range d.subdirs {
		b = s.encode(b)
	}
	return b
}

func TestParseDirectoryFlattensTreeIntoPaths(t *testing.T) {
	tree := testDir{
		name: "test_save",
		files: []testFile{
			{name: "game_info", ext: ".dat", offset: 0, compressed: 2369, size: 6209, time1: unsetTime},
			{name: "tc_history_dt", ext: ".cmt", offset: 2369, compressed: 274, size: 1142, time1: 1773345220},
		},
		subdirs: []testDir{{
			name:  "rgman",
			files: []testFile{{name: "comp_208999", ext: ".dat", offset: 2643, compressed: 1024, size: 2519, time1: unsetTime}},
		}},
	}

	dir, err := ParseDirectory(tree.encode(nil))
	if err != nil {
		t.Fatalf("ParseDirectory: %v", err)
	}

	if dir.Name != "test_save" {
		t.Errorf("Name = %q, want test_save", dir.Name)
	}
	want := []Entry{
		{Path: "game_info.dat", Offset: 0, CompressedSize: 2369, Size: 6209},
		{Path: "tc_history_dt.cmt", Offset: 2369, CompressedSize: 274, Size: 1142, ModTime: time.Unix(1773345220, 0).UTC()},
		{Path: "rgman/comp_208999.dat", Offset: 2643, CompressedSize: 1024, Size: 2519},
	}
	if len(dir.Files) != len(want) {
		t.Fatalf("got %d files, want %d: %+v", len(dir.Files), len(want), dir.Files)
	}
	for i := range want {
		if dir.Files[i] != want[i] {
			t.Errorf("file %d = %+v, want %+v", i, dir.Files[i], want[i])
		}
	}
}

func TestParseDirectoryRejectsTrailingBytes(t *testing.T) {
	b := testDir{name: "x"}.encode(nil)

	_, err := ParseDirectory(append(b, 0))

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseDirectoryRejectsTruncatedInput(t *testing.T) {
	b := testDir{name: "x", files: []testFile{{name: "a", ext: ".dat"}}}.encode(nil)

	_, err := ParseDirectory(b[:len(b)-3])

	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
}

func TestParseDirectoryRejectsExcessiveNesting(t *testing.T) {
	d := testDir{name: "leaf"}
	for range maxDirectoryDepth + 1 {
		d = testDir{name: "d", subdirs: []testDir{d}}
	}

	_, err := ParseDirectory(d.encode(nil))

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}

func TestParseDirectoryRejectsHugeFileCount(t *testing.T) {
	b := appendString(nil, "x")
	b = binary.LittleEndian.AppendUint32(b, maxDirectoryEntries+1)

	_, err := ParseDirectory(b)

	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v, want ErrCorrupt", err)
	}
}
