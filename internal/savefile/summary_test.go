package savefile

import (
	"encoding/binary"
	"testing"
)

func TestReadSummary(t *testing.T) {
	b := appendString(nil, "765,MALE,3,0")
	b = appendString(b, "26.2.0+2276383")
	b = binary.LittleEndian.AppendUint32(b, 2)
	b = appendString(b, "Sky Bet League Two")
	b = appendString(b, "WSL2")
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 0x002ea8c7)
	b = binary.LittleEndian.AppendUint16(b, 3727)
	b = append(b, 1)
	b = appendString(b, "Napas Mačiulaitis")
	b = appendString(b, "Spurs")
	b = binary.LittleEndian.AppendUint32(b, 728)
	b = append(b, 0xbc, 0x1a, 0xe9, 0x07)
	b = append(b, 0x9c, 0, 0, 0) // rest not decoded
	a := openSave(t, buildSave(t, testInner{path: "save_game_summary.dat", data: append(innerFile("tad.", 29, ""), b...)}))

	s, err := ReadSummary(a)
	if err != nil {
		t.Fatalf("ReadSummary: %v", err)
	}

	want := Summary{Build: "26.2.0+2276383", ManagerName: "Napas Mačiulaitis", ClubShortName: "Spurs", ClubUID: 728, GameDate: Date{Raw: 0x1abc, Year: 2025}}
	if s != want {
		t.Errorf("Summary = %+v, want %+v", s, want)
	}
}
