package savefile

import "fmt"

// FileHeaderSize is the size of the header at the start of every inner file.
const FileHeaderSize = 8

const innerFileKind = 3

// FileHeader is the decoded header of an inner file.
type FileHeader struct {
	Extension string // ".dat" or ".cmt"; stored reversed in the file ("tad.")
	Version   uint16 // per-file format version, e.g. 4000 for game_db.dat
}

// ParseFileHeader decodes the first FileHeaderSize bytes of an inner file.
func ParseFileHeader(b []byte) (FileHeader, error) {
	r := newReader(b)
	kind := r.u8()
	order := r.u8()
	tag := r.take(4)
	version := r.u16()
	if r.err != nil {
		return FileHeader{}, r.err
	}
	if kind != innerFileKind {
		return FileHeader{}, fmt.Errorf("%w: inner file kind %d", ErrCorrupt, kind)
	}
	if order != littleEndian {
		return FileHeader{}, fmt.Errorf("%w: byte order %d", ErrUnsupported, order)
	}
	ext := []byte{tag[3], tag[2], tag[1], tag[0]}
	return FileHeader{Extension: string(ext), Version: version}, nil
}
