package fm

import "github.com/Napas/fm-mcp/internal/savefile"

// Inputs of the actual playing time rule (game_plugin 26.3.2, 0x32703c0).
const (
	keeperRating    = 18 // a GK rating from which a player counts as a goalkeeper
	youthRuleMaxAge = 25 // the oldest age the game's youth test (0x377fae0) may accept
)

// noSquadNumber marks a contract without a squad number.
const noSquadNumber = 255

// setSquadStatus sets the playing times and squad number from the loan
// contract while on loan, else the main contract. Loan cores carry their own
// values; loanees wear the loan contract's number (verified on last-match
// lineups). mayBeYouth is whether the game's youth test may count him as a
// youth. He is at a club's first team when he plays for that contract's team
// and it is a first team.
func (p *Player) setSquadStatus(raw savefile.Player, mayBeYouth bool, w World) {
	c := raw.Contract
	if raw.Loan != nil {
		c = raw.Loan
	}
	if c == nil {
		return
	}
	p.playingTime, p.originalPlayingTime = c.PlayingTime, c.OriginalPlayingTime
	atFirstTeam := c.TeamIndex == raw.TeamIndex && w.isFirstTeam(c.TeamIndex)
	p.actualPlayingTime = actualPlayingTime(c, atFirstTeam, raw.Positions[savefile.GKPosition] >= keeperRating, mayBeYouth)
	p.SquadNumber = squadNumber(c)
}

// squadNumber is c's squad number; 0 when none.
func squadNumber(c *savefile.Contract) int {
	if c.SquadNumber == noSquadNumber {
		return 0
	}
	return int(c.SquadNumber)
}

// Playing time levels, 1 (worst) to 10 (best). Several statuses share one.
const (
	levelSurplus   = 1
	levelBTeam     = 2
	levelYoungster = 3
	levelProspect  = 4
	levelBackup    = 5
	levelFringe    = 6
	levelSquad     = 7
	levelRegular   = 8
	levelImportant = 9
	levelStar      = 10
)

// Statuses the score rule tests.
const (
	statusStar         = 1
	statusCupKeeperMin = 16 // 16-19 are the cup goalkeeper statuses
	statusCupKeeperMax = 19
)

// playingTimeLevels is each status's level (table 0x524f3e8), by status.
// The interest rules read the same values from table 0x5203960 (LEVEL).
var playingTimeLevels = [...]uint8{
	1: levelStar, 2: levelImportant, 3: levelRegular, 4: levelSquad, 5: levelSquad, 6: levelSquad,
	7: levelFringe, 8: levelFringe, 9: levelBackup, 10: levelProspect, 11: levelProspect, 12: levelProspect,
	13: levelYoungster, 14: levelBTeam, 15: levelRegular, 16: levelSquad, 17: levelSquad, 18: levelSquad,
	19: levelSquad, 20: levelSquad, 21: levelBackup, 22: levelSurplus,
}

// levelStatuses is the status a level gives (0x3780194): outfield players,
// then goalkeepers.
var levelStatuses = [...][2]savefile.PlayingTime{
	levelSurplus:   {22, 21}, // Surplus to Requirements; keepers Emergency Backup
	levelBTeam:     {14, 14}, // B Team Regular
	levelYoungster: {13, 13}, // Youngster
	levelProspect:  {11, 11}, // Future Prospect
	levelBackup:    {9, 21},  // Emergency Backup
	levelFringe:    {7, 20},  // Fringe Player; keepers Backup
	levelSquad:     {4, 16},  // Squad Player; keepers Cup Goalkeeper
	levelRegular:   {3, 15},  // Regular Starter; keepers First-Choice Goalkeeper
	levelImportant: {2, 2},   // Important Player
	levelStar:      {1, 1},   // Star Player
}

// statusLevel is s's level; 0 for 0 and unknown values.
func statusLevel(s savefile.PlayingTime) uint8 {
	if int(s) >= len(playingTimeLevels) {
		return 0
	}
	return playingTimeLevels[s]
}

// actualPlayingTime is the contract screen's headline (game rule 0x32703c0),
// or 0 when the contract has no playing time, when the player is not at a
// club's first team (atFirstTeam: reserve and youth players' scores are not
// checked in game), or when the result may come from a part of the rule not
// decoded: a branch named at ruleHeadline, or, when mayBeYouth, the game's
// youth test changing the result.
func actualPlayingTime(c *savefile.Contract, atFirstTeam, keeper, mayBeYouth bool) savefile.PlayingTime {
	if !atFirstTeam {
		return 0
	}
	s, decoded := ruleHeadline(c, keeper, false)
	if !decoded {
		return 0
	}
	if mayBeYouth {
		if youth, ok := ruleHeadline(c, keeper, true); !ok || youth != s {
			return 0
		}
	}
	return s
}

// ruleHeadline is rule 0x32703c0 for a youth flag: the level of the
// contract's playing-time score, or the contract's playing time when that has
// the same level. decoded is false at level 7, where a match-list rule not
// decoded runs before the same-level step and can give Squad Player or Impact
// Sub. It is also false when the level → status step (0x3780194) runs at
// level 4 or 6, where the game can give Breakthrough Prospect (an ability
// test), Impact Sub (players 33 and over) or Fringe Player (a competition
// value). The game's clamps for youth contracts, playing-time promises and
// team settings are not applied. s is 0 when the contract has no playing
// time.
func ruleHeadline(c *savefile.Contract, keeper, youth bool) (s savefile.PlayingTime, decoded bool) {
	own := statusLevel(c.PlayingTime)
	if own == 0 {
		return 0, false
	}
	level := scoreLevel(c.PlayingTime, c.PlayingTimeScore, c.RecentPlayingTimeScore, keeper, youth)
	switch {
	case level == own:
		s = c.PlayingTime
	case keeper:
		s = levelStatuses[level][1]
	default:
		s = levelStatuses[level][0]
	}
	if level == levelSquad {
		return s, false
	}
	return s, level == own || level != levelProspect && level != levelFringe
}

// scoreLevel rates the playing-time score (0x32705ec) of a contract with
// playing time b and recent score recent.
func scoreLevel(b savefile.PlayingTime, score, recent int8, keeper, youth bool) uint8 {
	cupKeeper := b >= statusCupKeeperMin && b <= statusCupKeeperMax
	switch {
	case score >= 85 && b == statusStar:
		return levelStar
	case score >= 85:
		return levelImportant
	case score >= 60:
		return levelRegular
	case cupKeeper && recent >= 0, score >= 30 && !keeper:
		return levelSquad
	case score >= 10 && youth:
		return levelProspect
	case score >= 10:
		return levelFringe
	case youth:
		return levelYoungster
	case score >= -10:
		return levelBackup
	default:
		return levelSurplus
	}
}

// playingTimeName is the name the game shows for s; empty for 0 and
// unknown values.
func playingTimeName(s savefile.PlayingTime) string {
	n, _ := s.Name()
	return n
}
