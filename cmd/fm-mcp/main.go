// Command fm-mcp is an MCP server (stdio) that reads Football Manager 26 and
// 27 saves and exposes them as assistant-manager tools.
//
// Environment:
//
//	FM_SAVE       save to load on first use (default: newest save found)
//	FM_SAVE_DIRS  folders to search for saves, separated like PATH
//	              (default: the FM27, then FM26 save folders under ~/Library/Application Support)
//	FM_GAME_DB_DIR the game's bundled database folder, the source of nation,
//	              competition and injury names (default: the newest installed
//	              game's Steam install)
//	FM_FOG_OF_WAR off, 0 or false starts with fog of war off (default: on)
//	FM_PREFS_DIR  the game's preferences folder, the source of the currency and
//	              wage period (default: the folder of the game the save belongs
//	              to, else the newest game's)
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Napas/fm-mcp/internal/fm"
	"github.com/Napas/fm-mcp/internal/mcpserver"
)

var version = "dev"

func main() {
	log.SetOutput(os.Stderr)
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	dbDir := gameDBDir(home, os.Getenv("FM_GAME_DB_DIR"), dirExists)
	prefsEnv := os.Getenv("FM_PREFS_DIR")
	load := func(path string) (*fm.Game, error) {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		return fm.Load(path, fm.LoadOptions{GameDBDir: dbDir, PrefsDir: prefsDir(home, prefsEnv, abs, dirExists)})
	}
	srv := mcpserver.New(mcpserver.Options{
		SaveDirs:    saveDirs(home, os.Getenv("FM_SAVE_DIRS")),
		Load:        load,
		DefaultSave: os.Getenv("FM_SAVE"),
		Version:     version,
		FogOfWarOff: !fogOfWar(os.Getenv("FM_FOG_OF_WAR")),
	})
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// games lists the supported games' folder names, newest first.
var games = []string{"Football Manager 27", "Football Manager 26"}

// userDir is a game's user data folder on macOS: saves and preferences.
func userDir(home, game string) string {
	return filepath.Join(home, "Library", "Application Support", "Sports Interactive", game)
}

func saveDirs(home, env string) []string {
	if env != "" {
		return filepath.SplitList(env)
	}
	var dirs []string
	for _, g := range games {
		base := userDir(home, g)
		dirs = append(dirs, filepath.Join(base, "cloud", "games"), filepath.Join(base, "games"))
	}
	return dirs
}

// gameDBDir returns env, else the newest installed game's database folder,
// else FM27's. FM27 also ships FM26's database (label 2600).
func gameDBDir(home, env string, exists func(string) bool) string {
	if env != "" {
		return env
	}
	return newestExisting(exists, func(g string) string {
		return filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", g,
			"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db")
	})
}

// prefsDir returns env, else the preferences folder of the game whose user
// folder holds save, else the newest existing one, else FM27's.
func prefsDir(home, env, save string, exists func(string) bool) string {
	if env != "" {
		return env
	}
	prefs := func(g string) string { return filepath.Join(userDir(home, g), "Preferences", "version 6") }
	for _, g := range games {
		if rel, err := filepath.Rel(userDir(home, g), save); err == nil && filepath.IsLocal(rel) {
			return prefs(g)
		}
	}
	return newestExisting(exists, prefs)
}

// newestExisting returns the first existing path in games order, else the
// newest game's.
func newestExisting(exists func(string) bool, path func(game string) string) string {
	for _, g := range games {
		if p := path(g); exists(p) {
			return p
		}
	}
	return path(games[0])
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fogOfWar reports whether FM_FOG_OF_WAR leaves fog of war on: anything but
// off, 0 or false does.
func fogOfWar(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "off", "0", "false":
		return false
	}
	return true
}
