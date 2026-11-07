package fm

import (
	"slices"
)

// Follows game_plugin 27.0.0: every address in this file is in its arm64
// slice. FM26 saves get the same rules.
//
// The free-agent path of the interest score, 0x1f70254, reduced to the two
// terms decoded: the club reputation gap against his previous club
// (0x1f79b0c, reason 2) and the playing time at the best level (0x1f9af3c,
// no-contract branch).
const (
	abilityScale       = 220    // 0x14ea848: ability = current ability × 220
	squadLevelPlayers  = 12     // 0xe02d5c: a club's level is the mean of its 12 best players
	clubLevel7Share    = 0.92   // 0xe03b7c: level 7 is 92% of the club's level
	clubLevel7Cap      = 33_000 // 0xe03b7c
	formerClubCapNum   = 6      // a player below his former club's level 7 counts it at most 6/5 × the target
	formerClubCapDen   = 5
	leftLongAgoDays    = 90 // from here his former club counts 4/5
	leftAWhileAgoDays  = 30 // from here it counts 9/10
	freeAgentYouthAge  = 22 // below this age a free agent wants level 3
	freeAgentYouthWant = 3
	freeAgentWant      = 7    // when his world reputation is at most 95% of the target's
	freeAgentStarWant  = 8    // 9 when the target's division COMP +0x1aa > 0, which is not read; 8 is the lenient value
	freeAgentWorldRep  = 0.95 // share of the target's reputation
	// interestedLevel is FM's "Interested" filter level (FM.UI.PlayerInterestLevel).
	interestedLevel = 20_000
)

// clubLevelSteps is 0xe033bc: a club's level from its first-team reputation
// R when fewer than 12 of its players are loaded: (a·max(R, 50) − b)/1000 + c
// for the first step with R ≥ from. The division factor (COMP +0x1aa) is
// not read and counts as 1.
var clubLevelSteps = [...]struct{ from, a, b, c int }{
	{9000, 3500, 31_500_000, 36_500},
	{8000, 3500, 28_000_000, 33_000},
	{7000, 3000, 21_000_000, 30_000},
	{6000, 4000, 24_000_000, 26_000},
	{5000, 4000, 20_000_000, 22_000},
	{4000, 4000, 16_000_000, 18_000},
	{3000, 3000, 9_000_000, 15_000},
	{2000, 3500, 7_000_000, 11_500},
	{1000, 2500, 2_500_000, 9_000},
	{0, 1500, 0, 7_500},
}

func clubLevelFromReputation(rep int) int {
	r := max(rep, 50)
	for _, s := range clubLevelSteps {
		if rep >= s.from {
			return (s.a*r-s.b)/1000 + s.c
		}
	}
	return clubLevelSteps[len(clubLevelSteps)-1].c
}

// clubSquadLevels is 0xe036b4 for every club with at least 12 players: the
// mean ability of its 12 best, counting the players of each of its teams
// except those on loan there. The game uses its staff's estimate of ability
// and smooths the value over time; this uses true current ability.
func clubSquadLevels(players []*Player, w World) map[int64]int {
	abilities := make(map[int64][]int)
	for _, p := range players {
		if p.Loan != nil {
			continue
		}
		if club, ok := w.clubOf(p.TeamIndex); ok {
			abilities[club] = append(abilities[club], p.CurrentAbility)
		}
	}
	levels := make(map[int64]int)
	for club, cas := range abilities {
		if len(cas) < squadLevelPlayers {
			continue
		}
		slices.SortFunc(cas, func(a, b int) int { return b - a })
		sum := 0
		for _, ca := range cas[:squadLevelPlayers] {
			sum += ca
		}
		levels[club] = sum * abilityScale / squadLevelPlayers
	}
	return levels
}

// clubLevel7 is 0xe0327c at level 7 for an AI club: 92% of its squad level,
// else of its level from reputation.
func (g *Game) clubLevel7(club int64) int {
	level, ok := g.clubLevels[club]
	if !ok {
		level = clubLevelFromReputation(clubReputation(g.world, club))
	}
	return min(int(clubLevel7Share*float64(level)), clubLevel7Cap)
}

// freeAgentGap is reason 2 for a free agent: freeAgentGapTerms summed.
func (g *Game) freeAgentGap(p *Player, in interestInputs, rt int) int {
	club, world := g.freeAgentGapTerms(p, in, rt)
	return club + world
}

// freeAgentGapTerms are the two terms of reason 2 for a free agent: the
// target's first-team reputation against his previous club's, stepped, and
// the world reputation term. Both 0 without a previous club with a first
// team. Not applied: the game skips the term when his career stats array is
// empty (PLAYER_CAREER_STATS[5], not read), and 0xd00c50 raises T by 10%
// for a player of 30 or more when both clubs play in the Premier League
// (partly read).
func (g *Game) freeAgentGapTerms(p *Player, in interestInputs, rt int) (club, world int) {
	first := clubFirstTeam(g.world, in.previousClub)
	if first == nil {
		return 0, 0
	}
	target, former := min(4*rt, reputationCap), min(4*first.Reputation, reputationCap)
	if p.CurrentAbility*abilityScale < g.clubLevel7(in.previousClub) {
		former = min(former, target*formerClubCapNum/formerClubCapDen)
	}
	switch days := gameDaysSince(g.Info.Date, in.joined); {
	case days >= leftLongAgoDays:
		former = former * 4 / 5
	case days >= leftAWhileAgoDays:
		former = former * 9 / 10
	}
	return steppedGap(target - former), worldReputationTerm(target, p.Reputation.World)
}

// freeAgentPlayingTime is 0x1f9af3c without a contract, at the best level
// (10): the level he wants is 3 under age 22, else 7, or 8 when his world
// reputation is above 95% of the target's.
func freeAgentPlayingTime(p *Player, rt int) int {
	want := freeAgentWant
	switch {
	case p.Age < freeAgentYouthAge:
		want = freeAgentYouthWant
	case float64(p.Reputation.World) > freeAgentWorldRep*float64(rt):
		want = freeAgentStarWant
	}
	cr := p.Reputation.Current
	return max(0, 5*(rt-cr)+(transferLevel-want)*cr)
}

// judgeFreeAgent sets c's transfer verdict and terms for a free agent past
// the hard gates. The transfer is out when reason 2 is negative (his previous
// club or his world reputation out-sizes the target) and the sum stays below
// FM's Interested level. About 20 free-agent factors are not decoded, so
// every other case is uncertain, never likely.
func (g *Game) judgeFreeAgent(c *interestCase) {
	c.freeAgent = true
	c.terms.gap = g.freeAgentGap(c.p, c.in, c.to.reputation)
	c.terms.pt10 = freeAgentPlayingTime(c.p, c.to.reputation)
	if c.terms.gap < 0 && c.terms.gap+c.terms.pt10 < interestedLevel {
		c.verdicts.Transfer = VerdictOut
	}
}
