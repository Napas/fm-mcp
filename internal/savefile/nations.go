package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	tagContinent = 0
	tagNation    = 0
	tagContainer = 0
	// Memory pool names; the CONTINENT table has no pool.
	nationName    = "NATION"
	containerName = "NATIONAL_TEAM_CONTAINER"

	maxContinents = 64
	// maxNations caps the NATION pool count (251 in every save seen).
	maxNations = 1000
	// continentStrengthOffset is CONTINENT +0xe4 in the record (writer
	// 0x3260970): u8 tag, 3 × u8, f32.
	continentStrengthOffset = 4

	// The NATION record (writer 0x35bf574) ends with a tail of fixed grammar
	// before its database record: u8, i32 continent, 3 × i32, u8, u8, f32,
	// u8, then four counted lists and nationTailEndSize bytes.
	nationTailFixed   = 1 + 4*4 + 1 + 1 + 4 + 1
	nationTailEndSize = 10*4 + 2 + 1 + 11
	// maxNationTailHead bounds the tail's first byte (+0x1f3): candidate
	// tails starting with a larger byte are skipped.
	maxNationTailHead  = 20
	maxNationTailItems = 1000

	// containerFactorOffset is the i16 reputation factor (+0x572) in a
	// NATIONAL_TEAM_CONTAINER record (writer 0x35f0368).
	containerFactorOffset = 169
	maxContainerRecord    = 1 << 20

	// maxNationCandidates caps the places each reader tries, and the spans
	// below bound how far one try scans, so crafted input cannot make the
	// readers quadratic. Spans seen (26.2.0, 26.3.2): CONTINENT up to
	// 121 KB, NATION up to 367 KB, the men's containers up to 1.9 MB.
	maxNationCandidates = 64
	maxContinentTable   = 1 << 20
	maxNationTable      = 1 << 20
	maxContainerTable   = 16 << 20
)

// nationTailLists are the item sizes of the tail's u32-counted lists, after
// a u8-counted list of 5-byte items.
var nationTailLists = []int{28, 28, 12}

// Continent is a CONTINENT record.
type Continent struct {
	Index    uint32
	UID      uint32
	Strength float32 // +0xe4, 0..1: Europe 0.99, South America 1, World 0, the others 0.85 (26.3.2)
}

// Nation is a NATION record, with the reputation factor of its men's
// national-team container (container index = nation index).
type Nation struct {
	Index      uint32
	UID        uint32
	Continent  int32 // CONTINENT index; -1 for none (defunct nations such as the USSR)
	MensFactor int16 // men's container +0x572, 0 when the container is not read
}

// readNationData reads the CONTINENT and NATION tables and the men's
// national-team containers' factors. The memory pools give the NATION,
// container and CURRENCY_RECORD counts.
func readNationData(body []byte, pools map[string]uint32) ([]Continent, []Nation, error) {
	cs, err := readContinents(body, pools[currencyName])
	if err != nil {
		return nil, nil, err
	}
	ns, err := readNations(body, pools[nationName], len(cs))
	if err != nil {
		return nil, nil, err
	}
	if err := readNationFactors(body, pools[containerName], ns); err != nil {
		return nil, nil, err
	}
	return cs, ns, nil
}

// readContinents reads the CONTINENT table, which ends right before the
// CURRENCY_RECORD table. The last record's index gives the count.
func readContinents(body []byte, currencies uint32) ([]Continent, error) {
	if currencies > 0 {
		// Each currency head and each continent head tried counts.
		tries := 0
		for p := range tableHeads(body, tableHead(currencies, tagCurrency)) {
			if tries++; tries > maxNationCandidates {
				return nil, tooManyCandidates("CONTINENT")
			}
			if cs, ok := continentsEndingAt(body, p-tableEndSize, &tries); ok {
				return cs, nil
			}
			if tries > maxNationCandidates {
				return nil, tooManyCandidates("CONTINENT")
			}
		}
	}
	return nil, fmt.Errorf("%w: CONTINENT table", ErrNotFound)
}

// tooManyCandidates is the error of a reader that found more than
// maxNationCandidates places to try.
func tooManyCandidates(table string) error {
	return fmt.Errorf("%w: over %d %s table candidates", ErrCorrupt, maxNationCandidates, table)
}

// continentsEndingAt parses the CONTINENT table whose end marker is at end,
// starting at most maxContinentTable bytes before it. It adds the heads it
// tries to tries and stops once tries exceeds maxNationCandidates.
func continentsEndingAt(body []byte, end int, tries *int) ([]Continent, bool) {
	if end < databaseRecordSize {
		return nil, false
	}
	last := binary.LittleEndian.Uint32(body[end-databaseRecordSize:])
	if last >= maxContinents {
		return nil, false
	}
	head := tableHead(last+1, tagContinent)
	// Records are split on their database records, so the search stops at
	// the end marker.
	table := body[:end+tableEndSize]
	from := max(0, end-maxContinentTable)
	for q := end; ; {
		i := bytes.LastIndex(body[from:q], head)
		if i < 0 {
			return nil, false
		}
		p := from + i
		q = p
		if !afterTableEnd(body, p) {
			continue
		}
		if *tries++; *tries > maxNationCandidates {
			return nil, false
		}
		recs, stop, err := splitTable(table, p, competitionTrailer)
		if err != nil || stop != end {
			continue
		}
		if cs, ok := continents(body, recs); ok {
			return cs, true
		}
	}
}

func continents(body []byte, recs []Record) ([]Continent, bool) {
	out := make([]Continent, 0, len(recs))
	for _, rec := range recs {
		if rec.Tag != tagContinent || rec.End-rec.Start < continentStrengthOffset+4+databaseRecordSize {
			return nil, false
		}
		s := math.Float32frombits(binary.LittleEndian.Uint32(body[rec.Start+continentStrengthOffset:]))
		if !(s >= 0 && s <= 1) {
			return nil, false
		}
		out = append(out, Continent{Index: rec.Index, UID: rec.UID, Strength: s})
	}
	return out, true
}

// readNations reads the NATION table of count records. Nations are stored
// in ascending uid order, and every continent must be below continents. A
// count above maxNations is corrupt.
func readNations(body []byte, count uint32, continents int) ([]Nation, error) {
	if count > maxNations {
		return nil, fmt.Errorf("%w: %d NATION records", ErrCorrupt, count)
	}
	if count > 0 {
		tries := 0
		for p := range tableHeads(body, tableHead(count, tagNation)) {
			if tries++; tries > maxNationCandidates {
				return nil, tooManyCandidates(nationName)
			}
			if ns, ok := nations(body[:min(len(body), p+maxNationTable)], p, count, continents); ok {
				return ns, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: NATION table with %d records", ErrNotFound, count)
}

func nations(body []byte, start int, count uint32, continents int) ([]Nation, bool) {
	recs, end, err := splitTable(body, start, nationTrailer)
	if err != nil || uint32(len(recs)) != count || !bytes.HasPrefix(body[end:], tableEnd) { //nolint:gosec // record counts fit in uint32
		return nil, false
	}
	out := make([]Nation, 0, len(recs))
	for i, rec := range recs {
		if rec.Tag != tagNation || i > 0 && rec.UID <= recs[i-1].UID {
			return nil, false
		}
		c, ok := nationContinent(body[rec.Start:rec.End-databaseRecordSize], continents)
		if !ok {
			return nil, false
		}
		out = append(out, Nation{Index: rec.Index, UID: rec.UID, Continent: c})
	}
	return out, true
}

// nationTrailer is competitionTrailer, except that of two matches one byte
// apart the later wins: a record whose last byte is 0 makes nation 0's
// database record (index 0, any uid below 2^24) also match one byte early.
func nationTrailer(body []byte, pos, j int) bool {
	if !competitionTrailer(body, pos, j) {
		return false
	}
	k := j + 1
	if !competitionTrailer(body, pos, k) || !bytes.Equal(body[j:j+4], body[k:k+4]) {
		return true
	}
	uid := binary.LittleEndian.Uint32(body[k+4:])
	return uid == 0 || uid != binary.LittleEndian.Uint32(body[k+8:])
}

// nationContinent returns the continent of the NATION record b (without its
// database record), -1 or below continents. The lists before the tail are
// not decoded, so it takes the last position from which the tail parses to
// the end of b: the first one is wrong for the United States and Ecuador
// (26.3.2).
func nationContinent(b []byte, continents int) (int32, bool) {
	for p := len(b) - nationTailFixed - 1 - 3*4 - nationTailEndSize; p >= 1; p-- {
		if b[p] > maxNationTailHead {
			continue
		}
		c := int32(binary.LittleEndian.Uint32(b[p+1:])) //nolint:gosec // signed field
		if c >= -1 && int(c) < continents && nationTailEnd(b, p) == len(b) {
			return c, true
		}
	}
	return 0, false
}

// nationTailEnd returns where a tail starting at p ends, or -1.
func nationTailEnd(b []byte, p int) int {
	q := p + nationTailFixed
	if q >= len(b) {
		return -1
	}
	q += 1 + 5*int(b[q])
	for _, size := range nationTailLists {
		if q+4 > len(b) {
			return -1
		}
		n := binary.LittleEndian.Uint32(b[q:])
		if n > maxNationTailItems {
			return -1
		}
		q += 4 + size*int(n)
	}
	return q + nationTailEndSize
}

// readNationFactors sets each nation's MensFactor from the NATIONAL_TEAM_CONTAINER
// table of count records, whose first records are the men's containers in
// nation order. Each ends with its nation's database record.
func readNationFactors(body []byte, count uint32, ns []Nation) error {
	if len(ns) > 0 && count >= uint32(len(ns)) { //nolint:gosec // record counts fit in uint32
		head := tableHead(count, tagContainer)
		tries := 0
		for p := range tableHeads(body, head) {
			if tries++; tries > maxNationCandidates {
				return tooManyCandidates(containerName)
			}
			pos := p + len(head) - 1
			if fs, ok := containerFactors(body[:min(len(body), pos+maxContainerTable)], pos, ns, count); ok {
				for i := range ns {
					ns[i].MensFactor = fs[i]
				}
				return nil
			}
		}
	}
	return fmt.Errorf("%w: %s table with %d records", ErrNotFound, containerName, count)
}

// containerFactors reads the factors of the first len(ns) container records
// starting at pos.
func containerFactors(body []byte, pos int, ns []Nation, count uint32) ([]int16, bool) {
	out := make([]int16, 0, len(ns))
	for _, n := range ns {
		if pos >= len(body) || body[pos] != tagContainer {
			return nil, false
		}
		key := binary.LittleEndian.AppendUint32(nil, n.Index)
		key = binary.LittleEndian.AppendUint32(key, n.UID)
		key = binary.LittleEndian.AppendUint32(key, n.UID)
		k := bytes.Index(body[pos:min(len(body), pos+maxContainerRecord)], key)
		if k < containerFactorOffset+2 {
			return nil, false
		}
		out = append(out, int16(binary.LittleEndian.Uint16(body[pos+containerFactorOffset:]))) //nolint:gosec // signed field
		pos += k + databaseRecordSize
	}
	// A further container starts with its tag; the last one is followed by
	// the end marker.
	more := uint32(len(ns)) < count //nolint:gosec // record counts fit in uint32
	return out, more && pos < len(body) && body[pos] == tagContainer || !more && bytes.HasPrefix(body[pos:], tableEnd)
}
