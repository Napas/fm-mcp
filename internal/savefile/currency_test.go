package savefile

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

type testCurrency struct {
	index, uid          uint32
	name, short, symbol string
	rate                float32
	history             []float32
}

func (c testCurrency) encode(b []byte) []byte {
	b = append(b, 0)
	b = appendString(b, c.name)
	b = appendString(b, c.short)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = appendString(b, c.symbol)
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(c.rate))
	b = binary.LittleEndian.AppendUint32(b, uint32(len(c.history))) //nolint:gosec // small test data
	for i, h := range c.history {
		b = append(b, 1)
		b = binary.LittleEndian.AppendUint16(b, uint16(2020+i)) //nolint:gosec // small test data
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(h))
	}
	return dbRecord(b, c.index, c.uid)
}

func TestReadCurrencyTable(t *testing.T) {
	var recs []byte
	recs = testCurrency{index: 0, uid: 28, name: "European Euro", short: "Euro", symbol: "€", rate: 1.15713, history: []float32{1.1, 1.2}}.encode(recs)
	recs = testCurrency{index: 1, uid: 86, name: "British Pound", short: "Pound", symbol: "£", rate: 1}.encode(recs)
	recs = testCurrency{index: 2, uid: 99, name: "Nowhere Dollar", short: "Dollar", rate: 3.5}.encode(recs)
	body := append([]byte{1, 2, 3}, tableBytes(3, recs)...)

	cur, end, err := readCurrencyTable(body, 3)
	if err != nil {
		t.Fatalf("readCurrencyTable: %v", err)
	}

	if len(cur) != 3 || end != len(body)-tableEndSize {
		t.Fatalf("got %d currencies ending at %d, want 3 ending at %d", len(cur), end, len(body)-tableEndSize)
	}
	want := Currency{Index: 0, UID: 28, Name: "European Euro", ShortName: "Euro", Symbol: "€", PerGBP: 1.15713}
	if cur[0] != want {
		t.Errorf("euro = %+v, want %+v", cur[0], want)
	}
	if cur[2].Symbol != "" || cur[2].PerGBP != 3.5 || cur[2].UID != 99 {
		t.Errorf("symbol-less currency = %+v", cur[2])
	}
}

func TestReadCurrencyTableReportsMissingTable(t *testing.T) {
	_, _, err := readCurrencyTable([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 3)

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReadCurrencyTableRejectsWrongRecordTag(t *testing.T) {
	recs := testCurrency{index: 0, uid: 28, name: "Euro", rate: 1}.encode(nil)
	second := len(recs)
	recs = testCurrency{index: 1, uid: 86, name: "Pound", rate: 1}.encode(recs)
	recs[second] = 7

	if _, _, err := readCurrencyTable(tableBytes(2, recs), 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCurrencyByUID(t *testing.T) {
	db := &GameDB{Currencies: []Currency{{Index: 0, UID: 2}, {Index: 1, UID: 86, PerGBP: 1}}}

	if c, ok := db.CurrencyByUID(86); !ok || c.Index != 1 {
		t.Errorf("CurrencyByUID(86) = %+v, %v", c, ok)
	}
	if _, ok := db.CurrencyByUID(79); ok {
		t.Error("CurrencyByUID(79) found a currency")
	}
}

func TestReadCurrencyTableRejectsUnusableRates(t *testing.T) {
	for _, rate := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		recs := testCurrency{index: 0, uid: 28, name: "Euro", rate: rate}.encode(nil)

		if _, _, err := readCurrencyTable(tableBytes(1, recs), 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("rate %v: err = %v, want ErrNotFound", rate, err)
		}
	}
}

func TestReadCurrenciesUsesPoolCount(t *testing.T) {
	recs := testCurrency{index: 0, uid: 86, name: "Pound", symbol: "£", rate: 1}.encode(nil)
	body := tableBytes(1, recs)

	if cur, err := readCurrencies(body, map[string]uint32{"CURRENCY_RECORD": 1}); err != nil || len(cur) != 1 || cur[0].UID != 86 {
		t.Errorf("readCurrencies = %+v, %v", cur, err)
	}
	if _, err := readCurrencies(body, map[string]uint32{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("no pool: err = %v, want ErrNotFound", err)
	}
}
