package savefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	compFilePrefix = "rgman/comp_"
	compFileSuffix = ".dat"

	// tableStageOffset is how far before a table's count byte its stage's
	// i16 STAGE_NAME index sits.
	tableStageOffset = 17
	// tableFirstStatOffset is from a table's count byte to its first row's
	// first stat record: count, ref (u8, i32), 14 × u8, i32.
	tableFirstStatOffset = 1 + 5 + 14 + 4
	tableRowHeadSize     = 14
	tableRowTailSize     = 12
	tableRoundItemSize   = 8 // i32, u16, u8, u8
	tableFormSize        = 10
	tableRowStats        = 5
	tablePositionOffset  = 6 // from the count byte: the first row's position

	maxTableRows       = 64
	maxTableHeadToHead = 64
	maxTableRoundItems = 64
	maxTableRowChain   = 8
	minTableRows       = 2
	// A file's scan may parse at most tableScanFactor × its size plus
	// tableScanSlack bytes of candidates, keeping crafted files linear.
	tableScanFactor = 16
	tableScanSlack  = 1 << 20
)

// firstStatRef is a row's first stat record's ref: u8 0, i32 -1 (this row's team).
var firstStatRef = []byte{0, 0xff, 0xff, 0xff, 0xff}

// LeagueTable is the first row list of a LEAGUE_STAGE (load 0x404291c): the
// stored table in the game's order.
type LeagueTable struct {
	CompetitionUID uint32 // from the comp file's name
	Stage          int16  // STAGE_NAME index; -1 when unset or out of range
	Rows           []TableRow
}

// TableRow is a table row (loader 0x4044190, base 0x4046740).
type TableRow struct {
	Team                int32 // TEAM index
	Position            uint8 // 0-based
	Overall, Home, Away TableStats
	Form                [10]uint8 // the last ten results, most recent first: 3 win, 2 draw, 1 loss, other = none
}

// TableStats is a 17-byte stat record (0x4077f90).
type TableStats struct {
	Played, Won, Drawn, Lost uint8
	ShootoutLosses           uint8 // in groups where drawn matches go to penalties; there Drawn counts shoot-out wins (inference)
	GoalsFor, GoalsAgainst   uint16
	Points                   int16 // as stored: not always 3 × won + drawn
}

// LeagueTables are the tables of every comp file.
type LeagueTables struct {
	Tables  []LeagueTable
	Skipped int // comp files that could not be read or exceeded the scan budget
}

// ReadLeagueTables reads the tables of every rgman/comp_<uid>.dat. Stage
// indices at or above stageCount are dropped. It fails only when no comp
// file can be read.
func ReadLeagueTables(a *Archive, stageCount int) (LeagueTables, error) {
	var out LeagueTables
	files, read := 0, 0
	for _, e := range a.Directory().Files {
		uid, ok := compFileUID(e.Path)
		if !ok {
			continue
		}
		files++
		f, err := a.ReadFile(e.Path)
		if err != nil {
			out.Skipped++
			continue
		}
		read++
		tables, truncated := parseLeagueTables(f.Body, uid, stageCount)
		out.Tables = append(out.Tables, tables...)
		if truncated {
			out.Skipped++
		}
	}
	if files > 0 && read == 0 {
		return LeagueTables{}, fmt.Errorf("%w: none of %d comp files could be read", ErrCorrupt, files)
	}
	return out, nil
}

// compFileUID returns the competition uid of a path "rgman/comp_<digits>.dat".
func compFileUID(path string) (uint32, bool) {
	name, ok := strings.CutPrefix(path, compFilePrefix)
	if !ok {
		return 0, false
	}
	if name, ok = strings.CutSuffix(name, compFileSuffix); !ok {
		return 0, false
	}
	uid, err := strconv.ParseUint(name, 10, 32)
	return uint32(uid), err == nil
}

// parseLeagueTables finds the tables of one comp file. A candidate is a
// count byte 1..64 whose first row has ref byte 0, position 0 and a first
// stat record of team -1. It is kept when every row parses and validates
// (see validTable); the partner list that follows it, holding the same
// teams in another order, is skipped. truncated reports that the scan
// stopped at its budget.
func parseLeagueTables(body []byte, uid uint32, stageCount int) (tables []LeagueTable, truncated bool) {
	return scanLeagueTables(body, uid, stageCount, tableScanFactor*len(body)+tableScanSlack)
}

// scanLeagueTables is parseLeagueTables with a budget: the bytes every
// candidate and partner-list parse may read in total.
func scanLeagueTables(body []byte, uid uint32, stageCount, budget int) (tables []LeagueTable, truncated bool) {
	for pos, done := 0, 0; ; {
		i := bytes.Index(body[pos:], firstStatRef)
		if i < 0 {
			return tables, false
		}
		hit := pos + i
		pos = hit + 1
		o := hit - tableFirstStatOffset
		if o < done || body[o] == 0 || body[o] > maxTableRows || body[o+1] != 0 || body[o+tablePositionOffset] != 0 {
			continue
		}
		rows, end, ok := readTableList(body, o)
		if budget -= end - o; budget < 0 {
			return tables, true
		}
		if !ok || !validTable(rows) {
			continue
		}
		partner, pend, pok := readTableList(body, end)
		if budget -= pend - end; budget < 0 {
			return tables, true
		}
		if pok && sameTeams(rows, partner) {
			end = pend
		}
		tables = append(tables, LeagueTable{CompetitionUID: uid, Stage: tableStage(body, o, stageCount), Rows: rows})
		pos, done = end, end
	}
}

func tableStage(body []byte, o, stageCount int) int16 {
	if o < tableStageOffset {
		return -1
	}
	s := int16(binary.LittleEndian.Uint16(body[o-tableStageOffset:])) //nolint:gosec // signed field
	if s < -1 || int(s) >= stageCount {
		return -1
	}
	return s
}

// readTableList reads a u8 count and that many rows at o. It returns where
// reading stopped, also on failure.
func readTableList(body []byte, o int) ([]TableRow, int, bool) {
	r := newReader(body)
	r.off = o
	n := int(r.u8())
	if n == 0 || n > maxTableRows {
		return nil, r.off, false
	}
	var rows []TableRow // not sized from n: most candidates fail early
	for range n {
		row, ok := readTableRow(r, 0)
		if !ok {
			return nil, r.off, false
		}
		rows = append(rows, row)
	}
	return rows, r.off, r.err == nil
}

// readTableRow reads a row: the base, i16 n + n round positions, i8 m +
// m × 8 bytes, u8 sub-object flag (1 is not decoded and rejects the row),
// 10 form bytes, u8 flag (1 = a chained row follows; read and dropped).
func readTableRow(r *reader, depth int) (TableRow, bool) {
	row, ok := readTableRowBase(r)
	if !ok {
		return TableRow{}, false
	}
	if n := int16(r.u16()); n > 0 { //nolint:gosec // signed field
		r.take(int(n))
	}
	m := int8(r.u8()) //nolint:gosec // signed field
	if m > maxTableRoundItems {
		return TableRow{}, false
	}
	if m > 0 {
		r.take(tableRoundItemSize * int(m))
	}
	if r.u8() == 1 {
		return TableRow{}, false
	}
	copy(row.Form[:], r.take(tableFormSize))
	if r.u8() == 1 {
		if depth >= maxTableRowChain {
			return TableRow{}, false
		}
		if _, ok := readTableRow(r, depth+1); !ok {
			return TableRow{}, false
		}
	}
	return row, r.err == nil
}

// readTableRowBase reads the row base: ref team, 14 × u8 (the first is the
// position), i32, 5 stat records (overall, home, away, then two not
// decoded), u8 flag + u8 n + n × 2 head-to-head stat records, 12 bytes.
func readTableRowBase(r *reader) (TableRow, bool) {
	var row TableRow
	r.u8()
	row.Team = r.i32()
	if h := r.take(tableRowHeadSize); h != nil {
		row.Position = h[0]
	}
	r.i32()
	var stats [tableRowStats]TableStats
	for i := range stats {
		stats[i] = readTableStats(r)
	}
	row.Overall, row.Home, row.Away = stats[0], stats[1], stats[2]
	if r.u8() != 0 {
		n := r.u8()
		if n > maxTableHeadToHead {
			return TableRow{}, false
		}
		for range 2 * int(n) {
			readTableStats(r)
		}
	}
	r.take(tableRowTailSize)
	return row, r.err == nil
}

// readTableStats reads ref (u8, i32), u8 played, u8 (equal to played),
// won, drawn, lost, shoot-out losses, u16 for, u16 against, i16 points.
func readTableStats(r *reader) TableStats {
	r.u8()
	r.i32()
	var s TableStats
	s.Played = r.u8()
	r.u8()
	s.Won, s.Drawn, s.Lost, s.ShootoutLosses = r.u8(), r.u8(), r.u8(), r.u8()
	s.GoalsFor, s.GoalsAgainst = r.u16(), r.u16()
	s.Points = int16(r.u16()) //nolint:gosec // signed field
	return s
}

// validTable accepts at least minTableRows rows where each row's team is a
// TEAM index, its position is its index, overall = home + away, and played
// = won + drawn + lost + shoot-out losses. Points are never checked: the
// game stores them (deductions, carried-over points, shoot-out points).
func validTable(rows []TableRow) bool {
	if len(rows) < minTableRows {
		return false
	}
	for i, row := range rows {
		o, h, a := row.Overall, row.Home, row.Away
		if row.Team < 0 || int(row.Position) != i ||
			o.Played != h.Played+a.Played || o.Won != h.Won+a.Won || o.Drawn != h.Drawn+a.Drawn || o.Lost != h.Lost+a.Lost ||
			o.GoalsFor != h.GoalsFor+a.GoalsFor || o.GoalsAgainst != h.GoalsAgainst+a.GoalsAgainst ||
			int(o.Played) != int(o.Won)+int(o.Drawn)+int(o.Lost)+int(o.ShootoutLosses) {
			return false
		}
	}
	return true
}

func sameTeams(a, b []TableRow) bool {
	if len(a) != len(b) {
		return false
	}
	teams := make(map[int32]int, len(a))
	for _, r := range a {
		teams[r.Team]++
	}
	for _, r := range b {
		if teams[r.Team]--; teams[r.Team] < 0 {
			return false
		}
	}
	return true
}
