package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// ClubRef names a club and its first team.
type ClubRef struct {
	Club      string `json:"club"`                 // see Club.Name
	TeamIndex *int64 `json:"team_index,omitempty"` // the club's first team; nil when it has none
}

func clubRef(c Club) ClubRef {
	r := ClubRef{Club: c.Name}
	if c.FirstTeam != noTeam {
		r.TeamIndex = ptr(c.FirstTeam)
	}
	return r
}

// FavouredClub is one of FM's profile "Favoured Clubs".
type FavouredClub struct {
	ClubRef
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
		f := FavouredClub{ClubRef: clubRef(c)}
		f.As, _ = a.ReasonName()
		out = append(out, f)
	}
	return out
}
