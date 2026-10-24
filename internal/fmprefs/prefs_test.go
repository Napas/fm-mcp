package fmprefs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	uidGBP = 86
	uidEUR = 28
	uidUSD = 87
)

func intPref(v string) string {
	return "<record>\n\t<integer id=\"value\" value=\"" + v + "\"/>\n</record>\n"
}

func symbolPref(v string) string {
	return "<record>\n\t<symbol id=\"value\" value=\"" + v + "\"/>\n</record>\n"
}

func prefsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadReadsCurrencyAndWagePeriod(t *testing.T) {
	dir := prefsDir(t, map[string]string{
		currencyFile:   intPref("7"),
		wagePeriodFile: intPref("3"),
		languageFile:   symbolPref("english"),
	})

	p, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p != (Prefs{CurrencyUID: uidEUR, WagePeriod: PerYear}) {
		t.Errorf("Load = %+v, want euro per year", p)
	}
}

func TestLoadMapsCurrencyIndex(t *testing.T) {
	tests := map[string]uint32{"0": uidGBP, "7": uidEUR, "24": uidUSD, "53": 71006962, "65": 26}
	for v, want := range tests {
		p, err := Load(prefsDir(t, map[string]string{currencyFile: intPref(v)}))
		if err != nil || p.CurrencyUID != want {
			t.Errorf("selected_currency %s: uid %d, err %v; want %d", v, p.CurrencyUID, err, want)
		}
	}
	if len(currencyUIDs) != 66 {
		t.Errorf("%d currency indices, want 66 (0..65)", len(currencyUIDs))
	}
}

func TestLoadDefaultsWithoutFiles(t *testing.T) {
	p, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p != (Prefs{CurrencyUID: uidGBP, WagePeriod: PerWeek}) {
		t.Errorf("Load = %+v, want GBP per week", p)
	}
}

func TestLoadReportsAMissingFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, currencyFile)
	if err := os.WriteFile(file, []byte(intPref("7")), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"missing": filepath.Join(dir, "missing"), "a file": file} {
		p, err := Load(path)
		if err == nil {
			t.Errorf("%s: no error", name)
		}
		if p != (Prefs{CurrencyUID: uidGBP, WagePeriod: PerWeek}) {
			t.Errorf("%s: Load = %+v, want GBP per week", name, p)
		}
	}
}

func TestLoadDefaultsCurrencyByLanguage(t *testing.T) {
	tests := map[string]uint32{
		"english":               uidGBP,
		"german":                uidEUR,
		"english (us)":          uidUSD,
		"spanish (latam)":       uidUSD,
		"portuguese (bra)":      13,
		"danish":                23,
		"chinese (traditional)": 17,
		"klingon":               uidGBP,
	}
	for lang, want := range tests {
		p, err := Load(prefsDir(t, map[string]string{languageFile: symbolPref(lang)}))
		if err != nil || p.CurrencyUID != want {
			t.Errorf("language %q: uid %d, err %v; want %d", lang, p.CurrencyUID, err, want)
		}
	}
}

func TestLoadReportsBadValuesAndUsesDefaults(t *testing.T) {
	tests := map[string]map[string]string{
		"currency out of range": {currencyFile: intPref("66")},
		"negative currency":     {currencyFile: intPref("-1")},
		"currency not a number": {currencyFile: intPref("euro")},
		"not XML":               {currencyFile: "garbage"},
		"no value element":      {currencyFile: "<record></record>"},
		"unknown wage period":   {wagePeriodFile: intPref("4")},
		"zero wage period":      {wagePeriodFile: intPref("0")},
		"file too large":        {currencyFile: intPref("7") + strings.Repeat(" ", maxFileSize)},
	}
	for name, files := range tests {
		files[languageFile] = symbolPref("german")
		p, err := Load(prefsDir(t, files))

		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if p != (Prefs{CurrencyUID: uidEUR, WagePeriod: PerWeek}) {
			t.Errorf("%s: Load = %+v, want the defaults (euro per week)", name, p)
		}
	}
}

func TestLoadReportsBadLanguageAndUsesGBP(t *testing.T) {
	p, err := Load(prefsDir(t, map[string]string{languageFile: "<record>"}))

	if !errors.Is(err, ErrInvalid) || p.CurrencyUID != uidGBP {
		t.Errorf("Load = %+v, %v; want GBP and ErrInvalid", p, err)
	}
}

func TestPeriodLabel(t *testing.T) {
	tests := []struct {
		p     Period
		label string
	}{
		{PerWeek, "p/w"},
		{PerMonth, "p/m"},
		{PerYear, "p/a"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.label {
			t.Errorf("%d.String() = %q, want %q", tt.p, got, tt.label)
		}
	}
}
