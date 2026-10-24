package fm

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/Napas/fm-mcp/internal/savefile"
)

var gameDate = time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC)

var testWorld = World{
	Teams: map[int64]*Team{
		547:   {Index: 547, ClubIndex: 416, Name: "Tottenham Hotspur", ShortName: "Spurs", Club: "Tottenham Hotspur", Division: "Premier League", Reputation: 7750},
		27845: {Index: 27845, ClubIndex: 416, Name: "Tottenham Hotspur U21", ShortName: "Spurs", Club: "Tottenham Hotspur", Reputation: 4000},
		20739: {Index: 20739, ClubIndex: 9, Name: "Chelsea", Club: "Chelsea", Division: "Barclays Women's Super League", Reputation: 9000, Women: true},
		900:   {Index: 900, ClubIndex: 20, Name: "Leeds United", Club: "Leeds United", Reputation: 6000},
		1233:  {Index: 1233, ClubIndex: noTeam, Name: "Argentina", National: true, Reputation: 9000},
	},
	Clubs: map[int64]Club{
		416: {Name: "Spurs", FirstTeam: 547},
		9:   {Name: "Chelsea Women", FirstTeam: 20739},
		20:  {Name: "Leeds United", FirstTeam: 900},
		276: {Name: "Svendborg fB", FirstTeam: noTeam},
	},
	Nations:  map[int64]string{139: "England"},
	Injuries: map[int64]string{27: "torn anterior cruciate ligament"},
}

func rawPlayer(uid uint32, first, last string) savefile.Player {
	p := savefile.Player{
		UID: uid, FirstName: first, LastName: last,
		BirthDate:   savefile.Date{Raw: 328, Year: 1996}, // 1996-11-23
		NationIndex: 139, TeamIndex: 547,
		CurrentAbility: 151, PotentialAbility: 163,
		HomeReputation: 7500, CurrentReputation: 7550, WorldReputation: 6500,
	}
	for i := range p.Attributes {
		p.Attributes[i] = 50
	}
	p.Positions[10] = 20 // AMC
	p.Positions[7] = 18  // MC
	p.Positions[0] = 1   // GK
	return p
}

func TestNewPlayerDerivesDisplayFields(t *testing.T) {
	raw := rawPlayer(28093567, "James", "Maddison")
	raw.Attributes[2] = 68 // Finishing 13.6 → 14
	raw.Attributes[3] = 3  // Heading 0.6 → 1

	p := newPlayer(raw, gameDate, testWorld)

	if p.Name != "James Maddison" || p.Club != "Tottenham Hotspur" || p.Nation != "England" {
		t.Errorf("name/club/nation = %q %q %q", p.Name, p.Club, p.Nation)
	}
	if p.Age != 28 || !p.BirthDate.Equal(time.Date(1996, 11, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("age/birth = %d %s", p.Age, p.BirthDate)
	}
	if p.Attributes.Get("Finishing") != 14 || p.Attributes.Get("Heading") != 1 || p.Attributes.Get("Passing") != 10 {
		t.Errorf("attributes = %v", p.Attributes)
	}
	if got := p.BestPositions(); len(got) != 2 || got[0] != "AMC" || got[1] != "MC" {
		t.Errorf("BestPositions = %v, want [AMC MC]", got)
	}
}

func TestNewPlayerPrefersCommonName(t *testing.T) {
	raw := rawPlayer(1, "", "")
	raw.CommonName = "Richarlison"

	if p := newPlayer(raw, gameDate, testWorld); p.Name != "Richarlison" {
		t.Errorf("Name = %q, want Richarlison", p.Name)
	}
}

func TestAgeCountsWholeYears(t *testing.T) {
	birth := time.Date(2006, 2, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		on   time.Time
		want int
	}{
		{time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), 18},
		{time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC), 19},
		{time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC), 19},
	}
	for _, tt := range tests {
		if got := age(birth, tt.on); got != tt.want {
			t.Errorf("age on %s = %d, want %d", tt.on.Format(time.DateOnly), got, tt.want)
		}
	}
}

func TestNewPlayerReadsGender(t *testing.T) {
	raw := rawPlayer(9, "Alyssa", "Thompson")
	raw.Female, raw.TeamIndex = true, 20739

	if p := newPlayer(raw, gameDate, testWorld); !p.Female || p.Club != "Chelsea" {
		t.Errorf("player = %+v", p)
	}
}

func TestAttributeScoreMatchesGameDisplay(t *testing.T) {
	for raw, want := range map[uint8]int{0: 1, 1: 1, 62: 12, 63: 13, 98: 20, 100: 20, 255: 20} {
		if got := attributeScore(raw); got != want {
			t.Errorf("attributeScore(%d) = %d, want %d", raw, got, want)
		}
	}
}

func TestNewPlayerNamesQualificationBonus(t *testing.T) {
	raw := rawPlayer(2002100552, "Carlo", "Bernardi")
	raw.Contract = &savefile.Contract{TeamIndex: 547, Bonuses: []savefile.CompetitionBonus{
		{Type: 10, Competition: 11, Target: 637, Value: 60964},
		{Type: 10, Competition: 11, Target: 639, Value: 30000}, // target without a name
		{Type: 6, Competition: 11, Target: -1, Value: 12028},   // undecoded type
	}}
	w := testWorld
	w.Competitions = map[int64]string{11: "Ligue 1", 637: "UEFA Champions League"}

	got := newPlayer(raw, gameDate, w).Contract.Clauses

	want := []ClauseInfo{
		{Name: "UEFA Champions League qualification bonus", Amount: 60964},
		{Name: "qualification bonus", Amount: 30000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Clauses = %+v, want %+v", got, want)
	}
}

func TestNewPlayerContractInjuryAndPhysical(t *testing.T) {
	raw := rawPlayer(28093567, "James", "Maddison")
	raw.Value, raw.Height, raw.Condition, raw.Sharpness = 51500000, 175, 9300, 5400
	raw.Contract = &savefile.Contract{
		TeamIndex: 547, WeeklyWage: 147000,
		Start: savefile.Date{Raw: 179, Year: 2023}, End: savefile.Date{Raw: 182, Year: 2028}, // 2028 is a leap year
		Clauses: []savefile.Clause{{Type: 0, Value: 60000000, Extra: 0xffff}, {Type: 33, Value: 20000, Extra: 0xffff}},
	}
	raw.Loan = &savefile.Contract{TeamIndex: 27845, WeeklyWage: 1000, End: savefile.Date{Raw: 181, Year: 2026}}
	raw.Injuries = []savefile.Injury{{ID: 27, Start: savefile.Date{Raw: 188, Year: 2025}, DaysMin: 267, DaysMax: 297}}

	p := newPlayer(raw, gameDate, testWorld)

	if p.Value != 51500000 || p.HeightCM != 175 || p.Condition != 93 || p.Sharpness != 54 {
		t.Errorf("value/height/condition/sharpness = %d %d %v %v", p.Value, p.HeightCM, p.Condition, p.Sharpness)
	}
	c := p.Contract
	if c == nil || c.Team != "Tottenham Hotspur" || c.WeeklyWage != 147000 || !c.End.Equal(time.Date(2028, 6, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Contract = %+v", c)
	}
	if len(c.Clauses) != 2 || c.Clauses[0].Name != "minimum fee release clause" || c.Clauses[0].Amount != 60000000 || c.Clauses[1].Name != "goal bonus" {
		t.Errorf("Clauses = %+v", c.Clauses)
	}
	if p.Loan == nil || p.Loan.Team != "Tottenham Hotspur U21" {
		t.Errorf("Loan = %+v", p.Loan)
	}
	if !p.Injured() || p.Injuries[0].ReturnEarliest.Format(time.DateOnly) != "2026-03-31" || p.Injuries[0].ReturnLatest.Format(time.DateOnly) != "2026-05-01" {
		t.Errorf("Injuries = %+v", p.Injuries)
	}
	if p.Injuries[0].Name != "torn anterior cruciate ligament" {
		t.Errorf("injury name = %q", p.Injuries[0].Name)
	}
}

func TestNewPlayerReadsPlayingTimeAndSquadNumberFromTheCurrentContract(t *testing.T) {
	main := &savefile.Contract{TeamIndex: 547, PlayingTime: 1, OriginalPlayingTime: 4, PlayingTimeScore: 81, SquadNumber: 16}
	type times struct{ current, original, actual savefile.PlayingTime }
	tests := []struct {
		name           string
		team           int64
		contract, loan *savefile.Contract
		want           times
		squadNumber    int
	}{
		{"main contract", 547, main, nil, times{1, 4, 3}, 16},
		{"on loan", 900, main, &savefile.Contract{TeamIndex: 900, PlayingTime: 14, OriginalPlayingTime: 22, PlayingTimeScore: -15, SquadNumber: 28}, times{14, 22, 22}, 28},
		{"loan without playing time or number", 900, main, &savefile.Contract{TeamIndex: 900, SquadNumber: 255}, times{}, 0},
		{"BASIC contract", 547, &savefile.Contract{TeamIndex: 547}, nil, times{}, 0},
		{"free agent", -1, nil, nil, times{}, 0},
	}
	for _, tt := range tests {
		raw := rawPlayer(1, "Luke", "Reddy")
		raw.TeamIndex, raw.Contract, raw.Loan = tt.team, tt.contract, tt.loan
		p := newPlayer(raw, gameDate, testWorld)
		if got := (times{p.playingTime, p.originalPlayingTime, p.actualPlayingTime}); got != tt.want || p.SquadNumber != tt.squadNumber {
			t.Errorf("%s: playing times %+v, squad number %d; want %+v, %d", tt.name, got, p.SquadNumber, tt.want, tt.squadNumber)
		}
	}
	raw := rawPlayer(1, "Grégory", "Le Ven")
	raw.TeamIndex, raw.Contract = 900, main
	raw.Loan = &savefile.Contract{TeamIndex: 900, SquadNumber: 14}
	if p := newPlayer(raw, gameDate, testWorld); p.Contract.SquadNumber != 16 || p.Loan.SquadNumber != 14 {
		t.Errorf("contract squad numbers %d and %d, want 16 (main) and 14 (loan)", p.Contract.SquadNumber, p.Loan.SquadNumber)
	}
}

// A keeper is a player rated 18 or more in goal. The game may count a
// player up to 25 as a youth, so a result the youth test changes is left out.
func TestNewPlayerRatesActualPlayingTimeForKeepersAndAge(t *testing.T) {
	keeper := rawPlayer(1, "Bruno", "Sami")
	keeper.Positions[0] = 18
	keeper.Contract = &savefile.Contract{TeamIndex: 547, PlayingTime: 20, PlayingTimeScore: 60}
	notKeeper := keeper
	notKeeper.Positions[0] = 17
	young := rawPlayer(2, "Jesús", "Lobede")
	young.BirthDate = savefile.Date{Raw: 189, Year: 1999} // 25 on 2025-07-07
	young.Contract = &savefile.Contract{TeamIndex: 547, PlayingTime: 4, PlayingTimeScore: -50}
	older := young
	older.BirthDate = savefile.Date{Raw: 188, Year: 1999} // 26 that day

	for _, tt := range []struct {
		name string
		raw  savefile.Player
		want savefile.PlayingTime
	}{
		{"keeper", keeper, 15},
		{"not a keeper", notKeeper, 3},
		{"25", young, 0},
		{"26", older, 22},
	} {
		if got := newPlayer(tt.raw, gameDate, testWorld).actualPlayingTime; got != tt.want {
			t.Errorf("%s: actual playing time %d, want %d", tt.name, got, tt.want)
		}
	}
}

// Actual playing time is served only at the current contract's team when that
// is a club's first team: reserve and youth players' scores are not checked in
// game.
func TestNewPlayerLeavesOutActualPlayingTimeAwayFromAFirstTeam(t *testing.T) {
	contract := func(team int64) *savefile.Contract {
		return &savefile.Contract{TeamIndex: team, PlayingTime: 4, PlayingTimeScore: -100, SquadNumber: 24}
	}
	for _, tt := range []struct {
		name     string
		team     int64
		contract *savefile.Contract
		want     savefile.PlayingTime
	}{
		{"first team", 547, contract(547), 22},
		{"at the reserves", 27845, contract(547), 0},
		{"contracted to the reserves", 27845, contract(27845), 0},
	} {
		raw := rawPlayer(1, "Bassirou", "Sow")
		raw.TeamIndex, raw.Contract = tt.team, tt.contract
		p := newPlayer(raw, gameDate, testWorld)
		if p.actualPlayingTime != tt.want || p.playingTime != 4 || p.SquadNumber != 24 {
			t.Errorf("%s: actual %d, playing time %d, squad number %d; want %d, 4, 24", tt.name, p.actualPlayingTime, p.playingTime, p.SquadNumber, tt.want)
		}
	}
}

// The game counts days to return from the next day: 0–2 days from today
// shows as "1–2 days".
func TestInjuryReturnIsNotBeforeTheNextDay(t *testing.T) {
	for _, c := range []struct {
		start            uint16 // day of 2025; the game date is day 188
		daysMin, daysMax uint16
		earliest, latest string
	}{
		{188, 0, 2, "2025-07-08", "2025-07-09"},   // from today
		{186, 0, 3, "2025-07-08", "2025-07-08"},   // earliest return passed
		{184, 1, 3, "2025-07-08", "2025-07-08"},   // latest return was yesterday
		{184, 1, 4, "2025-07-08", "2025-07-08"},   // latest return is today
		{150, 30, 60, "2025-07-08", "2025-07-29"}, // 2025-05-30 + 30…60 days
		{188, 5, 9, "2025-07-12", "2025-07-17"},   // earliest return still ahead: latest + 1 day
	} {
		raw := rawPlayer(1, "Danilo", "Veiga")
		raw.Injuries = []savefile.Injury{{ID: 84, Start: savefile.Date{Raw: c.start, Year: 2025}, DaysMin: c.daysMin, DaysMax: c.daysMax}}

		in := newPlayer(raw, gameDate, testWorld).Injuries[0]

		if got := in.ReturnEarliest.Format(time.DateOnly) + " " + in.ReturnLatest.Format(time.DateOnly); got != c.earliest+" "+c.latest {
			t.Errorf("start day %d, %d–%d days: return %s, want %s %s", c.start, c.daysMin, c.daysMax, got, c.earliest, c.latest)
		}
	}
}

// Real 26.3.2 samples, shown in game as "Expected to return in …".
func TestInjuryReturnMatchesTheGame(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, c := range []struct {
		name             string
		start            uint16 // day of 2035: 12-01 = 335
		daysMin, daysMax uint16
		on               string
		earliest, latest string
	}{
		{"Leroux", 338, 7, 12, "2035-12-09", "2035-12-11", "2035-12-17"},
		{"Bernardi", 341, 0, 3, "2035-12-09", "2035-12-10", "2035-12-10"},
		{"Veiga", 336, 0, 2, "2035-12-02", "2035-12-03", "2035-12-04"},
		{"Bernardi on 12-02", 336, 0, 3, "2035-12-02", "2035-12-03", "2035-12-05"},
		{"earliest is the next day", 335, 8, 10, "2035-12-08", "2035-12-09", "2035-12-12"},
	} {
		in := savefile.Injury{ID: 84, Start: savefile.Date{Raw: c.start, Year: 2035}, DaysMin: c.daysMin, DaysMax: c.daysMax}

		got := injuryInfo(in, day(c.on), testWorld)

		if g := got.ReturnEarliest.Format(time.DateOnly) + " " + got.ReturnLatest.Format(time.DateOnly); g != c.earliest+" "+c.latest {
			t.Errorf("%s: return %s, want %s %s", c.name, g, c.earliest, c.latest)
		}
	}
}

func TestAttributesAccessAndJSON(t *testing.T) {
	var a Attributes
	for i := range a {
		a[i] = 10
	}
	a[2] = 14 // Finishing

	if a.Get("Finishing") != 14 || a.Get("finishing") != 14 || a.Get("Speed") != 0 {
		t.Errorf("Get = %d %d %d", a.Get("Finishing"), a.Get("finishing"), a.Get("Speed"))
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]int
	if err := json.Unmarshal(b, &m); err != nil || m["Finishing"] != 14 || m["Ambition"] != 10 || len(m) != 62 {
		t.Errorf("JSON = %s (%v)", b, err)
	}
	var back Attributes
	if err := json.Unmarshal(b, &back); err != nil || back != a {
		t.Errorf("round trip = %v, %v", back, err)
	}
}

func TestAttributesJSONLeavesOutUnknownAttributes(t *testing.T) {
	var a Attributes
	a[2] = 14 // Finishing; the rest are unknown (0)

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `{"Finishing":14}` {
		t.Errorf("JSON = %s, want only Finishing", b)
	}
}

func TestNewPlayerTreatsTheValueCapAsNotForSale(t *testing.T) {
	raw := rawPlayer(28124569, "Tino", "Livramento")
	raw.Value = 300_000_000

	p := newPlayer(raw, gameDate, testWorld)

	if !p.NotForSale || p.Value != 0 {
		t.Errorf("NotForSale = %v, Value = %d; want true and 0", p.NotForSale, p.Value)
	}
	raw.Value = 299_999_999
	if p := newPlayer(raw, gameDate, testWorld); p.NotForSale || p.Value != 299_999_999 {
		t.Errorf("below the cap: NotForSale = %v, Value = %d", p.NotForSale, p.Value)
	}
}

func TestNewPlayerLeavesOutUnsetDates(t *testing.T) {
	raw := rawPlayer(2002128293, "Stephen", "Connolly")
	raw.JoinedClub = savefile.Date{Raw: 1, Year: 1900}
	raw.Contract = &savefile.Contract{TeamIndex: 547, Start: savefile.Date{Raw: 1, Year: 1900}, End: savefile.Date{Raw: 2, Year: 1900}}
	raw.Loan = &savefile.Contract{TeamIndex: 27845, Start: savefile.Date{Raw: 5, Year: 1900}, End: savefile.Date{Raw: 365, Year: 1900}}

	p := newPlayer(raw, gameDate, testWorld)

	if p.JoinedClub != nil {
		t.Errorf("JoinedClub = %s, want none for a 1900 date", p.JoinedClub)
	}
	for name, c := range map[string]*Contract{"contract": p.Contract, "loan": p.Loan} {
		if !c.Start.IsZero() || !c.Joined.IsZero() || !c.End.IsZero() {
			t.Errorf("%s dates = start %s, joined %s, end %s; want none for 1900 dates", name, c.Start, c.Joined, c.End)
		}
	}
	raw.JoinedClub = savefile.Date{Raw: 170, Year: 2034}
	raw.Contract.End = savefile.Date{Raw: 181, Year: 1901}
	p = newPlayer(raw, gameDate, testWorld)
	if p.JoinedClub == nil || p.JoinedClub.Format(time.DateOnly) != "2034-06-19" {
		t.Errorf("JoinedClub = %v, want 2034-06-19", p.JoinedClub)
	}
	if p.Contract.End.Year() != 1901 {
		t.Errorf("contract end = %s, want 1901 kept", p.Contract.End)
	}
}

func TestContractStartIsTheSignedDate(t *testing.T) {
	// Enzo Fernández (2036-01-01 save): joined 2023-01-31; the game shows Start 2 Feb 34.
	raw := rawPlayer(14229525, "Enzo", "Fernández")
	raw.Contract = &savefile.Contract{TeamIndex: 456, Start: savefile.Date{Raw: 31, Year: 2023}, Signed: savefile.Date{Raw: 33, Year: 2034}}

	p := newPlayer(raw, gameDate, testWorld)

	if got := p.Contract.Start.Format(time.DateOnly); got != "2034-02-02" {
		t.Errorf("contract start = %s, want the signed date 2034-02-02", got)
	}
	if got := p.Contract.Joined.Format(time.DateOnly); got != "2023-01-31" {
		t.Errorf("contract joined = %s, want 2023-01-31", got)
	}
	if info := (View{}).contract(p.Contract, 0); info.Joined != p.Contract.Joined {
		t.Errorf("served joined = %s, want %s", info.Joined, p.Contract.Joined)
	}
	raw.Contract.Signed = savefile.Date{}
	if got := newPlayer(raw, gameDate, testWorld).Contract.Start.Format(time.DateOnly); got != "2023-01-31" {
		t.Errorf("contract start without a signed date = %s, want the start 2023-01-31", got)
	}
}

func TestSearchIgnoresUnsetContractEnds(t *testing.T) {
	raw := rawPlayer(1, "Stephen", "Connolly")
	raw.Contract = &savefile.Contract{TeamIndex: 547, End: savefile.Date{Raw: 2, Year: 1900}}
	g := NewGame(Info{Date: gameDate}, []savefile.Player{raw}, testWorld, PlayerExtras{})

	res, err := g.View(false).Search(Filter{ContractExpiresBy: "2026-06-30"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Errorf("an unset contract end matched contract_expires_by: %v", uids(res.Players))
	}
}

// The remaining loyalty bonus is the weekly instalment times the whole weeks
// left on the main contract.
func TestLoyaltyBonusRemainingFollowsTheGame(t *testing.T) {
	day := func(doy, year uint16) savefile.Date { return savefile.Date{Raw: doy, Year: year} } // the game date is day 188 of 2025
	tests := []struct {
		name     string
		contract *savefile.Contract
		want     int64
	}{
		{"13 days left: one week", &savefile.Contract{End: day(201, 2025), WeeklyLoyalty: 1000}, 1000},
		{"7 days left", &savefile.Contract{End: day(195, 2025), WeeklyLoyalty: 1000}, 1000},
		{"6 days left", &savefile.Contract{End: day(194, 2025), WeeklyLoyalty: 1000}, 0},
		{"ended", &savefile.Contract{End: day(100, 2025), WeeklyLoyalty: 1000}, 0},
		{"across years", &savefile.Contract{End: day(181, 2027), WeeklyLoyalty: 2299}, 103 * 2299}, // 2027-06-30: 723 days
		{"end unset (1900-01-02)", &savefile.Contract{End: day(2, 1900), WeeklyLoyalty: 1000}, 0},
		{"end 1900-01-01 counts 1825 days", &savefile.Contract{End: day(1, 1900), WeeklyLoyalty: 1000}, 260 * 1000},
		{"end 1900-01-01 with upper date bits", &savefile.Contract{End: day(1|13<<9, 1900), WeeklyLoyalty: 1000}, 260 * 1000},
		{"end another 1900 date", &savefile.Contract{End: day(64, 1900), WeeklyLoyalty: 1000}, 0},
		{"at most 30000 days", &savefile.Contract{End: day(1, 2200), WeeklyLoyalty: 10}, 4285 * 10},
		{"100 or less is nothing", &savefile.Contract{End: day(201, 2025), WeeklyLoyalty: 100}, 0},
		{"101 is paid", &savefile.Contract{End: day(201, 2025), WeeklyLoyalty: 101}, 101},
		{"no weekly instalment", &savefile.Contract{End: day(181, 2027), LoyaltyBonus: 500_000}, 0},
		{"BASIC contract", &savefile.Contract{WeeklyLoyalty: 1000}, 0},
		{"free agent", nil, 0},
	}
	for _, tt := range tests {
		raw := rawPlayer(1, "Chris", "Ashby")
		raw.Contract = tt.contract
		if got := newPlayer(raw, gameDate, testWorld).LoyaltyBonusRemaining; got != tt.want {
			t.Errorf("%s: remaining loyalty bonus %d, want %d", tt.name, got, tt.want)
		}
	}
}

// Only the main contract carries a loyalty bonus; loan and future contracts
// store none.
func TestLoyaltyBonusRemainingIgnoresLoans(t *testing.T) {
	raw := rawPlayer(1, "Chris", "Ashby")
	raw.Contract = &savefile.Contract{TeamIndex: 547, End: savefile.Date{Raw: 201, Year: 2025}, WeeklyLoyalty: 1000}
	raw.Loan = &savefile.Contract{TeamIndex: 900, End: savefile.Date{Raw: 201, Year: 2026}, WeeklyLoyalty: 5000}

	if got := newPlayer(raw, gameDate, testWorld).LoyaltyBonusRemaining; got != 1000 {
		t.Errorf("remaining loyalty bonus %d, want the main contract's 1000", got)
	}
}

// TestPlayerAuxFollowsPlayerOrder checks that each player's aux index is
// his place in the game and leads to his own per-player data.
func TestPlayerAuxFollowsPlayerOrder(t *testing.T) {
	raw := []savefile.Player{rawPlayer(1, "Chris", "Ashby"), rawPlayer(2, "Bruno", "Sami")}
	raw[1].Attributes[0], raw[1].CurrentAbility = 77, 133
	raw[1].ClubAffinities = []savefile.ClubAffinity{{Club: 416, Value: 100, Reason: 1}}
	g := NewGame(Info{Date: gameDate}, raw, testWorld, PlayerExtras{})

	for i, p := range g.players {
		if int(p.auxIdx) != i {
			t.Errorf("player %d: aux index %d, want %d", p.UID, p.auxIdx, i)
		}
		if got, want := g.auxOf(p).roles, newRoleInputs(raw[i]); got != want {
			t.Errorf("player %d: role inputs %+v, want %+v", p.UID, got, want)
		}
	}
	if fc := g.auxOf(g.players[1]).favoured; len(fc) != 1 || g.auxOf(g.players[0]).favoured != nil {
		t.Errorf("favoured clubs %+v, %+v", g.auxOf(g.players[0]).favoured, fc)
	}
}

// With ~140 000 players, one byte past 512 moves each into the 576-byte
// allocation class.
func TestPlayerFitsA512ByteAllocation(t *testing.T) {
	if size := unsafe.Sizeof(Player{}); size > 512 {
		t.Errorf("Player is %d bytes, want at most 512", size)
	}
}

// playerAux exists once per player (~140 000 in a real save): growing it must be deliberate.
func TestPlayerAuxSize(t *testing.T) {
	if size := unsafe.Sizeof(playerAux{}); size > 208 {
		t.Errorf("playerAux is %d bytes, want at most 208", size)
	}
}
