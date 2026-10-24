package fm

import (
	"encoding/json"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// attributeNames labels Attributes: savefile.AttributeNames, then
// savefile.HiddenAttributeNames.
var attributeNames = func() (names [len(savefile.AttributeNames) + len(savefile.HiddenAttributeNames)]string) {
	n := copy(names[:], savefile.AttributeNames[:])
	copy(names[n:], savefile.HiddenAttributeNames[:])
	return names
}()

// Attributes holds a player's attributes on the 1-20 scale, in
// attributeNames order; 0 means unknown. It marshals to JSON as a
// name → value map without the unknown ones.
type Attributes [len(attributeNames)]uint8

var attributeIndex = func() map[string]int {
	m := make(map[string]int, len(attributeNames))
	for i, n := range attributeNames {
		m[strings.ToLower(n)] = i
	}
	return m
}()

// attributeOf returns the index of the named attribute, ignoring case.
func attributeOf(name string) (int, bool) {
	i, ok := attributeIndex[strings.ToLower(name)]
	return i, ok
}

// mustAttribute returns the attribute's index; it panics on an unknown name.
func mustAttribute(name string) int {
	i, ok := attributeOf(name)
	if !ok {
		panic("fm: unknown attribute " + name)
	}
	return i
}

// Get returns the named attribute, ignoring case; 0 for unknown names.
func (a Attributes) Get(name string) int {
	if i, ok := attributeOf(name); ok {
		return int(a[i])
	}
	return 0
}

// MarshalJSON encodes the known attributes as a name → value object.
func (a Attributes) MarshalJSON() ([]byte, error) {
	m := make(map[string]int, len(a))
	for i, v := range a {
		if v > 0 {
			m[attributeNames[i]] = int(v)
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON decodes a name → value object; unknown names are ignored.
func (a *Attributes) UnmarshalJSON(b []byte) error {
	var m map[string]int
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*a = Attributes{}
	for name, v := range m {
		if i, ok := attributeOf(name); ok {
			a[i] = uint8(min(max(v, 0), 20)) //nolint:gosec // clamped to 0..20
		}
	}
	return nil
}
