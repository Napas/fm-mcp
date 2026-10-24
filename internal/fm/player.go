// Package fm is the domain layer: players and clubs from a decoded save,
// with the derived values an assistant manager works with (age, 1-20
// attributes, best positions) and queries over them.
package fm

import (
	"slices"
	"strings"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// naturalRating is the position rating from which a player counts as
// accomplished in that position (20 = natural).
const naturalRating = 15

// Reputation holds the home, current and world reputation, 0..10000
// (verified by code).
type Reputation struct {
	Home    int `json:"home"`
	Current int `json:"current"`
	World   int `json:"world"`
}

// PositionRating is a player's rating, 1..20, in one position.
type PositionRating struct {
	Position string `json:"position"`
	Rating   int    `json:"rating"`
}

// PlayerBasics are the facts about a player that fog of war never hides.
type PlayerBasics struct {
	UID         uint32    `json:"uid"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name,omitempty"`
	BirthDate   time.Time `json:"birth_date"`
	Age         int       `json:"age"`
	NationIndex int64     `json:"nation_index"`
	Nation      string    `json:"nation,omitempty"`
	TeamIndex   int64     `json:"team_index"`
	Club        string    `json:"club,omitempty"`
	// SquadNumber is the number registered with his current contract's team
	// (his loan club while on loan); 0 when none.
	SquadNumber int `json:"squad_number,omitempty"`
	// The national team the player is called up to; he still plays for TeamIndex.
	NationalTeamIndex *int64           `json:"national_team_index,omitempty"`
	NationalTeam      string           `json:"national_team,omitempty"`
	Reputation        Reputation       `json:"reputation"`
	Positions         []PositionRating `json:"positions"`
	PlayerCoach       bool             `json:"player_coach,omitempty"`
	Female            bool             `json:"female,omitempty"`
	HeightCM          int              `json:"height_cm,omitempty"`
	Condition         float64          `json:"condition_pct"`         // guess
	Sharpness         float64          `json:"match_sharpness_pct"`   // guess
	JoinedClub        *time.Time       `json:"joined_club,omitempty"` // nil when the save has none
	Injuries          []InjuryInfo     `json:"injuries,omitempty"`
	Season            *SeasonStats     `json:"season,omitempty"` // nil without a match this season
}

// Player is a player as the save stores him. A View turns him into the
// Profile the human manager may see.
type Player struct {
	PlayerBasics
	CurrentAbility   int
	PotentialAbility int
	Attributes       Attributes // 1..20
	Value            int64      // GBP; 0 when NotForSale, -1 when unknown (free agents)
	NotForSale       bool       // the value is the game's cap, shown in game as "Not for Sale"
	// Small fields sit in NotForSale's padding to keep Player within 512 bytes.
	knowledge      uint8 // the human manager's effective knowledge, 0..100
	negativeLabels bool  // personality and media handling may give negative labels: the save's flag bit 3
	devoted        bool  // see isDevoted
	// Playing times of the current contract (see setSquadStatus); 0 when unknown.
	playingTime, originalPlayingTime, actualPlayingTime savefile.PlayingTime
	auxIdx                                              uint32    // index of the player's data in Game.aux
	Contract                                            *Contract // nil for free agents
	Loan                                                *Contract // set while on loan at TeamIndex
	FutureContract                                      *Contract // pre-agreed move
	// LoyaltyBonusRemaining is the main contract's unpaid loyalty bonus, GBP.
	LoyaltyBonusRemaining int64

	traits    uint64   // savefile.Player.Traits
	names     []string // normalized name words, for matching
	nationKey string   // see nationKey
}

// notForSaleValue is the game's value cap. Verified in game: players saved
// with it show "Not for Sale".
const notForSaleValue = 300_000_000

// unsetYear is the year of unset dates, e.g. the joined date of loanees and
// free agents (1900-01-01) and some contract ends (1900-01-02).
const unsetYear = 1900

// knownDate converts d; unset gives the zero time.
func knownDate(d savefile.Date) time.Time {
	if d.IsZero() || d.Year == unsetYear {
		return time.Time{}
	}
	return d.Time()
}

func newPlayer(raw savefile.Player, on time.Time, w World) *Player {
	p := &Player{
		PlayerBasics: PlayerBasics{
			UID:         raw.UID,
			Name:        displayName(raw),
			FullName:    raw.FullName,
			BirthDate:   raw.BirthDate.Time(),
			NationIndex: raw.NationIndex,
			Nation:      w.Nations[raw.NationIndex],
			TeamIndex:   raw.TeamIndex,
			Club:        w.teamName(raw.TeamIndex),
			Reputation: Reputation{
				Home:    int(raw.HomeReputation),
				Current: int(raw.CurrentReputation),
				World:   int(raw.WorldReputation),
			},
			PlayerCoach: raw.PlayerCoach,
			Female:      raw.Female,
		},
		CurrentAbility:   int(raw.CurrentAbility),
		PotentialAbility: int(raw.PotentialAbility),
	}
	p.Age = age(p.BirthDate, on)
	if t := w.Teams[int64(raw.NationalTeamRef)]; raw.NationalTeamRef >= 0 && t != nil && t.National {
		p.NationalTeamIndex, p.NationalTeam = ptr(t.Index), t.Name
	}
	p.HeightCM = int(raw.Height)
	if raw.Value == notForSaleValue {
		p.NotForSale = true
	} else {
		p.Value = int64(raw.Value)
	}
	p.Condition, p.Sharpness = percent(raw.Condition), percent(raw.Sharpness)
	if t := knownDate(raw.JoinedClub); !t.IsZero() {
		p.JoinedClub = &t
	}
	p.Contract, p.Loan, p.FutureContract = newContract(raw.Contract, w), newContract(raw.Loan, w), newContract(raw.FutureContract, w)
	p.setSquadStatus(raw, p.Age <= youthRuleMaxAge, w)
	p.LoyaltyBonusRemaining = loyaltyRemaining(raw.Contract, on)
	for _, in := range raw.Injuries {
		p.Injuries = append(p.Injuries, injuryInfo(in, on, w))
	}
	for i, v := range raw.Positions {
		p.Positions = append(p.Positions, PositionRating{Position: savefile.PositionNames[i], Rating: int(v)})
	}
	for i, v := range raw.Attributes {
		p.Attributes[i] = uint8(attributeScore(v)) //nolint:gosec // 1..20
	}
	for i, v := range raw.Hidden {
		p.Attributes[len(raw.Attributes)+i] = min(v, 20)
	}
	p.traits, p.negativeLabels, p.devoted = raw.Traits, raw.FlagBit3, isDevoted(raw, w)
	p.nationKey = nationKey(p.Nation)
	p.names = nameWords(raw.FirstName, raw.LastName, raw.CommonName, raw.FullName)
	return p
}

// attributeScore converts the stored 1..100 value to the 1..20 scale the
// game shows: clamp((v+2)/5, 1, 20) (game_plugin 26.3.2, 0x48b9fc0).
func attributeScore(v uint8) int {
	return min(20, max(1, (int(v)+2)/5))
}

func displayName(raw savefile.Player) string {
	if raw.CommonName != "" {
		return raw.CommonName
	}
	if n := strings.TrimSpace(raw.FirstName + " " + raw.LastName); n != "" {
		return n
	}
	return raw.FullName
}

func (b PlayerBasics) Injured() bool { return len(b.Injuries) > 0 }

// ExpectedReturn is the earliest date the player is fit again: the latest
// earliest-return of their current injuries. Zero when not injured.
func (b PlayerBasics) ExpectedReturn() time.Time {
	var back time.Time
	for _, in := range b.Injuries {
		if in.ReturnEarliest.After(back) {
			back = in.ReturnEarliest
		}
	}
	return back
}

// plays reports whether p plays for team: his current team or the
// national team he is called up to.
func (p *Player) plays(team int64) bool {
	return p.TeamIndex == team || p.NationalTeamIndex != nil && *p.NationalTeamIndex == team
}

// WeeklyWage is the main contract's wage, 0 without a contract.
func (p *Player) WeeklyWage() int64 {
	if p.Contract == nil {
		return 0
	}
	return p.Contract.WeeklyWage
}

// hasValue reports whether the save knows p's value.
func (p *Player) hasValue() bool { return !p.NotForSale && p.Value >= 0 }

func percent(v uint16) float64 { return float64(v) / 100 }

func (b PlayerBasics) BestPositions() []string {
	var best []PositionRating
	for _, r := range b.Positions {
		if r.Rating >= naturalRating {
			best = append(best, r)
		}
	}
	slices.SortStableFunc(best, func(a, b PositionRating) int { return b.Rating - a.Rating })
	out := make([]string, len(best))
	for i, r := range best {
		out[i] = r.Position
	}
	return out
}

func age(birth, on time.Time) int {
	years := on.Year() - birth.Year()
	if on.Month() < birth.Month() || (on.Month() == birth.Month() && on.Day() < birth.Day()) {
		years--
	}
	return years
}
