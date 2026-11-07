package fm

import "testing"

// kzReputation is Kauno Žalgiris's first-team reputation on the 2026-02-11
// save.
const kzReputation = 3200

func TestClubLevelFromReputation(t *testing.T) {
	for _, tt := range []struct{ rep, want int }{
		{0, 7575}, // reputation counts as at least 50
		{500, 8250},
		{1500, 10250},
		{2850, 14475},
		{3999, 17997},
		{4000, 18000},
		{5850, 25400}, // FC Luzern's cached level on the 2026-02-11 save
		{6500, 28000},
		{7650, 31950},
		{8500, 34750},
		{9500, 38250},
	} {
		if got := clubLevelFromReputation(tt.rep); got != tt.want {
			t.Errorf("clubLevelFromReputation(%d) = %d, want %d", tt.rep, got, tt.want)
		}
	}
}

// squadWorld has club 1 with first team 10 and reserves 11, and club 2 with
// first team 20.
func squadWorld() World {
	return World{
		Teams: map[int64]*Team{
			10: {Index: 10, ClubIndex: 1, Reputation: 5000},
			11: {Index: 11, ClubIndex: 1, Reputation: 1000},
			20: {Index: 20, ClubIndex: 2, Reputation: 2000},
		},
		Clubs: map[int64]Club{1: {FirstTeam: 10}, 2: {FirstTeam: 20}},
	}
}

func TestClubSquadLevels(t *testing.T) {
	w := squadWorld()
	var players []*Player
	add := func(team int64, ca int, loan bool) {
		p := &Player{PlayerBasics: PlayerBasics{TeamIndex: team}, CurrentAbility: ca}
		if loan {
			p.Loan = &Contract{TeamIndex: team}
		}
		players = append(players, p)
	}
	for ca := 100; ca < 110; ca++ {
		add(10, ca, false)
	}
	add(11, 60, false) // reserves count
	add(11, 50, false) // the 13th best is left out
	add(10, 200, true) // on loan in: not counted
	for range 11 {     // another club, one player short of a squad level
		add(20, 100, false)
	}
	got := clubSquadLevels(players, w)
	want := (100 + 101 + 102 + 103 + 104 + 105 + 106 + 107 + 108 + 109 + 60 + 50) * abilityScale / 12
	if got[1] != want {
		t.Errorf("club 1 level = %d, want %d", got[1], want)
	}
	if _, ok := got[2]; ok {
		t.Errorf("club 2 with 11 players has level %d", got[2])
	}
}

func TestClubLevel7(t *testing.T) {
	g := &Game{world: squadWorld(), clubLevels: map[int64]int{1: 33484}}
	if got := g.clubLevel7(1); got != 30805 {
		t.Errorf("level 7 from squad = %d, want 30805", got)
	}
	if got := g.clubLevel7(2); got != int(0.92*float64(clubLevelFromReputation(2000))) {
		t.Errorf("level 7 from reputation = %d", got)
	}
	g.clubLevels[1] = 40000
	if got := g.clubLevel7(1); got != clubLevel7Cap {
		t.Errorf("capped level 7 = %d, want %d", got, clubLevel7Cap)
	}
}

// freeAgentGame is KŽ (club 1, team 10, reputation 3200) and one free agent
// whose last club is club 2 (team 20).
func freeAgentGame(prevRep, prevLevel, ca, world, daysSinceLeft int) (*Game, *Player) {
	p, in := interestPlayer()
	in.main, in.previousClub, in.joined = nil, 2, gameDaysAgo(interestToday, daysSinceLeft)
	p.TeamIndex, p.Club, p.CurrentAbility, p.Reputation.World = noTeam, "", ca, world
	w := interestWorld(kzReputation, true)
	w.Clubs = map[int64]Club{1: {Name: "FK Kauno Žalgiris", FirstTeam: 10}, 2: {Name: "Tottenham Hotspur", FirstTeam: 20}}
	w.Teams[20].Reputation = prevRep
	g := &Game{Info: Info{ManagerTeamIndex: 10, Date: interestToday}, world: w, aux: []playerAux{{interest: in}}, clubLevels: map[int64]int{}}
	if prevLevel > 0 {
		g.clubLevels[2] = prevLevel
	}
	return g, p
}

// The values of Ashcroft, Ghrissi and Wenig on the 2026-02-11 save
// (previous club's level from its squad).
func TestFreeAgentGap(t *testing.T) {
	for _, tt := range []struct {
		name                                string
		prevRep, prevLevel, ca, world, days int
		want                                int
	}{
		{"Ashcroft: capped, 23 days", 7650, 33484, 94, 2500, 23, -10240},
		{"Ghrissi: capped, 225 days", 7000, 28798, 78, 750, 225, 512},
		{"Wenig: not capped", 4450, 22990, 102, 3000, 23, -30000},
		{"between 30 and 90 days", 4450, 22990, 102, 0, 45, (12800 - 16020) * 5},
		{"world reputation", 3000, 22990, 102, 4000, 23, 800 - 3200*8},
		{"29 days", 4450, 22990, 102, 0, 29, (12800 - 17800) * 6},
		{"30 days", 4450, 22990, 102, 0, 30, (12800 - 16020) * 5},
		{"89 days", 4450, 22990, 102, 0, 89, (12800 - 16020) * 5},
		{"90 days", 4450, 22990, 102, 0, 90, (12800 - 14240) * 2},
		{"ability at level 7: not capped", 7650, 23914, 100, 0, 23, (12800 - 30600) * 6}, // 0.92 × 23 914 = 22 000.88
		{"ability below level 7: capped", 7650, 23914, 99, 0, 23, (12800 - 15360) * 4},
	} {
		g, p := freeAgentGame(tt.prevRep, tt.prevLevel, tt.ca, tt.world, tt.days)
		if got := g.freeAgentGap(p, g.auxOf(p).interest, kzReputation); got != tt.want {
			t.Errorf("%s: gap = %d, want %d", tt.name, got, tt.want)
		}
	}
	g, p := freeAgentGame(7650, 33484, 94, 2500, 23)
	g.aux[0].interest.previousClub = noTeam
	if got := g.freeAgentGap(p, g.auxOf(p).interest, kzReputation); got != 0 {
		t.Errorf("no previous club: gap = %d", got)
	}
}

// The values of Kazlauskas (15), Ghrissi (21) and two older players.
func TestFreeAgentPlayingTime(t *testing.T) {
	for _, tt := range []struct {
		age, current, world, want int
	}{
		{15, 2050, 200, 20100},
		{21, 3900, 750, 23800},
		{25, 5350, 3000, 5300}, // wants level 7
		{25, 5350, 3040, 5300}, // world reputation exactly 0.95 × 3200: still level 7
		{25, 5350, 3100, 0},    // world reputation above 0.95 × 3200: wants level 8
		{22, 3900, 750, 8200},  // from age 22: level 7
	} {
		p := &Player{PlayerBasics: PlayerBasics{Age: tt.age, Reputation: Reputation{Current: tt.current, World: tt.world}}}
		if got := freeAgentPlayingTime(p, kzReputation); got != tt.want {
			t.Errorf("%+v: playing time = %d, want %d", tt, got, tt.want)
		}
	}
}

func TestInterestFreeAgentVerdicts(t *testing.T) {
	t.Run("bigger last club, below Interested", func(t *testing.T) {
		g, p := freeAgentGame(7650, 33484, 94, 2500, 23) // Ashcroft: gap -10 240, playing time +25 400
		p.Age, p.Reputation.Current = 21, 4700
		got := g.interestOf(p)
		want := InterestFactor{FactorClubReputationGap, "Tottenham Hotspur is a bigger club than FK Kauno Žalgiris"}
		if got.Transfer != VerdictOut || got.TransferReason == nil || *got.TransferReason != want {
			t.Errorf("interest = %+v, reason %+v", got, got.TransferReason)
		}
	})
	t.Run("bigger last club, above Interested", func(t *testing.T) {
		g, p := freeAgentGame(7650, 33484, 94, 2500, 23)
		p.Age, p.Reputation.Current = 18, 7200 // playing time 16 000 + 2 × 7200 = 30 400
		got := g.interestOf(p)
		if got.Transfer != VerdictUncertain || got.TransferReason != nil || !slicesContainsCode(got.TransferFactors, FactorClubReputationGap) {
			t.Errorf("interest = %+v", got)
		}
	})
	t.Run("smaller last club, below Interested", func(t *testing.T) {
		g, p := freeAgentGame(1000, 0, 94, 0, 23)
		p.Age, p.Reputation.Current = 30, 6000 // playing time 0
		got := g.interestOf(p)
		if got.Transfer != VerdictUncertain || len(got.TransferFactors) != 0 {
			t.Errorf("interest = %+v", got)
		}
	})
	t.Run("bigger last club, exactly Interested", func(t *testing.T) {
		g, p := freeAgentGame(7650, 33484, 94, 2500, 23) // gap -10 240
		p.Age, p.Reputation.Current = 18, 7120           // playing time 16 000 + 2 × 7120 = 30 240
		if got := g.interestOf(p); got.Transfer != VerdictUncertain {
			t.Errorf("interest = %+v", got)
		}
	})
	t.Run("no previous club, below Interested", func(t *testing.T) {
		g, p := freeAgentGame(7650, 33484, 94, 2500, 23)
		g.aux[0].interest.previousClub = noTeam
		p.Age, p.Reputation.Current = 30, 6000 // gap 0, playing time 0
		got := g.interestOf(p)
		if got.Transfer != VerdictUncertain || len(got.TransferFactors) != 0 {
			t.Errorf("interest = %+v", got)
		}
	})
	t.Run("high score stays uncertain", func(t *testing.T) {
		g, p := freeAgentGame(1000, 0, 94, 0, 23)
		p.Age, p.Reputation.Current = 15, 3000 // playing time 5 × 200 + 7 × 3000 = 22 000, gap +8 800
		if got := g.interestOf(p); got.Transfer != VerdictUncertain {
			t.Errorf("interest = %+v", got)
		}
	})
	t.Run("never a loan", func(t *testing.T) {
		g, p := freeAgentGame(1000, 0, 94, 0, 23)
		got := g.interestOf(p)
		if got.Loan != VerdictOut || got.LoanReason == nil || *got.LoanReason != (InterestFactor{FactorFreeAgent, "Is a free agent, so can't be loaned"}) {
			t.Errorf("interest = %+v, loan reason %+v", got, got.LoanReason)
		}
		if got.For(InterestTransferOrLoan) == VerdictOut {
			t.Error("transfer_or_loan out with the transfer open")
		}
	})
}
