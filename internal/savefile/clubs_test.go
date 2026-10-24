package savefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

func le32(b []byte, v int32) []byte { return binary.LittleEndian.AppendUint32(b, uint32(v)) } //nolint:gosec // two's complement intended

func refList8(b []byte, refs ...int32) []byte {
	b = append(b, byte(len(refs))) //nolint:gosec // small test data
	for _, r := range refs {
		b = le32(b, r)
	}
	return b
}

func dbRecord(b []byte, index, uid uint32) []byte {
	b = binary.LittleEndian.AppendUint32(b, index)
	b = binary.LittleEndian.AppendUint32(b, uid)
	return binary.LittleEndian.AppendUint32(b, uid)
}

// teamContainer encodes the TEAM_CONTAINER base that ends club records.
func teamContainer(b []byte, women bool, teams, affiliated []int32, index, uid uint32) []byte {
	w := byte(0)
	if women {
		w = 1
	}
	b = append(b, 0, 0xff, w)
	b = refList8(b, teams...)
	b = refList8(b, affiliated...)
	b = append(b, 0, 0, 0) // three empty person lists
	b = append(b, 1)       // one 16-byte item
	b = append(b, make([]byte, 16)...)
	b = binary.LittleEndian.AppendUint16(b, 1) // one 28-byte item
	b = append(b, make([]byte, 28)...)
	return dbRecord(b, index, uid)
}

func financeBase(b []byte, balance int32) []byte {
	b = le32(b, balance)
	b = append(b, 0, 0, 0, 0)
	b = append(b, make([]byte, 12)...) // three dates
	b = append(b, 0)
	b = le32(b, 0)
	return append(b, make([]byte, 20)...)
}

// fullFinance encodes CLUB_FINANCE with the given budgets and bank balance.
func fullFinance(b []byte, transfer, wage, balance int32) []byte {
	b = le32(b, 5634364) // +0x7d0
	b = le32(b, transfer)
	b = le32(b, 2353289) // next season's transfer budget
	b = append(b, 0)
	b = append(b, make([]byte, 9*4+4+1+4+4+4)...)
	b = le32(b, wage)
	for range 4 {
		b = append(b, make([]byte, 1+4+230)...)
		b = append(b, 1)
		b = append(b, make([]byte, 8)...)
		b = append(b, 0)
	}
	b = append(b, make([]byte, 24+2+8+5+8+4+16)...)
	b = append(b, 1)
	b = append(b, make([]byte, 230)...)
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = append(b, make([]byte, 49)...)
	b = append(b, make([]byte, 4+1+20+8+16+2+20)...)
	b = append(b, 1)
	b = append(b, make([]byte, 19)...)
	b = append(b, make([]byte, 22)...)
	return financeBase(b, balance)
}

type testClub struct {
	index, uid  uint32
	name, short string
	nation      int32
	women       bool
	finance     byte // 0 base, 1 full, 2 none
	teams       []int32
}

func (c testClub) encode(b []byte) []byte {
	b = append(b, 0)
	b = le32(b, c.nation)
	for range 3 {
		b = le32(b, -1)
	}
	b = le32(b, 1040)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = append(b, 0, 0)
	b = appendString(b, c.name)
	b = appendString(b, c.short)
	b = append(b, 0)
	b = binary.LittleEndian.AppendUint16(b, 5598)
	b = le32(b, -1)
	b = append(b, 0)
	b = refList8(b, 7)
	b = append(b, c.finance)
	switch c.finance {
	case 0:
		b = financeBase(b, 242466987)
	case 1:
		b = fullFinance(b, 8329074, 1069817, 242466987)
	}
	b = append(b, 1)
	b = append(b, make([]byte, 25)...)
	b = append(b, 1)
	b = append(b, make([]byte, 12)...)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = le32(b, -1)
	return teamContainer(b, c.women, c.teams, nil, c.index, c.uid)
}

type testTeam struct {
	index, uid uint32
	national   bool
	container  int32
	reputation uint16
	division   int32
	teamType   byte
	ageLimit   byte
	players    []int32
}

func (t testTeam) encode(b []byte, dataVersion uint16) []byte {
	if t.national {
		b = append(b, 11)
		b = append(b, make([]byte, 12+1+4+24)...)
		b = append(b, make([]byte, 7)...)
		if dataVersion >= 0xe94 {
			b = append(b, 0)
		}
		if dataVersion >= 0xe93 {
			b = append(b, 0, 0, 0, 0)
		}
		b = append(b, 0)
	} else {
		b = append(b, 10)
	}
	b = append(b, 0, 17)
	b = binary.LittleEndian.AppendUint16(b, t.reputation)
	b = le32(b, t.division)
	b = le32(b, -1)
	b = le32(b, -1)
	b = le32(b, 9802)  // stadium
	b = le32(b, 88351) // manager
	b = le32(b, 637)   // continental competition
	b = append(b, 0)
	b = le32(b, -1)
	b = binary.LittleEndian.AppendUint16(b, uint16(len(t.players))) //nolint:gosec // small test data
	for _, p := range t.players {
		b = le32(b, p)
	}
	b = le32(b, 7046)  // captain
	b = le32(b, 12458) // vice-captain
	b = append(b, make([]byte, 5)...)
	b = le32(b, -1)
	b = append(b, 0)
	national := byte(0)
	if t.national {
		national = 1
	}
	b = append(b, t.teamType, national, 0, t.ageLimit, 0, 0)
	b = le32(b, t.container)
	b = appendString(b, "")
	b = append(b, 0)
	b = appendString(b, "")
	b = append(b, 0)
	return dbRecord(b, t.index, t.uid)
}

// tableBytes wraps encoded records as a table: preceded by the previous
// table's end marker, then u32 count, u8 flag, records, end marker.
func tableBytes(count int, records []byte) []byte {
	b := []byte{0xaa, 0, 0, 0, 0, 1, 0, 0, 0, 0}
	b = binary.LittleEndian.AppendUint32(b, uint32(count)) //nolint:gosec // small test data
	b = append(b, 0)
	b = append(b, records...)
	return append(b, 0, 0, 0, 0, 1, 0, 0, 0, 0)
}

func TestReadClubTable(t *testing.T) {
	var recs []byte
	recs = testClub{index: 0, uid: 728, name: "Tottenham Hotspur", short: "Spurs", nation: 139, finance: 1, teams: []int32{1}}.encode(recs)
	recs = testClub{index: 1, uid: 2000099871, name: "Chelsea", short: "Chelsea", nation: 139, women: true, finance: 0, teams: []int32{0}}.encode(recs)
	recs = testClub{index: 2, uid: 99, name: "", short: "", nation: -1, finance: 2}.encode(recs)
	body := append([]byte{1, 2, 3}, tableBytes(3, recs)...)

	clubs, end, err := readClubTable(body, 3)
	if err != nil {
		t.Fatalf("readClubTable: %v", err)
	}

	if len(clubs) != 3 || end != len(body)-9 {
		t.Fatalf("got %d clubs ending at %d, want 3 ending at %d", len(clubs), end, len(body)-9)
	}
	spurs := clubs[0]
	if spurs.UID != 728 || spurs.Name != "Tottenham Hotspur" || spurs.ShortName != "Spurs" || spurs.NationIndex != 139 || spurs.Women {
		t.Errorf("Spurs = %+v", spurs)
	}
	if spurs.Balance != 242466987 || len(spurs.Teams) != 1 || spurs.Teams[0] != 1 {
		t.Errorf("Spurs finances/teams = %+v", spurs)
	}
	if want := (Budgets{Transfer: 8329074, WeeklyWage: 1069817}); spurs.Budgets == nil || *spurs.Budgets != want {
		t.Errorf("Spurs budgets = %+v, want %+v", spurs.Budgets, want)
	}
	if !clubs[1].Women || clubs[1].Balance != 242466987 || clubs[1].Budgets != nil {
		t.Errorf("Chelsea women = %+v", clubs[1])
	}
}

func TestReadTeamTableHandlesNationalTeamVersions(t *testing.T) {
	for _, version := range []uint16{3727, 3733} {
		var recs []byte
		recs = testTeam{index: 0, uid: 5, national: true, container: 0, reputation: 6000, division: -1, teamType: 33, ageLimit: 100}.encode(recs, version)
		recs = testTeam{index: 1, uid: 728, container: 0, reputation: 7750, division: 7, ageLimit: 100, players: []int32{8804, 7046}}.encode(recs, version)
		recs = testTeam{index: 2, uid: 27845, container: 0, reputation: 4000, division: 90, teamType: 10, ageLimit: 21}.encode(recs, version)
		body := tableBytes(3, recs)

		teams, _, err := readTeamTable(body, 3, version)
		if err != nil {
			t.Fatalf("version %d: readTeamTable: %v", version, err)
		}

		if !teams[0].National || teams[0].Container != 0 {
			t.Errorf("version %d: national team = %+v", version, teams[0])
		}
		first := teams[1]
		if first.UID != 728 || first.Reputation != 7750 || first.Division != 7 || first.Manager != 88351 || len(first.Players) != 2 || first.Players[0] != 8804 {
			t.Errorf("version %d: first team = %+v", version, first)
		}
		if teams[2].AgeLimit != 21 || teams[2].TeamType != 10 {
			t.Errorf("version %d: U21 = %+v", version, teams[2])
		}
	}
}

func TestReadClubTableReportsMissingTable(t *testing.T) {
	_, _, err := readClubTable([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 3)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestTeamLabel(t *testing.T) {
	tests := map[string]Team{
		"":         {TeamType: 0, AgeLimit: 100},
		"U21":      {TeamType: 10, AgeLimit: 21},
		"B":        {TeamType: 3, AgeLimit: 255},
		"II":       {TeamType: 16, AgeLimit: 100},
		"Reserves": {TeamType: 1, AgeLimit: 100},
	}
	for want, team := range tests {
		if got := team.Label(); got != want {
			t.Errorf("%+v.Label() = %q, want %q", team, got, want)
		}
	}
}

func TestFirstTeamSharesClubUID(t *testing.T) {
	db := &GameDB{Teams: []Team{{Index: 0, UID: 5, National: true}, {Index: 1, UID: 728}, {Index: 2, UID: 27845}}}

	if i, ok := db.FirstTeam(728); !ok || i != 1 {
		t.Errorf("FirstTeam(728) = %d, %v", i, ok)
	}
	if _, ok := db.FirstTeam(5); ok {
		t.Error("FirstTeam matched a national team")
	}
}
