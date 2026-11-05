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
	// Slots: at most 11; none means the current tactic's.
	Slots          []XISlot
	IncludeInjured bool
}

// XISlot is a slot's in-possession position and role, and optionally its
// out-of-possession ones.
type XISlot struct {
	SlotRole
	OutOfPossession *SlotRole `json:"out_of_possession,omitempty"`
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
	XISlot
	Player       *XIPlayer  `json:"player,omitempty"`
	Alternatives []XIPlayer `json:"alternatives,omitempty"`
}

type PlayerRef struct {
	UID  uint32 `json:"uid"`
	Name string `json:"name"`
}

func refOf(p *Player) PlayerRef { return PlayerRef{UID: p.UID, Name: p.Name} }

// XIPlayer is a player with his role abilities in a slot. The
// out-of-possession one is nil without an out-of-possession role.
// StartsAtSlot is set on an alternative the XI starts elsewhere.
type XIPlayer struct {
	PlayerRef
	RoleAbility                int  `json:"role_ability"`
	OutOfPossessionRoleAbility *int `json:"out_of_possession_role_ability,omitempty"`
	StartsAtSlot               *int `json:"starts_at_slot,omitempty"`
}

const xiAlternatives = 3

// xiTarget is a slot to fill.
type xiTarget struct {
	slot int
	show XISlot
	ip   phaseTarget
	oop  *phaseTarget // nil: in possession only
}

// phaseTarget is a slot's role and place in one phase.
type phaseTarget struct {
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
	// cands[i][j] is squad[j] in targets[i]; score[i][j] is his total.
	cands, score := make([][]XIPlayer, len(targets)), make([][]int, len(targets))
	for i, t := range targets {
		cands[i], score[i] = make([]XIPlayer, len(squad)), make([]int, len(squad))
		for j, p := range squad {
			c := XIPlayer{PlayerRef: refOf(p), RoleAbility: v.g.roleAbility(inPossession, p, t.ip.role, t.ip.at)}
			// A slot without an out-of-possession role counts its role
			// ability twice, on the same scale as one with both.
			score[i][j] = 2 * c.RoleAbility
			if t.oop != nil {
				a := v.g.roleAbility(outOfPossession, p, t.oop.role, t.oop.at)
				c.OutOfPossessionRoleAbility = &a
				score[i][j] = c.RoleAbility + a
			}
			cands[i][j] = c
		}
	}
	picks := assign(score)
	startsAt := make(map[int]int, len(picks)) // squad index → slot number
	for i, j := range picks {
		if j >= 0 {
			startsAt[j] = targets[i].slot
		}
	}
	out.Slots = make([]XIPick, len(targets))
	for i, t := range targets {
		pick := XIPick{Slot: t.slot, XISlot: t.show}
		if j := picks[i]; j >= 0 {
			p := cands[i][j]
			pick.Player = &p
		}
		for _, j := range bestOthers(score[i], picks[i], xiAlternatives) {
			a := cands[i][j]
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
		slot := XISlot{SlotRole: s.InPossession}
		if s.OutOfPossession.Role != "" {
			slot.OutOfPossession = &s.OutOfPossession
		}
		t, err := xiTargetOf(s.Slot, slot)
		if err != nil {
			return nil, false, fmt.Errorf("%w; pass slots", err)
		}
		out[i] = t
	}
	return out, true, nil
}

// xiTargetOf checks slot n's positions, sides and roles.
func xiTargetOf(n int, s XISlot) (xiTarget, error) {
	t := xiTarget{slot: n}
	var err error
	if t.ip, t.show.SlotRole, err = phaseTargetOf(inPossession, n, s.SlotRole); err != nil {
		return xiTarget{}, err
	}
	if s.OutOfPossession != nil {
		oop, show, err := phaseTargetOf(outOfPossession, n, *s.OutOfPossession)
		if err != nil {
			return xiTarget{}, err
		}
		t.oop, t.show.OutOfPossession = &oop, &show
	}
	return t, nil
}

// phaseTargetOf checks slot n's position, side and role in one phase.
func phaseTargetOf(ph rolePhase, n int, s SlotRole) (phaseTarget, SlotRole, error) {
	where := ""
	if ph.name == outOfPossession.name {
		where = " out of possession"
	}
	invalid := func(err error) (phaseTarget, SlotRole, error) {
		return phaseTarget{}, SlotRole{}, fmt.Errorf("%w: slot %d%s: %w", ErrInvalidFilter, n, where, err)
	}
	pos, err := positionNamed(s.Position)
	if err != nil {
		return invalid(err)
	}
	code := savefile.PositionNames[pos]
	if s.Role == "" {
		return phaseTarget{}, SlotRole{}, fmt.Errorf("%w: slot %d has no %s role; pass one offered at %s", ErrInvalidFilter, n, ph.name, code)
	}
	mask, ok := savefile.SidedPosition(code, s.Side)
	if !ok {
		return invalid(fmt.Errorf("side %q; only DC, DM, MC, AMC and ST take left or right, others none", s.Side))
	}
	role, err := offeredRole(ph, s.Role, pos)
	if err != nil {
		return invalid(err)
	}
	name, _ := ph.nameOf(role)
	show := SlotRole{Position: code, Side: savefile.SlotRole{Position: mask}.Side(), Role: name}
	return phaseTarget{role: role, at: place{index: pos, mask: mask}}, show, nil
}
