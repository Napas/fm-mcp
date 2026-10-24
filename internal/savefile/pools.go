package savefile

import "fmt"

// readPoolCounts decodes memory_pools.dat: u16 count, then count × (string
// type name, u32 instance count). Some names repeat (SMALL_OBJECT_POOL);
// the first count is kept.
func readPoolCounts(a *Archive) (map[string]uint32, error) {
	f, err := a.ReadFile("memory_pools.dat")
	if err != nil {
		return nil, err
	}
	r := newReader(f.Body)
	n := r.u16()
	pools := make(map[string]uint32, n)
	for range n {
		name, count := r.str(), r.u32()
		if _, seen := pools[name]; !seen {
			pools[name] = count
		}
	}
	if r.err != nil {
		return nil, fmt.Errorf("memory_pools.dat: %w", r.err)
	}
	return pools, nil
}

// poolSum adds the named pool counts, rejecting totals no table can hold.
func poolSum(pools map[string]uint32, names ...string) (uint32, error) {
	var sum uint64
	for _, n := range names {
		sum += uint64(pools[n])
	}
	if sum > maxTableRecords {
		return 0, fmt.Errorf("%w: %v pools hold %d records", ErrCorrupt, names, sum)
	}
	return uint32(sum), nil //nolint:gosec // bounded by maxTableRecords
}
