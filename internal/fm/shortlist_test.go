package fm

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// shortlistGame has players 1-5 (person indexes 11-15) at Leeds, player 5 a
// woman at Chelsea, the manager at Spurs and three shortlists.
func shortlistGame(lists []savefile.Shortlist) *Game {
	var raw []savefile.Player
	for i, name := range []string{"Adamonis", "Siaulys", "Gašpuitis", "Wesley", "Kirby"} {
		p := rawPlayer(uint32(i+1), "", name)    //nolint:gosec // test sizes
		p.Index, p.TeamIndex = uint32(i+11), 900 //nolint:gosec // test sizes
		p.CurrentAbility = int16(100 + i)        //nolint:gosec // test sizes
		raw = append(raw, p)
	}
	raw[4].Female, raw[4].TeamIndex = true, 20739
	return NewGame(Info{Date: gameDate, ManagerTeamIndex: 547}, raw, testWorld, PlayerExtras{Shortlists: lists})
}

var testShortlists = []savefile.Shortlist{
	{Name: "interested", Persons: []uint32{11, 13, 999}}, // 999 is no player
	{Name: "", Persons: []uint32{12, 15}},
	{Name: "wingers", Persons: []uint32{}},
}

func TestSearchShortlist(t *testing.T) {
	// Fog of war hides Leeds and Chelsea players' ability, so rows keep save order.
	v := shortlistGame(testShortlists).View(true)
	for _, tt := range []struct {
		name string
		f    Filter
		want []uint32
	}{
		{"named", Filter{Shortlist: "interested"}, []uint32{1, 3}},
		{"case ignored", Filter{Shortlist: "Interested"}, []uint32{1, 3}},
		{"default", Filter{Shortlist: "default"}, []uint32{2, 5}},
		{"default as shown", Filter{Shortlist: "(Default)"}, []uint32{2, 5}},
		{"empty list", Filter{Shortlist: "wingers"}, nil},
		{"with another filter", Filter{Shortlist: "interested", Nation: "england", MaxAge: 99}, []uint32{1, 3}},
		{"with gender", Filter{Shortlist: "default", Gender: "men"}, []uint32{2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if got := uids(res.Players); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearchShortlistExactNameWins(t *testing.T) {
	v := shortlistGame([]savefile.Shortlist{{Name: "", Persons: []uint32{12}}, {Name: "Default", Persons: []uint32{11}}}).View(false)
	res, err := v.Search(Filter{Shortlist: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if got := uids(res.Players); !slices.Equal(got, []uint32{1}) {
		t.Errorf("got %v, want the list named Default", got)
	}
}

func TestSearchShortlistErrors(t *testing.T) {
	_, err := shortlistGame(testShortlists).View(false).Search(Filter{Shortlist: "targets"})
	if !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("err = %v, want ErrInvalidFilter", err)
	}
	for _, name := range []string{`"interested"`, `"default"`, `"wingers"`} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("err = %q, want it to list %s", err, name)
		}
	}
	if _, err := shortlistGame(nil).View(false).Search(Filter{Shortlist: "interested"}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("unreadable shortlists: err = %v, want ErrInvalidFilter", err)
	}
}
