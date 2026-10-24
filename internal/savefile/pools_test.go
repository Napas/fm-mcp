package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestReadPoolCountsKeepsFirstOfRepeatedNames(t *testing.T) {
	b := binary.LittleEndian.AppendUint16(nil, 3)
	for _, e := range []struct {
		name  string
		count uint32
	}{{"NATION", 251}, {"SMALL_OBJECT_POOL", 7}, {"SMALL_OBJECT_POOL", 9}} {
		b = appendString(b, e.name)
		b = binary.LittleEndian.AppendUint32(b, e.count)
	}
	a := openSave(t, buildSave(t, testInner{path: "memory_pools.dat", data: append(innerFile("tad.", 0, ""), b...)}))

	pools, err := readPoolCounts(a)
	if err != nil {
		t.Fatalf("readPoolCounts: %v", err)
	}

	if pools["NATION"] != 251 || pools["SMALL_OBJECT_POOL"] != 7 || len(pools) != 2 {
		t.Errorf("pools = %v", pools)
	}
}

func TestPoolSumRejectsOverflowAndHugeCounts(t *testing.T) {
	if n, err := poolSum(map[string]uint32{"CLUB": 25315, "LEAGUE_BODY_CLUB": 462}, "CLUB", "LEAGUE_BODY_CLUB"); err != nil || n != 25777 {
		t.Errorf("poolSum = %d, %v; want 25777", n, err)
	}
	if _, err := poolSum(map[string]uint32{"A": 4_000_000_000, "B": 4_000_000_000}, "A", "B"); !errors.Is(err, ErrCorrupt) {
		t.Errorf("overflowing sum: err = %v, want ErrCorrupt", err)
	}
}
