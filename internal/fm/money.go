package fm

import (
	"cmp"
	"fmt"
	"math"

	"github.com/Napas/fm-mcp/internal/fmprefs"
	"github.com/Napas/fm-mcp/internal/savefile"
)

// Currency names the currency money is shown in.
type Currency struct {
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	Symbol    string `json:"symbol"` // the short name when the game has no symbol
}

// gbp is the save's own currency (uid 86): every amount in a save is GBP.
var gbp = savefile.Currency{UID: 86, Name: "United Kingdom Pound", ShortName: "Pound", Symbol: "£", PerGBP: 1}

// Money converts the save's GBP amounts and weekly wages to the user's
// currency and wage period. The zero value is GBP per week.
type Money struct {
	cur    savefile.Currency
	period fmprefs.Period
}

func NewMoney(cur savefile.Currency, period fmprefs.Period) Money {
	return Money{cur: cur, period: period}
}

func (m Money) currency() savefile.Currency {
	if m.cur.PerGBP == 0 {
		return gbp
	}
	return m.cur
}

func (m Money) Currency() Currency {
	c := m.currency()
	return Currency{Name: c.Name, ShortName: c.ShortName, Symbol: cmp.Or(c.Symbol, c.ShortName)}
}

// WagePeriod is the game's label for the wage period: "p/w", "p/m" or "p/a".
func (m Money) WagePeriod() string { return m.period.String() }

func (m Money) Amount(amount int64) int64 { return fromGBP(m.currency(), float64(amount)) }

// Wage converts a weekly GBP wage to the user's currency and period,
// rounding once as the game does.
func (m Money) Wage(weeklyGBP int64) int64 {
	return fromGBP(m.currency(), float64(weeklyGBP)*weeksIn(m.period))
}

// weeksIn is the number of weeks in a wage period: 52 a year (verified), a
// twelfth of that a month (inferred).
func weeksIn(p fmprefs.Period) float64 {
	switch p {
	case fmprefs.PerMonth:
		return 52 / 12.0
	case fmprefs.PerYear:
		return 52
	}
	return 1
}

// fromGBP converts a GBP amount to c as the game does (0x43de5b0):
// round(amount × rate). amount is a float so a wage is scaled to its period
// before the single rounding. Results beyond int64 saturate; amount must
// not be NaN.
func fromGBP(c savefile.Currency, amount float64) int64 {
	v := math.Round(amount * float64(c.PerGBP))
	switch {
	case v >= math.MaxInt64: // float64(MaxInt64) is 2^63, one past the range
		return math.MaxInt64
	case v < math.MinInt64:
		return math.MinInt64
	}
	return int64(v)
}

// readMoney reads the currency and wage period from the preferences folder
// dir. An empty dir, or a problem (returned as a warning), gives GBP or per week.
func readMoney(db *savefile.GameDB, dir string) (Money, []string) {
	if dir == "" {
		return Money{}, nil
	}
	var warnings []string
	prefs, err := fmprefs.Load(dir)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("display preferences unreadable, so the unreadable settings use defaults: %v", err))
	}
	m := Money{period: prefs.WagePeriod}
	switch cur, ok := db.CurrencyByUID(prefs.CurrencyUID); {
	case ok:
		m.cur = cur
	case prefs.CurrencyUID == gbp.UID: // the save's own currency needs no rate
	case db.CurrencyErr != nil:
		warnings = append(warnings, fmt.Sprintf("currency rates unavailable, so money is in GBP: %v", db.CurrencyErr))
	default:
		warnings = append(warnings, fmt.Sprintf("the save has no rate for your currency (uid %d), so money is in GBP", prefs.CurrencyUID))
	}
	return m, warnings
}
