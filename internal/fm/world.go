package fm

import (
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Team is a squad: a club's first team, a reserve or youth team, or a
// national team. Players belong to teams.
type Team struct {
	Index       int64  `json:"team_index"`
	ClubIndex   int64  `json:"-"`    // the club owning the team (0 is a real club); noTeam for national teams
	NationIndex int64  `json:"-"`    // its club's nation, or the national team's; noTeam when unknown
	Name        string `json:"name"` // e.g. "Tottenham Hotspur U21"
	Club        string `json:"club,omitempty"`
	ShortName   string `json:"short_name,omitempty"`
	Nation      string `json:"nation,omitempty"`
	Division    string `json:"division,omitempty"`
	// divisionIndex is the division's COMP index; -1 when none.
	divisionIndex int64
	Reputation    int   `json:"reputation"`
	Women         bool  `json:"women,omitempty"`
	National      bool  `json:"national,omitempty"`
	HasManager    bool  `json:"-"`                      // the save names a manager for the team
	BankBalance   int64 `json:"bank_balance,omitempty"` // GBP in the save; the user's currency from a View, which hides other clubs' under fog of war
	// Budgets are set on the club's first team; nil without full finances.
	// Only View.Budgets serves them, for the manager's club.
	Budgets *Budgets `json:"-"`
}

// Budgets are a club's transfer and wage budgets. On a Team: GBP, wage per
// week. From View.Budgets: the user's currency and wage period.
type Budgets struct {
	TransferBudget int64 `json:"transfer_budget"` // what the club can spend on fees now; may be negative
	WageBudget     int64 `json:"wage_budget"`     // the total; FM's available amount subtracts the current wage bill
}

// newBudgets reads b as the engine does: a transfer budget of £1–499 is 0
// (getter at 0x1318b10 and about 40 other sites).
func newBudgets(b savefile.Budgets) *Budgets {
	transfer := int64(b.Transfer)
	if transfer >= 1 && transfer <= 499 {
		transfer = 0
	}
	return &Budgets{TransferBudget: transfer, WageBudget: int64(b.WeeklyWage)}
}

// Club is a CLUB record as favoured clubs show it.
type Club struct {
	Name      string // the short name (in game: "Osijek" for NK Osijek), else the name; "Women" added for women's clubs
	FirstTeam int64  // noTeam when the club has none
}

// World holds what player records refer to: teams by team index, clubs by
// CLUB index, nation names and scales by nation index, injury names by
// injury index and competition names by COMP index. Missing entries leave
// names empty.
type World struct {
	Teams        map[int64]*Team
	Clubs        map[int64]Club
	Nations      map[int64]string
	Injuries     map[int64]string
	Competitions map[int64]string
	NationScales map[int64]NationScale // nil when the save's nations do not decode
	// CompetitionUIDs are the COMP records' uids by COMP index; names may be missing.
	CompetitionUIDs []uint32
	Stages          map[int64]string          // stage names by STAGE_NAME index
	Keys            []savefile.CompetitionKey // ACTUAL_COMP_ID records by key index
}

// isFirstTeam reports whether team i is its club's first team.
func (w World) isFirstTeam(i int64) bool {
	club, ok := w.clubOf(i)
	if !ok {
		return false
	}
	c, ok := w.Clubs[club]
	return ok && c.FirstTeam == i
}

func (w World) teamName(i int64) string {
	if t := w.Teams[i]; t != nil {
		return t.Name
	}
	return ""
}

// hasNation reports whether a nation name's key contains key.
func (w World) hasNation(key string) bool {
	for _, name := range w.Nations {
		if strings.Contains(nationKey(name), key) {
			return true
		}
	}
	return false
}

// clubTeams returns the teams of the club owning team: its first team and
// its reserve, B and youth sides. Nil for national or unknown teams.
func (w World) clubTeams(team int64) map[int64]bool {
	club, ok := w.clubOf(team)
	if !ok {
		return nil
	}
	out := make(map[int64]bool)
	for _, o := range w.Teams {
		if c, ok := w.clubOf(o.Index); ok && c == club {
			out[o.Index] = true
		}
	}
	return out
}

// sameClub reports whether teams a and b belong to one club. False for
// national or unknown teams.
func (w World) sameClub(a, b int64) bool {
	ca, okA := w.clubOf(a)
	cb, okB := w.clubOf(b)
	return okA && okB && ca == cb
}

// clubOf returns the club owning team; false for national or unknown teams.
func (w World) clubOf(team int64) (int64, bool) {
	t := w.Teams[team]
	if t == nil || t.National || t.ClubIndex == noTeam {
		return 0, false
	}
	return t.ClubIndex, true
}
