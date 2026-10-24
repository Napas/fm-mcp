package savefile

import (
	"errors"
	"fmt"
)

var (
	// ErrNotSave means the bytes are not an FM save container.
	ErrNotSave = errors.New("savefile: not an FM save")
	// ErrUnsupported means the data is valid but uses a variant this package
	// cannot read: a big-endian or non-version-8 container, or an inner-file
	// variant such as a several-human humans.dat.
	ErrUnsupported = errors.New("savefile: unsupported save variant")
)

// HeaderSize is the size of the container header at the start of a save.
const HeaderSize = 26

// sectionPrefixSize is the size of the prefix shared by the container
// header and the directory header.
const sectionPrefixSize = 9

// maxOffset caps the header's offsets so that adding them to file
// positions cannot overflow; real saves are under 1 GB.
const maxOffset = 1 << 48

// offsetBase is the position the header's u64 offsets are relative to.
const offsetBase = sectionPrefixSize

const (
	containerKind      = 2
	littleEndian       = 1
	supportedVersion   = 8
	magicSave          = "fmf."
	magicAlternateSave = "afe."
)

// Header is the decoded container header.
type Header struct {
	Magic           string
	Version         uint16
	DirectoryOffset int64 // absolute file offset of the directory header
	DataOffset      int64 // absolute file offset of the first inner file
	Byte25          uint8 // meaning unknown; 3 in every save seen
}

// ParseHeader decodes the container header from the first HeaderSize bytes
// of a save.
func ParseHeader(b []byte) (Header, error) {
	r := newReader(b)
	magic, version, err := readSectionPrefix(r)
	if err != nil {
		return Header{}, err
	}
	dirOff := r.u64()
	dataOff := r.u64()
	b25 := r.u8()
	if r.err != nil {
		return Header{}, r.err
	}
	if dirOff > maxOffset || dataOff > maxOffset {
		return Header{}, fmt.Errorf("%w: offsets %d and %d exceed %d", ErrCorrupt, dirOff, dataOff, uint64(maxOffset))
	}
	return Header{
		Magic:           magic,
		Version:         version,
		DirectoryOffset: offsetBase + int64(dirOff),  //nolint:gosec // bounded by maxOffset
		DataOffset:      offsetBase + int64(dataOff), //nolint:gosec // bounded by maxOffset
		Byte25:          b25,
	}, nil
}

// readSectionPrefix reads and validates the 9-byte prefix: kind, byte
// order, magic, version, one unused byte.
func readSectionPrefix(r *reader) (magic string, version uint16, err error) {
	kind := r.u8()
	order := r.u8()
	magic = string(r.take(4))
	version = r.u16()
	r.u8()
	if r.err != nil {
		return "", 0, r.err
	}
	if kind != containerKind || (magic != magicSave && magic != magicAlternateSave) {
		return "", 0, fmt.Errorf("%w: kind %d, magic %q", ErrNotSave, kind, magic)
	}
	if order != littleEndian {
		return "", 0, fmt.Errorf("%w: byte order %d", ErrUnsupported, order)
	}
	if version != supportedVersion {
		return "", 0, fmt.Errorf("%w: container version %d", ErrUnsupported, version)
	}
	return magic, version, nil
}
