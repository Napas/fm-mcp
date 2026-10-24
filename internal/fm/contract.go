package fm

import (
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Contract is a contract as saved: GBP, wage per week.
type Contract struct {
	TeamIndex  int64
	Team       string
	WeeklyWage int64
	Start      time.Time // signed date, the contract screen's Start (see contractStart); zero when unset
	Joined     time.Time // joined the contract's team; zero when unset
	End        time.Time // zero when unset
	Clauses    []ClauseInfo
	// ReleaseFee is the lowest minimum fee release clause in GBP; 0 when none.
	ReleaseFee int64
	// SquadNumber is the number registered with the contract's team; 0 when
	// none.
	SquadNumber int
}

// ContractInfo is a Contract in the user's currency and wage period.
type ContractInfo struct {
	TeamIndex int64        `json:"team_index"`
	Team      string       `json:"team,omitempty"`
	Wage      *int64       `json:"wage,omitempty"` // nil when fog of war hides it
	Start     time.Time    `json:"start,omitzero"`
	Joined    time.Time    `json:"joined,omitzero"`
	End       time.Time    `json:"end,omitzero"`
	Clauses   []ClauseInfo `json:"clauses,omitempty"`
}

// ClauseInfo is a contract clause or bonus. Names are inferred from the
// game's negotiation strings and from which players hold them; treat them
// as guesses.
type ClauseInfo struct {
	Name    string `json:"name"`
	Amount  int64  `json:"amount,omitempty"`  // a fee or a bonus: GBP in a Contract, the user's currency in a ContractInfo
	Percent int    `json:"percent,omitempty"` // for wage rises, sell-on…
	Count   int    `json:"count,omitempty"`   // years or games, where the clause has one
}

func newContract(c *savefile.Contract, w World) *Contract {
	if c == nil {
		return nil
	}
	out := &Contract{TeamIndex: c.TeamIndex, Team: w.teamName(c.TeamIndex), WeeklyWage: int64(c.WeeklyWage), Start: contractStart(c), Joined: knownDate(c.Start), End: knownDate(c.End), SquadNumber: squadNumber(c)}
	for _, cl := range c.Clauses {
		out.Clauses = append(out.Clauses, clauseInfo(cl))
		if v, ok := cl.Amount(); ok && v > 0 && cl.MinimumFeeRelease() && (out.ReleaseFee == 0 || v < out.ReleaseFee) {
			out.ReleaseFee = v
		}
	}
	for _, b := range c.Bonuses {
		if info, ok := bonusInfo(b, w); ok {
			out.Clauses = append(out.Clauses, info)
		}
	}
	return out
}

// contractStart is the signed date, or the joining date when the signed date
// is unset (no contract seen has a joining date without a signed date).
func contractStart(c *savefile.Contract) time.Time {
	if d := knownDate(c.Signed); !d.IsZero() {
		return d
	}
	return knownDate(c.Start)
}

// Limits of the remaining loyalty bonus rule (game_plugin 26.3.2, 0x34b5a24).
const (
	loyaltyNoEndDays    = 1825  // days counted when the contract's end is 1900-01-01
	loyaltyMaxDays      = 30000 // days counted at most
	loyaltyMinRemaining = 100   // GBP; this much or less counts as nothing
)

// loyaltyRemaining is c's loyalty bonus left on date on, in GBP (game rule
// 0x34b5a24): the weekly instalment times the whole weeks to the contract's
// end; an end of 1900-01-01 counts loyaltyNoEndDays. 0 when the end is
// unset (any other 1900 date), under 7 days remain or the result is 100 or
// less.
func loyaltyRemaining(c *savefile.Contract, on time.Time) int64 {
	if c == nil || c.End.IsZero() {
		return 0
	}
	end := c.End.Time()
	var days int64
	switch {
	case c.End.Year != unsetYear:
		days = min(int64(end.Sub(on)/(24*time.Hour)), loyaltyMaxDays)
	case end.YearDay() == 1:
		days = loyaltyNoEndDays
	default:
		return 0
	}
	if days < 7 {
		return 0
	}
	if rem := int64(c.WeeklyLoyalty) * (days / 7); rem > loyaltyMinRemaining {
		return rem
	}
	return 0
}

// bonusInfo names a competition bonus after its target competition, e.g.
// "UEFA Champions League qualification bonus". Undecoded types are left out.
func bonusInfo(b savefile.CompetitionBonus, w World) (ClauseInfo, bool) {
	name, ok := b.Name()
	if !ok {
		return ClauseInfo{}, false
	}
	if target := w.Competitions[int64(b.Target)]; target != "" {
		name = target + " " + name
	}
	out := ClauseInfo{Name: name}
	if v, ok := b.Amount(); ok {
		out.Amount = v
	}
	return out, true
}

func clauseInfo(c savefile.Clause) ClauseInfo {
	out := ClauseInfo{Name: c.Name()}
	if v, ok := c.Amount(); ok {
		out.Amount = v
	}
	if v, ok := c.Percent(); ok {
		out.Percent = v
	} else if v, ok := c.Count(); ok {
		out.Count = v
	}
	return out
}

// InjuryInfo is a current injury with its expected return window.
type InjuryInfo struct {
	ID             int       `json:"injury_id"`
	Name           string    `json:"name,omitempty"` // e.g. "bruised ankle"; empty without the game's database
	Since          time.Time `json:"since"`
	ReturnEarliest time.Time `json:"return_earliest"`
	ReturnLatest   time.Time `json:"return_latest"`
}

// injuryInfo returns the injury as the game shows it on date on. The game
// counts days left from the next day ("1–2 days" for 0–2 stored), so no
// return date is earlier than that. Before the stored earliest return, the
// latest is one day later than stored.
func injuryInfo(in savefile.Injury, on time.Time, w World) InjuryInfo {
	start := in.Start.Time()
	storedEarliest := start.AddDate(0, 0, int(in.DaysMin))
	earliest := laterOf(storedEarliest, on.AddDate(0, 0, 1))
	latest := laterOf(start.AddDate(0, 0, int(in.DaysMax)), earliest)
	if storedEarliest.After(on) {
		latest = latest.AddDate(0, 0, 1)
	}
	return InjuryInfo{
		ID:             int(in.ID),
		Name:           w.Injuries[int64(in.ID)],
		Since:          start,
		ReturnEarliest: earliest,
		ReturnLatest:   latest,
	}
}

func laterOf(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
