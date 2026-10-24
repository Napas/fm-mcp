package fm

import (
	"slices"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// balanced is a personality input that matches no rule but the last.
var balanced = personalityInput{
	determination: 10, ambition: 10, loyalty: 10, pressure: 10, professionalism: 10,
	sportsmanship: 10, temperament: 10, leadership: 10, age: 25,
}

func TestPersonalityFollowsTheGamesRule(t *testing.T) {
	with := func(change func(*personalityInput)) personalityInput {
		in := balanced
		change(&in)
		return in
	}
	tests := []struct {
		want string
		in   personalityInput
	}{
		{"Balanced", balanced},
		{"Model Citizen", with(func(in *personalityInput) {
			in.determination, in.ambition, in.professionalism, in.temperament, in.pressure, in.loyalty, in.sportsmanship = 14, 12, 15, 15, 14, 15, 15
		})},
		{"Fairly Professional", with(func(in *personalityInput) { // Model Citizen but sportsmanship 14
			in.determination, in.ambition, in.professionalism, in.temperament, in.pressure, in.loyalty, in.sportsmanship = 14, 12, 15, 15, 14, 15, 14
		})},
		{"Perfectionist", with(func(in *personalityInput) {
			in.determination, in.ambition, in.professionalism, in.temperament = 14, 14, 14, 9
		})},
		{"Balanced", with(func(in *personalityInput) {
			in.determination, in.ambition, in.professionalism, in.temperament = 14, 14, 14, 10
		})},
		{"Temperamental", with(func(in *personalityInput) { in.negative, in.temperament, in.professionalism = true, 4, 10 })},
		{"Balanced", with(func(in *personalityInput) { in.temperament, in.professionalism = 4, 10 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.temperament, in.professionalism = true, 5, 10 })},
		{"Born Leader", with(func(in *personalityInput) { in.age, in.leadership, in.determination = 23, 20, 20 })},
		{"Determined", with(func(in *personalityInput) { in.age, in.leadership, in.determination = 22, 20, 20 })},
		{"Leader", with(func(in *personalityInput) { in.age, in.leadership, in.determination = 23, 19, 20 })},
		{"Charismatic Leader", with(func(in *personalityInput) { in.age, in.leadership, in.temperament, in.sportsmanship = 23, 19, 18, 18 })},
		{"Leader", with(func(in *personalityInput) { in.age, in.leadership, in.temperament, in.sportsmanship = 23, 19, 17, 18 })},
		{"Balanced", with(func(in *personalityInput) { in.age, in.leadership = 30, 18 })},
		{"Model Professional", with(func(in *personalityInput) { in.age, in.professionalism = 23, 20 })},
		{"Professional", with(func(in *personalityInput) { in.age, in.professionalism = 22, 20 })},
		{"Professional", with(func(in *personalityInput) { in.professionalism = 18 })},
		{"Fairly Professional", with(func(in *personalityInput) { in.professionalism, in.temperament = 18, 9 })},
		{"Driven", with(func(in *personalityInput) { in.determination, in.ambition = 18, 12 })},
		{"Determined", with(func(in *personalityInput) { in.determination, in.ambition = 18, 11 })},
		{"Fairly Determined", with(func(in *personalityInput) { in.determination, in.ambition = 17, 12 })},
		{"Slack", with(func(in *personalityInput) { in.negative, in.professionalism, in.determination = true, 1, 9 })},
		{"Casual", with(func(in *personalityInput) { in.negative, in.professionalism, in.determination = true, 4, 9 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.professionalism, in.determination = true, 5, 9 })},
		{"Mercenary", with(func(in *personalityInput) { in.negative, in.loyalty, in.ambition = true, 3, 16 })},
		{"Fickle", with(func(in *personalityInput) { in.negative, in.loyalty, in.ambition = true, 6, 16 })},
		{"Ambitious", with(func(in *personalityInput) { in.loyalty, in.ambition = 3, 16 })},
		{"Very Ambitious", with(func(in *personalityInput) { in.ambition, in.loyalty = 20, 9 })},
		{"Ambitious", with(func(in *personalityInput) { in.ambition, in.loyalty = 16, 9 })},
		{"Fairly Ambitious", with(func(in *personalityInput) { in.ambition, in.loyalty = 16, 10 })},
		{"Unambitious", with(func(in *personalityInput) { in.negative, in.ambition, in.loyalty = true, 5, 11 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.ambition, in.loyalty = true, 5, 10 })},
		{"Honest", with(func(in *personalityInput) { in.sportsmanship, in.determination = 20, 9 })},
		{"Sporting", with(func(in *personalityInput) { in.sportsmanship, in.determination = 18, 9 })},
		{"Fairly Sporting", with(func(in *personalityInput) { in.sportsmanship = 18 })},
		{"Unsporting", with(func(in *personalityInput) { in.negative, in.sportsmanship, in.determination = true, 1, 11 })},
		{"Realist", with(func(in *personalityInput) { in.negative, in.sportsmanship, in.determination = true, 4, 11 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.sportsmanship, in.determination = true, 5, 11 })},
		{"Easily Discouraged", with(func(in *personalityInput) { in.negative, in.determination, in.ambition = true, 1, 9 })},
		{"Low Determination", with(func(in *personalityInput) { in.negative, in.determination, in.ambition = true, 5, 9 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.determination, in.ambition = true, 6, 9 })},
		{"Very Loyal", with(func(in *personalityInput) { in.loyalty, in.ambition = 20, 7 })},
		{"Devoted", with(func(in *personalityInput) { in.loyalty, in.ambition, in.devoted = 20, 7, true })},
		{"Loyal", with(func(in *personalityInput) { in.loyalty, in.ambition, in.devoted = 19, 7, true })},
		{"Loyal", with(func(in *personalityInput) { in.loyalty, in.ambition = 18, 7 })},
		{"Fairly Loyal", with(func(in *personalityInput) { in.loyalty, in.ambition = 18, 8 })},
		{"Iron Willed", with(func(in *personalityInput) { in.pressure, in.determination = 20, 15 })},
		{"Resilient", with(func(in *personalityInput) { in.pressure, in.determination = 17, 15 })},
		{"Jovial", with(func(in *personalityInput) { in.pressure, in.determination = 17, 14 })},
		{"Spineless", with(func(in *personalityInput) { in.negative, in.pressure, in.determination = true, 1, 9 })},
		{"Low Self Belief", with(func(in *personalityInput) { in.negative, in.pressure, in.determination = true, 3, 9 })},
		{"Balanced", with(func(in *personalityInput) { in.negative, in.pressure, in.determination = true, 4, 9 })},
		{"Light-Hearted", with(func(in *personalityInput) { in.pressure, in.sportsmanship = 15, 15 })},
		{"Spirited", with(func(in *personalityInput) { in.pressure, in.professionalism = 15, 11 })},
		{"Jovial", with(func(in *personalityInput) { in.pressure = 15 })},
		{"Balanced", with(func(in *personalityInput) { in.pressure, in.temperament = 15, 9 })},
		{"Fairly Determined", with(func(in *personalityInput) { in.determination, in.professionalism = 15, 14 })},
		{"Resolute", with(func(in *personalityInput) { in.determination, in.professionalism = 15, 15 })},
		{"Fairly Ambitious", with(func(in *personalityInput) { in.ambition = 15 })},
		{"Fairly Loyal", with(func(in *personalityInput) { in.loyalty = 15 })},
		{"Fairly Sporting", with(func(in *personalityInput) { in.sportsmanship = 15 })},
		{"Balanced", with(func(in *personalityInput) {
			in.determination, in.ambition, in.loyalty, in.pressure, in.professionalism, in.sportsmanship = 14, 14, 14, 14, 14, 14
		})},
		// In-game samples (26.3.2).
		{"Fairly Professional", personalityInput{ambition: 11, loyalty: 16, pressure: 12, professionalism: 15, sportsmanship: 17, temperament: 17, determination: 14, leadership: 14, age: 25}}, // Ashby
		{"Driven", personalityInput{ambition: 19, loyalty: 13, pressure: 8, professionalism: 13, sportsmanship: 8, temperament: 10, determination: 20, leadership: 8, age: 24}},                 // Sami
		{"Light-Hearted", personalityInput{ambition: 14, loyalty: 14, pressure: 15, professionalism: 17, sportsmanship: 15, temperament: 16, determination: 15, leadership: 11, age: 33}},       // Livramento
		{"Light-Hearted", personalityInput{ambition: 16, loyalty: 14, pressure: 16, professionalism: 15, sportsmanship: 18, temperament: 15, determination: 15, leadership: 7, age: 25}},        // Bernardi
		{"Fairly Determined", personalityInput{ambition: 11, loyalty: 13, pressure: 11, professionalism: 13, sportsmanship: 12, temperament: 11, determination: 15, leadership: 9, age: 29}},    // Leroux
	}
	for _, tt := range tests {
		if got := tt.in.label(); got != tt.want {
			t.Errorf("%+v: %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPlayerPersonalityReadsHiddenAttributesDeterminationLeadershipAndAge(t *testing.T) {
	raw := rawPlayer(2002078397, "Chris", "Ashby")
	raw.Hidden = [8]uint8{7, 11, 16, 12, 15, 17, 17, 3}
	raw.Attributes[attrDetermination] = 70 // 14 on the 1-20 scale
	raw.Attributes[attrLeadership] = 70    // 14

	p := newPlayer(raw, gameDate, testWorld)

	if got := p.personality(); got != "Fairly Professional" {
		t.Errorf("personality = %q, want Fairly Professional", got)
	}
	if p.Attributes.Get("Adaptability") != 7 || p.Attributes.Get("Ambition") != 11 || p.Attributes.Get("Controversy") != 3 {
		t.Errorf("hidden attributes = %v", p.Attributes)
	}

	raw.Hidden = [8]uint8{10, 16, 3, 10, 10, 10, 10, 10} // Mercenary when negative labels apply
	raw.Attributes[attrDetermination], raw.Attributes[attrLeadership] = 50, 50
	if got := newPlayer(raw, gameDate, testWorld).personality(); got != "Ambitious" {
		t.Errorf("without the flag: %q, want Ambitious", got)
	}
	raw.FlagBit3 = true
	if got := newPlayer(raw, gameDate, testWorld).personality(); got != "Mercenary" {
		t.Errorf("with the flag: %q, want Mercenary", got)
	}
}

// Devoted needs the player's affinity for his club above 99: the club of his
// current team, else of his main contract's team.
func TestPlayerIsDevotedWithAffinityAbove99ForHisClub(t *testing.T) {
	tests := []struct {
		name      string
		team      int64
		contract  int64 // main contract's team; noTeam for none
		favourite savefile.ClubAffinity
		want      string
	}{
		{"current club", 547, 547, savefile.ClubAffinity{Club: 416, Value: 100, Reason: 1}, "Devoted"},
		{"affinity 99", 547, 547, savefile.ClubAffinity{Club: 416, Value: 99, Reason: 1}, "Very Loyal"},
		{"another club", 547, 547, savefile.ClubAffinity{Club: 20, Value: 100, Reason: 2}, "Very Loyal"},
		{"loan club", 900, 547, savefile.ClubAffinity{Club: 20, Value: 100}, "Devoted"},
		{"parent club of a loanee", 900, 547, savefile.ClubAffinity{Club: 416, Value: 100}, "Very Loyal"},
		{"no team, contract club", noTeam, 547, savefile.ClubAffinity{Club: 416, Value: 100}, "Devoted"},
		{"free agent", noTeam, noTeam, savefile.ClubAffinity{Club: 416, Value: 100}, "Very Loyal"},
	}
	for _, tt := range tests {
		raw := rawPlayer(1, "Yaser", "Yousefi")
		raw.Hidden = [8]uint8{10, 7, 20, 10, 10, 10, 10, 10} // ambition 7, loyalty 20
		raw.TeamIndex = tt.team
		if tt.contract != noTeam {
			raw.Contract = &savefile.Contract{TeamIndex: tt.contract}
		}
		raw.ClubAffinities = []savefile.ClubAffinity{{Club: 9, Value: 100}, tt.favourite}
		if got := newPlayer(raw, gameDate, testWorld).personality(); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPlayerPersonalityCountsAgeAtTheGameDate(t *testing.T) {
	raw := rawPlayer(1, "Born", "Leader")
	raw.Attributes[attrDetermination], raw.Attributes[attrLeadership] = 100, 100 // 20
	raw.BirthDate = savefile.Date{Raw: 188, Year: 2002}                          // 23 on 2025-07-07
	if got := newPlayer(raw, gameDate, testWorld).personality(); got != "Born Leader" {
		t.Errorf("23 on the game date: %q, want Born Leader", got)
	}
	if got := newPlayer(raw, gameDate.Add(-24*time.Hour), testWorld).personality(); got != "Determined" {
		t.Errorf("22 the day before: %q, want Determined", got)
	}
}

func TestTraitNamesFollowBitOrderAndSkipUnnamedBits(t *testing.T) {
	tests := []struct {
		mask uint64
		want []string
	}{
		{0, nil},
		{0x8000, []string{"Marks Opponent Tightly"}},
		{0x0100000000000002, []string{"Runs With Ball Down Right", "Refrains From Taking Long Shots"}},
		{0x400008000, []string{"Marks Opponent Tightly", "Avoids Using Weaker Foot"}},
		{1<<48 | 1<<53 | 1<<63, []string{"Plays Ball With Feet"}},
	}
	for _, tt := range tests {
		if got := traitNames(tt.mask); !slices.Equal(got, tt.want) {
			t.Errorf("traitNames(%#x) = %q, want %q", tt.mask, got, tt.want)
		}
	}
}
