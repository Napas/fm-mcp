package fm

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// XIRequest asks for a team's best XI.
type XIRequest struct {
	TeamIndex int64
	// Slots are the positions and in-possession roles to fill, at most 11;
	// none means the current tactic's.
	Slots          []SlotRole
	IncludeInjured bool
}

// BestXI places a team's players in slots for the highest total role ability.
type BestXI struct {
	TeamIndex  int64    `json:"team_index"`
	Team       string   `json:"team,omitempty"`
	FromTactic bool     `json:"from_tactic"` // the slots are the current tactic's
	Slots      []XIPick `json:"slots"`
	// LeftOutInjured are the injured players not considered.
	LeftOutInjured []PlayerRef `json:"left_out_injured,omitempty"`
	// HiddenByFog counts the players not considered because fog of war
	// hides their current ability.
	HiddenByFog int `json:"hidden_by_fog,omitempty"`
}

// XIPick is a slot with its player (nil when the team has too few) and the
// next best candidates.
type XIPick struct {
	Slot int `json:"slot"`
	SlotRole
	Player       *XIPlayer  `json:"player,omitempty"`
	Alternatives []XIPlayer `json:"alternatives,omitempty"`
}

type PlayerRef struct {
	UID  uint32 `json:"uid"`
	Name string `json:"name"`
}

func refOf(p *Player) PlayerRef { return PlayerRef{UID: p.UID, Name: p.Name} }

// XIPlayer is a player with his role ability in a slot. StartsAtSlot is set
// on an alternative the XI starts elsewhere.
type XIPlayer struct {
	PlayerRef
	RoleAbility  int  `json:"role_ability"`
	StartsAtSlot *int `json:"starts_at_slot,omitempty"`
}

const xiAlternatives = 3

// xiTarget is a slot to fill.
type xiTarget struct {
	slot int
	show SlotRole
	role uint64
	at   place
}

// BestXI fills the request's slots with the team's players (including national
// call-ups) for the highest total role ability. Players whose current ability
// fog of war hides are left out, as are injured ones unless requested. Bad
// slots, an unknown team or no usable tactic are an ErrInvalidFilter.
func (v View) BestXI(r XIRequest) (BestXI, error) {
	if err := v.g.checkTeam(r.TeamIndex); err != nil {
		return BestXI{}, err
	}
	targets, fromTactic, err := v.g.xiTargets(r)
	if err != nil {
		return BestXI{}, err
	}
	out := BestXI{TeamIndex: r.TeamIndex, Team: v.g.world.teamName(r.TeamIndex), FromTactic: fromTactic}
	var squad []*Player
	for _, x := range v.bestFirst(func(p *Player) bool { return p.plays(r.TeamIndex) }) {
		switch {
		case !x.s.full():
			out.HiddenByFog++
		case x.p.Injured() && !r.IncludeInjured:
			out.LeftOutInjured = append(out.LeftOutInjured, refOf(x.p))
		default:
			squad = append(squad, x.p)
		}
	}
	score := make([][]int, len(targets))
	for i, t := range targets {
		score[i] = make([]int, len(squad))
		for j, p := range squad {
			score[i][j] = v.g.roleAbility(p, t.role, t.at)
		}
	}
	picks := assign(score)
	startsAt := make(map[int]int, len(picks)) // squad index → slot number
	for i, j := range picks {
		if j >= 0 {
			startsAt[j] = targets[i].slot
		}
	}
	player := func(i, j int) XIPlayer {
		return XIPlayer{PlayerRef: refOf(squad[j]), RoleAbility: score[i][j]}
	}
	out.Slots = make([]XIPick, len(targets))
	for i, t := range targets {
		pick := XIPick{Slot: t.slot, SlotRole: t.show}
		if j := picks[i]; j >= 0 {
			p := player(i, j)
			pick.Player = &p
		}
		for _, j := range bestOthers(score[i], picks[i], xiAlternatives) {
			a := player(i, j)
			if s, ok := startsAt[j]; ok {
				a.StartsAtSlot = &s
			}
			pick.Alternatives = append(pick.Alternatives, a)
		}
		out.Slots[i] = pick
	}
	return out, nil
}

// bestOthers returns the indices of the n highest scores other than
// scores[skip], best first; ties keep index order.
func bestOthers(scores []int, skip, n int) []int {
	var out []int
	for j := range scores {
		if j != skip {
			out = append(out, j)
		}
	}
	slices.SortStableFunc(out, func(a, b int) int { return cmp.Compare(scores[b], scores[a]) })
	return out[:min(n, len(out))]
}

// xiTargets checks the request's slots, or takes the current tactic's when it
// has none (true then).
func (g *Game) xiTargets(r XIRequest) ([]xiTarget, bool, error) {
	if len(r.Slots) > 0 {
		if len(r.Slots) > xiSize {
			return nil, false, fmt.Errorf("%w: %d slots; pass at most 11 slots", ErrInvalidFilter, len(r.Slots))
		}
		out := make([]xiTarget, len(r.Slots))
		for i, s := range r.Slots {
			t, err := xiTargetOf(i, s)
			if err != nil {
				return nil, false, err
			}
			out[i] = t
		}
		return out, false, nil
	}
	switch {
	case g.tactic == nil:
		return nil, false, fmt.Errorf("%w: the save has no current tactic; pass slots", ErrInvalidFilter)
	case g.tactic.TeamIndex != r.TeamIndex:
		return nil, false, fmt.Errorf("%w: the current tactic is team %d's; pass slots", ErrInvalidFilter, g.tactic.TeamIndex)
	}
	out := make([]xiTarget, len(g.tactic.Slots))
	for i, s := range g.tactic.Slots {
		if s.InPossession.Role == "" {
			return nil, false, fmt.Errorf("%w: slot %d has no named in-possession role; pass slots", ErrInvalidFilter, s.Slot)
		}
		t, err := xiTargetOf(s.Slot, s.InPossession)
		if err != nil {
			return nil, false, fmt.Errorf("%w; pass slots", err)
		}
		out[i] = t
	}
	return out, true, nil
}

// xiTargetOf checks slot n's position, side and role.
func xiTargetOf(n int, s SlotRole) (xiTarget, error) {
	invalid := func(err error) (xiTarget, error) {
		return xiTarget{}, fmt.Errorf("%w: slot %d: %w", ErrInvalidFilter, n, err)
	}
	pos, err := positionNamed(s.Position)
	if err != nil {
		return invalid(err)
	}
	code := savefile.PositionNames[pos]
	if s.Role == "" {
		return xiTarget{}, fmt.Errorf("%w: slot %d has no role; pass an in-possession role offered at %s", ErrInvalidFilter, n, code)
	}
	mask, ok := savefile.SidedPosition(code, s.Side)
	if !ok {
		return invalid(fmt.Errorf("side %q; only DC, DM, MC, AMC and ST take left or right, others none", s.Side))
	}
	role, err := offeredRole(s.Role, pos)
	if err != nil {
		return invalid(err)
	}
	name, _ := savefile.InPossessionRoleName(role)
	show := SlotRole{Position: code, Side: savefile.SlotRole{Position: mask}.Side(), Role: name}
	return xiTarget{slot: n, show: show, role: role, at: place{index: pos, mask: mask}}, nil
}
