package fm

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// defaultShortlist is the name the filter takes for the unnamed list, which
// the game shows as "(Default)".
const defaultShortlist = "default"

type shortlist struct {
	name    string // "" for the default list
	players map[*Player]bool
}

// newShortlists resolves lists' person indexes through persons, which holds
// every listed person NewGame found; others are left out.
func newShortlists(lists []savefile.Shortlist, persons map[uint32]*Player) []shortlist {
	out := make([]shortlist, len(lists))
	for i, l := range lists {
		out[i] = shortlist{name: l.Name, players: make(map[*Player]bool, len(l.Persons))}
		for _, idx := range l.Persons {
			if p := persons[idx]; p != nil {
				out[i].players[p] = true
			}
		}
	}
	return out
}

// shortlistFilter returns the players on the list called name, ignoring case,
// or nil when name is empty. An exact name wins over "default" and
// "(default)" for the unnamed list.
func (g *Game) shortlistFilter(name string) (map[*Player]bool, error) {
	if name == "" {
		return nil, nil
	}
	if g.shortlists == nil {
		return nil, fmt.Errorf("%w: shortlists could not be read (game_overview's warnings say why)", ErrInvalidFilter)
	}
	for _, l := range g.shortlists {
		if strings.EqualFold(l.name, name) {
			return l.players, nil
		}
	}
	if n := strings.ToLower(name); n == defaultShortlist || n == "("+defaultShortlist+")" {
		for _, l := range g.shortlists {
			if l.name == "" {
				return l.players, nil
			}
		}
	}
	names := make([]string, len(g.shortlists))
	for i, l := range g.shortlists {
		names[i] = strconv.Quote(cmp.Or(l.name, defaultShortlist))
	}
	return nil, fmt.Errorf("%w: no shortlist %q; the save's shortlists: %s", ErrInvalidFilter, name, strings.Join(names, ", "))
}
