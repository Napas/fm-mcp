// Package fmprefs reads the FM26 preferences that decide how the game shows
// money: the selected currency and the wage period. It only reads files.
package fmprefs

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrInvalid means a preference file exists but holds no usable value.
var ErrInvalid = errors.New("fmprefs: invalid preference")

const (
	currencyFile   = "selected_currency.xml"
	wagePeriodFile = "wage_string_type.xml"
	languageFile   = "current_language.xml"

	// maxFileSize caps a preference file read; real ones are under 100 bytes.
	maxFileSize = 4096
)

// currencyUIDs maps the selected_currency index to a currency uid in the
// save's CURRENCY_RECORD table (game_plugin 26.3.2, table at 0x524fba4).
var currencyUIDs = [...]uint32{
	86, 4, 5, 11, 13, 20, 23, 28, 30, 31, 32, 56, 33, 41, 43, 45, 58, 63, 64, 67, 74, 77,
	78, 84, 87, 73, 6, 95, 16, 18, 61, 94, 14, 17, 22, 34, 35, 38, 37, 36, 42, 51, 54, 66,
	72, 69, 70, 71, 80, 91, 2, 92, 93, 71006962, 15, 62, 57, 79, 68, 85, 19, 96, 48, 65, 89, 26,
}

// languageCurrency is the currency index the game picks for a UI language
// when selected_currency.xml is missing. Other languages get 0 (GBP).
var languageCurrency = map[string]int{
	"german": 7, "french": 7, "italian": 7, "spanish": 7, "dutch": 7, "greek": 7, "portuguese": 7,
	"english (us)": 24, "spanish (latam)": 24,
	"portuguese (bra)": 4, "danish": 6, "japanese": 15, "norwegian": 16, "polish": 17,
	"russian": 19, "swedish": 21, "turkish": 23, "korean": 25,
	"chinese": 33, "chinese (traditional)": 33, "indonesian": 37, "thai": 48, "arabic": 58,
}

// Period is the wage period the game shows (wage_string_type.xml).
type Period uint8

const (
	PerWeek  Period = 1
	PerMonth Period = 2 // inferred from the order of the in-game options
	PerYear  Period = 3
)

// String returns the game's suffix: "p/w", "p/m" or "p/a".
func (p Period) String() string {
	switch p {
	case PerMonth:
		return "p/m"
	case PerYear:
		return "p/a"
	}
	return "p/w"
}

// Prefs are the money display preferences.
type Prefs struct {
	// CurrencyUID is a uid in the save's CURRENCY_RECORD table. Uids 79
	// and 96 are selectable but have no record in the saves seen.
	CurrencyUID uint32
	WagePeriod  Period
}

// Load reads the preferences in dir. A missing file gives the game's
// default: the UI language's currency (else GBP) and wages per week.
// Prefs is always usable. err reports an unreadable dir (everything is then
// GBP per week) or files that exist but cannot be used (ErrInvalid or an OS
// error); their settings use the defaults.
func Load(dir string) (Prefs, error) {
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		err = fmt.Errorf("preferences folder %s is not a folder", dir)
	}
	if err != nil {
		return Prefs{CurrencyUID: currencyUIDs[0], WagePeriod: PerWeek}, err
	}
	currency, currencyErr := loadCurrency(dir)
	period, periodErr := loadWagePeriod(dir)
	return Prefs{CurrencyUID: currency, WagePeriod: period}, errors.Join(currencyErr, periodErr)
}

func loadCurrency(dir string) (uint32, error) {
	index, found, err := readInt(dir, currencyFile)
	if found {
		if index >= 0 && index < len(currencyUIDs) {
			return currencyUIDs[index], nil
		}
		err = fmt.Errorf("%w: %s: currency %d", ErrInvalid, currencyFile, index)
	}
	lang, _, langErr := readValue(dir, languageFile, "symbol")
	return currencyUIDs[languageCurrency[lang]], errors.Join(err, langErr)
}

func loadWagePeriod(dir string) (Period, error) {
	v, found, err := readInt(dir, wagePeriodFile)
	if !found {
		return PerWeek, err
	}
	if v >= int(PerWeek) && v <= int(PerYear) {
		return Period(v), nil
	}
	return PerWeek, fmt.Errorf("%w: %s: wage period %d", ErrInvalid, wagePeriodFile, v)
}

func readInt(dir, name string) (v int, found bool, err error) {
	s, found, err := readValue(dir, name, "integer")
	if err != nil || !found {
		return 0, found, err
	}
	v, err = strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false, fmt.Errorf("%w: %s: %w", ErrInvalid, name, err)
	}
	return v, true, nil
}

// prefRecord is a preference file: <record><kind id="value" value="…"/></record>.
type prefRecord struct {
	Items []struct {
		XMLName xml.Name
		ID      string `xml:"id,attr"`
		Value   string `xml:"value,attr"`
	} `xml:",any"`
}

// readValue returns the value attribute of the <kind id="value"> element in
// the preference file dir/name. found is false when the file does not exist.
func readValue(dir, name, kind string) (value string, found bool, err error) {
	f, err := os.Open(filepath.Join(dir, name)) //nolint:gosec // reading the caller's preferences folder is the purpose
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer func() { _ = f.Close() }()

	b, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return "", false, err
	}
	if len(b) > maxFileSize {
		return "", false, fmt.Errorf("%w: %s is over %d bytes", ErrInvalid, name, maxFileSize)
	}
	var rec prefRecord
	if err := xml.Unmarshal(b, &rec); err != nil {
		return "", false, fmt.Errorf("%w: %s: %w", ErrInvalid, name, err)
	}
	for _, it := range rec.Items {
		if it.XMLName.Local == kind && it.ID == "value" {
			return it.Value, true, nil
		}
	}
	return "", false, fmt.Errorf("%w: %s has no %s value", ErrInvalid, name, kind)
}
