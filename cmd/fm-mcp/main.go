// Command fm-mcp is an MCP server (stdio) that reads Football Manager 26
// saves and exposes them as assistant-manager tools.
//
// Environment:
//
//	FM_SAVE       save to load on first use (default: newest save found)
//	FM_SAVE_DIRS  folders to search for saves, separated like PATH
//	              (default: the FM26 save folders under ~/Library/Application Support)
//	FM_GAME_DB_DIR the game's bundled database folder, the source of nation,
//	              competition and injury names (default: the Steam install)
//	FM_FOG_OF_WAR off, 0 or false starts with fog of war off (default: on)
//	FM_PREFS_DIR  the game's preferences folder, the source of the currency and
//	              wage period (default: the FM26 folder)
package main

import (
	"cmp"
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
	opts := fm.LoadOptions{GameDBDir: gameDBDir(home, os.Getenv("FM_GAME_DB_DIR")), PrefsDir: prefsDir(home, os.Getenv("FM_PREFS_DIR"))}
	srv := mcpserver.New(mcpserver.Options{
		SaveDirs:    saveDirs(home, os.Getenv("FM_SAVE_DIRS")),
		Load:        func(path string) (*fm.Game, error) { return fm.Load(path, opts) },
		DefaultSave: os.Getenv("FM_SAVE"),
		Version:     version,
		FogOfWarOff: !fogOfWar(os.Getenv("FM_FOG_OF_WAR")),
	})
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// userDir is the FM26 user data folder on macOS: saves and preferences.
func userDir(home string) string {
	return filepath.Join(home, "Library", "Application Support", "Sports Interactive", "Football Manager 26")
}

func saveDirs(home, env string) []string {
	if env != "" {
		return filepath.SplitList(env)
	}
	base := userDir(home)
	return []string{filepath.Join(base, "cloud", "games"), filepath.Join(base, "games")}
}

func gameDBDir(home, env string) string {
	return cmp.Or(env, filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Football Manager 26",
		"fm.app", "Contents", "PlugIns", "game_plugin.bundle", "Contents", "Resources", "shared", "data", "database", "db"))
}

func prefsDir(home, env string) string {
	return cmp.Or(env, filepath.Join(userDir(home), "Preferences", "version 6"))
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
