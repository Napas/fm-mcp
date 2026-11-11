package fm

import (
	"fmt"
	"slices"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Where a tactic's players come from (Tactic.LineupSource).
const (
	LineupSelected  = "selected"   // the XI and bench picked for the next match
	LineupLastMatch = "last_match" // the team's last match; nothing is picked
	LineupNone      = "none"       // the save has neither
)

// xiSize is the number of starters; the rest of a lineup is the bench.
const xiSize = 11

// Tactic is the human manager's current first-team tactic with its players.
// Names of roles FM26 renamed are guesses.
type Tactic struct {
	TeamIndex    int64  `json:"team_index"`
	Team         string `json:"team,omitempty"`
	Name         string `json:"name"`
	Mentality    string `json:"mentality,omitempty"` // empty when the stored value is off the scale
	LineupSource string `json:"lineup_source"`
	// TeamInstructions lists the options set (in possession, out of
	// possession, set pieces); unlisted instructions are at their default.
	// Nil when none is set or the bitset is not decoded (26.x saves, or
	// an unknown version, which checkTactics reports).
	TeamInstructions []TeamInstruction `json:"team_instructions,omitempty"`
	// UnknownTeamInstructionBits are set bits with no known option.
	UnknownTeamInstructionBits []int          `json:"unknown_team_instruction_bits,omitempty"`
	Slots                      []TacticSlot   `json:"slots"`
	Bench                      []TacticPlayer `json:"bench,omitempty"`
}

// TeamInstruction is one option a tactic sets on the Team Instructions screen.
type TeamInstruction struct {
	Phase       string `json:"phase"`       // in_possession, out_of_possession or set_pieces
	Instruction string `json:"instruction"` // e.g. "Passing Directness"
	Option      string `json:"option"`      // e.g. "Shorter"
}

// PhaseSetPieces is a team-instruction phase beside PhaseInPossession and
// PhaseOutOfPossession.
const PhaseSetPieces = "set_pieces"

// phaseNames are the names of savefile's instruction phases, by value.
var phaseNames = [...]string{
	savefile.InPossessionInstructions:    PhaseInPossession,
	savefile.OutOfPossessionInstructions: PhaseOutOfPossession,
	savefile.SetPieceInstructions:        PhaseSetPieces,
}

// teamInstructions returns the set options in screen order (by phase, then
// bit) and the unnamed set bits; none when the bitset is not decoded.
func teamInstructions(bits savefile.Bitset) ([]TeamInstruction, []int) {
	set, unknown, err := bits.TeamInstructions()
	if err != nil {
		return nil, nil
	}
	slices.SortStableFunc(set, func(a, b savefile.TeamInstruction) int { return int(a.Phase) - int(b.Phase) })
	var out []TeamInstruction
	for _, o := range set {
		out = append(out, TeamInstruction{Phase: phaseNames[o.Phase], Instruction: o.Instruction, Option: o.Option})
	}
	return out, unknown
}

// TacticSlot is one of the tactic's 11 positions and the player in it.
type TacticSlot struct {
	Slot            int           `json:"slot"` // 0..10
	InPossession    SlotRole      `json:"in_possession"`
	OutOfPossession SlotRole      `json:"out_of_possession"`
	Player          *TacticPlayer `json:"player,omitempty"` // nil when the slot is empty
	// RoleAbility is the player's role ability in the in-possession role and
	// position; nil without both, or when fog of war hides his current ability.
	RoleAbility *int `json:"role_ability,omitempty"`
	// OutOfPossessionRoleAbility is RoleAbility for the out-of-possession role:
	// never halved, FM27's rules for every build.
	OutOfPossessionRoleAbility *int `json:"out_of_possession_role_ability,omitempty"`
	// PlayerInstructions are the current tactic's; nil when none are set.
	PlayerInstructions *PlayerInstructions `json:"player_instructions,omitempty"`
}

// PlayerInstructions are a slot's FM27 player instructions, in bit order.
// Defaults are not stored, so they are not listed.
type PlayerInstructions struct {
	InPossession    []string `json:"in_possession,omitempty"`
	OutOfPossession []string `json:"out_of_possession,omitempty"`
	// Set bits with no known instruction.
	UnknownInPossessionBits    []int `json:"unknown_in_possession_bits,omitempty"`
	UnknownOutOfPossessionBits []int `json:"unknown_out_of_possession_bits,omitempty"`
}

// playerInstructions returns s's player instructions; nil when none are set
// or decoded. A phase that is not decoded lists none (checkTactics warns).
func playerInstructions(s savefile.TacticSlot) *PlayerInstructions {
	var out PlayerInstructions
	out.InPossession, out.UnknownInPossessionBits, _ = s.InPossession.Instructions.PlayerInstructions()
	out.OutOfPossession, out.UnknownOutOfPossessionBits, _ = s.OutOfPossession.Instructions.PlayerInstructions()
	if out.InPossession == nil && out.OutOfPossession == nil && out.UnknownInPossessionBits == nil && out.UnknownOutOfPossessionBits == nil {
		return nil
	}
	return &out
}

// SlotRole is a slot's position and role in one phase; empty where unnamed.
type SlotRole struct {
	Position string `json:"position,omitempty"` // a position code, e.g. "DC"
	Side     string `json:"side,omitempty"`     // "left" or "right" for one of a pair, e.g. two DC
	Role     string `json:"role,omitempty"`
}

// TacticPlayer is a person in the lineup.
type TacticPlayer struct {
	UID         uint32 `json:"uid,omitempty"`
	Name        string `json:"name,omitempty"`
	ShirtNumber int    `json:"shirt_number,omitempty"` // last lineups only; 0 when none
	// Unnamed marks a person the save does not decode as a player; UID and
	// Name are then empty.
	Unnamed bool `json:"unnamed_player,omitempty"`
}

// checkTactics returns the tactics to keep and load warnings. Unreadable
// tactics are dropped; of a team other than managerTeam, only the last
// lineups are kept.
func checkTactics(t savefile.Tactics, err error, managerTeam int64) (*savefile.Tactics, []string) {
	if err != nil {
		return nil, []string{fmt.Sprintf("current tactic and last-match lineups unavailable: %v", err)}
	}
	var warnings []string
	if t.TacticErr != nil {
		warnings = append(warnings, fmt.Sprintf("current tactic unavailable: %v", t.TacticErr))
	}
	if t.WalkErr != nil {
		w := fmt.Sprintf("tactics layout not fully recognised; last lineups found by searching the file (stopped at %s)", t.WalkErr.Part)
		if t.Team < 0 && t.TacticErr == nil {
			w = "current tactic unavailable; " + w
		}
		warnings = append(warnings, w)
	}
	if t.Team >= 0 && managerTeam != noTeam && int64(t.Team) != managerTeam {
		warnings = append(warnings, fmt.Sprintf("current tactic unavailable: the saved tactic is for team %d, not the manager's first team %d", t.Team, managerTeam))
		t.Tactics = nil
	}
	if tac := current(&t); tac != nil {
		if _, _, err := tac.TeamInstructions.TeamInstructions(); err != nil {
			warnings = append(warnings, fmt.Sprintf("team instructions not decoded: %v", err))
		}
		if err := playerInstructionsErr(tac); err != nil {
			warnings = append(warnings, fmt.Sprintf("player instructions not decoded: %v", err))
		}
	}
	if t.BadFormations > 0 {
		warnings = append(warnings, fmt.Sprintf("%d teams' last-match formations could not be decoded and are missing", t.BadFormations))
	}
	return &t, warnings
}

// playerInstructionsErr returns the first error decoding a slot's player
// instructions.
func playerInstructionsErr(tac *savefile.Tactic) error {
	for _, s := range tac.Slots {
		for _, p := range [...]savefile.SlotRole{s.InPossession, s.OutOfPossession} {
			if _, _, err := p.Instructions.PlayerInstructions(); err != nil {
				return err
			}
		}
	}
	return nil
}

// lineup returns the XI by slot, then the bench, as person indices, and their
// source: the selection, else the team's last match.
func lineup(t *savefile.Tactics) ([]int32, string) {
	if hasPlayers(t.Selection[:xiSize]) {
		return t.Selection[:], LineupSelected
	}
	if last, ok := t.LastLineups[t.Team]; ok && hasPlayers(last.Players[:xiSize]) {
		return last.Players[:], LineupLastMatch
	}
	return nil, LineupNone
}

func hasPlayers(persons []int32) bool {
	for _, p := range persons {
		if p >= 0 {
			return true
		}
	}
	return false
}

// current returns the active tactic, the first if the active index is out of
// range, or nil when there is none.
func current(t *savefile.Tactics) *savefile.Tactic {
	if t == nil || len(t.Tactics) == 0 {
		return nil
	}
	if t.Active >= 0 && int(t.Active) < len(t.Tactics) {
		return &t.Tactics[t.Active]
	}
	return &t.Tactics[0]
}

// newTactic builds the current tactic from the lineup's players by person
// index and the players' per-player data. Nil when the save has no tactic.
func newTactic(t *savefile.Tactics, w World, players map[uint32]*Player, aux []playerAux, rules roleRules) *Tactic {
	tac := current(t)
	if tac == nil {
		return nil
	}
	persons, source := lineup(t)
	l := lineupPeople{persons: persons, players: players}
	out := &Tactic{
		TeamIndex: int64(t.Team), Team: w.teamName(int64(t.Team)), Name: tac.Name, LineupSource: source,
		Slots: l.slots(tac.Slots[:]), Bench: l.bench(),
	}
	out.Mentality, _ = tac.MentalityName()
	out.TeamInstructions, out.UnknownTeamInstructionBits = teamInstructions(tac.TeamInstructions)
	for i, s := range tac.Slots {
		out.Slots[i].PlayerInstructions = playerInstructions(s)
		p := l.player(i)
		if p == nil {
			continue
		}
		roles := &aux[p.auxIdx].roles
		if at, ok := placeOf(s.InPossession); ok {
			if v, ok := roles.ability(s.InPossession.RoleBit(), at, rules); ok {
				out.Slots[i].RoleAbility = &v
			}
		}
		if at, ok := placeOf(s.OutOfPossession); ok {
			if v, ok := roles.oopAbility(s.OutOfPossession.RoleBit(), at, rules); ok {
				out.Slots[i].OutOfPossessionRoleAbility = &v
			}
		}
	}
	return out
}

// noShirtNumber marks a lineup place without a shirt number.
const noShirtNumber = 255

// lineupPeople is a lineup: person indices by place (the XI, then the bench;
// -1 = empty), their shirt numbers (nil when unknown) and the players among
// them by person index.
type lineupPeople struct {
	persons []int32
	shirts  []uint8
	players map[uint32]*Player
}

// slots returns the 11 slots with the positions and roles of slots (none
// when slots is nil) and the XI.
func (l lineupPeople) slots(slots []savefile.TacticSlot) []TacticSlot {
	out := make([]TacticSlot, xiSize)
	for i := range out {
		out[i].Slot = i
		if i < len(slots) {
			s := slots[i]
			out[i].InPossession = slotRole(s.InPossession, s.InPossession.InPossessionRoleName)
			out[i].OutOfPossession = slotRole(s.OutOfPossession, s.OutOfPossession.OutOfPossessionRoleName)
		}
		out[i].Player = l.at(i)
	}
	return out
}

// bench returns the people after the XI.
func (l lineupPeople) bench() []TacticPlayer {
	var out []TacticPlayer
	for i := xiSize; i < len(l.persons); i++ {
		if tp := l.at(i); tp != nil {
			out = append(out, *tp)
		}
	}
	return out
}

// at returns the person in place i; nil for an empty place.
func (l lineupPeople) at(i int) *TacticPlayer {
	if i >= len(l.persons) {
		return nil
	}
	out := tacticPlayer(l.players, l.persons[i])
	if out != nil && i < len(l.shirts) && l.shirts[i] != noShirtNumber {
		out.ShirtNumber = int(l.shirts[i])
	}
	return out
}

// tacticPlayer returns the person with index i, marked unnamed when players
// does not hold i; nil for no person (negative i).
func tacticPlayer(players map[uint32]*Player, i int32) *TacticPlayer {
	if i < 0 {
		return nil
	}
	if p := players[uint32(i)]; p != nil {
		return &TacticPlayer{UID: p.UID, Name: p.Name}
	}
	return &TacticPlayer{Unnamed: true}
}

// player returns the player in place i; nil for an empty place or a person
// who is not a player.
func (l lineupPeople) player(i int) *Player {
	if i >= len(l.persons) || l.persons[i] < 0 {
		return nil
	}
	return l.players[uint32(l.persons[i])]
}

func slotRole(s savefile.SlotRole, roleName func() (string, bool)) SlotRole {
	out := SlotRole{Side: s.Side()}
	out.Position, _ = s.PositionCode()
	out.Role, _ = roleName()
	return out
}

// Tactic returns the human manager's current tactic; false when the save has
// none. Fog of war hides only role abilities, with the current ability. Its
// slices are shared: read-only.
func (v View) Tactic() (Tactic, bool) {
	if v.g.tactic == nil {
		return Tactic{}, false
	}
	t := *v.g.tactic
	if v.fog {
		t.Slots = slices.Clone(t.Slots)
		for i, s := range t.Slots {
			if (s.RoleAbility != nil || s.OutOfPossessionRoleAbility != nil) && !v.see(v.g.byUID[s.Player.UID]).s.full() {
				t.Slots[i].RoleAbility, t.Slots[i].OutOfPossessionRoleAbility = nil, nil
			}
		}
	}
	return t, true
}
