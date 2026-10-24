package fm

import (
	"slices"
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// mediaOf runs the media handling rule on hidden attributes in save order
// (Adaptability, Ambition, Loyalty, Pressure, Professionalism, Sportsmanship,
// Temperament, Controversy) through a Player.
func mediaOf(hidden [8]uint8, flagBit3 bool) []string {
	raw := rawPlayer(1, "Media", "Test")
	raw.Hidden, raw.FlagBit3 = hidden, flagBit3
	return newPlayer(raw, gameDate, testWorld).mediaHandling()
}

// The six in-game samples of 2035-12-14.
func TestMediaHandlingMatchesTheGame(t *testing.T) {
	tests := []struct {
		name     string
		hidden   [8]uint8
		flagBit3 bool
		want     []string
	}{
		{"Bernardi", [8]uint8{10, 16, 14, 16, 15, 18, 15, 6}, true, []string{"Evasive", "Unflappable"}},
		{"Leroux", [8]uint8{11, 11, 13, 11, 13, 12, 11, 5}, false, []string{"Level-headed"}},
		{"Matković", [8]uint8{14, 17, 12, 13, 12, 9, 11, 6}, false, []string{"Media-friendly"}},
		{"Assongo", [8]uint8{10, 10, 10, 10, 14, 10, 10, 10}, false, []string{"Media-friendly"}},
		{"Čuber Potočnik", [8]uint8{10, 15, 8, 6, 14, 12, 15, 14}, false, []string{"Media-friendly"}},
		{"Lobede", [8]uint8{5, 15, 13, 17, 13, 2, 3, 14}, true, []string{"Volatile", "Confrontational"}},
	}
	for _, tt := range tests {
		if got := mediaOf(tt.hidden, tt.flagBit3); !slices.Equal(got, tt.want) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMediaHandlingFollowsTheGamesRule(t *testing.T) {
	level := mediaInput{loyalty: 13, pressure: 11, professionalism: 13, sportsmanship: 12, temperament: 11, controversy: 10}
	with := func(change func(*mediaInput)) mediaInput {
		in := level
		change(&in)
		return in
	}
	tests := []struct {
		name string
		in   mediaInput
		want uint16
	}{
		{"nothing matches", level, mediaLevelHeaded},
		{"short-tempered", with(func(in *mediaInput) { in.negative, in.temperament = true, 2 }), mediaShortTempered},
		{"volatile", with(func(in *mediaInput) { in.negative, in.temperament = true, 6 }), mediaVolatile},
		{"evasive", with(func(in *mediaInput) {
			in.professionalism, in.pressure, in.temperament, in.sportsmanship = 15, 15, 14, 13
		}), mediaEvasive},
		{"evasive by low controversy", with(func(in *mediaInput) {
			in.professionalism, in.pressure, in.sportsmanship, in.controversy = 15, 15, 12, 14
		}), mediaEvasive},
		{"not evasive at temperament 16", with(func(in *mediaInput) {
			in.professionalism, in.pressure, in.temperament, in.sportsmanship = 15, 15, 16, 13
		}), mediaUnflappable},
		{"outspoken", with(func(in *mediaInput) { in.controversy = 15 }), mediaOutspoken},
		{"media-friendly by loyalty", with(func(in *mediaInput) { in.loyalty = 10 }), mediaFriendly},
		{"media-friendly by low professionalism and sportsmanship", with(func(in *mediaInput) { in.professionalism, in.sportsmanship = 12, 11 }), mediaFriendly},
		{"confrontational", with(func(in *mediaInput) { in.negative, in.sportsmanship, in.temperament = true, 7, 7 }), mediaConfrontational},
		{"reserved", with(func(in *mediaInput) { in.controversy, in.professionalism = 5, 15 }), mediaReserved},
		{"not reserved when volatile", with(func(in *mediaInput) { in.negative, in.controversy, in.professionalism, in.temperament = true, 5, 15, 6 }), mediaVolatile},
		// Without flag bit 3 the negative styles turn into Outspoken; one
		// already Outspoken turns into Evasive.
		{"volatile without the flag", with(func(in *mediaInput) { in.temperament = 6 }), mediaOutspoken},
		{"confrontational without the flag", with(func(in *mediaInput) { in.sportsmanship, in.temperament = 7, 7 }), mediaOutspoken},
		{"outspoken and volatile without the flag", with(func(in *mediaInput) { in.controversy, in.temperament = 15, 6 }), mediaEvasive},
	}
	for _, tt := range tests {
		if got := tt.in.mask(); got != tt.want {
			t.Errorf("%s: %#x, want %#x", tt.name, got, tt.want)
		}
	}
}

func TestMediaHandlingNamesFollowBitOrder(t *testing.T) {
	got := mediaNames(mediaOutspoken | mediaEvasive | mediaVolatile | mediaMindGames | mediaUnflappable | mediaFriendly | mediaLevelHeaded | mediaReserved | mediaShortTempered | mediaConfrontational)
	want := []string{"Outspoken", "Evasive", "Volatile", "Plays Mind Games", "Unflappable", "Media-friendly", "Level-headed", "Reserved", "Short-tempered", "Confrontational"}
	if !slices.Equal(got, want) {
		t.Errorf("names %q, want %q", got, want)
	}
}

// Media handling comes from hidden attributes, so fog of war hides it with
// the personality.
func TestMediaHandlingIsHiddenWithThePersonality(t *testing.T) {
	g := fogGame()
	for uid, want := range map[uint32]bool{1: true, 6: false} { // own player; unscouted
		p, _ := g.View(true).Profile(uid)
		if got := len(p.MediaHandling) > 0; got != want || got != (p.Personality != "") {
			t.Errorf("uid %d: media handling %q, personality %q; want shown %v", uid, p.MediaHandling, p.Personality, want)
		}
	}
	raw := rawPlayer(1, "Media", "Test")
	raw.Hidden = [8]uint8{12, 12, 12, 12, 12, 12, 12, 12}
	g = NewGame(Info{Date: gameDate}, []savefile.Player{raw}, testWorld, PlayerExtras{})
	if p, _ := g.View(false).Profile(1); !slices.Equal(p.MediaHandling, []string{"Level-headed"}) {
		t.Errorf("without fog of war: %q", p.MediaHandling)
	}
}
