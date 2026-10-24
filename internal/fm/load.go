package fm

import (
	"cmp"
	"fmt"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// noTeam marks an unknown team index.
const noTeam = -1

// LoadOptions configures Load.
type LoadOptions struct {
	// GameDBDir is the game's bundled database folder (…/shared/data/database/db).
	// Nation, competition and injury names come from there; without it they
	// stay empty.
	GameDBDir string
	// PrefsDir is the game's preferences folder, the source of the currency
	// and wage period. Empty means GBP per week.
	PrefsDir string
}

// Load decodes the save at path. The save and the game's files are only
// read. Data that cannot be decoded is reported in Info.Warnings rather
// than failing the load.
func Load(path string, opts LoadOptions) (*Game, error) {
	a, err := savefile.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = a.Close() }()
	db, err := savefile.ReadGameDB(a)
	if err != nil {
		return nil, err
	}
	info := Info{SaveName: a.Directory().Name, Date: db.Header.GameDate.Time(), ManagerTeamIndex: noTeam}
	info.Money, info.Warnings = readMoney(db, opts.PrefsDir)
	if w := buildWarning(db.Header.DatabaseVersion); w != "" {
		info.Warnings = append(info.Warnings, w)
	}
	if db.SkippedPlayers > 0 {
		info.Warnings = append(info.Warnings, fmt.Sprintf("%d player records could not be decoded and are missing", db.SkippedPlayers))
	}

	if w := nationWarning(db); w != "" {
		info.Warnings = append(info.Warnings, w)
	}
	info.Warnings = append(info.Warnings, competitionWarnings(db)...)
	lang, warnings := readLang(db.Header.Label, opts.GameDBDir, db.NationCount, db.InjuryCount)
	info.Warnings = append(info.Warnings, warnings...)
	w := buildWorld(db, lang)

	if s, err := savefile.ReadSummary(a); err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("manager unknown: %v", err))
	} else {
		info.Build, info.ManagerName, info.ManagerClub = s.Build, s.ManagerName, s.ClubShortName
		if t, ok := db.FirstTeam(s.ClubUID); ok {
			info.ManagerTeamIndex = t
			info.ManagerClub = w.teamName(t)
		}
	}
	if msg := interestWarning(w, info.ManagerTeamIndex); msg != "" {
		info.Warnings = append(info.Warnings, msg)
	}
	scouted, err := readScouting(a)
	if err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("scouting knowledge unavailable, so fog of war uses only your club and reputations: %v", err))
	}
	records, err := savefile.ReadSeasonStats(a)
	if err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("season stats unavailable for every player: %v", err))
	}
	transfers, err := readTransfers(a)
	if err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("arranged transfers unavailable, so interest cannot tell who has agreed a move: %v", err))
	}
	var fixtures []savefile.Fixture
	if len(db.CompetitionKeys) > 0 {
		if fixtures, err = savefile.ReadFixtures(a); err != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("fixtures unavailable: %v", err))
		} else if fixtures == nil {
			fixtures = []savefile.Fixture{} // read, but none: not the same as unreadable
		}
	}
	var tables *savefile.LeagueTables
	if t, err := savefile.ReadLeagueTables(a, len(db.StageUIDs)); err != nil {
		info.Warnings = append(info.Warnings, fmt.Sprintf("league tables unavailable: %v", err))
	} else {
		tables = &t
	}
	raw, err := savefile.ReadTactics(a)
	tactics, warnings := checkTactics(raw, err, info.ManagerTeamIndex)
	info.Warnings = append(info.Warnings, warnings...)
	return NewGame(info, db.Players, w, PlayerExtras{Scouted: scouted, Season: records, Tactics: tactics, Transfers: transfers, Fixtures: fixtures, Tables: tables, LeagueStages: db.LeagueStageCount}), nil
}

// readTransfers is swapped in tests.
var readTransfers = savefile.ReadTransfers

// readScouting returns the human manager's stored scouting knowledge by
// player person index.
func readScouting(a *savefile.Archive) (map[uint32]uint8, error) {
	human, err := savefile.ReadHumanIndex(a)
	if err != nil {
		return nil, err
	}
	fog, err := savefile.ReadFogOfWar(a)
	if err != nil {
		return nil, err
	}
	return storedKnowledge(fog, human)
}

func storedKnowledge(fog savefile.FogOfWar, human uint32) (map[uint32]uint8, error) {
	h, ok := fog.For(human)
	if !ok {
		return nil, fmt.Errorf("fog_of_war.dat has no entry for human %d", human)
	}
	return h.Players, nil
}

type langNames struct {
	nations      map[int64]string  // by nation index
	competitions map[uint32]string // by competition uid
	injuries     map[int64]string  // by injury index
	stages       map[uint32]string // by stage uid
}

// readLang reads nation, competition and injury names for label. The save's
// nation and injury counts are needed to map their indices to names. It
// returns one warning per problem.
func readLang(label, dbDir string, saveNations, saveInjuries uint32) (langNames, []string) {
	const noNames = "nation, competition and injury names unavailable: "
	path := savefile.LangDBPath(dbDir, label)
	if path == "" {
		return langNames{}, []string{fmt.Sprintf(noNames+"no language database for %q in %q", label, dbDir)}
	}
	ldb, err := savefile.OpenLangDB(path)
	if err != nil {
		return langNames{}, []string{noNames + err.Error()}
	}
	defer func() { _ = ldb.Close() }()
	return langNamesFrom(ldb, saveNations, saveInjuries)
}

type langTables interface {
	Nations() ([]savefile.LangName, error)
	Competitions() ([]savefile.LangName, error)
	Injuries() ([]savefile.LangName, error)
	Stages() ([]savefile.LangName, error)
}

// langNamesFrom maps each language table to the save. A table that cannot
// be read or mapped is left out with a warning.
func langNamesFrom(ldb langTables, saveNations, saveInjuries uint32) (langNames, []string) {
	var out langNames
	var warnings []string
	var err error
	if out.nations, err = namesByRank("nation", "nations", ldb.Nations, saveNations); err != nil {
		warnings = append(warnings, err.Error())
	}
	if out.competitions, err = competitionNames(ldb); err != nil {
		warnings = append(warnings, err.Error())
	}
	if out.injuries, err = namesByRank("injury", "injury types", ldb.Injuries, saveInjuries); err != nil {
		warnings = append(warnings, err.Error())
	}
	if out.stages, err = namesByUID("stage", ldb.Stages); err != nil {
		warnings = append(warnings, err.Error()+", so fixture and table stages are unlabelled")
	}
	return out, warnings
}

// namesByRank reads a language table and maps it to the save by uid rank
// (see savefile.NamesByRank). kind and saveRecords name the table and the
// save's records in errors, e.g. "nation" and "nations".
func namesByRank(kind, saveRecords string, read func() ([]savefile.LangName, error), saveCount uint32) (map[int64]string, error) {
	names, err := read()
	if err != nil {
		return nil, fmt.Errorf("%s names unavailable: %w", kind, err)
	}
	out := savefile.NamesByRank(names, saveCount)
	if out == nil {
		return nil, fmt.Errorf("%s names unavailable: save has %d %s, language database %d", kind, saveCount, saveRecords, len(names))
	}
	return out, nil
}

// competitionNames returns competition names by uid.
func competitionNames(ldb langTables) (map[uint32]string, error) {
	return namesByUID("competition", ldb.Competitions)
}

// namesByUID reads a language table and returns its names by uid. kind
// names the table in errors, e.g. "stage".
func namesByUID(kind string, read func() ([]savefile.LangName, error)) (map[uint32]string, error) {
	names, err := read()
	if err != nil {
		return nil, fmt.Errorf("%s names unavailable: %w", kind, err)
	}
	out := make(map[uint32]string, len(names))
	for _, n := range names {
		out[n.UID] = n.Name
	}
	return out, nil
}

// buildWorld resolves every team to a name: the club's or nation's name,
// with "Women" for women's sides and a label such as "U21" for other teams.
// Every club gets its short name (else its long name) the same way.
func buildWorld(db *savefile.GameDB, lang langNames) World {
	w := World{
		Teams: make(map[int64]*Team, len(db.Teams)), Clubs: make(map[int64]Club, len(db.Clubs)),
		Nations: lang.nations, Injuries: lang.injuries, NationScales: nationScales(db),
	}
	for i, c := range db.Clubs {
		w.Clubs[int64(i)] = Club{Name: womensName(cmp.Or(c.ShortName, c.Name, "Unnamed club"), c.Women), FirstTeam: noTeam}
	}
	w.CompetitionUIDs, w.Keys = db.CompetitionUIDs, db.CompetitionKeys
	w.Stages = make(map[int64]string, len(db.StageUIDs))
	for i, uid := range db.StageUIDs {
		if name := lang.stages[uid]; name != "" {
			w.Stages[int64(i)] = name
		}
	}
	w.Competitions = make(map[int64]string, len(lang.competitions))
	for i, uid := range db.CompetitionUIDs {
		if name := lang.competitions[uid]; name != "" {
			w.Competitions[int64(i)] = name
		}
	}
	for _, t := range db.Teams {
		team := &Team{Index: int64(t.Index), ClubIndex: noTeam, NationIndex: noTeam, Reputation: int(t.Reputation), National: t.National, HasManager: t.Manager >= 0, divisionIndex: noTeam}
		if w.isCompetition(int64(t.Division)) {
			team.divisionIndex = int64(t.Division)
		}
		team.Division = w.Competitions[int64(t.Division)]
		label := t.Label()
		switch c := int(t.Container); {
		case t.National:
			// Men's container index = nation index; women's containers follow,
			// so nation index = container - nation count (guess, supported by
			// reputations: 390 is England).
			nation := int64(c)
			if team.Women = c >= int(db.NationCount); team.Women {
				nation -= int64(db.NationCount)
			}
			team.NationIndex, team.Nation = nation, lang.nations[nation]
			if label == "" && t.TeamType != 0 {
				label = "(other team)"
			}
			team.Name = joinLabel(womensName(cmp.Or(team.Nation, "National team"), team.Women), label)
		case c >= 0 && c < len(db.Clubs):
			club := db.Clubs[c]
			team.ClubIndex, team.Club, team.ShortName, team.Women = int64(c), club.Name, club.ShortName, club.Women
			team.NationIndex = int64(club.NationIndex)
			team.Nation = lang.nations[team.NationIndex]
			// The first team shares the club's uid; it alone carries the
			// club's finances and its bare name.
			if t.UID == club.UID {
				if c, ok := w.Clubs[team.ClubIndex]; ok {
					c.FirstTeam = team.Index
					w.Clubs[team.ClubIndex] = c
				}
				team.BankBalance = int64(club.Balance)
				if club.Budgets != nil {
					team.Budgets = newBudgets(*club.Budgets)
				}
			} else if label == "" {
				label = "(other team)"
			}
			team.Name = joinLabel(womensName(cmp.Or(club.Name, club.ShortName, "Unnamed club"), club.Women), label)
		}
		w.Teams[team.Index] = team
	}
	return w
}

// nationScales returns the reputation rule's inputs by nation index; nil
// when the save's nations did not decode.
func nationScales(db *savefile.GameDB) map[int64]NationScale {
	if len(db.Nations) == 0 {
		return nil
	}
	out := make(map[int64]NationScale, len(db.Nations))
	for i, n := range db.Nations {
		s := NationScale{Factor: n.MensFactor}
		if c := int(n.Continent); c >= 0 && c < len(db.Continents) {
			s.ContinentStrength, s.HasContinent = db.Continents[c].Strength, true
		}
		out[int64(i)] = s
	}
	return out
}

// competitionWarnings reports why competition keys or stage names are missing.
func competitionWarnings(db *savefile.GameDB) []string {
	var out []string
	if db.CompetitionKeysErr != nil {
		out = append(out, fmt.Sprintf("competition keys unavailable, so fixtures cannot be read: %v", db.CompetitionKeysErr))
	}
	if db.StagesErr != nil {
		out = append(out, fmt.Sprintf("stage names unavailable, so fixture and table stages are unlabelled: %v", db.StagesErr))
	}
	return out
}

// nationWarning reports why reputation levels are missing; "" when they are not.
func nationWarning(db *savefile.GameDB) string {
	if db.NationsErr == nil {
		return ""
	}
	return fmt.Sprintf("reputation levels unavailable: %v", db.NationsErr)
}

// womensName marks women's sides, which otherwise share their names with
// the men's.
func womensName(name string, women bool) string {
	if women {
		return name + " Women"
	}
	return name
}

func joinLabel(name, label string) string {
	if label == "" {
		return name
	}
	return name + " " + label
}

func buildWarning(dbVersion uint16) string {
	if savefile.VerifiedDatabaseVersion(dbVersion) {
		return ""
	}
	return fmt.Sprintf("database version %d has not been checked; player values may be decoded wrongly", dbVersion)
}
