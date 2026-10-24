package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// FavouredClub is one of FM's profile "Favoured Clubs".
type FavouredClub struct {
	Club      string `json:"club"`                 // see Club.Name
	TeamIndex *int64 `json:"team_index,omitempty"` // the club's first team; nil when it has none
	// As is "supporter" or "player"; empty for an unknown reason.
	As string `json:"as,omitempty"`
}

// minFavouredValue is the lowest affinity the game lists (0x2f87260,
// 0x1392144, 0x17839c8).
const minFavouredValue = 50

// favouredClubs lists the affinities the game shows, in save order, skipping
// clubs missing from clubs.
func favouredClubs(raw []savefile.ClubAffinity, clubs map[int64]Club) []FavouredClub {
	var out []FavouredClub
	for _, a := range raw {
		c, ok := clubs[a.Club]
		if !ok || a.Value < minFavouredValue {
			continue
		}
		f := FavouredClub{Club: c.Name}
		f.As, _ = a.ReasonName()
		if c.FirstTeam != noTeam {
			f.TeamIndex = &c.FirstTeam
		}
		out = append(out, f)
	}
	return out
}
