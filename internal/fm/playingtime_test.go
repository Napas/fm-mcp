package fm

import (
	"testing"

	"github.com/Napas/fm-mcp/internal/savefile"
)

func TestActualPlayingTimeFollowsTheScoreBands(t *testing.T) {
	const keeper, youth = true, true
	tests := []struct {
		name          string
		playingTime   savefile.PlayingTime
		score, recent int8
		keeper, youth bool
		want          savefile.PlayingTime
	}{
		{"85 for a Star Player stays Star", 1, 85, 0, false, false, 1},
		{"85 is Important Player", 3, 85, 0, false, false, 2},
		{"84 is Regular Starter", 1, 84, 0, false, false, 3},
		{"60 is Regular Starter", 1, 60, 0, false, false, 3},
		{"59 is Squad Player", 3, 59, 0, false, false, 4},
		{"30 is Squad Player", 3, 30, 0, false, false, 4},
		{"Impact Sub keeps its status in its band", 5, 45, 0, false, false, 5},
		{"29 is Fringe Player", 3, 29, 0, false, false, 7},
		{"10 is Fringe Player", 3, 10, 0, false, false, 7},
		{"10 for a youth is Future Prospect", 3, 10, 0, false, youth, 11},
		{"9 is Emergency Backup", 3, 9, 0, false, false, 9},
		{"-10 is Emergency Backup", 3, -10, 0, false, false, 9},
		{"-11 is Surplus", 4, -11, 0, false, false, 22},
		{"-11 for a youth is Youngster", 3, -11, 0, false, youth, 13},
		{"Surplus keeps its status in its band", 22, -50, 0, false, false, 22},
		{"keeper agreed Surplus keeps it", 22, -50, 0, keeper, false, 22},
		{"keeper 97 is Important Player", 15, 97, 0, keeper, false, 2},
		{"keeper 70 is First-Choice Goalkeeper", 20, 70, 0, keeper, false, 15},
		{"keeper 30 is Backup", 15, 30, 0, keeper, false, 20},
		{"keeper -11 is Emergency Backup, never Surplus", 15, -11, 0, keeper, false, 21},
		{"cup keeper with a recent score of 0 stays", 16, 0, 0, keeper, false, 16},
		{"cup keeper with a negative recent score", 16, 0, -1, keeper, false, 21},
		{"no playing time", 0, 90, 0, false, false, 0},
		{"unknown playing time", 23, 90, 0, false, false, 0},
	}
	for _, tt := range tests {
		c := &savefile.Contract{PlayingTime: tt.playingTime, PlayingTimeScore: tt.score, RecentPlayingTimeScore: tt.recent}
		if got, _ := ruleHeadline(c, tt.keeper, tt.youth); got != tt.want {
			t.Errorf("%s: ruleHeadline = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// actualPlayingTime leaves out results that may come from rules not decoded:
// level 7's match-list rule, the level → status branches of levels 4 and 6,
// the youth test for players up to 25, and anything away from a club's first
// team.
func TestActualPlayingTimeLeavesOutUndecodedBranches(t *testing.T) {
	const atFirstTeam, keeper, mayBeYouth = true, true, true
	tests := []struct {
		name                            string
		playingTime                     savefile.PlayingTime
		score                           int8
		atFirstTeam, keeper, mayBeYouth bool
		want                            savefile.PlayingTime
	}{
		{"level 9 from Regular Starter (Bernardi)", 3, 88, atFirstTeam, false, false, 2},
		{"level 1 from Squad Player (Assongo)", 4, -15, atFirstTeam, false, false, 22},
		{"level 5 from Regular Starter", 3, 0, atFirstTeam, false, false, 9},
		{"keeper level 1", 15, -50, atFirstTeam, keeper, false, 21},
		{"level 7 from Regular Starter", 3, 45, atFirstTeam, false, false, 0},
		{"level 6 from Regular Starter", 3, 20, atFirstTeam, false, false, 0},
		{"keeper level 6", 15, 20, atFirstTeam, keeper, false, 0},
		{"level 7 for a Squad Player: the match-list rule", 4, 45, atFirstTeam, false, false, 0},
		{"level 7 for an Impact Sub: the match-list rule", 5, 45, atFirstTeam, false, false, 0},
		{"level 7 for a cup keeper: the match-list rule", 16, 0, atFirstTeam, keeper, false, 0},
		{"level 6 for a Fringe Player keeps it", 7, 20, atFirstTeam, false, false, 7},
		{"youth test changes the result (Lobede)", 4, -15, atFirstTeam, false, mayBeYouth, 0},
		{"a Youngster at -15 may be Surplus", 13, -15, atFirstTeam, false, mayBeYouth, 0},
		{"a youth's level 4 is not decoded", 7, 20, atFirstTeam, false, mayBeYouth, 0},
		{"youth test does not matter at 88 (Bernardi at 25)", 3, 88, atFirstTeam, false, mayBeYouth, 2},
		{"away from a club's first team", 3, 88, false, false, false, 0},
		{"no playing time", 0, 90, atFirstTeam, false, false, 0},
	}
	for _, tt := range tests {
		c := &savefile.Contract{PlayingTime: tt.playingTime, PlayingTimeScore: tt.score}
		if got := actualPlayingTime(c, tt.atFirstTeam, tt.keeper, tt.mayBeYouth); got != tt.want {
			t.Errorf("%s: actualPlayingTime = %d, want %d", tt.name, got, tt.want)
		}
	}
}
