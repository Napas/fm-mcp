package fm

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// ErrInvalidFilter means a filter or name query is invalid, e.g. an unknown
// position, attribute or sort key, or a name without letters or digits.
var ErrInvalidFilter = errors.New("fm: invalid filter")

const (
	defaultLimit = 25
	maxLimit     = 200
)

// Info describes the loaded save.
type Info struct {
	SaveName         string    `json:"save_name,omitempty"`
	Build            string    `json:"build,omitempty"`
	Date             time.Time `json:"game_date"`
	ManagerName      string    `json:"manager_name,omitempty"`
	ManagerTeamIndex int64     `json:"manager_team_index"`
	ManagerClub      string    `json:"manager_club,omitempty"`
	Warnings         []string  `json:"warnings,omitempty"` // data that could not be decoded
	Money            Money     `json:"-"`                  // the user's currency and wage period
}

// Game holds the players of one save and answers queries over them.
type Game struct {
	Info    Info
	players []*Player
	byUID   map[uint32]*Player
	world   World
	// teamNames holds name and short name words by team index.
	teamNames map[int64][]string
	calledUp  map[int64]bool // national teams with call-ups
	tactic    *Tactic        // the human manager's current tactic; nil when none
	// lastLineups: every team's last match by team index; nil when unreadable.
	lastLineups map[int32]savefile.LastLineup
	// lineupPlayers: players of the selection and last lineups by person index.
	lineupPlayers map[uint32]*Player
	aux           []playerAux     // by Player.auxIdx
	hasSeason     bool            // false when the save's season stats could not be read
	hasFixtures   bool            // false when fix_man.dat could not be read
	hasTables     bool            // false when the comp files could not be read
	fixtures      []storedFixture // in date order
	tables        []Table
	tableWarnings []string // why some tables are missing
}

// playerAux is per-player data kept outside Player to keep Player within 512
// bytes. NewGame fills it in player order with data derived at load, and
// nothing writes it afterwards. Values computed on demand from Player fields
// stay methods.
type playerAux struct {
	roles    roleInputs
	favoured []FavouredClub
	season   savefile.SeasonRecord // this season's scopes; profiles convert them
	interest interestInputs
}

func (g *Game) auxOf(p *Player) *playerAux { return &g.aux[p.auxIdx] }

// PlayerExtras is save data stored outside the player records: player data
// (maps by person index), tactics, transfers, fixtures and league tables.
// nil means none or, where a field says so, unreadable.
type PlayerExtras struct {
	Scouted   map[uint32]uint8                 // the human manager's stored scouting knowledge
	Season    map[uint32]savefile.SeasonRecord // current season totals (rgman/player_stats.dat)
	Tactics   *savefile.Tactics                // the human manager's tactics and lineups (tactics_man.dat)
	Transfers []savefile.Transfer              // transfer_man.dat; nil when it could not be read
	Fixtures  []savefile.Fixture               // fix_man.dat; nil when it could not be read
	Tables    *savefile.LeagueTables           // comp files' tables; nil when they could not be read
	// LeagueStages is the LEAGUE_STAGE memory pool's count, which tables are
	// counted against.
	LeagueStages uint32
}

// NewGame builds a Game from decoded player records, the teams and nations
// they refer to, and extras. It adds warnings about fixtures and tables it
// cannot serve to info.Warnings.
func NewGame(info Info, raw []savefile.Player, w World, extras PlayerExtras) *Game {
	g := &Game{
		Info: info, players: make([]*Player, 0, len(raw)), byUID: make(map[uint32]*Player, len(raw)),
		world: w, teamNames: make(map[int64][]string, len(w.Teams)), calledUp: make(map[int64]bool),
		aux: make([]playerAux, 0, len(raw)), hasSeason: extras.Season != nil,
		hasFixtures: extras.Fixtures != nil, hasTables: extras.Tables != nil,
	}
	for i, t := range w.Teams {
		g.teamNames[i] = nameWords(t.Name, t.ShortName)
	}
	transfers := make(map[uint32]savefile.Transfer, len(extras.Transfers))
	for _, t := range extras.Transfers {
		transfers[t.ID] = t
	}
	own := w.clubTeams(info.ManagerTeamIndex)
	inLineup := lineupPersons(extras.Tactics)
	for _, r := range raw {
		p := newPlayer(r, info.Date, w)
		if _, ok := inLineup[r.Index]; ok {
			inLineup[r.Index] = p
		}
		p.knowledge = uint8(knowledgeOf(p, own, extras.Scouted[r.Index])) //nolint:gosec // 0..100
		rec := extras.Season[r.Index]
		if rec.Club != nil {
			p.Season = seasonStats(*rec.Club)
		}
		p.auxIdx = uint32(len(g.aux)) //nolint:gosec // fewer players than 2^32
		g.aux = append(g.aux, playerAux{roles: newRoleInputs(r), favoured: favouredClubs(r.ClubAffinities, w.Clubs), season: rec, interest: newInterestInputs(r, transfers)})
		g.players = append(g.players, p)
		g.byUID[p.UID] = p
		if p.NationalTeamIndex != nil {
			g.calledUp[*p.NationalTeamIndex] = true
		}
	}
	maps.DeleteFunc(inLineup, func(_ uint32, p *Player) bool { return p == nil })
	g.tactic = newTactic(extras.Tactics, w, inLineup, g.aux)
	g.lineupPlayers = inLineup
	if extras.Tactics != nil {
		g.lastLineups = extras.Tactics.LastLineups
	}
	g.tables, g.tableWarnings = newTables(extras.Tables, extras.LeagueStages, w)
	g.Info.Warnings = append(g.Info.Warnings, g.tableWarnings...)
	var unresolved int
	if g.fixtures, unresolved = resolveFixtures(extras.Fixtures, w); unresolved > 0 {
		g.Info.Warnings = append(g.Info.Warnings, fmt.Sprintf("fixtures left out because their competition key does not resolve: %d", unresolved))
	}
	relabel(g.tables, g.fixtures, w)
	return g
}

// PlayerCount is the number of players in the save.
func (g *Game) PlayerCount() int { return len(g.players) }

// teamsNamed returns the indexes of teams where each of words starts a name word.
func (g *Game) teamsNamed(words []string) map[int64]bool {
	out := make(map[int64]bool)
	for i, names := range g.teamNames {
		if matchesNameStarts(names, words) {
			out[i] = true
		}
	}
	return out
}

// clubTeamsNamed is teamsNamed without national teams. It also returns the
// dropped national teams that have call-ups, most reputable first.
func (g *Game) clubTeamsNamed(words []string) (map[int64]bool, []*Team) {
	out := g.teamsNamed(words)
	var national []*Team
	maps.DeleteFunc(out, func(i int64, _ bool) bool {
		t := g.world.Teams[i]
		if t.National && g.calledUp[i] {
			national = append(national, t)
		}
		return t.National
	})
	slices.SortFunc(national, byReputation)
	return out, national
}

// findTeams returns teams matching query (see teamsNamed), most reputable first, money in GBP.
func (g *Game) findTeams(query string, limit int) ([]*Team, error) {
	words, err := queryWords("name", query)
	if err != nil {
		return nil, err
	}
	return truncate(g.teamsByReputation(g.teamsNamed(words)), limit), nil
}

// teamsByReputation returns the teams with the given indexes, most reputable
// first.
func (g *Game) teamsByReputation(indexes map[int64]bool) []*Team {
	out := make([]*Team, 0, len(indexes))
	for i := range indexes {
		out = append(out, g.world.Teams[i])
	}
	slices.SortFunc(out, byReputation)
	return out
}

// byReputation orders teams most reputable first, then by index.
func byReputation(a, b *Team) int {
	return cmp.Or(cmp.Compare(b.Reputation, a.Reputation), cmp.Compare(a.Index, b.Index))
}

// Filter selects players for Search. Zero values do not filter, except
// Gender: without a team or club filter it defaults to the gender of the
// manager's team ("any" for both).
type Filter struct {
	Gender              string
	Position            string
	Role                string // an in-possession role; needs Position, ranks by role ability
	MinRoleAbility      int    // needs Role
	MinAge              int
	MaxAge              int
	MinHeightCM         int
	MaxHeightCM         int
	MinCurrentAbility   int
	MaxCurrentAbility   int
	MinPotentialAbility int
	Club                string
	TeamIndex           *int64
	ContractedTeamIndex *int64 // main contract with this team, including players out on loan
	ExcludeTeamIndex    *int64
	Nation              string
	NationIndex         *int64
	MaxValue            int64 // in the user's currency
	MaxWage             int64 // in the user's currency and wage period
	MaxReleaseClause    int64 // lowest minimum fee release clause, in the user's currency
	ContractExpiresBy   string
	ExcludeInjured      bool
	FreeAgentsOnly      bool
	MinMonthsAtClub     int // full months at his current club
	MinSeasonMinutes    int // minutes this season in club competitions
	MinAttributes       map[string]int
	MinPer90            map[string]float64 // SeasonDetail JSON name → minimum per 90 minutes
	Traits              []string           // a player needs all
	Personalities       []string           // a player needs one
	Interest            string             // "", "transfer", "loan" or "transfer_or_loan": leave out players who certainly won't join the manager's club
	SortBy              string
	Ascending           bool
	Limit               int
}

// checkRefs rejects team and nation references that match nothing, so an
// empty result is never caused by a typo.
func (g *Game) checkRefs(f Filter) error {
	for _, ref := range []*int64{f.TeamIndex, f.ContractedTeamIndex, f.ExcludeTeamIndex} {
		if ref != nil {
			if err := g.checkTeam(*ref); err != nil {
				return err
			}
		}
	}
	if f.NationIndex != nil && g.world.Nations != nil && g.world.Nations[*f.NationIndex] == "" {
		return fmt.Errorf("%w: no nation with nation_index %d", ErrInvalidFilter, *f.NationIndex)
	}
	if f.Nation == "" {
		return nil
	}
	if _, err := queryWords("nation", f.Nation); err != nil {
		return err
	}
	if len(g.world.Nations) == 0 {
		return fmt.Errorf("%w: nation names are not available for this save; filter by nation_index instead", ErrInvalidFilter)
	}
	if !g.world.hasNation(nationKey(f.Nation)) {
		return fmt.Errorf(`%w: nation %q matches no nation name; use nation_index or the game's official name, e.g. "United States", "Netherlands", "Türkiye"`, ErrInvalidFilter, f.Nation)
	}
	return nil
}

// checkTeam rejects a team index that matches no team.
func (g *Game) checkTeam(index int64) error {
	if g.world.Teams[index] == nil {
		return fmt.Errorf("%w: no team with team_index %d; use find_clubs to look one up", ErrInvalidFilter, index)
	}
	return nil
}

// queryTeam checks the team of a team or competition query. With neither
// set it is the manager's team.
func (g *Game) queryTeam(team, competition *int64) (*int64, error) {
	if team == nil && competition == nil {
		if g.Info.ManagerTeamIndex < 0 {
			return nil, fmt.Errorf("%w: the manager's club is not known for this save; pass team_index (from find_clubs or a player's team_index)", ErrInvalidFilter)
		}
		team = &g.Info.ManagerTeamIndex
	}
	if team != nil {
		if err := g.checkTeam(*team); err != nil {
			return nil, err
		}
	}
	if competition != nil && !g.world.isCompetition(*competition) {
		return nil, fmt.Errorf("%w: no competition with competition_index %d; pass a competition name or an index from get_fixtures", ErrInvalidFilter, *competition)
	}
	return team, nil
}

// managerWomen reports whether the manager's team is a women's team; nil if unknown.
func (g *Game) managerWomen() *bool {
	t, ok := g.world.Teams[g.Info.ManagerTeamIndex]
	if !ok {
		return nil
	}
	women := t.Women
	return &women
}

// genderFilter returns nil for both genders, or whether to keep women.
func genderFilter(g string) (*bool, error) {
	switch strings.ToLower(g) {
	case "", "any", "both", "all":
		return nil, nil
	case "men", "male", "m":
		women := false
		return &women, nil
	case "women", "female", "f":
		women := true
		return &women, nil
	}
	return nil, fmt.Errorf("%w: gender %q", ErrInvalidFilter, g)
}

func truncate[T any](ps []T, limit int) []T {
	if limit <= 0 {
		limit = defaultLimit
	}
	return ps[:min(len(ps), limit, maxLimit)]
}
