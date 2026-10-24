package fm

import (
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"
)

// per90Prefix starts a per-90 sort key, e.g. "per90_progressive_passes".
const per90Prefix = "per90_"

// per90DefaultMinutes is the club minutes a per-90 sort or filter needs unless
// Filter.MinSeasonMinutes is set; fewer make the rate noisy.
const per90DefaultMinutes = 450

// per90Fields maps SeasonDetail count names to field indexes, minus minutes,
// appearances and rating.
var per90Fields = func() map[string][]int {
	skip := map[string]bool{"minutes": true, "starts": true, "sub_appearances": true, "average_rating": true}
	out := map[string][]int{}
	for _, f := range reflect.VisibleFields(reflect.TypeFor[SeasonDetail]()) {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if k := f.Type.Kind(); (k == reflect.Int || k == reflect.Float64) && !skip[name] {
			out[name] = f.Index
		}
	}
	return out
}()

func per90Field(name string) (string, error) {
	n := strings.ToLower(name)
	if _, ok := per90Fields[n]; !ok {
		return "", fmt.Errorf("%w: per-90 stat %q; valid names: %s", ErrInvalidFilter, name, strings.Join(slices.Sorted(maps.Keys(per90Fields)), ", "))
	}
	return n, nil
}

// clubDetail is p's club SeasonDetail; false without club matches.
func (g *Game) clubDetail(p *Player) (SeasonDetail, bool) {
	c := g.auxOf(p).season.Club
	if c == nil {
		return SeasonDetail{}, false
	}
	return seasonDetail(CompetitionClub, *c, p.hasKeeperStats()), true
}

// per90 is d's field per 90 minutes; false without minutes or for a stat of
// the other side.
func per90(d SeasonDetail, field string) (float64, bool) {
	if keeper, ok := sideStats[field]; d.Minutes == 0 || ok && keeper != d.Goalkeeper {
		return 0, false
	}
	v := reflect.ValueOf(d).FieldByIndex(per90Fields[field])
	n := float64(0)
	if v.CanInt() {
		n = float64(v.Int())
	} else {
		n = v.Float()
	}
	return n * 90 / float64(d.Minutes), true
}

// per90Key sorts by the club per-90 rate of field, in thousandths. Fog of war
// does not hide it; players without minutes or of the other side come last.
func (g *Game) per90Key(field string) sortKeyFunc {
	return func(p *Player, _ sight) (int, bool) {
		d, ok := g.clubDetail(p)
		if !ok {
			return 0, false
		}
		v, ok := per90(d, field)
		return int(math.Round(v * 1000)), ok
	}
}

// meetsPer90 reports whether p reaches every per-90 minimum.
func (g *Game) meetsPer90(p *Player, minimums map[string]float64) bool {
	d, _ := g.clubDetail(p)
	for field, minimum := range minimums {
		if v, ok := per90(d, field); !ok || v < minimum {
			return false
		}
	}
	return true
}

// per90Row is p's per-90 rates of fields to two decimals; nil without fields
// or club minutes.
func (g *Game) per90Row(p *Player, fields []string) map[string]float64 {
	if len(fields) == 0 {
		return nil
	}
	d, ok := g.clubDetail(p)
	if !ok {
		return nil
	}
	out := make(map[string]float64, len(fields))
	for _, f := range fields {
		if v, ok := per90(d, f); ok {
			out[f] = math.Round(v*100) / 100
		}
	}
	return out
}
