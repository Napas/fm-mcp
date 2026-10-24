package savefile

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func fogBody(humans ...fogHuman) []byte {
	b := binary.LittleEndian.AppendUint32([]byte{1}, uint32(len(humans))) //nolint:gosec // test sizes
	for _, h := range humans {
		b = binary.LittleEndian.AppendUint32(b, h.index)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(h.items))) //nolint:gosec // test sizes
		for _, it := range h.items {
			b = append(b, it.level)
			b = binary.LittleEndian.AppendUint32(b, it.person)
		}
	}
	return b
}

type fogHuman struct {
	index uint32
	items []fogItem
}

type fogItem struct {
	level  uint8
	person uint32
}

func TestParseFogOfWar(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want FogOfWar
	}{
		{
			name: "one human",
			body: fogBody(fogHuman{88351, []fogItem{{100, 9480}, {60, 54212}}}),
			want: FogOfWar{Humans: []HumanKnowledge{{HumanIndex: 88351, Players: map[uint32]uint8{9480: 100, 54212: 60}}}},
		},
		{
			name: "two humans, one knowing nobody",
			body: fogBody(fogHuman{7, []fogItem{{85, 65605}}}, fogHuman{8, nil}),
			want: FogOfWar{Humans: []HumanKnowledge{
				{HumanIndex: 7, Players: map[uint32]uint8{65605: 85}},
				{HumanIndex: 8, Players: map[uint32]uint8{}},
			}},
		},
		{
			name: "no humans",
			body: fogBody(),
			want: FogOfWar{Humans: []HumanKnowledge{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFogOfWar(tt.body)
			if err != nil {
				t.Fatalf("parseFogOfWar: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseFogOfWarRejectsBadInput(t *testing.T) {
	valid := fogBody(fogHuman{7, []fogItem{{85, 65605}}})
	hugeHumans := binary.LittleEndian.AppendUint32([]byte{1}, 0xffffffff)
	hugeItems := binary.LittleEndian.AppendUint32([]byte{1}, 1)
	hugeItems = binary.LittleEndian.AppendUint32(hugeItems, 7)
	hugeItems = binary.LittleEndian.AppendUint32(hugeItems, 0xffffffff)

	tests := []struct {
		name string
		body []byte
		want error
	}{
		{"truncated item", valid[:len(valid)-1], ErrTruncated},
		{"trailing bytes", append(valid, 0), ErrCorrupt},
		{"more humans than the cap", hugeHumans, ErrCorrupt},
		{"item count beyond the data", hugeItems, ErrTruncated},
		{"empty", nil, ErrTruncated},
		{"knowledge above 100", fogBody(fogHuman{7, []fogItem{{101, 65605}}}), ErrCorrupt},
		{"negative knowledge", fogBody(fogHuman{7, []fogItem{{0xff, 65605}}}), ErrCorrupt},
		{"player listed twice", fogBody(fogHuman{7, []fogItem{{85, 65605}, {20, 65605}}}), ErrCorrupt},
		{"human listed twice", fogBody(fogHuman{7, nil}, fogHuman{7, nil}), ErrCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseFogOfWar(tt.body); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFogOfWarFor(t *testing.T) {
	f := FogOfWar{Humans: []HumanKnowledge{{HumanIndex: 7, Players: map[uint32]uint8{1: 50}}}}
	if h, ok := f.For(7); !ok || h.Players[1] != 50 {
		t.Errorf("For(7) = %+v, %v", h, ok)
	}
	if _, ok := f.For(8); ok {
		t.Error("For(8) found a human that is not there")
	}
}

func TestReadFogOfWar(t *testing.T) {
	body := fogBody(fogHuman{88351, []fogItem{{80, 28560}}})
	a := openSave(t, buildSave(t, testInner{path: "fog_of_war.dat", data: append(innerFile("tad.", 2, ""), body...)}))

	f, err := ReadFogOfWar(a)
	if err != nil {
		t.Fatalf("ReadFogOfWar: %v", err)
	}
	if h, ok := f.For(88351); !ok || h.Players[28560] != 80 {
		t.Errorf("got %+v", f)
	}
}
