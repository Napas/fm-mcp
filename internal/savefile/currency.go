package savefile

import (
	"fmt"
	"math"
)

const (
	tagCurrency = 0
	// currencyName names both the record type and its memory pool.
	currencyName = "CURRENCY_RECORD"
	// currencyHistoryItemSize is one yearly rate: u8 1, u16 year, f32 rate.
	currencyHistoryItemSize = 1 + 2 + 4
)

// Currency is a CURRENCY_RECORD (layout in docs/format.md).
type Currency struct {
	Index     uint32
	UID       uint32 // 86 = GBP, 28 = EUR, 87 = USD
	Name      string
	ShortName string
	Symbol    string  // "€"; may be empty
	PerGBP    float32 // units per 1 GBP; finite and > 0
}

// readCurrencyTable returns the count CURRENCY_RECORDs and the offset of the
// table's end marker.
func readCurrencyTable(body []byte, count uint32) ([]Currency, int, error) {
	return readTable(body, count, []byte{tagCurrency}, currencyName, readCurrency)
}

func readCurrency(r *reader) (Currency, uint32) {
	var c Currency
	if tag := r.u8(); tag != tagCurrency {
		r.fail("currency tag %d", tag)
	}
	c.Name = r.str()
	c.ShortName = r.str()
	r.u32()
	c.Symbol = r.str()
	c.PerGBP = r.f32()
	if !(c.PerGBP > 0) || math.IsInf(float64(c.PerGBP), 0) {
		r.fail("currency rate %v", c.PerGBP)
	}
	r.take(currencyHistoryItemSize * int(r.u32()))
	c.Index, c.UID = readDatabaseRecord(r)
	return c, c.Index
}

// readCurrencies reads the CURRENCY_RECORD table; the memory pools give its size.
func readCurrencies(body []byte, pools map[string]uint32) ([]Currency, error) {
	count := pools[currencyName]
	if count == 0 {
		return nil, fmt.Errorf("%w: no %s memory pool", ErrNotFound, currencyName)
	}
	cur, _, err := readCurrencyTable(body, count)
	return cur, err
}

// CurrencyByUID returns the currency whose uid is uid.
func (db *GameDB) CurrencyByUID(uid uint32) (Currency, bool) {
	for _, c := range db.Currencies {
		if c.UID == uid {
			return c, true
		}
	}
	return Currency{}, false
}
