package main

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestSaveDirsDefaultsToFM26Folders(t *testing.T) {
	base := filepath.Join("/home/u", "Library", "Application Support", "Sports Interactive", "Football Manager 26")
	want := []string{filepath.Join(base, "cloud", "games"), filepath.Join(base, "games")}

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

func TestGameDBDir(t *testing.T) {
	want := filepath.Join("/home/u", "Library", "Application Support", "Steam", "steamapps", "common", "Football Manager 26",
		"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db")

	if got := gameDBDir("/home/u", ""); got != want {
		t.Errorf("gameDBDir = %q, want %q", got, want)
	}
	if got := gameDBDir("/home/u", "/x/db"); got != "/x/db" {
		t.Errorf("gameDBDir with env = %q, want /x/db", got)
	}
}

func TestFogOfWarFromEnvironment(t *testing.T) {
	for env, want := range map[string]bool{"": true, "on": true, "1": true, "yes": true, "off": false, "OFF": false, "0": false, "false": false, " False ": false} {
		if got := fogOfWar(env); got != want {
			t.Errorf("fogOfWar(%q) = %v, want %v", env, got, want)
		}
	}
}

func TestPrefsDir(t *testing.T) {
	want := filepath.Join("/home/u", "Library", "Application Support", "Sports Interactive", "Football Manager 26", "Preferences", "version 6")

	if got := prefsDir("/home/u", ""); got != want {
		t.Errorf("prefsDir = %q, want %q", got, want)
	}
	if got := prefsDir("/home/u", "/x/prefs"); got != "/x/prefs" {
		t.Errorf("prefsDir with env = %q", got)
	}
}
