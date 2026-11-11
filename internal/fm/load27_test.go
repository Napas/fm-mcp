package fm

import (
	"cmp"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	testSave27Path  = "../../data/test_save_27.fm"
	testSave27bPath = "../../data/test_save_27b.fm" // after Kauno Žalgiris's first match
	// The KŽ save whose players' homegrown rows were checked in game.
	testSave27HomegrownPath = "../../data/test_save_27_homegrown.fm"
	// Saved before and after setting three KŽ players' transfer statuses in
	// game, 2026-11-06.
	testSave27ListBeforePath = "../../data/test_save_27_list_before.fm"
	testSave27ListAfterPath  = "../../data/test_save_27_list_after.fm"
	// Saved 2026-02-11 in game, just after the agent-chat approaches in
	// TestLoadTestSave27Interest.
	testSave27InterestPath = "../../data/test_save_27_interest.fm"
	// testSave27InstructionsPath is the KŽ save of 2026-11-07 20:40 (game
	// date 2026-05-04), whose tactic has team instructions set in game.
	testSave27InstructionsPath = "../../data/test_save_27_instructions.fm"
	// The KŽ save of 2026-11-07 22:25, with player instructions set in game.
	testSave27PlayerInstructionsPath = "../../data/test_save_27_player_instructions.fm"
	kzTeam27                         = 16521    // Kauno Žalgiris's team index
	enzoUID27                        = 14229525 // Enzo Fernández
	salahUID27                       = 98028755 // Mohamed Salah
	mikelionisUID27                  = 47055959 // Deividas Mikelionis, KŽ's goalkeeper
)

func TestLoadTestSave27Info(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	want := time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)
	if !g.Info.Date.Equal(want) || g.Info.Build != "27.0.0+2493426" || g.Info.ManagerClub != "FK Kauno Žalgiris" {
		t.Errorf("date %s, build %q, club %q; want date %s", g.Info.Date.Format(time.DateOnly), g.Info.Build, g.Info.ManagerClub, want.Format(time.DateOnly))
	}
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "3754") { // unverified-build warnings name the version
			t.Errorf("warning %q", w)
		}
	}
}

func TestLoadTestSave27SeasonStats(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	if !g.hasSeason {
		t.Errorf("season stats not loaded; warnings %q", g.Info.Warnings)
	}
}

func TestLoadTestSave27Fixtures(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{})

	if !g.hasFixtures || len(g.fixtures) == 0 {
		t.Errorf("fixtures not loaded (%d); warnings %q", len(g.fixtures), g.Info.Warnings)
	}
}

func TestLoadTestSave27Tactics(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})
		for _, w := range g.Info.Warnings {
			if strings.Contains(w, "tactic") || strings.Contains(w, "lineup") || strings.Contains(w, "formation") {
				t.Errorf("%s: warning %q", path, w)
			}
		}
	}
}

// KŽ's team instructions as the Team Instructions screen showed them
// (2026-11-07, game date 2026-05-04): the 14 options that are not the default.
func TestLoadTestSave27Instructions(t *testing.T) {
	v := loadTestSaveAt(t, testSave27InstructionsPath, LoadOptions{}).View(false)

	tac, ok := v.Tactic()
	if !ok || tac.Name != "Positive 4-2-3-1 DM AM Wide - Custom Control Possession" || tac.Mentality != "Positive" {
		t.Fatalf("tactic %q, %q, %v", tac.Name, tac.Mentality, ok)
	}
	want := []TeamInstruction{
		{"in_possession", "Passing Directness", "Shorter"},
		{"in_possession", "Build-Up Strategy", "Play Through Press"},
		{"in_possession", "Shots from Distance", "Discourage"},
		{"in_possession", "Patience", "Work Ball Into Box"},
		{"in_possession", "Play for Set Pieces", "Keep Ball in Play"},
		{"in_possession", "GK Distribution", "Centre-Backs"},
		{"in_possession", "Goal Kicks", "Short"},
		{"in_possession", "Attacking Width", "Wider"},
		{"in_possession", "Time Wasting", "Standard"},
		{"out_of_possession", "Defensive Line", "Higher"},
		{"out_of_possession", "Line of Engagement", "High Press"},
		{"out_of_possession", "Trigger Press", "More Often"},
		{"out_of_possession", "Short Goalkeeper Distribution", "Yes"},
		{"out_of_possession", "Defensive Transition", "Counter-Press"},
	}
	if !reflect.DeepEqual(tac.TeamInstructions, want) || tac.UnknownTeamInstructionBits != nil {
		t.Errorf("team instructions %+v, unknown bits %v;\nwant %+v", tac.TeamInstructions, tac.UnknownTeamInstructionBits, want)
	}
}

// Player instructions set in game (2026-11-07): Švedkauskas (GK) More Direct
// passing and Ease Off Tackles; Krekovič (AML) Shoot Less Often and Tackle
// Harder. No other slot has any.
func TestLoadTestSave27PlayerInstructions(t *testing.T) {
	g := loadTestSaveAt(t, testSave27PlayerInstructionsPath, LoadOptions{})
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "instructions") {
			t.Errorf("warning %q", w)
		}
	}
	tac, ok := g.View(false).Tactic()
	if !ok {
		t.Fatal("no tactic")
	}
	want := map[int]PlayerInstructions{
		0:  {InPossession: []string{"More Direct Passes"}, OutOfPossession: []string{"Ease Off Tackles"}},
		10: {InPossession: []string{"Shoot Less Often"}, OutOfPossession: []string{"Tackle Harder"}},
	}
	for i, s := range tac.Slots {
		w, set := want[i]
		switch {
		case !set && s.PlayerInstructions != nil:
			t.Errorf("slot %d: %+v, want none", i, *s.PlayerInstructions)
		case set && (s.PlayerInstructions == nil || !reflect.DeepEqual(*s.PlayerInstructions, w)):
			t.Errorf("slot %d (%+v): %+v, want %+v", i, s.Player, s.PlayerInstructions, w)
		}
	}
	if gk, aml := tac.Slots[0].Player, tac.Slots[10].Player; gk == nil || !strings.Contains(gk.Name, "Švedkauskas") || aml == nil || !strings.Contains(aml.Name, "Krekovi") {
		t.Errorf("slot 0 %+v, slot 10 %+v; want Švedkauskas and Krekovič", gk, aml)
	}
}

// The interest save (game date 2026-02-11) holds an earlier set of KŽ's
// instructions: Tempo Lower in place of Wider and Discourage.
func TestLoadTestSave27InterestInstructions(t *testing.T) {
	v := loadTestSaveAt(t, testSave27InterestPath, LoadOptions{}).View(false)

	tac, ok := v.Tactic()
	if !ok || len(tac.TeamInstructions) != 13 || tac.UnknownTeamInstructionBits != nil {
		t.Fatalf("tactic %v: %d team instructions %+v, unknown bits %v; want 13, none", ok, len(tac.TeamInstructions), tac.TeamInstructions, tac.UnknownTeamInstructionBits)
	}
	if !slices.Contains(tac.TeamInstructions, TeamInstruction{"in_possession", "Tempo", "Lower"}) {
		t.Errorf("team instructions %+v lack Tempo: Lower", tac.TeamInstructions)
	}
}

// After its first match Kauno Žalgiris's tactic has no selection, so its
// players come from the last match, against FK Banga.
func TestLoadTestSave27bTactic(t *testing.T) {
	v := loadTestSaveAt(t, testSave27bPath, LoadOptions{}).View(false)

	tac, ok := v.Tactic()
	if !ok || tac.Name != "4-3-3 DM Wide" || tac.Mentality != "Balanced" || tac.LineupSource != LineupLastMatch {
		t.Fatalf("tactic %q, %q, %q, %v", tac.Name, tac.Mentality, tac.LineupSource, ok)
	}
	if p := tac.Slots[0].Player; p == nil || p.Name != "Deividas Mikelionis" {
		t.Errorf("goalkeeper %+v, want Deividas Mikelionis", p)
	}
	// As on the Tactics screen (2026-01-23, by role): the CM and AM drop to
	// the two CMs, the winger to wide midfield.
	for i, want := range map[int]struct{ ip, oop SlotRole }{
		6: {SlotRole{"MC", "right", "Central Midfielder"}, SlotRole{"MC", "right", "Central Midfielder"}},
		7: {SlotRole{"MC", "left", "Attacking Midfielder"}, SlotRole{"MC", "left", "Central Midfielder"}},
		8: {SlotRole{"AMR", "", "Winger"}, SlotRole{"MR", "", "Wide Midfielder"}},
	} {
		if s := tac.Slots[i]; s.InPossession != want.ip || s.OutOfPossession != want.oop {
			t.Errorf("slot %d: %+v → %+v, want %+v → %+v", i, s.InPossession, s.OutOfPossession, want.ip, want.oop)
		}
	}
	// By hand (0x4918654): Baldassarra 115 → 119 of 124, Kalik 111 → 117 of
	// 124, both ability 110. The 26.3.2 rules give 102 and 98.
	for i, want := range map[int]int{5: 105, 7: 103} {
		if a := tac.Slots[i].RoleAbility; a == nil || *a != want {
			t.Errorf("slot %d: role ability %v, want %d", i, a, want)
		}
	}
	// Out of possession (0x491aa68), by hand: Fiolić as Wide Midfielder at MR
	// 12 rates 105 less 20 % = 84 of his best 104, pulled up to 90; 90/104 × 105.
	if a := tac.Slots[8].OutOfPossessionRoleAbility; a == nil || *a != 90 {
		t.Errorf("slot 8: out-of-possession role ability %v, want 90", a)
	}
	// His best out-of-possession role gives his current ability, 105; the
	// other two values are this code's output, not checked in game.
	want := []RoleFit{{"Winger", "AMR", 105}, {"Inside Outlet Winger", "AMR", 103}, {"Wide Outlet Winger", "AMR", 103}}
	if prof, _ := v.Profile(tac.Slots[8].Player.UID); !slices.Equal(prof.BestOutOfPossessionRoles, want) {
		t.Errorf("Fiolić's best out-of-possession roles %+v, want %+v", prof.BestOutOfPossessionRoles, want)
	}
	for i, s := range tac.Slots {
		if s.OutOfPossessionRoleAbility == nil {
			t.Errorf("slot %d: no out-of-possession role ability", i)
		}
	}
	l, err := v.LastLineup(kzTeam27)
	if err != nil {
		t.Fatal(err)
	}
	if l.Opponent != "FK Banga" || l.InPossessionShape != "4-1-2-2-1" || l.OutOfPossessionShape != "4-1-4-1" || l.Slots[0].Player == nil || l.Slots[0].Player.ShirtNumber != 1 {
		t.Errorf("last lineup against %q, shapes %q / %q, goalkeeper %+v", l.Opponent, l.InPossessionShape, l.OutOfPossessionShape, l.Slots[0].Player)
	}
	// As the match report's OOP view shows (2026-01-23): the last match pairs
	// slots 6 to 8 as stored, unlike the tactic.
	for i, want := range map[int]struct {
		name string
		oop  SlotRole
	}{
		6: {"Vykintas Slivka", SlotRole{"MR", "", "Wide Midfielder"}},
		7: {"Anthony Kalik", SlotRole{"MC", "right", "Central Midfielder"}},
		8: {"Ivan Fiolić", SlotRole{"MC", "left", "Central Midfielder"}},
		9: {"Leon Kreković", SlotRole{"ML", "", "Wide Midfielder"}},
	} {
		if s := l.Slots[i]; s.Player == nil || s.Player.Name != want.name || s.OutOfPossession != want.oop {
			t.Errorf("last lineup slot %d: %+v → %+v, want %s → %+v", i, s.Player, s.OutOfPossession, want.name, want.oop)
		}
	}
}

// Serves KŽ's A Lyga table: nine teams, FK Banga first and KŽ fourth.
func TestLoadTestSave27Table(t *testing.T) {
	kz := int64(kzTeam27)
	for _, path := range []string{testSave27Path, testSave27bPath} {
		got, err := loadTestSaveAt(t, path, LoadOptions{}).View(false).Tables(&kz, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(got) != 1 || len(got[0].Rows) != 9 || got[0].Rows[3].Team != "FK Kauno Žalgiris" || got[0].Rows[0].Team != "FK Banga" {
			t.Errorf("%s: tables %+v", path, got)
		}
	}
}

// Ben Fox's injury in data/test_save_27b.fm: index 118 from 2026-01-01,
// 213–243 days.
func TestLoadTestSave27Injuries(t *testing.T) {
	g := loadTestSaveAt(t, testSave27bPath, LoadOptions{})

	injured := 0
	for _, p := range g.players {
		if p.Injured() {
			injured++
		}
	}
	if injured != 1632 {
		t.Errorf("%d injured players, want 1632", injured)
	}
	p := g.byUID[215502]
	if p == nil {
		t.Fatal("no Ben Fox")
	}
	if len(p.Injuries) != 1 {
		t.Fatalf("Ben Fox: %+v", p.Injuries)
	}
	if in := p.Injuries[0]; in.ID != 118 || in.Since.Format(time.DateOnly) != "2026-01-01" ||
		in.ReturnEarliest.Format(time.DateOnly) != "2026-08-02" || in.ReturnLatest.Format(time.DateOnly) != "2026-09-02" {
		t.Errorf("Ben Fox: %+v, want index 118 since 2026-01-01, back 2026-08-02..09-02", in)
	}
}

// 33 arranged moves count, 17 of them dated before the game date.
func TestLoadTestSave27ArrangedMoves(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})

		arranged := 0
		for _, a := range g.aux {
			if a.interest.arranged != nil {
				arranged++
			}
		}
		if arranged != 33 {
			t.Errorf("%s: %d players with an arranged move, want 33", path, arranged)
		}
		for _, uid := range []uint32{
			2000365091, // Cavan Sullivan, 2028-01-01
			14217414,   // Lucas Beltrán, 2025-01-01 to his current club
		} {
			p := g.byUID[uid]
			if p == nil {
				t.Fatalf("%s: no player with uid %d", path, uid)
			}
			if g.aux[p.auxIdx].interest.arranged == nil {
				t.Errorf("%s: uid %d has no arranged move", path, uid)
			}
		}
	}
}

// Offset 105 says 2026-09-02 for Enzo Fernández and 2026-08-05 for Mohamed
// Salah; the game shows 2026-01-19 for both.
func TestLoadTestSave27JoinedClub(t *testing.T) {
	for _, path := range []string{testSave27Path, testSave27bPath} {
		g := loadTestSaveAt(t, path, LoadOptions{})

		for _, p := range g.players {
			if p.JoinedClub != nil && p.JoinedClub.After(g.Info.Date) {
				t.Errorf("%s: %s joined %s, after the game date", path, p.Name, p.JoinedClub.Format(time.DateOnly))
				break
			}
		}
		for _, uid := range []uint32{enzoUID27, salahUID27} {
			p := g.byUID[uid]
			if p == nil {
				t.Fatalf("%s: no player with uid %d", path, uid)
			}
			if p.JoinedClub == nil || p.JoinedClub.Format(time.DateOnly) != "2026-01-19" {
				t.Errorf("%s: %s joined %v, want 2026-01-19", path, p.Name, p.JoinedClub)
			}
		}
	}
}

// Levels shown in game (2026-01-23 save; 27b is the same). Akusobi: KŽ B,
// Lithuania. Kadrić: on loan at FK Novi Pazar, Serbia.
func TestLoadTestSave27bReputationLevel(t *testing.T) {
	v := loadTestSaveAt(t, testSave27bPath, LoadOptions{}).View(false)

	for uid, name := range map[uint32]string{2000591747: "Davis Akusobi", 62220551: "Haris Kadrić"} {
		p, ok := v.Profile(uid)
		if !ok || p.Name != name {
			t.Fatalf("uid %d: %q, %v; want %s", uid, p.Name, ok, name)
		}
		if p.ReputationLevel != "National" {
			t.Errorf("%s: reputation level %q, want National", name, p.ReputationLevel)
		}
	}
}

// Values the game showed in the 2026-01-23 save; 27b holds the same ones.
func TestLoadTestSave27bMatchesGame(t *testing.T) {
	g := loadTestSaveAt(t, testSave27bPath, LoadOptions{})

	for _, tt := range []struct {
		uid  uint32
		wage int64
		end  string
	}{
		{enzoUID27, 250000, "2031-06-30"},  // €10.2M–17.24M a year shown
		{salahUID27, 351046, "2028-06-30"}, // €20.4M–26.03M a year shown
	} {
		p := g.byUID[tt.uid]
		if p == nil || p.Contract == nil {
			t.Fatalf("uid %d: no contract", tt.uid)
		}
		if end := p.Contract.End.Format(time.DateOnly); p.Contract.WeeklyWage != tt.wage || end != tt.end {
			t.Errorf("%s: wage %d, end %s; want %d, %s", p.Name, p.Contract.WeeklyWage, end, tt.wage, tt.end)
		}
	}
	p := g.byUID[mikelionisUID27] // the friendly against FK Banga
	if p == nil {
		t.Fatal("no Mikelionis")
	}
	f := g.auxOf(p).season.Friendlies
	if f == nil || f.Starts != 1 || f.GoalsConceded != 2 || f.RatingSum != 64 || f.Rated != 1 || f.Detail.Passes != 7 || f.Detail.PassesCompleted != 4 {
		t.Errorf("Mikelionis friendlies %+v, want 1 start, 2 conceded, 6.4, 4 of 7 passes", f)
	}
}

// Statuses set in game between the two transfer status saves: Gambo Transfer
// Listed, Trumpa Listed for Loan, Černych Unavailable for loan.
const gamboUID27, trumpaUID27, cernychUID27 = 2000591749, 2000425049, 17012660

func TestLoadTestSave27ExcludeUnavailableForLoan(t *testing.T) {
	v := loadTestSaveAt(t, testSave27ListAfterPath, LoadOptions{}).View(false)
	search := func(exclude []string) []uint32 {
		res, err := v.Search(Filter{Club: "Kauno Žalgiris", ExcludeTransferStatus: exclude, Limit: 200})
		if err != nil {
			t.Fatal(err)
		}
		return uids(res.Players)
	}
	if all := search(nil); !slices.Contains(all, gamboUID27) || !slices.Contains(all, cernychUID27) || !slices.Contains(all, trumpaUID27) {
		t.Fatalf("players %v; want Gambo, Černych and Trumpa without the exclude", all)
	}
	got := search([]string{"unavailable_for_loan"})
	if slices.Contains(got, gamboUID27) || slices.Contains(got, cernychUID27) || !slices.Contains(got, trumpaUID27) {
		t.Errorf("players %v; want Trumpa kept, Gambo and Černych left out", got)
	}
}

func TestLoadTestSave27TransferStatus(t *testing.T) {
	for _, tt := range []struct {
		path string
		want map[uint32][]string
	}{
		{testSave27ListBeforePath, map[uint32][]string{gamboUID27: {"unavailable_for_loan"}, trumpaUID27: {"unavailable_for_loan"}, cernychUID27: nil}},
		{testSave27ListAfterPath, map[uint32][]string{gamboUID27: {"transfer_listed", "unavailable_for_loan"}, trumpaUID27: {"loan_listed"}, cernychUID27: {"unavailable_for_loan"}}},
	} {
		v := loadTestSaveAt(t, tt.path, LoadOptions{}).View(false)
		for uid, want := range tt.want {
			p, ok := v.Profile(uid)
			if !ok {
				t.Fatalf("%s: no player %d", tt.path, uid)
			}
			if !slices.Equal(p.TransferStatus, want) {
				t.Errorf("%s: %s transfer status %v, want %v", tt.path, p.Name, p.TransferStatus, want)
			}
		}
	}
}

// Agent-chat approaches from Kauno Žalgiris (reputation 3200) around
// 2026-02-10, as the user reported them. Free agents who refused left a bigger
// club; those who talked did not. Dawid Kownacki (joined Lech Poznań days
// later) is left out.
func TestLoadTestSave27Interest(t *testing.T) {
	g := loadTestSaveAt(t, testSave27InterestPath, LoadOptions{})
	for _, tt := range []struct {
		uid  uint32
		name string
		out  bool
	}{
		{2002078681, "Vaidas Kazlauskas", false}, // signed
		{2000407867, "Chaïn Ghrissi", false},     // talked
		{2000246075, "Jhon Arcila", false},       // talked
		{2000204253, "Wesley", false},            // contracted at FC Hegelmann, talked
		{2000059169, "Tyrell Ashcroft", true},
		{2000066000, "Marcel Wenig", true},
		{48036261, "Mehdi Chahiri", true},
		{67186783, "Carlos Vigaray", true},
		{91156204, "Marco Meyerhöfer", true},
		{48048154, "Sofyan Chader", true},
		{67199101, "Carlos Nieto", true},
		{2000057465, "Javi Serrano", true},
		{62164327, "Luka Ilić", true},
		{13181735, "Ibrahima Niane", true},
	} {
		p := g.byUID[tt.uid]
		if p == nil || p.Name != tt.name {
			t.Fatalf("uid %d: %+v, want %s", tt.uid, p, tt.name)
		}
		in := g.interestOf(p)
		if in == nil {
			t.Fatalf("%s: no interest", tt.name)
		}
		if got := in.Transfer == VerdictOut; got != tt.out {
			t.Errorf("%s: transfer %s, want out %v", tt.name, in.Transfer, tt.out)
		}
		if tt.out && (in.TransferReason == nil || in.TransferReason.Code != FactorClubReputationGap) {
			t.Errorf("%s: reason %+v", tt.name, in.TransferReason)
		}
	}
}

// Every loanee fails the loan (FM's filter, 27.0.0 0xcffffc). Michelbrink is on loan
// at Energie Cottbus.
func TestLoadTestSave27AlreadyOnLoan(t *testing.T) {
	const michelbrink = 2000193965
	g := loadTestSaveAt(t, testSave27ListAfterPath, LoadOptions{})
	loanees := 0
	for _, p := range g.players {
		if p.Loan == nil {
			continue
		}
		c, ok := g.judgeInterest(p)
		if !ok {
			continue
		}
		loanees++
		if want := cmp.Or(c.gate, FactorAlreadyOnLoan); c.verdicts.Loan != VerdictOut || c.reasonCode(InterestLoan) != want {
			t.Fatalf("%s (%d): loan %s for %q, want out for %q", p.Name, p.UID, c.verdicts.Loan, c.reasonCode(InterestLoan), want)
		}
	}
	if loanees == 0 {
		t.Fatal("no loanee judged")
	}
	p := g.byUID[michelbrink]
	if p == nil {
		t.Fatal("no Michelbrink")
	}
	in := g.interestOf(p)
	if in == nil || in.LoanReason == nil || *in.LoanReason != (InterestFactor{FactorAlreadyOnLoan, "On loan at FC Energie Cottbus until 30 June 2027"}) {
		t.Errorf("Michelbrink interest = %+v", in)
	}
}

// Other nationalities are the relationship vector's nation items of type 9
// (see "Relationships and favoured clubs"); the pairs are the players' real
// ones.
func TestLoadTestSave27OtherNationalities(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)

	for uid, want := range map[uint32][]string{
		28124579: {"England", "Nigeria"}, // Jamal Musiala, Germany
		17012660: {"Russia"},             // Fedor Černych, Lithuania
	} {
		p, ok := v.Profile(uid)
		if !ok {
			t.Fatalf("player %d not found", uid)
		}
		var got []string
		for _, n := range p.OtherNationalities {
			got = append(got, n.Nation)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s (%s): other nationalities %v, want %v", p.Name, p.Nation, got, want)
		}
	}
	res, err := v.Search(Filter{Nation: "nigeria", Club: "bayern"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(res.Players, func(p Profile) bool { return p.UID == 28124579 }) {
		t.Errorf("Nigeria at Bayern = %v, want Musiala among them", uids(res.Players))
	}
}

// EU status follows the NATION trade-bloc lists (see "27.0.0 trade
// blocs"). England and Brazil are not EU members;
// Serbia joins on 2030-07-01.
func TestLoadTestSave27EUNational(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)

	for uid, want := range map[uint32]bool{
		17012660: true,  // Fedor Černych, Lithuania and Russia
		14192808: true,  // Franco Baldassarra, Argentina and Italy
		28049320: true,  // Harry Kane, England and Republic of Ireland
		62250559: false, // Marko Konatar, Serbia
		19320257: false, // Renan Oliveira, Brazil
	} {
		p, ok := v.Profile(uid)
		if !ok {
			t.Fatalf("player %d not found", uid)
		}
		if p.EUNational == nil || *p.EUNational != want {
			t.Errorf("%s (%s): EUNational = %v, want %v", p.Name, p.Nation, p.EUNational, want)
		}
	}
}

// The manager's league is A Lyga ("TOPLYGA" in the save); see "27.0.0 competition rules".
func TestLoadTestSave27CompetitionRules(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)

	r, err := v.CompetitionRules(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Competition != "TOPLYGA" || r.Nation != "Lithuania" || r.RuleGroup != "TOPLYGA" {
		t.Errorf("manager's league rules %+v, want Lithuania's TOPLYGA", r)
	}
	want := []SquadRule{
		{Rule: "max_squad_size", Value: ptr(30), Scope: 255},
		{Rule: "same_nation_players_in_11", Value: ptr(3), Scope: 1},
		{Rule: "under_22_players_in_squad", Value: ptr(5), Scope: 1},
	}
	if !reflect.DeepEqual(r.SquadRules, want) {
		t.Errorf("A Lyga squad rules %+v\nwant %+v", r.SquadRules, want)
	}
	pl, err := v.FindCompetition("Premier League")
	if err != nil {
		t.Fatal(err)
	}
	want = []SquadRule{
		{Rule: "homegrown_players_from_nation", Value: ptr(8), Scope: 1},
		{Rule: "max_squad_size", Value: ptr(25), Scope: 255},
	}
	if r, err := v.CompetitionRules(nil, &pl); err != nil || r.RuleGroup != "ENG_FIXTURE_RULES_PREM" || !reflect.DeepEqual(r.SquadRules, want) {
		t.Errorf("Premier League rules %+v, %v", r, err)
	}
}

// homegrownEntries lists p's homegrown clubs and nations as name[rules].
func homegrownEntries(p Profile) (clubs, nations []string) {
	for _, c := range p.TrainedAtClubs {
		clubs = append(clubs, fmt.Sprint(c.Club, c.Rules))
	}
	for _, n := range p.TrainedInNations {
		nations = append(nations, fmt.Sprint(n.Nation, n.Rules))
	}
	return clubs, nations
}

// Homegrown status is the relationship vector's club items of type 72 and
// nation items of type 70 (see "Relationships and favoured clubs").
// Musiala's clubs are his real academies: Chelsea, then Bayern.
func TestLoadTestSave27Homegrown(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)

	p, ok := v.Profile(28124579) // Jamal Musiala
	if !ok {
		t.Fatal("Musiala not found")
	}
	clubs, nations := homegrownEntries(p)
	if want := []string{"FC Bayern[1]", "Chelsea[10 2]"}; !slices.Equal(clubs, want) {
		t.Fatalf("trained at %v, want %v", clubs, want)
	}
	if want := []string{"England[2]", "Germany[1]"}; !slices.Equal(nations, want) {
		t.Errorf("trained in %v, want %v", nations, want)
	}

	chelsea := p.TrainedAtClubs[1].TeamIndex
	if chelsea == nil {
		t.Fatal("Chelsea has no team_index")
	}
	res, err := v.Search(Filter{TrainedAtTeamIndex: chelsea, TrainedInNationIndex: &p.NationIndex, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(res.Players, func(r Profile) bool { return r.UID == p.UID }) {
		t.Errorf("trained at Chelsea and in Germany = %v, want Musiala among them", uids(res.Players))
	}
}

// Players whose homegrown rows were checked in game ("Relationships and
// favoured clubs").
func TestLoadTestSave27HomegrownInGameSamples(t *testing.T) {
	g := loadTestSaveAt(t, testSave27HomegrownPath, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)

	for _, tt := range []struct {
		uid            uint32
		nations, clubs []string
	}{
		{2000500515, []string{"Lithuania[1]"}, []string{"Kauno Žalgiris[1]"}},                                               // Jonas Jankauskas
		{28049320, []string{"England[1]"}, []string{"Spurs[1 10]"}},                                                         // Harry Kane
		{2000585292, []string{"Italy[1 3 12]"}, []string{"Cagliari[1 7]"}},                                                  // Mamadou Barry
		{2000077193, []string{"Spain[1]", "Türkiye[12 1]"}, []string{"Fenerbahçe[1]", "R. Madrid[1]", "Gençlerbirliği[2]"}}, // Arda Güler
	} {
		p, ok := v.Profile(tt.uid)
		if !ok {
			t.Fatalf("player %d not found", tt.uid)
		}
		clubs, nations := homegrownEntries(p)
		if !slices.Equal(nations, tt.nations) || !slices.Equal(clubs, tt.clubs) {
			t.Errorf("%s: nations %v clubs %v, want %v %v", p.Name, nations, clubs, tt.nations, tt.clubs)
		}
	}
}

func TestLoadTestSave27WorkPermits(t *testing.T) {
	g := loadTestSaveAt(t, testSave27Path, LoadOptions{GameDBDir: gameDBDir()})
	v := g.View(false)
	for uid, want := range map[uint32][]WorkPermit{
		12092701: {{NationRef: NationRef{NationIndex: 139, Nation: "England"}, Year: 2026, ESCPlace: true}}, // Collins Sichenje, Charlton
		210609:   {{NationRef: NationRef{NationIndex: 167, Nation: "Scotland"}, Year: 2025}},                // Raphael Sallinger, Hibernian
	} {
		p, ok := v.Profile(uid)
		if !ok {
			t.Fatalf("player %d not found", uid)
		}
		if !reflect.DeepEqual(p.WorkPermits, want) {
			t.Errorf("%s: work permits %+v, want %+v", p.Name, p.WorkPermits, want)
		}
	}
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "work permits") {
			t.Errorf("warning %q", w)
		}
	}
}

// TestLoadTestSave27Match checks Shamrock Rovers 3–2 Kauno Žalgiris
// (2026-07-15) against the game's match report.
func TestLoadTestSave27Match(t *testing.T) {
	g := loadTestSaveAt(t, testSave27HomegrownPath, LoadOptions{GameDBDir: gameDBDir()})
	kz := int64(16521)
	m, err := g.View(false).Match(MatchSelector{Team: &kz, Date: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if m.MatchID == nil || *m.MatchID != 1022 || m.MatchData != MatchDataStored || m.HomeTeamIndex != 456 || m.Score == nil || m.Score.Home != 3 || m.Score.Away != 2 {
		t.Errorf("match = %+v", m.Fixture)
	}
	var got []string
	name := func(p *TacticPlayer) string {
		if p == nil {
			return "-"
		}
		return p.Name
	}
	for _, e := range m.Events {
		team := int64(-1)
		if e.TeamIndex != nil {
			team = *e.TeamIndex
		}
		got = append(got, fmt.Sprintf("%d' %s %s (%s) for %d", e.Minute, e.Type, name(e.Player), name(e.Assist), team))
	}
	want := []string{
		"9' goal John O'Reilly-O'Sullivan (Tunmise Sobowale) for 456",
		"20' goal Renan Oliveira (Amine Benchaib) for 16521",
		"26' goal Michael Noonan (Jack Byrne) for 456",
		"46' goal Renan Oliveira (Nosa Iyobosa Edokpolor) for 16521",
		"78' goal Jack Byrne (Danny Grant) for 456",
	}
	if !slices.Equal(got, want) {
		t.Errorf("events = %q\nwant %q", got, want)
	}
}

// TestLoadTestSave27MatchPlayers checks match statuses and the player lines
// of Shamrock Rovers 3–2 Kauno Žalgiris (2026-07-15) against the game's
// match report.
func TestLoadTestSave27MatchPlayers(t *testing.T) {
	g := loadTestSaveAt(t, testSave27HomegrownPath, LoadOptions{GameDBDir: gameDBDir()})
	for _, w := range g.Info.Warnings {
		if strings.Contains(w, "match") {
			t.Errorf("warning %q", w)
		}
	}
	statuses := map[MatchDataStatus]int{}
	for _, s := range g.fixtures {
		if newScore(s.Result) != nil {
			_, st := g.matchData(s.Result)
			statuses[st]++
		}
	}
	if want := map[MatchDataStatus]int{MatchDataStored: 821, MatchDataPruned: 206, MatchDataQuickSimulated: 2219}; !reflect.DeepEqual(statuses, want) {
		t.Errorf("statuses = %v, want %v", statuses, want)
	}

	id := int64(1022)
	m, err := g.View(false).Match(MatchSelector{MatchID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Players) != 32 || len(m.UnusedSubstitutes) != 12 {
		t.Fatalf("%d players, %d unused; want 32 and 12", len(m.Players), len(m.UnusedSubstitutes))
	}
	byName := map[string]MatchPlayer{}
	var xg [2]float64
	for _, p := range m.Players {
		byName[p.Player.Name] = p
		if p.TeamIndex == 456 {
			xg[0] += p.XG
		} else {
			xg[1] += p.XG
		}
	}
	byrne := byName["Jack Byrne"]
	if byrne.Rating != 8.19 || byrne.Player.ShirtNumber != 29 || !byrne.Started || byrne.TakenOff != nil {
		t.Errorf("Byrne = %+v, want 8.19, shirt 29, started, taken_off unknown (Shamrock's side)", byrne)
	}
	if k := byName["Lirim Kastrati"]; k.TakenOff == nil || !*k.TakenOff || k.Rating != 6.31 {
		t.Errorf("Kastrati = %+v, want taken off, 6.31", k)
	}
	if l := byName["Rokas Lekiatas"]; l.Started || l.TakenOff == nil || *l.TakenOff {
		t.Errorf("Lekiatas = %+v, want came on and stayed on", l)
	}
	if math.Abs(xg[0]-1.98) > 0.03 || math.Abs(xg[1]-2.18) > 0.03 {
		t.Errorf("xG %.2f–%.2f, want about 1.98–2.18 (per-player values are rounded)", xg[0], xg[1])
	}
	if len(m.TeamStats) != 2 {
		t.Fatalf("team stats = %+v, want home and away", m.TeamStats)
	}
	home, away := m.TeamStats[0], m.TeamStats[1]
	if home.TeamIndex != 456 || home.PassesCompleted != 402 || home.PassesCompletedByThird == nil || *home.PassesCompletedByThird != (ThirdPasses{118, 215, 69}) ||
		home.Corners != 6 || home.Fouls != 13 || home.YellowCards != 3 || home.HeadersWon != 16 || home.Headers != 26 || home.TacklesWon != 4 || home.Tackles != 6 || home.ClearCutChances != 5 || home.ShotsBlocked != 1 {
		t.Errorf("Shamrock = %+v, want the report's totals", home)
	}
	if away.PassesCompleted != 435 || away.Corners != 4 || away.Fouls != 7 || away.YellowCards != 2 || away.HeadersWon != 8 || away.TacklesWon != 7 || away.Tackles != 12 || away.ShotsBlocked != 4 {
		t.Errorf("KŽ = %+v, want the report's totals", away)
	}
}

// TestLoadTestSave27PlayerMatches checks Renan Oliveira's match log on
// 2026-07-16 against the 07-15 match report and his TOPLYGA season totals.
func TestLoadTestSave27PlayerMatches(t *testing.T) {
	g := loadTestSaveAt(t, testSave27HomegrownPath, LoadOptions{GameDBDir: gameDBDir()})
	const renan = 19320257
	l, err := g.View(false).PlayerMatches(renan, PlayerMatchFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if l.Player.Name != "Renan Oliveira" || l.Total != 26 {
		t.Fatalf("player %+v, %d matches; want Renan Oliveira and 26", l.Player, l.Total)
	}
	if m := l.Matches[0]; m.MatchID != 1022 || m.Opponent != "Shamrock Rovers" || m.Home || !m.Started || m.Rating != 7.93 || m.Goals != 2 {
		t.Errorf("newest = %+v, want the 07-15 match at Shamrock: started, 7.93, 2 goals", m)
	}
	var starts, subs, goals int
	for _, m := range l.Matches {
		if m.Competition != "TOPLYGA" {
			continue
		}
		if m.Started {
			starts++
		} else {
			subs++
		}
		goals += m.Goals
	}
	p := g.byUID[renan]
	league := g.auxOf(p).season.League
	if league == nil || starts != int(league.Starts) || subs != int(league.SubApps) || goals != int(league.Goals) {
		t.Errorf("TOPLYGA from matches: %d starts, %d subs, %d goals; season totals %+v", starts, subs, goals, league)
	}
}
