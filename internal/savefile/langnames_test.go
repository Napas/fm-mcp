package savefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNamesByRankRanksUIDs(t *testing.T) {
	lang := []LangName{{UID: 765, Name: "England"}, {UID: 5, Name: "Algeria"}, {UID: 776, Name: "Italy"}}

	got := NamesByRank(lang, 3)

	if got[0] != "Algeria" || got[1] != "England" || got[2] != "Italy" {
		t.Errorf("NamesByRank = %v", got)
	}
}

func TestNamesByRankNeedsMatchingCount(t *testing.T) {
	lang := []LangName{{UID: 765, Name: "England"}, {UID: 5, Name: "Algeria"}}

	if got := NamesByRank(lang, 3); len(got) != 0 {
		t.Errorf("NamesByRank with a count mismatch = %v, want none", got)
	}
}

func TestLangDBPathPrefersFullDatabase(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"2600/2600_fm/lang_db.dat", "2620/2620_fm/lang_update_db.dat"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct{ label, want string }{
		{"2600", filepath.Join(dir, "2600/2600_fm/lang_db.dat")},
		{"2620", filepath.Join(dir, "2620/2620_fm/lang_update_db.dat")},
		{"9999", ""},
		{"../x", ""},
	}
	for _, tt := range tests {
		if got := LangDBPath(dir, tt.label); got != tt.want {
			t.Errorf("LangDBPath(%q) = %q, want %q", tt.label, got, tt.want)
		}
	}
}
