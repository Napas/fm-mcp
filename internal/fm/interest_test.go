package fm

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// interestToday is the game date of the labelled save (2036-01-16).
var interestToday = time.Date(2036, 1, 16, 0, 0, 0, 0, time.UTC)

// metzReputation is FC Metz's first-team reputation on 2036-01-16.
const metzReputation = 7218

// gameDaysAgo is the date n days before today in the game's 365-day count.
func gameDaysAgo(today time.Time, n int) savefile.Date {
	y, d := today.Year(), today.YearDay()-n
	for d < 1 {
		y--
		d += 365
	}
	return savefile.Date{Raw: uint16(d), Year: uint16(y)} //nolint:gosec // test dates
}

func calendarDate(t time.Time) savefile.Date {
	return savefile.Date{Raw: uint16(t.YearDay()), Year: uint16(t.Year())} //nolint:gosec // test dates
}

var unsetDate = savefile.Date{Raw: 1, Year: unsetYear}

// interestWorld has club 1 (nation 5) with first team 10 and reserves 11,
// club 2 with first team 20, and national team 30.
func interestWorld(firstTeamRep int, manager bool) World {
	return World{
		Teams: map[int64]*Team{
			10: {Index: 10, ClubIndex: 1, NationIndex: 5, Reputation: firstTeamRep, HasManager: manager},
			11: {Index: 11, ClubIndex: 1, NationIndex: 5, Reputation: 1000},
			20: {Index: 20, ClubIndex: 2, NationIndex: 6, Reputation: 9000, HasManager: true},
			30: {Index: 30, ClubIndex: noTeam, NationIndex: 5, Reputation: 8000, National: true},
		},
		Clubs: map[int64]Club{1: {FirstTeam: 10}, 2: {FirstTeam: 20}},
	}
}

// interestPlayer is a 25-year-old of nation 5 at team 10, joined 153 days
// ago on a FULL contract signed then and ending in four years, Regular
// Starter with a score of 70, Loyalty 9.
func interestPlayer() (*Player, interestInputs) {
	p := &Player{PlayerBasics: PlayerBasics{Age: 25, NationIndex: 5, TeamIndex: 10, Reputation: Reputation{Current: 5000}}}
	p.Positions = []PositionRating{{Position: "GK", Rating: 1}}
	c := &interestContract{
		TeamIndex: 10, End: calendarDate(interestToday.AddDate(4, 0, 0)), Signed: gameDaysAgo(interestToday, 153),
		PlayingTime: 3, OriginalPlayingTime: 3, PlayingTimeScore: 70,
	}
	return p, interestInputs{joined: gameDaysAgo(interestToday, 153), main: c, loyalty: 9}
}

func TestJustJoined(t *testing.T) {
	openEvent := savefile.ContractEvent{Type: 12, ID: 650, Status: 3, Closed: unsetDate}
	justSigned := func(p *Player, in *interestInputs) {
		in.joined = gameDaysAgo(interestToday, 400)
		in.main.Signed = gameDaysAgo(interestToday, 2)
		in.main.End = calendarDate(interestToday.AddDate(0, 0, 600))
	}
	tests := []struct {
		name  string
		edit  func(p *Player, in *interestInputs)
		L     int
		loan  bool
		manag bool
		want  int
	}{
		{"Barthes-like: 153 days, Loyalty 9", func(*Player, *interestInputs) {}, 10, false, true, 98 * 9 * 100},
		{"settled after 251 days", func(_ *Player, in *interestInputs) {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 251), gameDaysAgo(interestToday, 251)
		}, 10, false, true, 0},
		{"250 days: settled", func(_ *Player, in *interestInputs) {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 250), gameDaysAgo(interestToday, 250)
		}, 10, false, true, 0},
		{"249 days", func(_ *Player, in *interestInputs) {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 249), gameDaysAgo(interestToday, 249)
		}, 10, false, true, 2 * 9 * 100},
		{"open ring item exempts", func(_ *Player, in *interestInputs) { in.main.Events = []savefile.ContractEvent{openEvent} }, 10, false, true, 0},
		{"open ring item with a blocking u14 does not exempt", func(_ *Player, in *interestInputs) {
			blocker := savefile.ContractEvent{Type: 5, ID: 700, Closed: unsetDate, U14: 239}
			in.main.Events = []savefile.ContractEvent{openEvent, blocker}
		}, 10, false, true, 98 * 9 * 100},
		{"closed ring item does not exempt", func(_ *Player, in *interestInputs) {
			closed := openEvent
			closed.Closed = savefile.Date{Raw: 100, Year: 2035}
			in.main.Events = []savefile.ContractEvent{closed}
		}, 10, false, true, 98 * 9 * 100},
		{"clause 55 exempts", func(_ *Player, in *interestInputs) { in.main.ClauseTypes = []uint8{55} }, 10, false, true, 0},
		{"clause 56 exempts", func(_ *Player, in *interestInputs) { in.main.ClauseTypes = []uint8{56} }, 10, false, true, 0},
		{"contract type 3 exempts", func(_ *Player, in *interestInputs) { in.main.Kind = 3 }, 10, false, true, 0},
		{"contract type 4 after 61 days exempts", func(_ *Player, in *interestInputs) { in.main.Kind = 4 }, 10, false, true, 0},
		{"expiry: 157 days, ends in 166 days", func(_ *Player, in *interestInputs) {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 157), gameDaysAgo(interestToday, 157)
			in.main.End = calendarDate(interestToday.AddDate(0, 0, 166))
		}, 10, false, true, 0},
		{"expiry needs 61 days", func(_ *Player, in *interestInputs) {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 60), gameDaysAgo(interestToday, 60)
			in.main.End = calendarDate(interestToday.AddDate(0, 0, 166))
		}, 10, false, true, 191 * 9 * 100},
		{"just signed", justSigned, 10, false, true, 99 * 9 * 100},
		{"just signed at 23 takes a quarter", func(p *Player, in *interestInputs) {
			justSigned(p, in)
			p.Age = 23
		}, 10, false, true, 99 * 9 * 100 / 4},
		{"just joined at 23 is whole", func(p *Player, _ *interestInputs) { p.Age = 23 }, 10, false, true, 98 * 9 * 100},
		{"renewed with under 300 days left counts from joining", func(_ *Player, in *interestInputs) {
			in.main.Signed = gameDaysAgo(interestToday, 2)
			in.main.End = calendarDate(interestToday.AddDate(0, 0, 299))
			in.joined = gameDaysAgo(interestToday, 40)
		}, 10, false, true, 211 * 9 * 100},
		{"contract type 2 multiplies by 1000", func(_ *Player, in *interestInputs) { in.main.Kind = 2 }, 10, false, true, 98 * 9 * 1000},
		{"L 7 multiplies by 4", func(*Player, *interestInputs) {}, 7, false, true, 4 * 98 * 9 * 100},
		{"+0x57 bit 0 exempts without a first-team manager", func(_ *Player, in *interestInputs) { in.main.CoreFlags = 1 }, 10, false, false, 0},
		{"+0x57 bit 3 exempts with an original playing time of 7", func(_ *Player, in *interestInputs) {
			in.main.CoreFlags, in.main.OriginalPlayingTime = 8, 7
		}, 10, false, true, 0},
		{"+0x57 bit 0 with a manager and original playing time 3", func(_ *Player, in *interestInputs) { in.main.CoreFlags = 1 }, 10, false, true, 98 * 9 * 100},
		{"loan, domestic", func(*Player, *interestInputs) {}, 7, true, true, 4 * 98 * 9 * 100},
		{"loan at 25 counts from joining", justSigned, 7, true, true, 0},
		{"loan, foreign national", func(p *Player, _ *interestInputs) { p.NationIndex = 6 }, 7, true, true, 0},
		{"loan, national of his contract club's nation while at a national team", func(p *Player, _ *interestInputs) { p.TeamIndex = 30 }, 7, true, true, 4 * 98 * 9 * 100},
		{"loan, b = Breakthrough Prospect (level 4)", func(_ *Player, in *interestInputs) { in.main.PlayingTime = 10 }, 7, true, true, 0},
		{"loan, b = Emergency Backup (level 5)", func(_ *Player, in *interestInputs) { in.main.PlayingTime = 9 }, 7, true, true, 4 * 98 * 9 * 100},
		{"joined and signed unset (1900)", func(_ *Player, in *interestInputs) { in.joined, in.main.Signed = unsetDate, unsetDate }, 10, false, true, 0},
		{"joined zero, signed unset", func(_ *Player, in *interestInputs) { in.joined, in.main.Signed = savefile.Date{}, unsetDate }, 10, false, true, 0},
		{"end unset", func(_ *Player, in *interestInputs) { in.main.End = savefile.Date{Raw: 2, Year: unsetYear} }, 10, false, true, 98 * 9 * 100},
		{"free agent", func(_ *Player, in *interestInputs) { in.main = nil }, 10, false, true, 0},
		{"on loan", func(p *Player, _ *interestInputs) { p.Loan = &Contract{TeamIndex: 20} }, 10, false, true, 0},
	}
	for _, tt := range tests {
		p, in := interestPlayer()
		tt.edit(p, &in)
		if got := justJoined(p, in, interestWorld(6000, tt.manag), interestToday, tt.L, tt.loan); got != tt.want {
			t.Errorf("%s: justJoined = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestClubReputationGap(t *testing.T) {
	// Rt 7218 gives T = 28 872; the player is Regular Starter (level 8).
	tests := []struct {
		name    string
		edit    func(p *Player, in *interestInputs)
		ftRep   int
		rt      int
		wantGap int
	}{
		{"d = -5000 is x6", nil, 8468, metzReputation, -30000},
		{"d = -3500 is x5", nil, 8093, metzReputation, -17500},
		{"d = -2500 is x4", nil, 7843, metzReputation, -10000},
		{"d = -1500 is x2", nil, 7593, metzReputation, -3000},
		{"d = -1000 is x1", nil, 7468, metzReputation, -1000},
		{"d = 4872", nil, 6000, metzReputation, 4872},
		{"T capped at 36 000", nil, 6000, 10000, 12000},
		{"world reputation 8000 adds -3128 x8", func(p *Player, _ *interestInputs) { p.Reputation.World = 8000 }, 6000, metzReputation, 4872 - 25024},
		{"world reputation term above -500 is x1", func(p *Player, _ *interestInputs) { p.Reputation.World = 7300 }, 6000, metzReputation, 4872 - 328},
		{"world reputation term never positive", func(p *Player, _ *interestInputs) { p.Reputation.World = 1000 }, 6000, metzReputation, 4872},
		{"clause 54 raises T by 15%", func(_ *Player, in *interestInputs) { in.main.ClauseTypes = []uint8{54} }, 6000, metzReputation, 33202 - 24000},
		{"clause 63 raises T in the world term too", func(p *Player, in *interestInputs) {
			in.main.ClauseTypes = []uint8{63}
			p.Reputation.World = 8500
		}, 6000, metzReputation, 33202 - 24000 + (33202-34000)*8},
		{"clause 54 at 30 does not", func(p *Player, in *interestInputs) {
			in.main.ClauseTypes = []uint8{54}
			p.Age = 30
		}, 6000, metzReputation, 4872},
		{"level 6 lowers C by 20%", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 7, 20
		}, 6000, metzReputation, 28872 - 19200},
		{"level 4 keeps C", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 10, -20
		}, 6000, metzReputation, 4872},
		{"the score's level counts", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 7, 70
		}, 6000, metzReputation, 4872},
		{"the main contract's club while on loan", func(p *Player, _ *interestInputs) {
			p.TeamIndex, p.Loan = 20, &Contract{TeamIndex: 20}
		}, 6000, metzReputation, 4872},
		{"reserve team reads its first team", func(_ *Player, in *interestInputs) { in.main.TeamIndex = 11 }, 6000, metzReputation, 4872},
		{"free agent", func(_ *Player, in *interestInputs) { in.main = nil }, 6000, metzReputation, 0},
		{"no playing time", func(_ *Player, in *interestInputs) { in.main.PlayingTime = 0 }, 6000, metzReputation, 0},
		{"contract with a national team", func(_ *Player, in *interestInputs) { in.main.TeamIndex = 30 }, 6000, metzReputation, 0},
	}
	for _, tt := range tests {
		p, in := interestPlayer()
		if tt.edit != nil {
			tt.edit(p, &in)
		}
		if got := clubReputationGap(p, in, interestWorld(tt.ftRep, true), tt.rt); got != tt.wantGap {
			t.Errorf("%s: clubReputationGap = %d, want %d", tt.name, got, tt.wantGap)
		}
	}
}

func TestClubReputationGapLoan(t *testing.T) {
	for _, tt := range []struct{ world, want int }{{8000, -25024}, {7300, -328}, {7000, 0}, {10000, (28872 - 36000) * 8}} {
		p, _ := interestPlayer()
		p.Reputation.World = tt.world
		if got := clubReputationGapLoan(p, metzReputation); got != tt.want {
			t.Errorf("world %d: clubReputationGapLoan = %d, want %d", tt.world, got, tt.want)
		}
	}
}

func TestPlayingTimeValue(t *testing.T) {
	cap8 := savefile.ContractEvent{Type: 12, ID: 650, Status: 2, Closed: unsetDate}
	tests := []struct {
		name   string
		edit   func(p *Player, in *interestInputs)
		ftRep  int
		rt, L  int
		loan   bool
		want   int
		wantOK bool
	}{
		{"d = 2, CR below Rt", nil, 6000, metzReputation, 10, false, 5*2218 + 2*5000, true},
		{"d = 2, CR and club above Rt", func(p *Player, _ *interestInputs) { p.Reputation.Current = 8000 }, 8000, metzReputation, 10, false, -3910 - 7820 + 16000, true},
		{"d = 2, CR above Rt, club below", func(p *Player, _ *interestInputs) { p.Reputation.Current = 8000 }, 7000, metzReputation, 10, false, -3910 + 16000, true},
		{"d >= 1 is never negative", func(p *Player, _ *interestInputs) { p.Reputation.Current = 9900 }, 9900, 2000, 10, false, 0, true},
		{"d = 0", nil, 6000, metzReputation, 8, false, 0, true},
		{"d = -1", nil, 6000, metzReputation, 7, false, -5000 - 11090, true},
		{"d < 0 is never positive", func(p *Player, _ *interestInputs) { p.Reputation.Current = 9500 }, 6000, metzReputation, 7, false, 0, true},
		{"d < 0 at L 6 is not modelled", nil, 6000, metzReputation, 6, false, 0, false},
		{"loan over 20 wants at least level 6", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 9, -10
		}, 6000, metzReputation, 7, true, 11090 + 5000, true},
		{"loan at 20 keeps level 5", func(p *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 9, -10
			p.Age = 20
		}, 6000, metzReputation, 7, true, 11090 + 2*5000, true},
		{"level 6 wants 6", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 7, 20
		}, 6000, metzReputation, 10, false, 11090 + 4*5000, true},
		{"level 6 keeper wants 7", func(p *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 7, 20
			p.Positions[0].Rating = 15
		}, 6000, metzReputation, 10, false, 11090 + 3*5000, true},
		{"level 4 wants 7", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 10, -20
		}, 6000, metzReputation, 10, false, 11090 + 3*5000, true},
		{"open ring item caps level 10 at 8", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 1, 90
			in.main.Events = []savefile.ContractEvent{cap8}
		}, 6000, metzReputation, 10, false, 11090 + 2*5000, true},
		{"level 10 without a cap", func(_ *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 1, 90
		}, 6000, metzReputation, 10, false, 0, true},
		{"no club first team counts as 1500", func(p *Player, in *interestInputs) {
			in.main.PlayingTime, in.main.PlayingTimeScore = 22, -50
			in.main.TeamIndex = 30
			p.Reputation.Current = 2000
		}, 6000, 1000, 10, false, -5000 - 5000 + 9*2000, true},
		{"free agent", func(_ *Player, in *interestInputs) { in.main = nil }, 6000, metzReputation, 10, false, 0, false},
	}
	for _, tt := range tests {
		p, in := interestPlayer()
		if tt.edit != nil {
			tt.edit(p, &in)
		}
		got, ok := playingTimeValue(p, in, interestWorld(tt.ftRep, true), tt.rt, tt.L, tt.loan)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("%s: playingTimeValue = %d, %v; want %d, %v", tt.name, got, ok, tt.want, tt.wantOK)
		}
	}
}

// goldenTerms is a row of testdata/interest_golden.tsv, the reference model's
// output for the labelled players of 2036-01-16.
type goldenTerms struct {
	uid  uint32
	name string
	want interestTerms
}

func readInterestGolden(t *testing.T) []goldenTerms {
	t.Helper()
	data, err := os.ReadFile("testdata/interest_golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var out []goldenTerms
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines[1:] { // header: uid name P_T r2 pt10 P_L pt7 r2L T L why
		c := strings.Split(line, "\t")
		if len(c) != 11 {
			t.Fatalf("golden row %q: %d columns", line, len(c))
		}
		if c[10] != "" { // gates and free agents carry no terms
			continue
		}
		uid, err := strconv.ParseUint(c[0], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		var v [6]int
		for i := range v {
			if v[i], err = strconv.Atoi(c[2+i]); err != nil {
				t.Fatalf("golden row %s column %d: %v", c[0], 2+i, err)
			}
		}
		out = append(out, goldenTerms{uint32(uid), c[1], interestTerms{v[0], v[1], v[2], v[3], v[4], v[5]}})
	}
	return out
}

// TestInterestTermsMatchReference compares the model with the reference
// model's output on the labelled players. Allowed differences: no loan
// penalty when his playing time's level is below 5, and a quarter of the
// "just signed" penalty at 23 or younger.
func TestInterestTermsMatchReference(t *testing.T) {
	rows := readInterestGolden(t)
	g := loadRealSave(t, "FM_REAL_SAVE_16JAN", interestToday)
	if rt := g.targetReputation(); rt != metzReputation {
		t.Fatalf("targetReputation = %d, want %d", rt, metzReputation)
	}
	allowed := 0
	for _, r := range rows {
		p := g.byUID[r.uid]
		if p == nil {
			t.Errorf("%d %s: not in the save", r.uid, r.name)
			continue
		}
		got := g.interestTerms(p, g.targetReputation())
		in := g.auxOf(p).interest
		if got.penaltyL != r.want.penaltyL && got.penaltyL == 0 && in.main != nil && statusLevel(in.main.PlayingTime) < loanMinLevel {
			t.Logf("%d %s: allowed: no loan penalty at level %d (reference %d)", r.uid, r.name, statusLevel(in.main.PlayingTime), r.want.penaltyL)
			got.penaltyL = r.want.penaltyL
			allowed++
		}
		if p.Age <= justSignedReducedAge {
			for _, pair := range [][2]*int{{&got.penaltyT, &r.want.penaltyT}, {&got.penaltyL, &r.want.penaltyL}} {
				if *pair[0] != *pair[1] && *pair[0] == *pair[1]/4 {
					t.Logf("%d %s: allowed: quarter just-signed penalty at %d", r.uid, r.name, p.Age)
					*pair[0] = *pair[1]
					allowed++
				}
			}
		}
		if got != r.want {
			t.Errorf("%d %s: terms %+v, reference %+v", r.uid, r.name, got, r.want)
		}
	}
	t.Logf("%d players compared, %d allowed differences", len(rows), allowed)
	if len(rows) != 86 {
		t.Errorf("%d golden rows with terms, want 86", len(rows))
	}
}

// interestGame is a game whose manager is at team 10 (club 1, rep 7218)
// and whose one player has the given inputs; his team is 20 (club 2).
// Team 21 is Stade Brestois (club 3).
func interestGame(p *Player, in interestInputs) *Game {
	w := interestWorld(metzReputation, true)
	w.Clubs = map[int64]Club{1: {Name: "FC Metz", FirstTeam: 10}, 2: {Name: "SM Caen", FirstTeam: 20}, 3: {Name: "Stade Brestois", FirstTeam: 21}}
	w.Teams[20].Reputation = 6000
	w.Teams[21] = &Team{Index: 21, ClubIndex: 3, NationIndex: 5, Reputation: 6500, Name: "Stade Brestois"}
	p.TeamIndex, p.Club = 20, "SM Caen"
	in.main.TeamIndex = 20
	return &Game{Info: Info{ManagerTeamIndex: 10, Date: interestToday}, world: w, aux: []playerAux{{interest: in}}}
}

func TestInterestJustJoinedAndTexts(t *testing.T) {
	p, in := interestPlayer()
	in.loyalty = 17
	g := interestGame(p, in)
	got := g.interestOf(p)
	if got == nil || got.Transfer != VerdictOut || got.Loan != VerdictUncertain {
		t.Fatalf("interest = %+v", got)
	}
	if got.TransferReason == nil || got.TransferReason.Code != FactorOnlyJustJoined || got.TransferReason.Text != "Joined SM Caen 153 days ago; won't consider a move yet" {
		t.Errorf("reason = %+v", got.TransferReason)
	}
}

func TestInterestTextsHideHiddenValues(t *testing.T) {
	for _, signed := range []bool{false, true} {
		p, in := interestPlayer()
		in.loyalty = 17
		p.Reputation.World = 100
		if signed {
			in.joined, in.main.Signed = gameDaysAgo(interestToday, 400), gameDaysAgo(interestToday, 2)
		}
		got := interestGame(p, in).interestOf(p)
		if got.TransferReason == nil {
			t.Fatalf("signed %v: no reason", signed)
		}
		for _, f := range append([]InterestFactor{*got.TransferReason}, got.TransferFactors...) {
			if strings.Contains(f.Text, "17") || strings.Contains(strings.ToLower(f.Text), "loyalty") {
				t.Errorf("signed %v: factor %s shows a hidden value: %q", signed, f.Code, f.Text)
			}
		}
	}
}

func TestInterestGates(t *testing.T) {
	tests := []struct {
		name string
		edit func(*interestInputs)
		code string
		text string
	}{
		{"arranged", func(in *interestInputs) {
			in.arranged = &arrangedMove{team: 21, date: time.Date(2036, 7, 1, 0, 0, 0, 0, time.UTC)}
		}, FactorTransferArranged, ""},
		{"retiring", func(in *interestInputs) { in.retiring = true }, FactorRetiring, "Is about to retire"},
		{"one club", func(in *interestInputs) { in.onlyOneClub = true }, FactorOnlyOneClub, "Will only join one particular club"},
		{"left", func(in *interestInputs) { in.previousClub = 1; in.joined = gameDaysAgo(interestToday, 3) }, FactorRecentlyLeftClub, "Left FC Metz 3 days ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, in := interestPlayer()
			tt.edit(&in)
			got := newOut(t, interestGame(p, in).interestOf(p))
			if got.TransferReason.Code != tt.code || tt.text != "" && got.TransferReason.Text != tt.text {
				t.Errorf("reason = %+v", got.TransferReason)
			}
		})
	}
	t.Run("arranged text", func(t *testing.T) {
		p, in := interestPlayer()
		in.arranged = &arrangedMove{team: 21, date: time.Date(2036, 7, 1, 0, 0, 0, 0, time.UTC)}
		g := interestGame(p, in)
		if text := g.interestOf(p).TransferReason.Text; text != "Has agreed a move to Stade Brestois on 1 July 2036" {
			t.Errorf("text = %q", text)
		}
	})
	t.Run("left long ago", func(t *testing.T) {
		p, in := interestPlayer()
		in.previousClub, in.joined = 1, gameDaysAgo(interestToday, 151)
		if got := interestGame(p, in).interestOf(p); got.TransferReason != nil && got.TransferReason.Code == FactorRecentlyLeftClub {
			t.Errorf("151 days counts as recently left")
		}
	})
}

func newOut(t *testing.T, in *Interest) *Interest {
	t.Helper()
	if in == nil || in.Transfer != VerdictOut || in.Loan != VerdictOut || in.TransferReason == nil {
		t.Fatalf("interest = %+v, want out both ways with a reason", in)
	}
	return in
}

func TestInterestFreeAgent(t *testing.T) {
	p, in := interestPlayer()
	g := interestGame(p, in)
	g.aux[0].interest.main = nil
	p.TeamIndex = noTeam
	if got := g.interestOf(p); got == nil || got.Transfer != VerdictUncertain || got.Loan != VerdictUncertain || got.TransferReason != nil {
		t.Errorf("free agent = %+v", got)
	}
	g.aux[0].interest.retiring = true
	if got := g.interestOf(p); got == nil || got.Transfer != VerdictOut {
		t.Errorf("retiring free agent = %+v", got)
	}
	g.aux[0].interest.retiring = false
	g.aux[0].interest.previousClub, g.aux[0].interest.joined = 1, gameDaysAgo(interestToday, 3)
	if got := g.interestOf(p); got.Transfer != VerdictUncertain {
		t.Errorf("recently left does not apply to free agents: %+v", got)
	}
}

func TestInterestReputationGapReason(t *testing.T) {
	p, in := interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	g := interestGame(p, in)
	g.world.Teams[20].Reputation = 9500
	got := g.interestOf(p)
	if got.Transfer != VerdictOut || got.TransferReason == nil || got.TransferReason.Code != FactorClubReputationGap || got.TransferReason.Text != "SM Caen is a bigger club than FC Metz" {
		t.Errorf("interest = %+v reason %+v", got, got.TransferReason)
	}
	if !slicesContainsCode(got.TransferFactors, FactorClubReputationGap) {
		t.Errorf("factors = %+v", got.TransferFactors)
	}
}

func slicesContainsCode(fs []InterestFactor, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestInterestNotEvaluated(t *testing.T) {
	t.Run("own player", func(t *testing.T) {
		p, in := interestPlayer()
		g := interestGame(p, in)
		p.TeamIndex = 11
		if g.interestOf(p) != nil {
			t.Error("reserve player of the human's club evaluated")
		}
		p.TeamIndex = 20
		g.aux[0].interest.main.TeamIndex = 10 // on loan elsewhere, contracted to the human's club
		if g.interestOf(p) != nil {
			t.Error("loanee of the human's club evaluated")
		}
	})
	t.Run("other gender", func(t *testing.T) {
		p, in := interestPlayer()
		g := interestGame(p, in)
		p.Female = true
		if g.interestOf(p) != nil {
			t.Error("woman evaluated for a men's team")
		}
	})
	t.Run("national team only", func(t *testing.T) {
		p, in := interestPlayer()
		g := interestGame(p, in)
		g.aux[0].interest.main = nil
		p.TeamIndex = 30
		if g.interestOf(p) != nil {
			t.Error("national-team-only player evaluated")
		}
	})
	t.Run("manager club unknown", func(t *testing.T) {
		p, in := interestPlayer()
		g := interestGame(p, in)
		g.Info.ManagerTeamIndex = noTeam
		if g.interestOf(p) != nil {
			t.Error("evaluated without a manager club")
		}
	})
}

func TestInterestFor(t *testing.T) {
	o, u, l := VerdictOut, VerdictUncertain, VerdictLikely
	tests := []struct {
		transfer, loan              Verdict
		wantT, wantL, wantTransLoan Verdict
	}{
		{o, o, o, o, o},
		{o, u, o, u, u},
		{u, o, u, o, u},
		{u, u, u, u, u},
		{l, o, l, o, l},
		{l, u, l, u, l},
	}
	for _, tt := range tests {
		in := Interest{Transfer: tt.transfer, Loan: tt.loan}
		if in.For(InterestTransfer) != tt.wantT || in.For(InterestLoan) != tt.wantL || in.For(InterestTransferOrLoan) != tt.wantTransLoan {
			t.Errorf("%s/%s: %s %s %s", tt.transfer, tt.loan, in.For(InterestTransfer), in.For(InterestLoan), in.For(InterestTransferOrLoan))
		}
	}
}

func TestInterestNeedsManagerReputation(t *testing.T) {
	p, in := interestPlayer()
	g := interestGame(p, in)
	g.world.Teams[10].Reputation = 0
	if g.interestOf(p) != nil {
		t.Error("evaluated with a first-team reputation of 0")
	}
	g = interestGame(p, in)
	g.world.Clubs[1] = Club{Name: "FC Metz", FirstTeam: noTeam}
	if g.interestOf(p) != nil {
		t.Error("evaluated for a club without a first team")
	}
}

func TestInterestJustSignedText(t *testing.T) {
	p, in := interestPlayer()
	in.joined, in.main.Signed = gameDaysAgo(interestToday, 400), gameDaysAgo(interestToday, 2)
	got := interestGame(p, in).interestOf(p)
	if got.Transfer != VerdictOut || got.TransferReason == nil || got.TransferReason.Code != FactorOnlyJustJoined || got.TransferReason.Text != "Signed a new contract with SM Caen 2 days ago; won't consider a move yet" {
		t.Errorf("reason = %+v", got.TransferReason)
	}
}

// sumAt is the transfer sum with team 20 at reputation rep.
func sumAt(g *Game, p *Player, rep int) int {
	g.world.Teams[20].Reputation = rep
	t := g.interestTerms(p, g.targetReputation())
	return -t.penaltyT + t.gap + t.pt10
}

func TestInterestLikelyBoundary(t *testing.T) {
	p, in := interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	g := interestGame(p, in)
	rep := 6000
	for sumAt(g, p, rep) < 30_000 {
		rep--
	}
	if got := sumAt(g, p, rep); got < 30_000 {
		t.Fatalf("sum %d", got)
	}
	if v := g.interestOf(p).Transfer; v != VerdictLikely {
		t.Errorf("sum %d: %s, want likely", sumAt(g, p, rep), v)
	}
	if below := sumAt(g, p, rep+1); below >= 30_000 || g.interestOf(p).Transfer != VerdictUncertain {
		t.Errorf("sum %d: %s, want uncertain", below, g.interestOf(p).Transfer)
	}
}

func TestInterestOutBoundary(t *testing.T) {
	p, in := interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	g := interestGame(p, in)
	rep := 6000
	for sumAt(g, p, rep) >= -20_000 && rep < 9000 {
		rep++
	}
	if sumAt(g, p, rep) >= -20_000 {
		t.Skip("no reputation reaches the cut")
	}
	got := g.interestOf(p)
	if got.Transfer != VerdictOut || got.TransferReason == nil || got.TransferReason.Code != FactorClubReputationGap {
		t.Errorf("sum %d: %+v", sumAt(g, p, rep), got)
	}
	if v := g.interestOf(p); sumAt(g, p, rep-1) < -20_000 || v == nil {
		t.Fatalf("boundary not found")
	}
	if g.interestOf(p).Transfer != VerdictUncertain {
		t.Errorf("sum %d: %s, want uncertain", sumAt(g, p, rep-1), g.interestOf(p).Transfer)
	}
}

func TestInterestRecentlyLeftBoundary(t *testing.T) {
	for days, wantOut := range map[int]bool{150: true, 151: false} {
		p, in := interestPlayer()
		in.previousClub, in.joined = 1, gameDaysAgo(interestToday, days)
		in.main.Signed = unsetDate
		got := interestGame(p, in).interestOf(p)
		isOut := got.TransferReason != nil && got.TransferReason.Code == FactorRecentlyLeftClub
		if isOut != wantOut {
			t.Errorf("%d days: reason %+v, want out %v", days, got.TransferReason, wantOut)
		}
	}
}

func hasCode(in *Interest, code string) bool { return slicesContainsCode(in.TransferFactors, code) }

func TestInterestDependsOnRole(t *testing.T) {
	p, in := interestPlayer()
	in.loyalty = 1 // a penalty of 9 800 at L 10 and 29 400 at L 8
	g := interestGame(p, in)
	got := g.interestOf(p)
	if got.Transfer != VerdictUncertain || !hasCode(got, FactorDependsOnRole) {
		t.Errorf("penalised player: %+v", got)
	}
	p, in = interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	got = interestGame(p, in).interestOf(p)
	if got.Transfer == VerdictOut || hasCode(got, FactorDependsOnRole) {
		t.Errorf("player without a penalty: %+v", got)
	}
	p, in = interestPlayer() // out by the penalty: no role factor
	got = interestGame(p, in).interestOf(p)
	if got.Transfer != VerdictOut || hasCode(got, FactorDependsOnRole) {
		t.Errorf("out player: %+v", got)
	}
}

func TestInterestLoanOnlyOut(t *testing.T) {
	p, in := interestPlayer()
	in.loyalty = 1 // transfer penalty 9 800, loan penalty 39 200
	g := interestGame(p, in)
	g.world.Teams[20].NationIndex = p.NationIndex // the loan penalty needs his nation's club
	got := g.interestOf(p)
	if got.Transfer == VerdictOut || got.Loan != VerdictOut {
		t.Fatalf("transfer %s, loan %s", got.Transfer, got.Loan)
	}
	if got.TransferReason != nil {
		t.Errorf("reason = %+v, want nil with the transfer open", got.TransferReason)
	}
	if got.LoanReason == nil || got.LoanReason.Code != FactorOnlyJustJoined || got.LoanReason.Text != "Joined SM Caen 153 days ago; won't consider a move yet" {
		t.Errorf("loan reason = %+v", got.LoanReason)
	}
	if got.For(InterestTransferOrLoan) == VerdictOut {
		t.Error("transfer_or_loan out with the transfer open")
	}
}

func TestInterestFreeAgentGates(t *testing.T) {
	for name, edit := range map[string]func(*interestInputs){
		"arranged": func(in *interestInputs) { in.arranged = &arrangedMove{team: 21, date: interestToday} },
		"one club": func(in *interestInputs) { in.onlyOneClub = true },
		"plain":    func(*interestInputs) {},
	} {
		p, in := interestPlayer()
		g := interestGame(p, in)
		g.aux[0].interest.main = nil
		p.TeamIndex = noTeam
		edit(&g.aux[0].interest)
		got := g.interestOf(p)
		want := VerdictOut
		if name == "plain" {
			want = VerdictUncertain
		}
		if got == nil || got.Transfer != want || got.Loan != want || (want == VerdictOut) != (got.TransferReason != nil) {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestInterestGateOrder(t *testing.T) {
	p, in := interestPlayer()
	in.arranged = &arrangedMove{team: 21, date: interestToday}
	in.retiring, in.onlyOneClub = true, true
	in.previousClub, in.joined = 1, gameDaysAgo(interestToday, 3)
	steps := []struct {
		code string
		drop func(*interestInputs)
	}{
		{FactorTransferArranged, func(in *interestInputs) { in.arranged = nil }},
		{FactorRetiring, func(in *interestInputs) { in.retiring = false }},
		{FactorOnlyOneClub, func(in *interestInputs) { in.onlyOneClub = false }},
		{FactorRecentlyLeftClub, func(*interestInputs) {}},
	}
	for _, st := range steps {
		got := interestGame(p, in).interestOf(p)
		if got.TransferReason == nil || got.TransferReason.Code != st.code {
			t.Fatalf("reason = %+v, want %s", got.TransferReason, st.code)
		}
		st.drop(&in)
	}
}

// A loan bound of -P without the playing-time term stays above the cut;
// adding a negative loan playing-time value would put it below.
func TestInterestLoanIgnoresNegativePlayingTime(t *testing.T) {
	p, in := interestPlayer()
	in.loyalty = 2
	in.joined, in.main.Signed = gameDaysAgo(interestToday, 245), gameDaysAgo(interestToday, 245)
	g := interestGame(p, in)
	g.world.Teams[20].NationIndex = p.NationIndex
	if terms := g.interestTerms(p, g.targetReputation()); terms.pt7 >= 0 || terms.penaltyL == 0 || -terms.penaltyL < -20_000 || -terms.penaltyL+terms.pt7 >= -20_000 {
		t.Fatalf("scenario: %+v", terms)
	}
	if got := g.interestOf(p); got.Loan != VerdictUncertain {
		t.Errorf("loan = %s", got.Loan)
	}
}

func TestInterestLoanOnlyOutByReputation(t *testing.T) {
	p, in := interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	p.Reputation.World = 9000
	g := interestGame(p, in)
	g.world.Teams[20].Reputation = 1000
	got := g.interestOf(p)
	if got.Transfer == VerdictOut || got.Loan != VerdictOut {
		t.Fatalf("transfer %s, loan %s", got.Transfer, got.Loan)
	}
	if got.TransferReason != nil {
		t.Errorf("reason = %+v, want nil with the transfer open", got.TransferReason)
	}
	if got.LoanReason == nil || got.LoanReason.Code != FactorClubReputationGap || got.LoanReason.Text != "His reputation is above FC Metz's" {
		t.Errorf("loan reason = %+v", got.LoanReason)
	}
}

// TestInterestMatchesFM checks the verdicts against FM's lists on 2036-01-16:
// no player FM passes is out. FM's fails are not required to be out.
func TestInterestMatchesFM(t *testing.T) {
	g := loadRealSave(t, "FM_REAL_SAVE_16JAN", interestToday)
	data, err := os.ReadFile("testdata/interest_labels.tsv")
	if err != nil {
		t.Fatal(err)
	}
	fails, failsOut := map[string]int{}, map[string]int{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n")[1:] {
		c := strings.Split(line, "\t")
		uid, err := strconv.ParseUint(c[1], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		p := g.byUID[uint32(uid)]
		if p == nil {
			t.Errorf("%s %s: not in the save", c[1], c[2])
			continue
		}
		in := g.interestOf(p)
		if in == nil {
			t.Errorf("%s %s: not evaluated", c[1], c[2])
			continue
		}
		reason := ""
		if in.TransferReason != nil {
			reason = in.TransferReason.Code
		}
		t.Logf("%s %s %s: transfer=%s loan=%s reason=%s fm=%s/%s", c[0], c[1], c[2], in.Transfer, in.Loan, reason, c[3], c[4])
		for i, k := range []InterestKind{InterestTransfer, InterestTransferOrLoan} {
			if 3+i >= len(c) || c[3+i] == "" {
				continue
			}
			label := c[3+i]
			key := c[0] + "/" + string(k)
			switch label {
			case "pass":
				if in.For(k) == VerdictOut {
					t.Errorf("%s %s: FM passes %s, model says out (%s)", c[1], c[2], k, reason)
				}
			case "fail":
				fails[key]++
				if in.For(k) == VerdictOut {
					failsOut[key]++
				}
			}
		}
		checkLabelledReason(t, c[1], in)
	}
	for key, n := range fails {
		t.Logf("%s: %d of %d FM fails are out", key, failsOut[key], n)
	}
}

// labelledReasons are the cases the research found certain.
var labelledReasons = map[string]string{
	"2002115699": FactorTransferArranged, // Marco
	"2002115328": FactorRecentlyLeftClub, // Vilas
	"2002077775": FactorRecentlyLeftClub, // Bayal
	"2002112631": FactorOnlyJustJoined,   // Barthes
	"2002115333": FactorOnlyJustJoined,   // Gamboa
}

func checkLabelledReason(t *testing.T, uid string, in *Interest) {
	t.Helper()
	if want, ok := labelledReasons[uid]; ok && (in.TransferReason == nil || in.TransferReason.Code != want) {
		t.Errorf("%s: reason %+v, want %s", uid, in.TransferReason, want)
	}
	if uid == "2002115106" || uid == "2002125884" { // Lafleur, Tomašević: loan only
		if in.For(InterestTransfer) != VerdictOut || in.For(InterestTransferOrLoan) == VerdictOut {
			t.Errorf("%s: transfer %s, transfer_or_loan %s", uid, in.For(InterestTransfer), in.For(InterestTransferOrLoan))
		}
	}
}

// searchInterestGame has three players at other clubs: uid 1 just joined
// (out), uid 2 settled at a club far below the manager's (likely) and uid 3
// settled with a sum near the line (uncertain). It returns the game and
// the three players.
func searchInterestGame(t *testing.T) *Game {
	t.Helper()
	w := interestWorld(metzReputation, true)
	w.Clubs = map[int64]Club{1: {Name: "FC Metz", FirstTeam: 10}, 2: {Name: "SM Caen", FirstTeam: 20}, 3: {Name: "Club C", FirstTeam: 21}, 4: {Name: "Club D", FirstTeam: 22}}
	w.Teams[20].Reputation = 6000
	w.Teams[21] = &Team{Index: 21, ClubIndex: 3, NationIndex: 6, Reputation: 1000, HasManager: true}
	w.Teams[22] = &Team{Index: 22, ClubIndex: 4, NationIndex: 6, Reputation: 1000, HasManager: true}
	g := &Game{Info: Info{ManagerTeamIndex: 10, Date: interestToday}, world: w}
	for i, team := range []int64{20, 21, 22} {
		p, in := interestPlayer()
		p.UID, p.TeamIndex, p.auxIdx = uint32(i+1), team, uint32(i) //nolint:gosec // small test values
		in.main.TeamIndex = team
		if i > 0 {
			in.joined, in.main.Signed = unsetDate, unsetDate
		} else {
			in.loyalty = 17
		}
		g.players = append(g.players, p)
		g.aux = append(g.aux, playerAux{interest: in})
	}
	g.byUID = map[uint32]*Player{}
	for _, p := range g.players {
		g.byUID[p.UID] = p
	}
	find := func(p *Player, want Verdict) {
		for rep := 6000; rep >= 0; rep-- {
			w.Teams[p.TeamIndex].Reputation = rep
			if got := g.interestOf(p); got != nil && got.Transfer == want {
				return
			}
		}
		t.Fatalf("no reputation gives %s", want)
	}
	find(g.players[1], VerdictLikely)
	find(g.players[2], VerdictUncertain)
	if v := g.interestOf(g.players[0]).Transfer; v != VerdictOut {
		t.Fatalf("player 1 = %s, want out", v)
	}
	return g
}

func TestSearchInterest(t *testing.T) {
	g := searchInterestGame(t)
	res, err := g.View(false).Search(Filter{Interest: "transfer"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || len(res.Players) != 2 {
		t.Fatalf("total %d, rows %d, want 2", res.Total, len(res.Players))
	}
	if d := res.InterestDropped; d == nil || d.Total != 1 || !maps.Equal(d.ByReason, map[string]int{FactorOnlyJustJoined: 1}) {
		t.Errorf("dropped = %+v, want 1 only_just_joined", d)
	}
	want := map[uint32]Verdict{2: VerdictLikely, 3: VerdictUncertain}
	for _, p := range res.Players {
		if p.Interest == nil || p.Interest.Transfer != want[p.UID] {
			t.Errorf("uid %d interest = %+v, want transfer %s", p.UID, p.Interest, want[p.UID])
		}
	}

	res, err = g.View(false).Search(Filter{})
	if err != nil || res.InterestDropped != nil || res.Total != 3 {
		t.Errorf("without interest: total %d, dropped %+v, err %v", res.Total, res.InterestDropped, err)
	}
	for _, p := range res.Players {
		if p.Interest != nil {
			t.Errorf("uid %d carries interest without the option", p.UID)
		}
	}
	// A loan keeps the just-joined player: only a transfer rules him out.
	if res, _ = g.View(false).Search(Filter{Interest: "loan"}); res.Total != 3 || res.InterestDropped == nil || res.InterestDropped.Total != 0 || res.InterestDropped.ByReason == nil {
		t.Errorf("loan: total %d, dropped %+v, want 3 and empty counts", res.Total, res.InterestDropped)
	}
}

func TestSearchInterestSkipsDroppedPlayersFromScouting(t *testing.T) {
	g := searchInterestGame(t)
	for _, p := range g.players[1:] {
		p.knowledge = 100
	}
	g.players[0].knowledge = 0 // the just-joined player is unscouted
	res, err := g.View(true).Search(Filter{Interest: "transfer", MinCurrentAbility: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsScouting != 0 || res.InterestDropped.Total != 0 {
		t.Errorf("needs_scouting %d, dropped %+v, want the unjudged dropped player in neither", res.NeedsScouting, res.InterestDropped)
	}
}

func TestSearchInterestRejectsBadInput(t *testing.T) {
	g := searchInterestGame(t)
	if _, err := g.View(false).Search(Filter{Interest: "bogus"}); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "transfer_or_loan") {
		t.Errorf("bogus: err = %v, want ErrInvalidFilter naming the values", err)
	}
	g.Info.ManagerTeamIndex = 424242
	if _, err := g.View(false).Search(Filter{Interest: "transfer"}); !errors.Is(err, ErrInvalidFilter) || !strings.Contains(err.Error(), "manager's team is unknown") {
		t.Errorf("unknown club: err = %v", err)
	}
}

// differentReasonsGame has player 1 out both ways: the transfer for
// only_just_joined, the loan for club_reputation_gap. Players 2 and 3 are
// not out.
func differentReasonsGame(t *testing.T) *Game {
	t.Helper()
	g := searchInterestGame(t)
	g.players[0].Reputation.World = 9000
	g.world.Teams[20].Reputation = 1000
	return g
}

func TestSearchInterestCountsTheReasonForTheKind(t *testing.T) {
	g := differentReasonsGame(t)
	in := g.interestOf(g.players[0])
	if in.Transfer != VerdictOut || in.Loan != VerdictOut || in.TransferReason == nil || in.LoanReason == nil ||
		in.TransferReason.Code != FactorOnlyJustJoined || in.LoanReason.Code != FactorClubReputationGap {
		t.Fatalf("interest = %+v reason %+v loan reason %+v, want out both ways for different reasons", in, in.TransferReason, in.LoanReason)
	}
	for kind, want := range map[string]string{"transfer": FactorOnlyJustJoined, "loan": FactorClubReputationGap, "transfer_or_loan": FactorOnlyJustJoined} {
		res, err := g.View(false).Search(Filter{Interest: kind})
		if err != nil {
			t.Fatal(err)
		}
		if res.InterestDropped.Total != 1 || res.InterestDropped.ByReason[want] != 1 {
			t.Errorf("%s: dropped = %+v, want 1 %s", kind, res.InterestDropped, want)
		}
	}
}

func TestSearchInterestTransferOrLoan(t *testing.T) {
	g := searchInterestGame(t)
	g.players[0].Reputation.World = 0
	res, err := g.View(false).Search(Filter{Interest: "transfer_or_loan"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || res.InterestDropped.Total != 0 {
		t.Fatalf("transfer_or_loan: total %d, dropped %+v, want the transfer-only out kept", res.Total, res.InterestDropped)
	}
	for _, p := range res.Players {
		if p.UID == 1 && (p.Interest == nil || p.Interest.Transfer != VerdictOut) {
			t.Errorf("uid 1 interest = %+v, want transfer out", p.Interest)
		}
	}
	g.world.Teams[20].Reputation = 1000
	g.players[0].Reputation.World = 9000
	res, _ = g.View(false).Search(Filter{Interest: "transfer_or_loan"})
	if res.Total != 2 || res.InterestDropped.Total != 1 {
		t.Errorf("out both ways: total %d, dropped %+v, want 2 and 1", res.Total, res.InterestDropped)
	}
}

func TestProfileInterest(t *testing.T) {
	g := searchInterestGame(t)
	for _, fog := range []bool{false, true} {
		p, ok := g.View(fog).Profile(1)
		if !ok {
			t.Fatal("no player 1")
		}
		if p.Interest == nil || p.Interest.Transfer != VerdictOut || p.Interest.TransferReason == nil || p.Interest.TransferReason.Code != FactorOnlyJustJoined {
			t.Errorf("fog %v: interest = %+v", fog, p.Interest)
		}
		if fog && p.Personality != "" {
			t.Errorf("fog on, unscouted: personality %q is shown", p.Personality)
		}
	}
	own := g.players[1]
	own.TeamIndex = 10
	g.aux[own.auxIdx].interest.main.TeamIndex = 10
	if p, _ := g.View(false).Profile(own.UID); p.Interest != nil {
		t.Errorf("own player carries interest %+v", p.Interest)
	}
}

func TestInterestGapTextFollowsTheLargerTerm(t *testing.T) {
	// Club term (36 000 - 28 872 stepped ×6) = -42 768; world term 0.
	p, in := interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	g := interestGame(p, in)
	g.world.Teams[20].Reputation = 9500
	if r := g.interestOf(p).TransferReason; r == nil || r.Text != "SM Caen is a bigger club than FC Metz" {
		t.Errorf("club term: reason %+v", r)
	}
	// Club term -2 872 ×4; world term (28 872 - 36 000) ×8 = -57 024.
	g.world.Teams[20].Reputation = 7936
	p.Reputation.World = 9500
	got := g.interestOf(p)
	if got.TransferReason == nil || got.TransferReason.Code != FactorClubReputationGap || got.TransferReason.Text != "His reputation is above FC Metz's" {
		t.Errorf("world term: reason %+v", got.TransferReason)
	}
	if got.LoanReason == nil || got.LoanReason.Text != "His reputation is above FC Metz's" {
		t.Errorf("world term: loan reason %+v", got.LoanReason)
	}
	p.Female = true
	g.world.Teams[10].Women = true
	if r := g.interestOf(p).TransferReason; r == nil || r.Text != "Her reputation is above FC Metz's" {
		t.Errorf("female player: reason %+v", r)
	}
}

func TestInterestKeptPlayerShowsTheJustJoinedPenalty(t *testing.T) {
	p, in := interestPlayer()
	in.loyalty = 1 // a transfer penalty of 9 800: not out
	got := interestGame(p, in).interestOf(p)
	want := InterestFactor{FactorOnlyJustJoined, "Joined SM Caen 153 days ago; won't consider a move yet"}
	if got.Transfer == VerdictOut || !slices.Contains(got.TransferFactors, want) {
		t.Errorf("transfer %s, factors %+v, want %+v", got.Transfer, got.TransferFactors, want)
	}
	p, in = interestPlayer()
	in.loyalty = 1
	in.joined, in.main.Signed = gameDaysAgo(interestToday, 400), gameDaysAgo(interestToday, 2)
	got = interestGame(p, in).interestOf(p)
	want = InterestFactor{FactorOnlyJustJoined, "Signed a new contract with SM Caen 2 days ago; won't consider a move yet"}
	if got.Transfer == VerdictOut || !slices.Contains(got.TransferFactors, want) {
		t.Errorf("just signed: transfer %s, factors %+v, want %+v", got.Transfer, got.TransferFactors, want)
	}
	p, in = interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	if got := interestGame(p, in).interestOf(p); hasCode(got, FactorOnlyJustJoined) {
		t.Errorf("settled player: factors %+v", got.TransferFactors)
	}
}

func TestInterestLoanFactors(t *testing.T) {
	// A loan penalty of 4 × 6 × 1 × 100 = 2 400 keeps the loan open.
	p, in := interestPlayer()
	in.loyalty = 1
	in.joined, in.main.Signed = gameDaysAgo(interestToday, 245), gameDaysAgo(interestToday, 245)
	g := interestGame(p, in)
	g.world.Teams[20].NationIndex = p.NationIndex
	got := g.interestOf(p)
	want := InterestFactor{FactorOnlyJustJoined, "Joined SM Caen 245 days ago; won't consider a move yet"}
	if got.Loan == VerdictOut || got.LoanReason != nil || !slices.Contains(got.LoanFactors, want) {
		t.Errorf("loan %s, reason %+v, factors %+v, want %+v", got.Loan, got.LoanReason, got.LoanFactors, want)
	}
	// A world reputation term of -328 keeps the loan open.
	p, in = interestPlayer()
	in.joined, in.main.Signed = unsetDate, unsetDate
	p.Reputation.World = 7300
	got = interestGame(p, in).interestOf(p)
	gap := InterestFactor{FactorClubReputationGap, "His reputation is above FC Metz's"}
	if got.Loan == VerdictOut || !slices.Equal(got.LoanFactors, []InterestFactor{gap}) {
		t.Errorf("loan %s, factors %+v, want only %+v", got.Loan, got.LoanFactors, gap)
	}
	// An out loan names the gap among its factors but not the penalty.
	p, in = interestPlayer()
	in.loyalty = 1
	p.Reputation.World = 7300
	g = interestGame(p, in)
	g.world.Teams[20].NationIndex = p.NationIndex
	got = g.interestOf(p)
	if got.Loan != VerdictOut || hasLoanCode(got, FactorOnlyJustJoined) || !hasLoanCode(got, FactorClubReputationGap) {
		t.Errorf("out loan %s, factors %+v", got.Loan, got.LoanFactors)
	}
}

func hasLoanCode(in *Interest, code string) bool { return slicesContainsCode(in.LoanFactors, code) }

func TestInterestJSONNames(t *testing.T) {
	f := &InterestFactor{"c", "t"}
	data, err := json.Marshal(Interest{Transfer: VerdictOut, Loan: VerdictOut, TransferReason: f, TransferFactors: []InterestFactor{*f}, LoanReason: f, LoanFactors: []InterestFactor{*f}})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(keys)); !slices.Equal(got, []string{"loan", "loan_factors", "loan_reason", "transfer", "transfer_factors", "transfer_reason"}) {
		t.Errorf("keys = %v", got)
	}
}

func TestInterestMoveToYourClub(t *testing.T) {
	for _, team := range []int32{10, 11} {
		p, in := interestPlayer()
		in.arranged = &arrangedMove{team: team, date: interestToday}
		g := interestGame(p, in)
		if got := g.interestOf(p); got != nil {
			t.Errorf("move to team %d of your club: interest %+v, want nil", team, got)
		}
		g.aux[0].interest.main = nil
		p.TeamIndex = noTeam
		if got := g.interestOf(p); got != nil {
			t.Errorf("free agent moving to team %d: interest %+v, want nil", team, got)
		}
	}
}

func TestSearchInterestRowsCarryFullInterest(t *testing.T) {
	g := searchInterestGame(t)
	g.players[1].Reputation.World = 7300 // a gap factor on a kept row
	res, err := g.View(false).Search(Filter{Interest: "transfer_or_loan"})
	if err != nil {
		t.Fatal(err)
	}
	texts := 0
	for _, row := range res.Players {
		want := g.interestOf(g.byUID[row.UID])
		if !reflect.DeepEqual(row.Interest, want) {
			t.Errorf("uid %d: row interest %+v, want %+v", row.UID, row.Interest, want)
		}
		if row.Interest != nil && (row.Interest.TransferReason != nil || len(row.Interest.TransferFactors) > 0 || len(row.Interest.LoanFactors) > 0) {
			texts++
		}
	}
	if texts < 2 {
		t.Errorf("%d rows with reasons or factors, want at least 2", texts)
	}
}

func TestInterestWarning(t *testing.T) {
	w := interestWorld(metzReputation, true)
	if got := interestWarning(w, 10); got != "" {
		t.Errorf("known club: warning %q", got)
	}
	want := "interest unavailable: your club's first team is not known"
	if got := interestWarning(w, noTeam); got != want {
		t.Errorf("unknown team: warning %q, want %q", got, want)
	}
	w.Teams[10].Reputation = 0
	if got := interestWarning(w, 10); got != want {
		t.Errorf("first-team reputation 0: warning %q, want %q", got, want)
	}
}

func TestLoadInterestWarning(t *testing.T) {
	g := loadTestSave(t)
	for _, w := range g.Info.Warnings {
		if strings.HasPrefix(w, "interest unavailable") {
			t.Errorf("warning %q with a known club", w)
		}
	}
	if interestWarning(g.world, g.Info.ManagerTeamIndex) != "" || g.targetReputation() == 0 {
		t.Errorf("Tottenham: reputation %d", g.targetReputation())
	}
}
