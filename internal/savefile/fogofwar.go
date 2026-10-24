package savefile

import "fmt"

const (
	fogItemSize     = 5    // u8 knowledge + u32 person index
	maxFogHumans    = 1000 // sanity cap; saves seen hold one human
	maxFogPrealloc  = 1 << 16
	maxFogKnowledge = 100
)

// FogOfWar is fog_of_war.dat: each human manager's stored scouting knowledge.
type FogOfWar struct {
	Humans []HumanKnowledge
}

// HumanKnowledge is one human manager's stored knowledge.
//
// The game uses the larger of the stored level and a run-time baseline (own
// club 100, reputation, nation knowledge…). A player missing here has only
// the baseline.
type HumanKnowledge struct {
	HumanIndex uint32           // person index of the human manager
	Players    map[uint32]uint8 // player person index → knowledge 0..100 (1..100 seen)
}

// For returns the knowledge of the human whose person index is human.
func (f FogOfWar) For(human uint32) (HumanKnowledge, bool) {
	for _, h := range f.Humans {
		if h.HumanIndex == human {
			return h, true
		}
	}
	return HumanKnowledge{}, false
}

// ReadFogOfWar decodes the save's fog_of_war.dat.
func ReadFogOfWar(a *Archive) (FogOfWar, error) {
	f, err := a.ReadFile("fog_of_war.dat")
	if err != nil {
		return FogOfWar{}, err
	}
	fog, err := parseFogOfWar(f.Body)
	if err != nil {
		return FogOfWar{}, fmt.Errorf("fog_of_war.dat: %w", err)
	}
	return fog, nil
}

// parseFogOfWar decodes the body of fog_of_war.dat (layout in docs/format.md).
func parseFogOfWar(body []byte) (FogOfWar, error) {
	r := newReader(body)
	r.u8()
	n := r.u32()
	if r.err != nil {
		return FogOfWar{}, r.err
	}
	if n > maxFogHumans {
		return FogOfWar{}, fmt.Errorf("%w: %d humans", ErrCorrupt, n)
	}
	fog := FogOfWar{Humans: make([]HumanKnowledge, 0, n)}
	for range n {
		h := HumanKnowledge{HumanIndex: r.u32()}
		items := r.u32()
		if r.err != nil {
			return FogOfWar{}, r.err
		}
		if _, dup := fog.For(h.HumanIndex); dup {
			return FogOfWar{}, fmt.Errorf("%w: human %d listed twice", ErrCorrupt, h.HumanIndex)
		}
		// The hint is capped too: a map entry costs more than its 5 file bytes.
		h.Players = make(map[uint32]uint8, min(capacity(items, r.remaining(), fogItemSize), maxFogPrealloc))
		for range items {
			level, person := r.u8(), r.u32()
			if r.err != nil {
				return FogOfWar{}, r.err
			}
			if level > maxFogKnowledge {
				return FogOfWar{}, fmt.Errorf("%w: knowledge %d of person %d", ErrCorrupt, level, person)
			}
			if _, dup := h.Players[person]; dup {
				return FogOfWar{}, fmt.Errorf("%w: person %d listed twice", ErrCorrupt, person)
			}
			h.Players[person] = level
		}
		fog.Humans = append(fog.Humans, h)
	}
	if r.remaining() != 0 {
		return FogOfWar{}, fmt.Errorf("%w: %d trailing bytes", ErrCorrupt, r.remaining())
	}
	return fog, nil
}
