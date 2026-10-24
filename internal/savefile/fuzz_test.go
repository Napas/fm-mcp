package savefile

import (
	"bytes"
	"testing"
)

// The fuzz targets assert that hostile input yields errors, not panics.

func FuzzNewArchive(f *testing.F) {
	f.Add(buildSave(f, testInner{path: "game_info.dat", data: innerFile("tad.", 46, "26.2.0")}))
	f.Fuzz(func(t *testing.T, b []byte) {
		a, err := NewArchive(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return
		}
		defer func() { _ = a.Close() }()
		for _, e := range a.Directory().Files {
			_, _ = a.ReadFile(e.Path)
		}
	})
}

func FuzzParseDirectory(f *testing.F) {
	f.Add(testDir{name: "s", files: []testFile{{name: "a", ext: ".dat", size: 3}}, subdirs: []testDir{{name: "rgman"}}}.encode(nil))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ParseDirectory(b)
	})
}

func FuzzParsePlayer(f *testing.F) {
	basic := basicPart{first: 1, last: 1, common: -1, dob: Date{Raw: 100, Year: 1998}, nation: 143}
	main := testContract{person: 77, team: 547, wage: 147000, start: Date{Raw: 179, Year: 2023}, end: Date{Raw: 182, Year: 2028}, signed: Date{Raw: 179, Year: 2023}}
	block := append(injuryList(Injury{ID: 27, Start: Date{Raw: 188, Year: 2025}, DaysMin: 1, DaysMax: 2}),
		contractBlock(fullSlot(main), emptySlot(), emptySlot(), emptySlot(), emptySlot())...)
	f.Add(playerRecordWith(77, 9, basic, block))
	knock := Injury{ID: 84, Start: Date{Raw: 0x6b50, Year: 2035}, DaysMin: 0, DaysMax: 2}
	f.Add(playerRecordWith(77, 9, basic, append([]byte{1}, injuryItem(knock, veigaMatch)...)))
	withClubs := basic
	withClubs.relations = relations(relationItem(1115, 1, relationClub, relationFavoured, 100), relationItem(6, 0, relationClub, 72, 1))
	f.Add(playerRecordWith(77, 9, withClubs, block))
	f.Fuzz(func(t *testing.T, rec []byte) {
		_, _ = parsePlayer(rec, Record{Tag: 2, Index: 77, UID: 9, Start: 0, End: len(rec)}, testNames)
	})
}

func FuzzReadTables(f *testing.F) {
	var recs []byte
	recs = testClub{index: 0, uid: 728, name: "Tottenham Hotspur", short: "Spurs", nation: 139, finance: 1, teams: []int32{0}}.encode(recs)
	f.Add(tableBytes(1, recs), uint32(1))
	f.Fuzz(func(t *testing.T, body []byte, count uint32) {
		_, _, _ = readClubTable(body, count)
		_, _, _ = readTeamTable(body, count, 3733)
		_, _, _ = splitTable(body, 0, personTrailer)
		_, _, _ = splitTable(body, 0, competitionTrailer)
		_, _, _ = findNameTables(body, 1)
	})
}

func FuzzReadCompetitionKeys(f *testing.F) {
	f.Add(tableBytes(1, competitionKeyRecord(0, CompetitionKey{8, 236, 17, 2, -1})))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = readCompetitionKeys(body)
		_, _ = readStageNames(body, 1)
	})
}

func FuzzParseFixtures(f *testing.F) {
	f.Add(fixtureFile(fixtureList(2025, fixtureItem(0, playedFixture, 1), fixtureItem(1, upcomingFixture, 0))))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = parseFixtures(body)
	})
}

func FuzzParseLeagueTables(f *testing.F) {
	f.Add(tableFile(74, testTableRows))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = parseLeagueTables(body, 1, 285)
	})
}

func FuzzParseFogOfWar(f *testing.F) {
	f.Add(fogBody(fogHuman{7, []fogItem{{85, 65605}}}))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseFogOfWar(b)
	})
}

func FuzzReadCurrencyTable(f *testing.F) {
	recs := testCurrency{index: 0, uid: 28, name: "European Euro", short: "Euro", symbol: "€", rate: 1.15713, history: []float32{1.1}}.encode(nil)
	f.Add(tableBytes(1, recs), uint32(1))
	f.Fuzz(func(t *testing.T, body []byte, count uint32) {
		_, _, _ = readCurrencyTable(body, count)
	})
}

func FuzzParseSeasonStats(f *testing.F) {
	rec := statsRecord{person: 146891, teams: []statsTeam{{team: 20758}}, seasons: 1}
	rec.scopes[seasonScope] = &ashby
	f.Add(statsBody(rec))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseSeasonStats(b)
	})
}

func FuzzReadNationTables(f *testing.F) {
	f.Add(continentBody(1, testContinent{index: 0, strength: 0.85}, testContinent{index: 1, strength: 0.99}), uint32(1))
	f.Add(nationBody(testNations...), uint32(3))
	f.Add(tableBytes(2, containerRecord(containerRecord(nil, 0, 5, 105), 1, 99, 50)), uint32(2))
	f.Fuzz(func(t *testing.T, body []byte, count uint32) {
		_, _ = readContinents(body, count)
		ns, _ := readNations(body, count, maxContinents)
		_ = readNationFactors(body, count, append(ns, Nation{Index: 0, UID: 5}))
	})
}

func FuzzReadTransfers(f *testing.F) {
	f.Add(transferBody(transferRecord(marcoTransfer, feb4, jul1), transferRecord(transferHead(3, 184193, 649, 10810, 15), feb4, jul1)))
	f.Fuzz(func(t *testing.T, b []byte) {
		ts, _ := parseTransfers(b)
		if len(ts) > len(b)/transferBytesPerRecord {
			t.Fatalf("%d records from %d bytes", len(ts), len(b))
		}
	})
}
