package savefile

import "fmt"

// ReadHumanIndex returns the person index of the save's human manager from
// humans.dat: u16 human count, then the first human's u32 person index. Only
// one-human saves have been seen, so other counts are ErrUnsupported.
func ReadHumanIndex(a *Archive) (uint32, error) {
	f, err := a.ReadFile("humans.dat")
	if err != nil {
		return 0, err
	}
	r := newReader(f.Body)
	if n := r.u16(); r.err == nil && n != 1 {
		return 0, fmt.Errorf("%w: %d humans in humans.dat", ErrUnsupported, n)
	}
	index := r.u32()
	if r.err != nil {
		return 0, fmt.Errorf("humans.dat: %w", r.err)
	}
	return index, nil
}
