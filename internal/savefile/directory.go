package savefile

import (
	"errors"
	"fmt"
	"path"
	"time"
)

// ErrCorrupt means the bytes have the right shape but inconsistent content.
var ErrCorrupt = errors.New("savefile: corrupt data")

const (
	// unsetTime is 0001-01-01 in Unix seconds; the game writes it for
	// files without a timestamp.
	unsetTime           int64 = -62135596800
	maxDirectoryDepth         = 16
	maxDirectoryEntries       = 1 << 20
)

// Directory is the decoded file tree stored at the end of a save.
type Directory struct {
	Name  string  // root name, e.g. the save's file name
	Files []Entry // every file in the tree, depth-first
}

// Entry describes one inner file. Offset is relative to Header.DataOffset.
type Entry struct {
	Path           string // slash-separated, relative to the root, e.g. "rgman/comp_1.dat"
	Offset         uint64
	CompressedSize uint64
	Size           uint64
	ModTime        time.Time // zero when the game left it unset
}

// ParseDirectory decodes the decompressed directory tree. Each entry holds
// two equal timestamps; only the first is kept.
func ParseDirectory(b []byte) (Directory, error) {
	r := newReader(b)
	name := r.str()
	var files []Entry
	if err := readDirectoryNode(r, "", 0, &files); err != nil {
		return Directory{}, err
	}
	if r.remaining() != 0 {
		return Directory{}, fmt.Errorf("%w: %d trailing bytes after directory", ErrCorrupt, r.remaining())
	}
	return Directory{Name: name, Files: files}, nil
}

// readDirectoryNode reads a node's files and subdirectories. Its
// name is already consumed.
func readDirectoryNode(r *reader, dir string, depth int, files *[]Entry) error {
	if depth > maxDirectoryDepth {
		return fmt.Errorf("%w: directory nesting deeper than %d", ErrCorrupt, maxDirectoryDepth)
	}
	n := r.u32()
	if n > maxDirectoryEntries || len(*files)+int(n) > maxDirectoryEntries {
		return fmt.Errorf("%w: %d files in %q", ErrCorrupt, n, dir)
	}
	for range n {
		e := Entry{Path: path.Join(dir, r.str()+r.str())}
		e.Offset = r.u64()
		e.CompressedSize = r.u64()
		e.Size = r.u64()
		if t := r.i64(); t != unsetTime {
			e.ModTime = time.Unix(t, 0).UTC()
		}
		r.i64()
		*files = append(*files, e)
	}
	subdirs := r.u32()
	if r.err != nil {
		return r.err
	}
	for range subdirs {
		name := r.str()
		if err := readDirectoryNode(r, path.Join(dir, name), depth+1, files); err != nil {
			return err
		}
	}
	return r.err
}
