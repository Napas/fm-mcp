package fm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

var (
	// ErrNoLastMatch means the save holds no last match for a team.
	ErrNoLastMatch = errors.New("fm: no last match")
	// ErrLastLineupsUnavailable means the save's last lineups could not be read.
	ErrLastLineupsUnavailable = errors.New("fm: last-match lineups unavailable")
)

// LastLineup is a team's lineup, formation and roles in its most recent
// match. Roles have no duty (not stored); the match date is unknown.
type LastLineup struct {
	TeamIndex     int64  `json:"team_index"`
	Team          string `json:"team,omitempty"`
	OpponentIndex int64  `json:"opponent_team_index"`
	Opponent      string `json:"opponent,omitempty"`
	// Mentality, the shapes and the slots' positions and roles are empty when
	// the formation does not decode.
	Mentality            string         `json:"mentality,omitempty"`
	InPossessionShape    string         `json:"in_possession_shape,omitempty"` // e.g. "4-2-3-1"
	OutOfPossessionShape string         `json:"out_of_possession_shape,omitempty"`
	Slots                []TacticSlot   `json:"slots"`
	Bench                []TacticPlayer `json:"bench,omitempty"`
}

// LastLineup returns the team's last match; fog of war hides nothing. An
// unknown team gives ErrInvalidFilter, a team without a last match
// ErrNoLastMatch, and a save whose last lineups could not be read
// ErrLastLineupsUnavailable.
func (v View) LastLineup(team int64) (LastLineup, error) {
	if err := v.g.checkTeam(team); err != nil {
		return LastLineup{}, err
	}
	if v.g.lastLineups == nil {
		return LastLineup{}, ErrLastLineupsUnavailable
	}
	l, ok := v.g.lastLineups[int32(team)] //nolint:gosec // a known team index
	if !ok || l.Opponent < 0 {
		return LastLineup{}, fmt.Errorf("%w for team_index %d in this save", ErrNoLastMatch, team)
	}
	var slots []savefile.TacticSlot
	if l.Formation != nil {
		slots = l.Formation.Slots[:]
	}
	people := lineupPeople{persons: l.Players[:], shirts: l.ShirtNumbers[:], players: v.g.lineupPlayers}
	out := LastLineup{
		TeamIndex: team, Team: v.g.world.teamName(team),
		OpponentIndex: int64(l.Opponent), Opponent: v.g.world.teamName(int64(l.Opponent)),
		Slots: people.slots(slots), Bench: people.bench(),
	}
	if l.Formation != nil {
		out.Mentality, _ = l.Formation.MentalityName()
		out.InPossessionShape, out.OutOfPossessionShape = shapes(out.Slots)
	}
	return out, nil
}

// shapeRows numbers formation rows from defence to attack. Wing-backs share
// the defensive midfielders' row. A sweeper is stored in the goalkeeper's
// cell, so last lineups never hold SW.
var shapeRows = map[string]int{
	"DL": 0, "DC": 0, "DR": 0,
	"WBL": 1, "WBR": 1, "DM": 1,
	"ML": 2, "MC": 2, "MR": 2,
	"AML": 3, "AMC": 3, "AMR": 3,
	"ST": 4,
}

// shapes returns the in- and out-of-possession shapes of slots.
func shapes(slots []TacticSlot) (inPossession, outOfPossession string) {
	ip, oop := make([]string, len(slots)), make([]string, len(slots))
	for i, s := range slots {
		ip[i], oop[i] = s.InPossession.Position, s.OutOfPossession.Position
	}
	return shape(ip), shape(oop)
}

// shape returns the players per row of positions, back to front, without the
// goalkeeper and empty rows, e.g. "4-2-3-1"; "" when a position is unknown.
func shape(positions []string) string {
	var rows [5]int
	n := 0
	for _, pos := range positions {
		if pos == "GK" {
			continue
		}
		r, ok := shapeRows[pos]
		if !ok {
			return ""
		}
		rows[r]++
		n++
	}
	if n == 0 {
		return ""
	}
	var parts []string
	for _, c := range rows {
		if c > 0 {
			parts = append(parts, strconv.Itoa(c))
		}
	}
	return strings.Join(parts, "-")
}
