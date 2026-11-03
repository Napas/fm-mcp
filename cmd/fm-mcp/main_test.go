package main

import (
	"path/filepath"
	"slices"
	"testing"
)

func userDirOf(game string) string {
	return filepath.Join("/home/u", "Library", "Application Support", "Sports Interactive", game)
}

func installDBOf(game string) string {
	return filepath.Join("/home/u", "Library", "Application Support", "Steam", "steamapps", "common", game,
		"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db")
}

// existsOnly reports only the given paths as existing.
func existsOnly(paths ...string) func(string) bool {
	return func(p string) bool { return slices.Contains(paths, p) }
}

func TestSaveDirsDefaultsToFM27ThenFM26Folders(t *testing.T) {
	fm27, fm26 := userDirOf("Football Manager 27"), userDirOf("Football Manager 26")
	want := []string{
		filepath.Join(fm27, "cloud", "games"), filepath.Join(fm27, "games"),
		filepath.Join(fm26, "cloud", "games"), filepath.Join(fm26, "games"),
	}

	if got := saveDirs("/home/u", ""); !slices.Equal(got, want) {
		t.Errorf("saveDirs = %v, want %v", got, want)
	}
}

func TestSaveDirsFromEnvironment(t *testing.T) {
	got := saveDirs("/home/u", "/a"+string(filepath.ListSeparator)+"/b")

	if !slices.Equal(got, []string{"/a", "/b"}) {
		t.Errorf("saveDirs = %v, want [/a /b]", got)
	}
}

func TestGameDBDirPicksTheNewestInstalledGame(t *testing.T) {
	fm27, fm26 := installDBOf("Football Manager 27"), installDBOf("Football Manager 26")
	tests := map[string]struct {
		env    string
		exists func(string) bool
		want   string
	}{
		"both installed":   {exists: existsOnly(fm27, fm26), want: fm27},
		"only FM26":        {exists: existsOnly(fm26), want: fm26},
		"neither":          {exists: existsOnly(), want: fm27},
		"from environment": {env: "/x/db", exists: existsOnly(fm27), want: "/x/db"},
	}
	for name, tt := range tests {
		if got := gameDBDir("/home/u", tt.env, tt.exists); got != tt.want {
			t.Errorf("%s: gameDBDir = %q, want %q", name, got, tt.want)
		}
	}
}

func TestPrefsDirFollowsTheSavesGame(t *testing.T) {
	fm27, fm26 := userDirOf("Football Manager 27"), userDirOf("Football Manager 26")
	prefs27 := filepath.Join(fm27, "Preferences", "version 6")
	prefs26 := filepath.Join(fm26, "Preferences", "version 6")
	both := existsOnly(prefs27, prefs26)
	tests := map[string]struct {
		env, save string
		exists    func(string) bool
		want      string
	}{
		"FM27 save":                     {save: filepath.Join(fm27, "games", "KŽ.fm"), exists: both, want: prefs27},
		"FM26 cloud save":               {save: filepath.Join(fm26, "cloud", "games", "KŽ.fm"), exists: both, want: prefs26},
		"save elsewhere":                {save: "/tmp/copy.fm", exists: both, want: prefs27},
		"save elsewhere, only FM26":     {save: "/tmp/copy.fm", exists: existsOnly(prefs26), want: prefs26},
		"save elsewhere, no prefs":      {save: "/tmp/copy.fm", exists: existsOnly(), want: prefs27},
		"look-alike folder name":        {save: filepath.Join(userDirOf("Football Manager 26 backup"), "x.fm"), exists: both, want: prefs27},
		"relative save path falls back": {save: filepath.Join("data", "x.fm"), exists: existsOnly(prefs26), want: prefs26},
		"from environment, FM26 save":   {env: "/x/prefs", save: filepath.Join(fm26, "games", "a.fm"), exists: both, want: "/x/prefs"},
	}
	for name, tt := range tests {
		if got := prefsDir("/home/u", tt.env, tt.save, tt.exists); got != tt.want {
			t.Errorf("%s: prefsDir = %q, want %q", name, got, tt.want)
		}
	}
}

func TestFogOfWarFromEnvironment(t *testing.T) {
	for env, want := range map[string]bool{"": true, "on": true, "1": true, "yes": true, "off": false, "OFF": false, "0": false, "false": false, " False ": false} {
		if got := fogOfWar(env); got != want {
			t.Errorf("fogOfWar(%q) = %v, want %v", env, got, want)
		}
	}
}
