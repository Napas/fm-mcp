package savefile

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/klauspost/compress/zstd"
)

const (
	// maxDirectorySize caps the decompressed directory; real saves hold
	// about 80 KB.
	maxDirectorySize = 64 << 20
	// maxFileSize caps one decompressed inner file; game_db.dat is about
	// 120 MB in a small save.
	maxFileSize = 4 << 30
	// maxPreallocRatio bounds the buffer reserved per compressed byte; real
	// inner files expand about 4 to 10 times.
	maxPreallocRatio = 32
	// maxWindow caps the zstd window a frame may ask for; saves use 1-2 MB.
	maxWindow = 64 << 20
)

// Archive gives access to the inner files of an FM save.
type Archive struct {
	r      io.ReaderAt
	closer io.Closer
	header Header
	dir    Directory
	index  map[string]Entry
	dec    *zstd.Decoder
}

// File is one decompressed inner file.
type File struct {
	Header FileHeader
	Body   []byte // bytes after the header
}

// Open opens the save at path. Call Close when done.
func Open(path string) (*Archive, error) {
	return openFile(path, func(f *os.File, size int64) (*Archive, error) { return NewArchive(f, size) })
}

// openFile opens path and builds an archive from it with newArchive. The
// archive closes the file; the file is closed at once if newArchive fails.
func openFile(path string, newArchive func(f *os.File, size int64) (*Archive, error)) (*Archive, error) {
	f, err := os.Open(path) //nolint:gosec // reading a caller-chosen file is the purpose
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	a, err := newArchive(f, st.Size())
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	a.closer = f
	return a, nil
}

// NewArchive reads the header and directory of a save held in r.
// Call Close when done.
func NewArchive(r io.ReaderAt, size int64) (*Archive, error) {
	hb := make([]byte, HeaderSize)
	if _, err := r.ReadAt(hb, 0); err != nil {
		return nil, fmt.Errorf("%w: reading header: %w", ErrTruncated, err)
	}
	h, err := ParseHeader(hb)
	if err != nil {
		return nil, err
	}
	if h.DataOffset < HeaderSize || h.DirectoryOffset < h.DataOffset || h.DirectoryOffset > size-sectionPrefixSize {
		return nil, fmt.Errorf("%w: data offset %d, directory offset %d, file size %d", ErrCorrupt, h.DataOffset, h.DirectoryOffset, size)
	}

	dec, err := newDecoder(maxFileSize)
	if err != nil {
		return nil, err
	}
	a := &Archive{r: r, header: h, dec: dec}
	if err := a.readDirectory(size); err != nil {
		dec.Close()
		return nil, err
	}
	return a, nil
}

// newDecoder returns a single-threaded zstd decoder that rejects output
// larger than maxSize bytes.
func newDecoder(maxSize uint64) (*zstd.Decoder, error) {
	return zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxSize), zstd.WithDecoderMaxWindow(maxWindow))
}

func (a *Archive) readDirectory(size int64) error {
	// The compressed directory is smaller than its 64 MB decoded cap.
	if size-a.header.DirectoryOffset > maxDirectorySize {
		return fmt.Errorf("%w: directory is %d bytes", ErrCorrupt, size-a.header.DirectoryOffset)
	}
	b := make([]byte, size-a.header.DirectoryOffset)
	if _, err := a.r.ReadAt(b, a.header.DirectoryOffset); err != nil {
		return fmt.Errorf("%w: reading directory: %w", ErrTruncated, err)
	}
	if _, _, err := readSectionPrefix(newReader(b)); err != nil {
		return fmt.Errorf("directory header: %w", err)
	}
	// A separate decoder bounds the directory's size while decoding.
	dec, err := newDecoder(maxDirectorySize)
	if err != nil {
		return err
	}
	raw, err := dec.DecodeAll(b[sectionPrefixSize:], nil)
	dec.Close()
	if err != nil {
		return fmt.Errorf("%w: decompressing directory: %w", ErrCorrupt, err)
	}
	dir, err := ParseDirectory(raw)
	if err != nil {
		return err
	}

	dataLen := uint64(a.header.DirectoryOffset - a.header.DataOffset) //nolint:gosec // non-negative, checked in NewArchive
	a.index = make(map[string]Entry, len(dir.Files))
	for _, e := range dir.Files {
		if e.Offset > dataLen || e.CompressedSize > dataLen-e.Offset {
			return fmt.Errorf("%w: %s lies outside the data area", ErrCorrupt, e.Path)
		}
		if e.Size > maxFileSize {
			return fmt.Errorf("%w: %s claims %d bytes", ErrCorrupt, e.Path, e.Size)
		}
		a.index[e.Path] = e
	}
	a.dir = dir
	return nil
}

// Header returns the container header.
func (a *Archive) Header() Header { return a.header }

// Directory returns the file tree.
func (a *Archive) Directory() Directory { return a.dir }

// ReadFile decompresses the inner file at path, e.g. "game_db.dat" or
// "rgman/comp_1.dat".
// It shares one decoder, so calls must not run concurrently.
func (a *Archive) ReadFile(path string) (File, error) {
	e, ok := a.index[path]
	if !ok {
		return File{}, &fs.PathError{Op: "read", Path: path, Err: fs.ErrNotExist}
	}
	frame := make([]byte, e.CompressedSize)
	if _, err := a.r.ReadAt(frame, a.header.DataOffset+int64(e.Offset)); err != nil { //nolint:gosec // bounded by the data area
		return File{}, fmt.Errorf("%w: reading %s: %w", ErrTruncated, path, err)
	}
	// Stream-decode at most the size the directory claims, into a buffer no
	// larger than the frame can plausibly expand to. DecodeAll would instead
	// allocate whatever content size the frame header claims.
	if err := a.dec.Reset(bytes.NewReader(frame)); err != nil {
		return File{}, fmt.Errorf("%w: decompressing %s: %w", ErrCorrupt, path, err)
	}
	buf := bytes.NewBuffer(make([]byte, 0, min(e.Size, uint64(len(frame))*maxPreallocRatio)))
	if _, err := io.Copy(buf, io.LimitReader(a.dec, int64(e.Size)+1)); err != nil { //nolint:gosec // e.Size ≤ maxFileSize
		return File{}, fmt.Errorf("%w: decompressing %s: %w", ErrCorrupt, path, err)
	}
	raw := buf.Bytes()
	if uint64(len(raw)) != e.Size {
		return File{}, fmt.Errorf("%w: %s is %d bytes, directory says %d", ErrCorrupt, path, len(raw), e.Size)
	}
	h, err := ParseFileHeader(raw)
	if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	return File{Header: h, Body: raw[FileHeaderSize:]}, nil
}

// Close releases the decoder and, for archives from Open, the file.
func (a *Archive) Close() error {
	a.dec.Close()
	if a.closer != nil {
		return a.closer.Close()
	}
	return nil
}
