package fm

import (
	"slices"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// TrainedAt is a club where a player counts as trained.
type TrainedAt struct {
	ClubRef
	// Rules are the save's homegrown rule ids, in save order; not named.
	Rules []int `json:"rules"`
}

// TrainedIn is a nation where a player counts as trained.
type TrainedIn struct {
	NationRef
	Rules []int `json:"rules"` // as TrainedAt.Rules
}

// homegrown is a player's homegrown status, one entry per club or nation.
type homegrown struct {
	clubs    []TrainedAt
	clubRefs []int64 // parallel to clubs
	nations  []TrainedIn
}

// newHomegrown groups the items by club and nation, skipping clubs missing
// from w; nil when none are left.
func newHomegrown(raw savefile.Player, w World) *homegrown {
	var h homegrown
	for _, t := range raw.TrainedAt {
		c, ok := w.Clubs[t.Ref]
		if !ok {
			continue
		}
		i := slices.Index(h.clubRefs, t.Ref)
		if i < 0 {
			i = len(h.clubs)
			h.clubs = append(h.clubs, TrainedAt{ClubRef: clubRef(c)})
			h.clubRefs = append(h.clubRefs, t.Ref)
		}
		h.clubs[i].Rules = append(h.clubs[i].Rules, int(t.Rule))
	}
	for _, t := range raw.TrainedIn {
		i := slices.IndexFunc(h.nations, func(n TrainedIn) bool { return n.NationIndex == t.Ref })
		if i < 0 {
			h.nations, i = append(h.nations, TrainedIn{NationRef: NationRef{NationIndex: t.Ref, Nation: w.Nations[t.Ref]}}), len(h.nations)
		}
		h.nations[i].Rules = append(h.nations[i].Rules, int(t.Rule))
	}
	if h.clubs == nil && h.nations == nil {
		return nil
	}
	return &h
}

// trainedAt and trainedIn accept a nil h.
func (h *homegrown) trainedAt(club int64) bool {
	return h != nil && slices.Contains(h.clubRefs, club)
}

func (h *homegrown) trainedIn(nation int64) bool {
	return h != nil && slices.ContainsFunc(h.nations, func(n TrainedIn) bool { return n.NationIndex == nation })
}

// homegrownFilter returns f's homegrown test, nil without one. checkRefs has
// validated f's refs.
func (g *Game) homegrownFilter(f Filter) func(*Player) bool {
	if f.TrainedAtTeamIndex == nil && f.TrainedInNationIndex == nil {
		return nil
	}
	var club int64
	if f.TrainedAtTeamIndex != nil {
		club, _ = g.world.clubOf(*f.TrainedAtTeamIndex)
	}
	return func(p *Player) bool {
		h := g.auxOf(p).homegrown
		return (f.TrainedAtTeamIndex == nil || h.trainedAt(club)) &&
			(f.TrainedInNationIndex == nil || h.trainedIn(*f.TrainedInNationIndex))
	}
}
