package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// WorkPermit is a work permit a nation granted the player. The game matches
// it to a club's nation; England, Scotland, Wales and Northern Ireland count
// as one.
type WorkPermit struct {
	NationRef
	Year int `json:"year"` // probably the year it was granted (inference)
}

// workPermits groups permits by player uid, in save order. Permits of
// non-players are dropped.
func workPermits(permits []savefile.WorkPermit, raw []savefile.Player, w World) map[uint32][]WorkPermit {
	byPerson := make(map[uint32][]WorkPermit)
	for _, p := range permits {
		byPerson[p.Person] = append(byPerson[p.Person], WorkPermit{NationRef: w.nationRef(p.Nation), Year: int(p.Year)})
	}
	out := make(map[uint32][]WorkPermit, len(byPerson))
	for _, r := range raw {
		if ps, ok := byPerson[r.Index]; ok {
			out[r.UID] = ps
		}
	}
	return out
}
