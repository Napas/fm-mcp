package fm

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Phase names in SquadRolesRequest and RoleDepth.
const (
	PhaseInPossession    = "in_possession"
	PhaseOutOfPossession = "out_of_possession"
)

// SquadRolesRequest filters by Position and Phase when set; Limit, when set,
// keeps the best players per role.
type SquadRolesRequest struct {
	TeamIndex int64
	Position  string
	Phase     string
	Limit     int
}

// SquadRoles ranks a team's players in each role offered at positions they
// are rated 10 or more at.
type SquadRoles struct {
	TeamIndex int64       `json:"team_index"`
	Team      string      `json:"team,omitempty"`
	Roles     []RoleDepth `json:"roles"`
	// HiddenByFog counts players left out because fog of war hides their
	// current ability.
	HiddenByFog int `json:"hidden_by_fog,omitempty"`
}

// RoleDepth is a position and role with its players, best first.
type RoleDepth struct {
	Position string       `json:"position"`
	Phase    string       `json:"phase"`
	Role     string       `json:"role"`
	Players  []RolePlayer `json:"players"`
}

// RolePlayer is a player's role ability in a role, without a slot side.
type RolePlayer struct {
	PlayerRef
	RoleAbility    int  `json:"role_ability"`
	PositionRating int  `json:"position_rating"`
	Injured        bool `json:"injured,omitempty"`
}

// SquadRoles includes national call-ups. Abilities take the better of left
// and right, as Search does. An unknown position or phase is an
// ErrInvalidFilter.
func (v View) SquadRoles(r SquadRolesRequest) (SquadRoles, error) {
	if err := v.g.checkTeam(r.TeamIndex); err != nil {
		return SquadRoles{}, err
	}
	pos, err := positionFilter(r.Position)
	if err != nil {
		return SquadRoles{}, err
	}
	if r.Limit < 0 {
		return SquadRoles{}, fmt.Errorf("%w: limit %d is negative", ErrInvalidFilter, r.Limit)
	}
	phases := []rolePhase{inPossession, outOfPossession}
	if r.Phase != "" {
		i := slices.IndexFunc(phases, func(ph rolePhase) bool { return strings.EqualFold(ph.key, r.Phase) })
		if i < 0 {
			return SquadRoles{}, fmt.Errorf("%w: phase %q; use %s or %s", ErrInvalidFilter, r.Phase, PhaseInPossession, PhaseOutOfPossession)
		}
		phases = phases[i : i+1]
	}
	out := SquadRoles{TeamIndex: r.TeamIndex, Team: v.g.world.teamName(r.TeamIndex), Roles: []RoleDepth{}}
	type key struct {
		pos, phase int
		role       uint64
	}
	rows := map[key][]RolePlayer{}
	for _, x := range v.bestFirst(func(p *Player) bool { return p.plays(r.TeamIndex) }) {
		if !x.s.full() {
			out.HiddenByFog++
			continue
		}
		in := &v.g.auxOf(x.p).roles
		for i, ph := range phases {
			for _, f := range in.fits(ph, bestRolePosition, v.g.roleRules) {
				if pos < 0 || f.pos == pos {
					k := key{f.pos, i, f.role}
					rows[k] = append(rows[k], RolePlayer{PlayerRef: refOf(x.p), RoleAbility: f.ability, PositionRating: int(in.positions[f.pos]), Injured: x.p.Injured()})
				}
			}
		}
	}
	keys := slices.SortedFunc(maps.Keys(rows), func(a, b key) int {
		return cmp.Or(cmp.Compare(a.pos, b.pos), cmp.Compare(a.phase, b.phase), cmp.Compare(a.role, b.role))
	})
	for _, k := range keys {
		players := rows[k]
		slices.SortStableFunc(players, func(a, b RolePlayer) int { return cmp.Compare(b.RoleAbility, a.RoleAbility) })
		if r.Limit > 0 {
			players = players[:min(r.Limit, len(players))]
		}
		name, _ := phases[k.phase].nameOf(k.role)
		out.Roles = append(out.Roles, RoleDepth{Position: savefile.PositionNames[k.pos], Phase: phases[k.phase].key, Role: name, Players: players})
	}
	return out, nil
}
