package fm

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Napas/fm-mcp/internal/fmprefs"
	"github.com/Napas/fm-mcp/internal/savefile"
)

var (
	euro  = savefile.Currency{UID: 28, Name: "European Euro", ShortName: "Euro", Symbol: "€", PerGBP: 1.1571305}
	krone = savefile.Currency{UID: 16, Name: "Norwegian Krone", ShortName: "Krone", PerGBP: 13.5}
)

func TestZeroMoneyIsPoundsPerWeek(t *testing.T) {
	var m Money

	if m.Amount(1234) != 1234 || m.Wage(1234) != 1234 || m.WagePeriod() != "p/w" {
		t.Errorf("Amount %d, Wage %d, period %q", m.Amount(1234), m.Wage(1234), m.WagePeriod())
	}
	if c := m.Currency(); c != (Currency{Name: "United Kingdom Pound", ShortName: "Pound", Symbol: "£"}) {
		t.Errorf("Currency = %+v", c)
	}
}

func TestMoneyConvertsToTheUsersCurrencyAndPeriod(t *testing.T) {
	m := NewMoney(euro, fmprefs.PerYear)

	// Checked in game: Ashby's £15 315 a week shows as €922K p/a, Metz's
	// £29 636 499 balance as €34 293 296.
	if got := m.Wage(15_315); got != 921_516 {
		t.Errorf("Wage = %d, want 921516", got)
	}
	if got := m.Amount(29_636_499); got != 34_293_296 {
		t.Errorf("Amount = %d, want 34293296", got)
	}
	if m.WagePeriod() != "p/a" || m.Currency().Symbol != "€" {
		t.Errorf("period %q currency %+v", m.WagePeriod(), m.Currency())
	}
	if got := NewMoney(euro, fmprefs.PerMonth).Wage(100_000); got != 501_423 {
		t.Errorf("monthly Wage = %d, want 501423", got)
	}
}

func TestFromGBPRoundsLikeTheGame(t *testing.T) {
	euro := savefile.Currency{PerGBP: math.Float32frombits(0x3f941cda)} // the save's euro rate, 1.1571305

	// Ashby: £15,315 a week for a year shows as "€922K p/a" in game.
	if got := fromGBP(euro, 15315*52); got != 921516 {
		t.Errorf("fromGBP = %d, want 921516", got)
	}
	// A monthly wage is rounded once, after the rate: 1001 × 52 / 12 × rate = 5019.4.
	if got := fromGBP(euro, 1001*52.0/12); got != 5019 {
		t.Errorf("monthly fromGBP = %d, want 5019", got)
	}
	if got := fromGBP(savefile.Currency{PerGBP: 1}, -147000); got != -147000 {
		t.Errorf("GBP fromGBP = %d, want -147000", got)
	}
	huge := savefile.Currency{PerGBP: math.MaxFloat32}
	if got := fromGBP(huge, 1e30); got != math.MaxInt64 {
		t.Errorf("overflowing fromGBP = %d, want MaxInt64", got)
	}
	if got := fromGBP(huge, -1e30); got != math.MinInt64 {
		t.Errorf("underflowing fromGBP = %d, want MinInt64", got)
	}
}

func TestMoneySymbolFallsBackToShortName(t *testing.T) {
	if got := NewMoney(krone, fmprefs.PerWeek).Currency().Symbol; got != "Krone" {
		t.Errorf("Symbol = %q, want Krone", got)
	}
}

func writePrefs(t *testing.T, currency, period string) string {
	t.Helper()
	dir := t.TempDir()
	for name, v := range map[string]string{"selected_currency.xml": currency, "wage_string_type.xml": period} {
		if v == "" {
			continue
		}
		body := `<record><integer id="value" value="` + v + `"/></record>`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadMoney(t *testing.T) {
	db := &savefile.GameDB{Currencies: []savefile.Currency{gbp, euro}}
	tests := []struct {
		name     string
		db       *savefile.GameDB
		dir      string
		want     savefile.Currency
		period   string
		warnings int
	}{
		{"no preferences folder", db, "", gbp, "p/w", 0},
		{"euro per year", db, writePrefs(t, "7", "3"), euro, "p/a", 0},
		{"currency without a record", db, writePrefs(t, "57", "1"), gbp, "p/w", 1}, // index 57 = uid 79
		{"broken preference", db, writePrefs(t, "x", "2"), gbp, "p/m", 1},
		{"no currency table", &savefile.GameDB{CurrencyErr: errors.New("boom")}, writePrefs(t, "7", "3"), gbp, "p/a", 1},
		{"pounds need no currency table", &savefile.GameDB{CurrencyErr: errors.New("boom")}, writePrefs(t, "0", "3"), gbp, "p/a", 0},
		{"missing preferences folder", db, filepath.Join(t.TempDir(), "missing"), gbp, "p/w", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, warnings := readMoney(tt.db, tt.dir)

			if m.Amount(1_000_000) != fromGBP(tt.want, 1_000_000) || m.Currency().ShortName != tt.want.ShortName || m.WagePeriod() != tt.period {
				t.Errorf("money = %+v %s", m.Currency(), m.WagePeriod())
			}
			if len(warnings) != tt.warnings {
				t.Errorf("warnings = %q, want %d", warnings, tt.warnings)
			}
		})
	}
}

// euroGame is fogGame shown in euros per year.
func euroGame() *Game {
	g := fogGame()
	g.Info.Money = NewMoney(euro, fmprefs.PerYear)
	g.byUID[1].Loan = &Contract{TeamIndex: 900, WeeklyWage: 1000}
	g.byUID[1].LoyaltyBonusRemaining = 308_066
	return g
}

func TestProfileMoneyIsInTheUsersCurrency(t *testing.T) {
	p, _ := euroGame().View(false).Profile(1) // value £12M, wage £12 000 p/w, clause £1M

	if *p.Value != 13_885_566 {
		t.Errorf("value = %d, want 13885566", *p.Value)
	}
	if *p.Contract.Wage != 722_049 {
		t.Errorf("wage = %d, want 722049 (12 000 × 52 × rate)", *p.Contract.Wage)
	}
	if p.Contract.Clauses[0].Amount != 1_157_130 {
		t.Errorf("clause = %+v, want 1157130", p.Contract.Clauses[0])
	}
	if *p.LoyaltyBonusRemaining != 356_473 {
		t.Errorf("loyalty bonus = %d, want 356473 (an amount, not a wage)", *p.LoyaltyBonusRemaining)
	}
	if *p.Loan.Wage != 60_171 {
		t.Errorf("loan wage = %d, want 60171", *p.Loan.Wage)
	}
}

func TestTeamsShowBankBalanceInTheUsersCurrency(t *testing.T) {
	g := NewGame(Info{Date: gameDate, Money: NewMoney(euro, fmprefs.PerYear)}, nil, World{Teams: map[int64]*Team{
		652: {Index: 652, Name: "FC Metz", BankBalance: 29_636_499},
	}}, PlayerExtras{})
	v := g.View(false)

	if team, _ := v.Team(652); team.BankBalance != 34_293_296 {
		t.Errorf("Team balance = %d", team.BankBalance)
	}
	if teams := findTeams(t, v, "metz", 1); teams[0].BankBalance != 34_293_296 {
		t.Errorf("FindTeams balance = %d", teams[0].BankBalance)
	}
	if team, _ := v.Team(652); team.BankBalance != 34_293_296 {
		t.Errorf("a second Team call converted twice: %d", team.BankBalance)
	}
}

func TestBudgetsAreTheManagersClubsInTheUsersCurrencyAndPeriod(t *testing.T) {
	g := NewGame(Info{Date: gameDate, ManagerTeamIndex: 652, Money: NewMoney(euro, fmprefs.PerYear)}, nil, World{Teams: map[int64]*Team{
		652: {Index: 652, Name: "FC Metz", Budgets: &Budgets{TransferBudget: 8_329_074, WageBudget: 1_069_817}},
		900: {Index: 900, Name: "Leeds United"},
	}}, PlayerExtras{})

	for _, fog := range []bool{true, false} {
		b := g.View(fog).Budgets()

		if want := (Budgets{TransferBudget: 9_637_825, WageBudget: 64_371_729}); b == nil || *b != want {
			t.Errorf("fog %v: budgets = %+v, want %+v", fog, b, want)
		}
	}
	if b := g.View(false).Budgets(); b == g.world.Teams[652].Budgets || g.world.Teams[652].Budgets.TransferBudget != 8_329_074 {
		t.Error("Budgets converted the game's own budgets")
	}
	for _, fog := range []bool{true, false} {
		v := g.View(fog)
		if team, _ := v.Team(652); team.Budgets != nil {
			t.Errorf("fog %v: Team(652) carries GBP budgets %+v", fog, team.Budgets)
		}
		if teams := findTeams(t, v, "metz", 1); teams[0].Budgets != nil {
			t.Errorf("fog %v: FindTeams(metz) carries GBP budgets %+v", fog, teams[0].Budgets)
		}
	}
	g.Info.ManagerTeamIndex = -1
	if b := g.View(false).Budgets(); b != nil {
		t.Errorf("unknown manager's club: budgets = %+v, want nil", b)
	}
	g.Info.ManagerTeamIndex = 900
	if b := g.View(false).Budgets(); b != nil {
		t.Errorf("club without finances: budgets = %+v, want nil", b)
	}
}

func TestMoneyFiltersUseTheUsersCurrencyAndPeriod(t *testing.T) {
	v := euroGame().View(false)
	tests := []struct {
		name string
		f    Filter
		want []uint32
	}{
		// Values £9M..£16M are €10.4M..€18.5M; €15M keeps up to £12M.
		{"max value", Filter{MaxValue: 15_000_000}, []uint32{1, 8, 3, 2}},
		// Wages £9 000..£16 000 a week are €541K..€963K a year; €700K keeps up to £11 000. 8 has no contract, so no wage.
		{"max wage", Filter{MaxWage: 700_000}, []uint32{3, 2}},
		{"sort by wage", Filter{SortBy: "wage", Limit: 2}, []uint32{4, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Search(tt.f)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Players
			if !slices.Equal(uids(got), tt.want) {
				t.Errorf("got %v, want %v", uids(got), tt.want)
			}
		})
	}
	if _, err := v.Search(Filter{SortBy: "weekly_wage"}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("sort by weekly_wage: err = %v", err)
	}
}

func TestLoadShowsMoneyFromPreferences(t *testing.T) {
	if _, err := os.Stat(testSavePath); err != nil {
		t.Skip(err)
	}
	g, err := Load(testSavePath, LoadOptions{GameDBDir: gameDBDir(), PrefsDir: writePrefs(t, "7", "3")})
	if err != nil {
		t.Fatal(err)
	}

	if g.Info.Money.Currency().ShortName != "Euro" || g.Info.Money.WagePeriod() != "p/a" {
		t.Errorf("money = %+v %s", g.Info.Money.Currency(), g.Info.Money.WagePeriod())
	}
	m := findPlayers(t, g.View(false), "james maddison", 1)[0]
	raw := g.byUID[m.UID]
	if *m.Contract.Wage != g.Info.Money.Wage(raw.Contract.WeeklyWage) || *m.Contract.Wage <= raw.Contract.WeeklyWage*52 {
		t.Errorf("Maddison's wage = %d from £%d p/w", *m.Contract.Wage, raw.Contract.WeeklyWage)
	}
}
