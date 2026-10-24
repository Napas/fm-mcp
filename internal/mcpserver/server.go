// Package mcpserver exposes a loaded FM save over MCP. It holds no parsing
// logic: tools call into the fm domain package.
package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Napas/fm-mcp/internal/fm"
)

// Options configures the server.
type Options struct {
	SaveDirs    []string                            // folders searched for .fm saves
	DefaultSave string                              // save loaded on first use; newest in SaveDirs when empty
	Load        func(path string) (*fm.Game, error) // fm.Load without names, in GBP per week, when nil
	Logf        func(format string, args ...any)    // log.Printf (stderr) when nil
	Version     string
	FogOfWarOff bool // start with fog of war off; it is on by default

	settleTime    time.Duration // how long a changed save must stay unchanged; 2 s when 0
	releaseMemory func()        // debug.FreeOSMemory when nil
}

type server struct {
	opts Options

	mu          sync.Mutex
	game        *fm.Game
	path        string
	stamp       fileStamp
	missing     bool      // the save is gone; noted once
	unsettledAt time.Time // when a reload last gave up on a changing file
	unreported  []string  // notes not yet shown
	fog         bool      // fog of war; kept across loads
}

// New returns an MCP server with the assistant-manager tools.
func New(opts Options) *mcp.Server {
	if opts.Load == nil {
		opts.Load = func(path string) (*fm.Game, error) { return fm.Load(path, fm.LoadOptions{}) }
	}
	if opts.Logf == nil {
		opts.Logf = log.Printf
	}
	if opts.settleTime == 0 {
		opts.settleTime = 2 * time.Second
	}
	if opts.releaseMemory == nil {
		opts.releaseMemory = debug.FreeOSMemory
	}
	s := &server{opts: opts, fog: !opts.FogOfWarOff}
	srv := mcp.NewServer(&mcp.Implementation{Name: "fm-mcp", Version: cmp.Or(opts.Version, "dev")}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	addPrompts(srv)
	addTool(srv, &mcp.Tool{Name: "list_saves", Description: "List Football Manager 26 save files, newest first."}, s.listSaves)
	addTool(srv, &mcp.Tool{Name: "load_save", Description: "Load a save file (read-only). Without a path, loads the configured default save or the newest one. Other tools do the same automatically when no save is loaded."}, s.loadSave)
	addTool(srv, &mcp.Tool{Name: "game_overview", Description: "In-game date, the human manager and club, the club's transfer and wage budgets, how many players the save holds, and whether fog of war is on."}, s.overview)
	addTool(srv, &mcp.Tool{Name: "set_fog_of_war", Description: "Turn fog of war on or off. Only call this when the user explicitly asks to."}, s.setFogOfWar)
	addTool(srv, &mcp.Tool{Name: "find_players", Description: "Find players by name, most reputable first. Each word must start a word of the name, ignoring case and accents ('son heung' finds Son Heung-Min; 'ly' does not find Reilly). Hyphenated and apostrophe names match from each part."}, withGame(s, findPlayers))
	addTool(srv, &mcp.Tool{Name: "get_player", Description: "Full profile of one player: attributes (1-20, hidden ones included), personality, media handling, traits, favoured clubs, position ratings (1-20; ratings below 10 are left out), current and potential ability (1-200), best_roles (his top 3 in-possession roles by role_ability, at positions rated 15+), reputation and reputation level, age, club and nation, and season stats per competition for the current season (club, league, domestic cups, continental, friendlies, internationals). With fog of war on, values your scouts do not know are left out. detail \"summary\" keeps only the club season_detail row and drops the interest factors." + interestDescription}, withGame(s, getPlayer))
	addTool(srv, &mcp.Tool{Name: "compare_players", Description: "Profiles of several players side by side, including best_roles and season stats per competition; position ratings below 10 are left out. By default (detail \"summary\") season_detail has only the club row and interest has no factors; pass detail \"full\" for every competition row and the factors." + interestDescription}, withGame(s, comparePlayers))
	addTool(srv, &mcp.Tool{Name: "find_clubs", Description: "Find teams (first, youth, reserve, national) by name, most reputable first. Each word must start a word of the name or short name; case and accents ignored ('tottenham', 'spurs', 'st pauli'). Returns team_index for get_squad, squad_depth and search_players."}, withGame(s, findClubs))
	addTool(srv, &mcp.Tool{Name: "get_squad", Description: "A team's details (division, reputation, finances), its players highest current ability first, and players contracted to it but out on loan. For a national team, the players are its called-up squad. Defaults to the human manager's first team."}, withGame(s, squad))
	addTool(srv, &mcp.Tool{Name: "squad_depth", Description: "For each position, the squad players rated at least 15 there, best first. Defaults to the human manager's club."}, withGame(s, squadDepth))
	addTool(srv, &mcp.Tool{Name: "get_tactic", Description: "The human manager's current first-team tactic: name, mentality, the 11 slots with in- and out-of-possession position and role and the player in each with his role_ability for the in-possession role, and the bench. lineup_source is \"selected\" for the players picked for the next match, or \"last_match\" when none are picked and the last match's lineup is shown, or \"none\" when the save has neither (the slots have no players); placing the last match's players in this tactic's slots assumes that match used this tactic. To find candidates for a slot, pass its position code and in-possession role to search_players (position, role) or look it up in squad_depth. When the manager's team is unknown, the tactic may belong to another of his teams. A player marked unnamed_player is a person the save does not decode as a player. Names of roles FM26 renamed are guesses; the server instructions list them."}, withGame(s, getTactic))
	addTool(srv, &mcp.Tool{Name: "get_last_lineup", Description: "A team's lineup, formation and roles in its most recent match, and who it played: opponent, mentality (checked in game for the manager's team only), the shape in and out of possession (e.g. 4-2-3-1), the 11 slots with in- and out-of-possession position and role and the player in each with his shirt_number, and the bench. Use it to scout an opponent; get_fixtures gives the next one. Roles have no duty and the match date is not known (get_fixtures has it). A player marked unnamed_player is a person the save does not decode as a player, so it has no name: 58% of filled XI places on the 12-09 save, mostly in women's teams and clubs outside Europe; big European men's clubs are complete. Fog of war does not hide it. Defaults to the human manager's first team. Names of roles FM26 renamed are guesses; the server instructions list them."}, withGame(s, getLastLineup))
	addTool(srv, &mcp.Tool{Name: "get_fixtures", Description: fixturesDescription}, withGame(s, getFixtures))
	addTool(srv, &mcp.Tool{Name: "get_table", Description: tableDescription}, withGame(s, getTable))
	addTool(srv, &mcp.Tool{Name: "search_players", Description: "Scout players with filters: position, age, current/potential ability, team or club, contracted team, nation, gender, value, wage, release clause, expiring contract, months at the current club, injuries, height, traits, personality and minimum attribute values (preferred foot: e.g. {\"LeftFoot\": 15}). Returns the total number of matches. Sorted by current ability unless sort_by is set; sort_by also takes this season's club stats (season_goals, season_average_rating…) and club stats per 90 minutes (per90_progressive_passes…), with min_season_minutes to leave out players with few minutes; min_per90 filters on them, and rows carry the values in per90. With role (an in-possession role offered at the position, with position), rows carry role_ability and are ranked by it unless sort_by is set; min_role_ability sets a floor. Without a team or club filter, gender defaults to that of the manager's team. With fog of war on, filters match only values your scouts know, and players whose sort value is hidden or missing come last; needs_scouting counts players who pass every filter you can see but whose value for another filter is hidden; they may match once scouted. With interest (transfer, loan or transfer_or_loan), players who certainly won't join the manager's club are left out and counted in interest_dropped; kept rows carry interest."}, withGame(s, searchPlayers))
	return srv
}

const fixturesDescription = "Fixtures and results for a team or a competition. Without team_index or a competition, the human manager's first team. results are played matches, most recent first; upcoming are the rest, soonest first; each list holds up to limit (default 10, max 200), totals count all. Each fixture has the date, competition, stage (e.g. \"Playoffs – Semi Final – First Leg\"), home and away team with team_index, and for played matches the score after 90 minutes plus after_extra_time, penalties and aggregate when they apply. competition takes a name (an exact name wins; otherwise the error lists matches with competition_index). Fog of war does not hide fixtures."

const tableDescription = "Stored league tables in the game's order: leagues, groups, conferences, league phases and playoff tables. With a competition, all of its stage tables; with team_index, the team's league table (else every table it is in); by default the human manager's first team. Rows carry position, team and team_index, played, won, drawn, lost, goals for and against, goal_difference, points as the game stores them (deductions and carried-over points included), home and away splits, and form (last ten results, most recent first). shootout_losses appears in cup groups where drawn matches go to penalties: there drawn counts shoot-out wins. label_source \"results\" means the table was matched to a sub-competition by its results or teams; \"file\" means it is labelled by the competition it is stored with. Several tables can share a label (e.g. a league's two conferences); their teams tell them apart. Tables are never computed: when none is stored the tool says so. Knockout rounds have no table; use get_fixtures."

// interestDescription ends the get_player and compare_players descriptions.
const interestDescription = " interest: would he join your club. Transfer and loan are each out, uncertain or likely (a loan is never likely). Out comes with transfer_reason or loan_reason; transfer_factors and loan_factors list what weighs against."

const instructions = `Tools for an assistant manager in Football Manager 26, reading the user's save file (read-only).
Start with game_overview: it names the human manager and club, and lists load warnings (e.g. missing names) that you should pass on.
Players belong to teams: a club's first team or its youth or reserve teams. team_index identifies a team; find_clubs maps names to team_index values. A player's "club" field is his team's name. A player called up to a national team also has national_team and national_team_index. get_squad, squad_depth and search_players by team_index treat a called-up player as a player of both teams; search_players' club filter matches club teams only. When a club filter finds no players, search_players returns a hint saying why; pass it on.
Scales: attributes and position ratings 1-20 (position 20 = natural, 15+ = accomplished); current and potential ability 1-200; reputation 0-10000.
Money is in the user's currency and wages per the user's wage period, both reported by game_overview (currency, wage_period). Write amounts as FM does: currency symbol, K or M, wage period after wages, e.g. €922K p/a, €34.29M.
game_overview's budgets are the manager's club's. transfer_budget is what the club can spend on fees now (FM's Current transfer budget; it can be negative). wage_budget is the total per the wage period, not what is left: FM's Available = wage_budget minus the current wage bill (probably players and staff), which the server does not compute. The amount spent this season is not decoded. No budgets means the manager's club is unknown or has no finances in the save: say the budgets are unknown, not zero.
Positions: GK SW DL DC DR DM ML MC MR AML AMC AMR ST WBL WBR.
get_tactic returns the manager's current first-team tactic: name, mentality, the bench, and per slot the player and the in_possession and out_of_possession position (side "left" or "right" for one of a pair, e.g. two DC) and role. lineup_source "selected" means the players picked for the next match; "last_match" means none are picked, so the players are the last match's lineup; "none" means the save has neither, so the slots have no players; placing the last match's players in this tactic's slots assumes that match used this tactic. To find a player for a slot, pass its position code and in-possession role to search_players (position, role) or look it up in squad_depth. When the manager's team is unknown, the tactic may belong to another of his teams.
get_last_lineup returns a team's most recent match (team_index; the manager's first team by default): opponent and opponent_team_index, mentality (checked in game for the manager's team only), in_possession_shape and out_of_possession_shape (players per row from defence to attack without the goalkeeper; wing-backs count with defensive midfielders), per slot the player and the in_possession and out_of_possession position and role, and the bench. Players carry shirt_number, the number worn, when the save has one. unnamed_player (here and in get_tactic) marks a person the save does not decode as a player: 58% of filled XI places on the 12-09 save, mostly in women's teams and clubs outside Europe. Use it to scout an opponent; get_fixtures names the next one. Roles have no duty, and the match date is not known. Fog of war does not hide it.
get_fixtures lists a team's or a competition's fixtures: results (most recent first) and upcoming (soonest first). stage labels knockout and playoff rounds as the game names them, e.g. "Playoffs – Semi Final – First Leg"; a two-division playoff also has second_competition. score is after 90 minutes; after_extra_time, penalties and aggregate appear when they apply. Competition names come from fixtures, a team's division, or the list an ambiguous name returns; pass competition_index to pick one.
get_table returns the stored tables of a competition or a team (by default the manager's league table), with stage names such as "Group A". Points are as the game stores them; never compute a table from results, and when get_table says none is stored, say so.
not_for_sale means the club will not sell the player; the save then has no value for him.
playing_time (get_player, compare_players, get_squad) is FM's Playing Time: the squad status the club has agreed with the player, as squad lists show it and the contract screen's "Agreed" line. actual_playing_time is the contract screen's headline: how his game time currently rates, e.g. agreed Squad Player but actual Surplus to Requirements. original_playing_time is the status agreed when the contract was signed; FM does not show it. For a player on loan, all three come from his loan contract. FM's "Expected" playing time is not available. actual_playing_time is left out for players away from their current contract's team or at a team that is not a club's first team (reserve and youth scores are not checked in game), for players 25 or younger whose result the game's youth test may change, and when a part of the game's rule not decoded may give another status (Squad Player, Impact Sub, Breakthrough Prospect or Fringe Player). The game's adjustments for youth contracts, playing-time promises and team settings are not applied, so a served actual_playing_time can still differ from FM's.
squad_number (get_player, compare_players, get_squad) is the number registered with his current contract's team (his loan club while on loan). get_squad leaves it out for players contracted to another team (e.g. first-team players in a reserve squad) and in national-team squads; in its out_on_loan rows it is the number at the club whose squad it is. Fog of war does not hide it. shirt_number (get_last_lineup) is the number worn in that match and can differ from squad_number.
A contract's start is when its current terms were signed (the contract screen's Start); joined is when the player joined that team. A player's joined_club is when he joined his current club, as his profile shows it; it can be later than his contract's joined.
loyalty_bonus_remaining (get_player, compare_players) is the main contract's loyalty bonus still to be paid, weekly until the contract ends; FM shows it rounded (in the samples seen: to the nearest €1K below €100K, to the nearest €25K above).
role_ability (get_tactic slots, search_players with role, best_roles in get_player and compare_players) is FM's role ability: how well a player suits an in-possession role at a position, on the current-ability scale (0-200). A player's best role usually equals his current ability: the game halves a role ability under the role's threshold (100 for Free Role, Wide Forward and Box To Box Playmaker, 80 or 60 for a few others), the best role included. The game shows it as stars against its own assessment of the player, which the server cannot reproduce, so use it to rank, not to predict stars. One player's roles came out in the game's order in every sample checked, but ties may be listed in another order; across players it agrees with the game's order about 3 times in 4. search_players has no side: for Wide and Overlapping Centre-Back at DC it uses the better of left and right, while get_tactic uses the slot's side, so the two can differ for the same player. It is hidden with current ability under fog of war.
reputation_level (get_player, compare_players, get_squad) is the reputation word on FM's player profile: Worldwide, Continental, National, Regional, Local or Obscure. Missing for women's players and player-coaches, and for everyone when game_overview warns that reputation levels are unavailable. Fog of war does not hide it.
personality (get_player, compare_players, get_squad) is FM's personality label, computed from the hidden attributes, and traits (get_player, compare_players) lists the player's traits (FM's player preferred moves). favoured_clubs (get_player, compare_players) lists the clubs the player favours (FM's Favoured Clubs) by short name, with team_index (the club's first team, when it has one) and as "supporter" or "player" when known.
media_handling (get_player, compare_players, get_squad) lists FM's media handling styles, e.g. ["Evasive", "Unflappable"], computed from the hidden attributes. Fog of war hides it with the personality (the server's choice; not checked in game).
season (find_players, get_squad and search_players rows, get_player and compare_players) totals the current season's club competitions: league, cups and continental, not friendlies or internationals. Write appearances as FM does, starts (sub_appearances), e.g. 16 (2). average_rating is the season average over matches with a rating. A missing count is 0. No season means no such match yet, unless game_overview or save_notes warn that season stats are unavailable: then a missing season means unknown. The server never hides season stats under fog of war. Rows' season also has minutes and player_of_the_match. get_player and compare_players add season_detail: one row per competition group, club (all club competitions, the game's "Overall (Club)"), league, domestic_cups, continental, friendlies and internationals (the senior national team's matches, probably those of this international season; club rows exclude them), with penalties_taken/penalties_scored, matches_won/matches_lost (drawn = starts + sub_appearances - matches_won - matches_lost), team_goals/team_conceded (his team's goals in his matches) and match stats (passes, tackles, fouls, fouls_against, pressures, expected_goals, non_penalty_expected_goals, distance_km…). compare_players defaults to detail "summary": only the club row and no interest factors; pass detail "full" for every row and the factors. continental counts all continental cups together. Goalkeepers (GK rating 15 or more) have "goalkeeper": true on their rows and get goals_conceded, clean_sheets, saves_held, saves_parried, saves_tipped, penalties_faced/penalties_saved and expected_goals_prevented (can be negative) instead of tackles/tackles_won, headers/headers_won, blocks and open_play_crosses_completed: the game stores those in the same places. headers_lost is not recorded for goalkeepers. FM shows many stats per 90 minutes: compute them as count × 90 / minutes. search_players sorts (per90_<stat>) and filters (min_per90) on the club row per 90 minutes, leaving out players under 450 club minutes unless min_season_minutes is set; the rate of a stat his side does not record (a keeper's tackles, an outfield player's goals_conceded) is missing, so min_per90 on it leaves him out and a sort puts him last; rows carry them in per90.
Fog of war (on by default, shown by game_overview) hides what the manager's scouts do not know. Each player then has knowledge (0-100) and scouting: "full" shows everything; "partial" hides current and potential ability, wage, clauses, playing time, loyalty bonus, hidden attributes, personality, media handling and favoured clubs; "required" also hides attributes, traits and value. Hidden fields are left out, as are other clubs' bank balances, and searches only match and rank on visible values. search_players reports needs_scouting: players who pass every filter you can see but whose value for another filter is hidden. When it is above 0, tell the user that many more players may match once scouted, even when nothing matches. When fog of war hides what the user needs, name the players who need scouting instead of guessing. Never call set_fog_of_war unless the user explicitly asks.
interest (search_players option) estimates FM's Player's interest: would a player join the manager's club for a transfer, a loan or either. Verdicts: likely, uncertain, out; a loan is never likely. Out players are dropped and counted per reason in interest_dropped: only_just_joined (joined or renewed recently), transfer_arranged (move already agreed), retiring, only_one_club (will join only one club), recently_left_your_club, club_reputation_gap (his club or his own reputation is far above yours). Uncertain players are never dropped. It is an estimate, so say so. Check game_overview's warnings before trusting likely: without arranged transfers, a player who agreed a move elsewhere is not ruled out. Pass the interest factors on to the user. get_player and compare_players carry interest regardless of fog of war: verdicts, transfer_reason and loan_reason when out, and transfer_factors and loan_factors (compare_players with detail "full" only). It is left out for your own players (and players moving to your club), players of the other gender than your team, players with only a national team, and every player when your club is unknown.
In tables of 1-20 values (attributes, position ratings), mark 11-15 with 🟡 and 16-20 with 🟢; leave 1-10 plain.
These fields are guesses; say so when they matter: condition, match sharpness, clause and bonus names, actual playing time (checked in game for 3 first-team outfield players; the others are predictions of the decoded rule), media_handling (for some players the game may show Volatile, Short-tempered or Confrontational instead), reputation_level words Regional and Obscure, and reputation_level of players without a team, and these tactic role names, which FM26 renamed: Ball-Playing Centre-Back, Advanced Centre-Back, Inside Wing-Back, Inside Full-Back, Anchor, Inside Winger, Free Role, Wide Target Forward, Playmaking Winger, Target Forward, False Nine, Second Striker, Line-Holding Goalkeeper, Inside Outlet Winger. The server reloads the loaded save when it changes and announces it in save_notes (in an error, after "Save notes:"). Pass the notes on to the user: earlier answers may be out of date. Only the loaded file is followed; use load_save for another save.`

// current returns the game under the fog setting, its path and the pending
// notes, which it clears. It loads the default save on first use and reloads
// a changed one.
func (s *server) current(ctx context.Context) (fm.View, string, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.game != nil {
		s.reloadIfChangedLocked(ctx)
	} else {
		path, err := s.savePath("")
		if err != nil {
			return fm.View{}, "", nil, err
		}
		if err := s.loadLocked(path); err != nil {
			return fm.View{}, "", nil, err
		}
	}
	notes := s.unreported
	s.unreported = nil
	return s.game.View(s.fog), s.path, notes, nil
}

func withGame[In, Out any, PO interface {
	*Out
	setNotes([]string)
}](s *server, h func(fm.View, In) (Out, error)) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		v, _, notes, err := s.current(ctx)
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		out, err := h(v, in)
		if err != nil {
			if len(notes) > 0 {
				err = fmt.Errorf("%w\nSave notes: %s", err, strings.Join(notes, "; "))
			}
			var zero Out
			return nil, zero, err
		}
		PO(&out).setNotes(notes)
		return nil, out, nil
	}
}

// note queues a note for the next tool result.
func (s *server) note(format string, args ...any) {
	s.unreported = append(s.unreported, fmt.Sprintf(format, args...))
}

// saveNotes puts the notes in the structured result, which some clients show the model.
type saveNotes struct {
	SaveNotes []string `json:"save_notes,omitempty"` // reloads and save file problems since the last call
}

func (n *saveNotes) setNotes(notes []string) { n.SaveNotes = notes }

// maxSettleWaits caps the wait for a changing save.
const maxSettleWaits = 5

// fileStamp is a file's mtime and size.
type fileStamp struct {
	modNano int64
	size    int64
}

// age is the time since the last modification; negative for a future mtime.
func (f fileStamp) age() time.Duration { return time.Since(time.Unix(0, f.modNano)) }

func stampOf(path string) (fileStamp, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{fi.ModTime().UnixNano(), fi.Size()}, nil
}

// errUnsettled means the file never settled.
var errUnsettled = errors.New("file still changing")

// waitForSettled waits until the file has been unchanged for settle and
// returns its stamp. A future mtime counts as settled.
func waitForSettled(ctx context.Context, path string, settle time.Duration) (fileStamp, error) {
	for i := 0; ; i++ {
		stamp, err := stampOf(path)
		if err != nil {
			return fileStamp{}, err
		}
		age := stamp.age()
		if age >= settle || age < 0 {
			return stamp, nil
		}
		if i == maxSettleWaits {
			return fileStamp{}, errUnsettled
		}
		select {
		case <-ctx.Done():
			return fileStamp{}, ctx.Err()
		case <-time.After(settle - age):
		}
	}
}

// reloadIfChangedLocked reloads a changed save once it settles and queues a
// note. The old game stays if the save is gone, unsettled or fails to load. A
// failed load is retried only after the file changes again; after an
// unsettled wait, calls within settle do not wait again.
func (s *server) reloadIfChangedLocked(ctx context.Context) {
	stamp, err := stampOf(s.path)
	if err != nil {
		s.markMissing()
		return
	}
	s.missing = false
	if stamp == s.stamp {
		return
	}
	if age := stamp.age(); time.Since(s.unsettledAt) < s.opts.settleTime && age >= 0 && age < s.opts.settleTime {
		s.note(noteUnsettled)
		return
	}
	stamp, err = waitForSettled(ctx, s.path, s.opts.settleTime)
	switch {
	case err == nil:
	case errors.Is(err, errUnsettled):
		s.unsettledAt = time.Now()
		s.note(noteUnsettled)
		return
	case ctx.Err() != nil:
		return
	default: // gone or unreadable
		s.markMissing()
		return
	}
	if err := s.loadLocked(s.path); err != nil {
		s.stamp = stamp
		s.opts.Logf("reload failed: %v", err)
		s.note("the save changed but could not be reloaded; answers use the earlier copy: %v", err)
		return
	}
	s.note("the save changed on disk and was reloaded (game date %s); earlier answers may be out of date", s.game.Info.Date.Format(time.DateOnly))
}

const noteUnsettled = "the save is still being written; answers use the earlier copy for now"

// markMissing notes once that the loaded save is gone.
func (s *server) markMissing() {
	if !s.missing {
		s.missing = true
		s.note("the loaded save %s is no longer on disk or cannot be read; answers use the earlier copy", s.path)
	}
}

func (s *server) loadLocked(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".fm") {
		return fmt.Errorf("%q is not an .fm save", path)
	}
	defer s.opts.releaseMemory()    // loading peaks at ~1.8 GB; the game keeps ~400 MB
	stamp, statErr := stampOf(path) // before loading, so a write during the load counts as a change
	start := time.Now()
	g, err := s.opts.Load(path)
	if err != nil {
		return fmt.Errorf("loading %s: %w", path, err)
	}
	if statErr != nil {
		stamp, _ = stampOf(path)
	}
	s.opts.Logf("loaded %s in %s: %d players", path, time.Since(start).Round(time.Millisecond), g.PlayerCount())
	for _, w := range g.Info.Warnings {
		s.opts.Logf("warning: %s", w)
	}
	s.game, s.path, s.stamp, s.missing, s.unreported = g, path, stamp, false, slices.Clone(g.Info.Warnings)
	return nil
}

// SaveFile is a save found on disk.
type SaveFile struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	SizeMB   float64   `json:"size_mb"`
	Modified time.Time `json:"modified"`
}

func (s *server) findSaves() []SaveFile {
	var out []SaveFile
	for _, dir := range s.opts.SaveDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".fm") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, SaveFile{
				Path:     filepath.Join(dir, e.Name()),
				Name:     e.Name(),
				SizeMB:   float64(info.Size()) / (1 << 20),
				Modified: info.ModTime(),
			})
		}
	}
	slices.SortFunc(out, func(a, b SaveFile) int { return b.Modified.Compare(a.Modified) })
	return out
}

// savePath returns requested, else the default save, else the newest save
// found.
func (s *server) savePath(requested string) (string, error) {
	if path := cmp.Or(requested, s.opts.DefaultSave); path != "" {
		return path, nil
	}
	return s.newestSave()
}

func (s *server) newestSave() (string, error) {
	saves := s.findSaves()
	if len(saves) == 0 {
		return "", fmt.Errorf("no .fm saves found in %s", strings.Join(s.opts.SaveDirs, ", "))
	}
	return saves[0].Path, nil
}

type empty struct{}

type savesOut struct {
	Saves []SaveFile `json:"saves"`
}

func (s *server) listSaves(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, savesOut, error) {
	return nil, savesOut{Saves: s.findSaves()}, nil
}

type loadIn struct {
	Path string `json:"path,omitempty" jsonschema:"path of the .fm save; the default save, else the newest, when empty"`
}

type overviewOut struct {
	saveNotes               // empty from load_save, whose overview shows the warnings
	Info        fm.Info     `json:"info"`
	PlayerCount int         `json:"player_count"`
	LoadedFrom  string      `json:"loaded_from"`
	FogOfWar    bool        `json:"fog_of_war"`
	Currency    fm.Currency `json:"currency"`          // all money is in this currency
	WagePeriod  string      `json:"wage_period"`       // wages are per "p/w", "p/m" or "p/a"
	Budgets     *fm.Budgets `json:"budgets,omitempty"` // the manager's club's; left out when unknown
}

func (s *server) loadSave(_ context.Context, _ *mcp.CallToolRequest, in loadIn) (*mcp.CallToolResult, overviewOut, error) {
	path, err := s.savePath(in.Path)
	if err != nil {
		return nil, overviewOut{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(path); err != nil {
		return nil, overviewOut{}, err
	}
	s.unreported = nil // the overview shows the warnings
	return nil, newOverview(s.game.View(s.fog), s.path), nil
}

func (s *server) overview(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, overviewOut, error) {
	v, path, notes, err := s.current(ctx)
	if err != nil {
		return nil, overviewOut{}, err
	}
	// Load warnings are already in the overview.
	out := newOverview(v, path)
	out.setNotes(slices.DeleteFunc(notes, func(w string) bool { return slices.Contains(v.Info().Warnings, w) }))
	return nil, out, nil
}

func newOverview(v fm.View, path string) overviewOut {
	return overviewOut{
		Info: v.Info(), PlayerCount: v.PlayerCount(), LoadedFrom: path, FogOfWar: v.FogOfWar(),
		Currency: v.Money().Currency(), WagePeriod: v.Money().WagePeriod(), Budgets: v.Budgets(),
	}
}

type fogIn struct {
	Enabled bool `json:"enabled" jsonschema:"true hides what the manager's scouts do not know, as in the game"`
}

type fogOut struct {
	FogOfWar bool `json:"fog_of_war"`
}

func (s *server) setFogOfWar(_ context.Context, _ *mcp.CallToolRequest, in fogIn) (*mcp.CallToolResult, fogOut, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fog = in.Enabled
	return nil, fogOut{FogOfWar: s.fog}, nil
}

// PlayerSummary is the short form of a player used in lists. Fields fog of
// war hides are left out.
type PlayerSummary struct {
	UID               uint32             `json:"uid"`
	Name              string             `json:"name"`
	Age               int                `json:"age"`
	TeamIndex         int64              `json:"team_index"`
	Club              string             `json:"club,omitempty"`
	NationalTeamIndex *int64             `json:"national_team_index,omitempty"` // called up to; TeamIndex stays his team
	NationalTeam      string             `json:"national_team,omitempty"`
	NationIndex       int64              `json:"nation_index"`
	Nation            string             `json:"nation,omitempty"`
	BestPositions     []string           `json:"best_positions"`
	Knowledge         *int               `json:"knowledge,omitempty"` // with fog of war on
	Scouting          fm.Scouting        `json:"scouting,omitempty"`  // with fog of war on
	CurrentAbility    *int               `json:"current_ability,omitempty"`
	PotentialAbility  *int               `json:"potential_ability,omitempty"`
	Value             *int64             `json:"value,omitempty"` // in the user's currency
	NotForSale        bool               `json:"not_for_sale,omitempty"`
	Wage              *int64             `json:"wage,omitempty"`          // per the wage period; nil when unknown or hidden
	ContractEnd       string             `json:"contract_end,omitempty"`  // YYYY-MM-DD
	JoinedClub        string             `json:"joined_club,omitempty"`   // YYYY-MM-DD
	OnLoanFrom        string             `json:"on_loan_from,omitempty"`  // the parent club while on loan
	Injury            string             `json:"injury,omitempty"`        // current injury names, comma-separated
	InjuredUntil      string             `json:"injured_until,omitempty"` // earliest expected return
	RoleAbility       *int               `json:"role_ability,omitempty"`  // in a search's role; nil otherwise or when hidden
	Per90             map[string]float64 `json:"per90,omitempty"`         // search's per-90 sort and filter stats

	Season *fm.SeasonStats `json:"season,omitempty"` // nil without a match this season

	Interest *fm.Interest `json:"interest,omitempty"` // in a search with interest; nil when not evaluated
}

func summarize(ps []fm.Profile) []PlayerSummary {
	out := make([]PlayerSummary, len(ps))
	for i, p := range ps {
		out[i] = PlayerSummary{
			UID: p.UID, Name: p.Name, Age: p.Age,
			TeamIndex: p.TeamIndex, Club: p.Club, NationalTeamIndex: p.NationalTeamIndex, NationalTeam: p.NationalTeam,
			NationIndex: p.NationIndex, Nation: p.Nation,
			BestPositions: p.BestPositions(), Knowledge: p.Knowledge, Scouting: p.Scouting,
			CurrentAbility: p.CurrentAbility, PotentialAbility: p.PotentialAbility,
			Value: p.Value, NotForSale: p.NotForSale, Season: p.Season, RoleAbility: p.RoleAbility, Per90: p.Per90, Interest: p.Interest,
		}
		if c := p.Contract; c != nil {
			out[i].Wage = c.Wage
			if !c.End.IsZero() {
				out[i].ContractEnd = c.End.Format(time.DateOnly)
			}
		}
		if p.JoinedClub != nil {
			out[i].JoinedClub = p.JoinedClub.Format(time.DateOnly)
		}
		if p.Loan != nil && p.Contract != nil {
			out[i].OnLoanFrom = p.Contract.Team
		}
		if p.Injured() {
			out[i].Injury = injuryNames(p.Injuries)
			out[i].InjuredUntil = p.ExpectedReturn().Format(time.DateOnly)
		}
	}
	return out
}

func injuryNames(injuries []fm.InjuryInfo) string {
	var names []string
	for _, in := range injuries {
		if in.Name != "" {
			names = append(names, in.Name)
		}
	}
	return strings.Join(names, ", ")
}

type playersOut struct {
	saveNotes
	Players []PlayerSummary `json:"players"`
}

type searchOut struct {
	saveNotes
	Players []PlayerSummary `json:"players"`
	Total   int             `json:"total"` // matches before the limit
	// NeedsScouting counts players who pass every filter on a visible value
	// while fog of war hides the value of another; 0 without fog of war.
	NeedsScouting int        `json:"needs_scouting"`
	NationalTeams []*fm.Team `json:"national_teams,omitempty" jsonschema:"national teams with call-ups matching the club and gender filters. Not searched: club covers club teams only. Use team_index for their call-ups"`
	Hint          string     `json:"hint,omitempty" jsonschema:"why the club filter found no players and what to try instead; pass it on to the user"`
	// InterestDropped counts the players the interest option left out, per
	// reason code; present whenever interest is set.
	InterestDropped *fm.InterestDropped `json:"interest_dropped,omitempty"`
}

// maxNationalTeams caps national_teams in search_players.
const maxNationalTeams = 5

type findIn struct {
	Name  string `json:"name" jsonschema:"name or part of it, e.g. 'maddison' or 'son heung'"`
	Limit int    `json:"limit,omitempty" jsonschema:"default 25, max 200"`
}

func findPlayers(v fm.View, in findIn) (playersOut, error) {
	ps, err := v.FindPlayers(in.Name, in.Limit)
	if err != nil {
		return playersOut{}, err
	}
	return playersOut{Players: summarize(ps)}, nil
}

type uidIn struct {
	UID    uint32 `json:"uid" jsonschema:"the player's uid from find_players, search_players or get_squad"`
	Detail detail `json:"detail,omitempty" jsonschema:"full (default) or summary: only the club season_detail row, no interest factors"`
}

// detail sets how much of a profile get_player and compare_players return.
type detail string

const (
	detailFull    detail = "full"
	detailSummary detail = "summary"
)

// minPositionRating is the lowest position rating profiles list.
const minPositionRating = 10

// apply trims p to d, copying the slices it changes (the game shares them).
// Trimmed positions no longer line up with savefile.PositionNames, so apply
// is the last step before output.
func (d detail) apply(p fm.Profile) (fm.Profile, error) {
	if d != detailFull && d != detailSummary {
		return p, fmt.Errorf("detail %q; valid values: %s, %s", d, detailSummary, detailFull)
	}
	p.Positions = slices.DeleteFunc(slices.Clone(p.Positions), func(r fm.PositionRating) bool { return r.Rating < minPositionRating })
	if d == detailSummary {
		p.SeasonDetail = slices.DeleteFunc(slices.Clone(p.SeasonDetail), func(s fm.SeasonDetail) bool { return s.Competition != fm.CompetitionClub })
		if p.Interest != nil {
			in := *p.Interest
			in.TransferFactors, in.LoanFactors = nil, nil
			p.Interest = &in
		}
	}
	return p, nil
}

type profileOut struct {
	saveNotes
	fm.Profile
}

func getPlayer(v fm.View, in uidIn) (profileOut, error) {
	p, ok := v.Profile(in.UID)
	if !ok {
		return profileOut{}, fmt.Errorf("no player with uid %d", in.UID)
	}
	p, err := cmp.Or(in.Detail, detailFull).apply(p)
	return profileOut{Profile: p}, err
}

type uidsIn struct {
	UIDs   []uint32 `json:"uids" jsonschema:"up to 10 player uids to compare"`
	Detail detail   `json:"detail,omitempty" jsonschema:"summary (default) or full: summary has only the club season_detail row and no interest factors"`
}

type profilesOut struct {
	saveNotes
	Players []fm.Profile `json:"players"`
}

// maxCompare caps compare_players, whose full profiles are large.
const maxCompare = 10

func comparePlayers(v fm.View, in uidsIn) (profilesOut, error) {
	if len(in.UIDs) > maxCompare {
		return profilesOut{}, fmt.Errorf("compare at most %d players at a time", maxCompare)
	}
	out := profilesOut{Players: make([]fm.Profile, 0, len(in.UIDs))}
	for _, uid := range in.UIDs {
		p, err := getPlayer(v, uidIn{UID: uid, Detail: cmp.Or(in.Detail, detailSummary)})
		if err != nil {
			return profilesOut{}, err
		}
		out.Players = append(out.Players, p.Profile)
	}
	return out, nil
}

type squadIn struct {
	TeamIndex *int64 `json:"team_index,omitempty" jsonschema:"team_index from find_clubs or a player's team_index; the human manager's first team when empty"`
}

// club returns the requested team, or the human manager's.
func (in squadIn) club(v fm.View) (int64, error) {
	if in.TeamIndex != nil {
		return *in.TeamIndex, nil
	}
	if v.Info().ManagerTeamIndex < 0 {
		return 0, errors.New("the manager's club is not known for this save; pass team_index (from find_clubs or a player's team_index)")
	}
	return v.Info().ManagerTeamIndex, nil
}

// SquadPlayer is a get_squad row: a PlayerSummary plus squad number, playing
// times, reputation level, personality and media handling.
type SquadPlayer struct {
	PlayerSummary
	SquadNumber         int      `json:"squad_number,omitempty"`
	PlayingTime         string   `json:"playing_time,omitempty"`
	OriginalPlayingTime string   `json:"original_playing_time,omitempty"`
	ActualPlayingTime   string   `json:"actual_playing_time,omitempty"`
	ReputationLevel     string   `json:"reputation_level,omitempty"`
	Personality         string   `json:"personality,omitempty"`
	MediaHandling       []string `json:"media_handling,omitempty"`
}

func squadRows(ps []fm.Profile) []SquadPlayer {
	out := make([]SquadPlayer, len(ps))
	for i, s := range summarize(ps) {
		p := ps[i]
		out[i] = SquadPlayer{
			PlayerSummary: s, SquadNumber: p.SquadNumber, ReputationLevel: p.ReputationLevel, Personality: p.Personality, MediaHandling: p.MediaHandling,
			PlayingTime: p.PlayingTime, OriginalPlayingTime: p.OriginalPlayingTime, ActualPlayingTime: p.ActualPlayingTime,
		}
	}
	return out
}

type squadOut struct {
	saveNotes
	Team      *fm.Team      `json:"team,omitempty"`
	Players   []SquadPlayer `json:"players"`
	OutOnLoan []SquadPlayer `json:"out_on_loan,omitempty"` // contracted here, playing elsewhere
}

func squad(v fm.View, in squadIn) (squadOut, error) {
	club, err := in.club(v)
	if err != nil {
		return squadOut{}, err
	}
	players, err := v.Squad(club)
	if err != nil {
		return squadOut{}, err
	}
	loaned, err := v.LoanedOut(club)
	if err != nil {
		return squadOut{}, err
	}
	team, _ := v.Team(club)
	return squadOut{Team: team, Players: squadRows(players), OutOnLoan: squadRows(loaned)}, nil
}

// searchIn is search_players' input.
type searchIn struct {
	Gender              string             `json:"gender,omitempty" jsonschema:"men, women or any; without a team or club filter it defaults to the gender of the manager's team"`
	Position            string             `json:"position,omitempty" jsonschema:"position code the player must be rated at least 15 in: GK SW DL DC DR DM ML MC MR AML AMC AMR ST WBL WBR"`
	Role                string             `json:"role,omitempty" jsonschema:"an in-possession role offered at that position, as get_tactic names it, e.g. 'Inside Forward'; needs position. Rows get role_ability, FM's role ability (0-200) at that position, and are ranked by it unless sort_by is set"`
	MinRoleAbility      int                `json:"min_role_ability,omitempty" jsonschema:"minimum role_ability (0-200) in role; needs role"`
	MinAge              int                `json:"min_age,omitempty"`
	MaxAge              int                `json:"max_age,omitempty"`
	MinHeightCM         int                `json:"min_height_cm,omitempty"`
	MaxHeightCM         int                `json:"max_height_cm,omitempty"`
	MinCurrentAbility   int                `json:"min_current_ability,omitempty" jsonschema:"1-200"`
	MaxCurrentAbility   int                `json:"max_current_ability,omitempty" jsonschema:"1-200"`
	MinPotentialAbility int                `json:"min_potential_ability,omitempty" jsonschema:"1-200"`
	Club                string             `json:"club,omitempty" jsonschema:"name words, each starting a word of the team name or short name ('spurs', 'st pauli'); also matches the club's youth and reserve teams; national teams are left out, use team_index for them"`
	TeamIndex           *int64             `json:"team_index,omitempty" jsonschema:"only players of this team; for a national team, its called-up players"`
	ContractedTeamIndex *int64             `json:"contracted_team_index,omitempty" jsonschema:"only players whose main contract is with this team, including those out on loan"`
	ExcludeTeamIndex    *int64             `json:"exclude_team_index,omitempty" jsonschema:"leave out players of this team, e.g. your own"`
	Nation              string             `json:"nation,omitempty" jsonschema:"part of the nation name"`
	NationIndex         *int64             `json:"nation_index,omitempty"`
	MaxValue            int64              `json:"max_value,omitempty" jsonschema:"in the currency game_overview reports"`
	MaxWage             int64              `json:"max_wage,omitempty" jsonschema:"in the currency and per the wage_period game_overview reports"`
	MaxReleaseClause    int64              `json:"max_release_clause,omitempty" jsonschema:"lowest minimum fee release clause at most this, in game_overview's currency; clause kinds (e.g. foreign clubs only) are in get_player's contract clauses"`
	ContractExpiresBy   string             `json:"contract_expires_by,omitempty" jsonschema:"YYYY-MM-DD; players whose main contract ends on or before this date"`
	ExcludeInjured      bool               `json:"exclude_injured,omitempty"`
	FreeAgentsOnly      bool               `json:"free_agents_only,omitempty" jsonschema:"players without a team"`
	MinMonthsAtClub     int                `json:"min_months_at_club,omitempty" jsonschema:"joined their current club at least this many months ago; players without joined_club (loanees, free agents) are left out"`
	MinSeasonMinutes    int                `json:"min_season_minutes,omitempty" jsonschema:"minimum minutes this season in club competitions, e.g. 900 to keep players with few matches out of a season_average_rating sort; 1 lifts the per-90 default of 450"`
	MinAttributes       map[string]int     `json:"min_attributes,omitempty" jsonschema:"minimum 1-20 attribute values, e.g. {\"Pace\": 15}"`
	MinPer90            map[string]float64 `json:"min_per90,omitempty" jsonschema:"minimum club stats per 90 minutes by season_detail name, e.g. {\"non_penalty_expected_goals\": 0.3}; leaves out players under 450 minutes unless min_season_minutes is set"`
	Traits              []string           `json:"traits,omitempty" jsonschema:"FM traits the player must all have, e.g. [\"Cuts Inside From Both Wings\"]"`
	Personalities       []string           `json:"personalities,omitempty" jsonschema:"FM personality labels; the player must have one, e.g. [\"Model Professional\", \"Professional\"]"`
	Interest            string             `json:"interest,omitempty" jsonschema:"transfer, loan or transfer_or_loan: leave out players who certainly won't join your club (FM's Player's interest). Kept rows carry interest (likely or uncertain) with transfer_factors and loan_factors. Uncertain players are kept."`
	SortBy              string             `json:"sort_by,omitempty" jsonschema:"current_ability (default), potential_ability, age, reputation, value, wage, role_ability (with role; the default then), an attribute name, or a season stat (club competitions): season_goals, season_assists, season_appearances, season_minutes, season_average_rating (players without a rated match come last), season_player_of_the_match, or per90_ and a season_detail stat name (club stats per 90 minutes, e.g. per90_progressive_passes; leaves out players under 450 minutes unless min_season_minutes is set)"`
	Ascending           bool               `json:"ascending,omitempty"`
	Limit               int                `json:"limit,omitempty" jsonschema:"default 25, max 200"`
}

func searchPlayers(v fm.View, in searchIn) (searchOut, error) {
	res, err := v.Search(fm.Filter(in))
	if err != nil {
		return searchOut{}, err
	}
	return searchOut{
		Players: summarize(res.Players), Total: res.Total, NeedsScouting: res.NeedsScouting, Hint: res.Hint,
		NationalTeams:   res.NationalTeams[:min(len(res.NationalTeams), maxNationalTeams)],
		InterestDropped: res.InterestDropped,
	}, nil
}

// DepthPlayer is a player's fit for one position.
type DepthPlayer struct {
	UID            uint32      `json:"uid"`
	Name           string      `json:"name"`
	Age            int         `json:"age"`
	Rating         int         `json:"rating"`
	Knowledge      *int        `json:"knowledge,omitempty"`       // with fog of war on
	Scouting       fm.Scouting `json:"scouting,omitempty"`        // with fog of war on
	CurrentAbility *int        `json:"current_ability,omitempty"` // left out when fog of war hides it
}

// DepthPosition lists the players who can play a position.
type DepthPosition struct {
	Position string        `json:"position"`
	Players  []DepthPlayer `json:"players"`
}

type depthOut struct {
	saveNotes
	TeamIndex int64           `json:"team_index"`
	Positions []DepthPosition `json:"positions"`
}

func squadDepth(v fm.View, in squadIn) (depthOut, error) {
	club, err := in.club(v)
	if err != nil {
		return depthOut{}, err
	}
	depth, err := v.Depth(club)
	if err != nil {
		return depthOut{}, err
	}
	out := depthOut{TeamIndex: club}
	for i, d := range depth {
		pos := DepthPosition{Position: d.Position, Players: make([]DepthPlayer, len(d.Players))}
		for j, p := range d.Players {
			pos.Players[j] = DepthPlayer{UID: p.UID, Name: p.Name, Age: p.Age, Rating: p.Positions[i].Rating, Knowledge: p.Knowledge, Scouting: p.Scouting, CurrentAbility: p.CurrentAbility}
		}
		out.Positions = append(out.Positions, pos)
	}
	return out, nil
}

type tacticOut struct {
	saveNotes
	fm.Tactic
}

func getTactic(v fm.View, _ empty) (tacticOut, error) {
	t, ok := v.Tactic()
	if !ok {
		return tacticOut{}, errors.New("no tactic for the manager's team in this save; game_overview's warnings say whether it could not be read")
	}
	return tacticOut{Tactic: t}, nil
}

type lastLineupOut struct {
	saveNotes
	fm.LastLineup
}

func getLastLineup(v fm.View, in squadIn) (lastLineupOut, error) {
	team, err := in.club(v)
	if err != nil {
		return lastLineupOut{}, err
	}
	l, err := v.LastLineup(team)
	if errors.Is(err, fm.ErrLastLineupsUnavailable) {
		return lastLineupOut{}, errors.New("the save's last-match lineups could not be read; game_overview's warnings say why")
	}
	if err != nil {
		return lastLineupOut{}, err
	}
	return lastLineupOut{LastLineup: l}, nil
}

type teamsOut struct {
	saveNotes
	Teams []*fm.Team `json:"teams"`
}

func findClubs(v fm.View, in findIn) (teamsOut, error) {
	teams, err := v.FindTeams(in.Name, in.Limit)
	if err != nil {
		return teamsOut{}, err
	}
	return teamsOut{Teams: teams}, nil
}

// tableIn is get_table's input and starts get_fixtures'.
type tableIn struct {
	TeamIndex        *int64 `json:"team_index,omitempty" jsonschema:"team_index from find_clubs; with no competition, the human manager's first team when empty"`
	Competition      string `json:"competition,omitempty" jsonschema:"competition name, e.g. 'Premier League'"`
	CompetitionIndex *int64 `json:"competition_index,omitempty" jsonschema:"competition_index from a fixture or an ambiguous-name error; wins over competition"`
}

// competition returns the competition index given, else the one named;
// nil when neither is set.
func (in tableIn) competition(v fm.View) (*int64, error) {
	if in.CompetitionIndex != nil {
		return in.CompetitionIndex, nil
	}
	if in.Competition == "" {
		return nil, nil //nolint:nilnil // no competition filter
	}
	i, err := v.FindCompetition(in.Competition)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

type fixturesIn struct {
	tableIn
	From   string `json:"from,omitempty" jsonschema:"first date, YYYY-MM-DD"`
	To     string `json:"to,omitempty" jsonschema:"last date, YYYY-MM-DD"`
	Status string `json:"status,omitempty" jsonschema:"played or upcoming; both when empty"`
	Limit  int    `json:"limit,omitempty" jsonschema:"per list; default 10, max 200"`
}

type fixturesOut struct {
	saveNotes
	fm.FixtureList
}

func getFixtures(v fm.View, in fixturesIn) (fixturesOut, error) {
	comp, err := in.competition(v)
	if err != nil {
		return fixturesOut{}, err
	}
	from, err := parseDay("from", in.From)
	if err != nil {
		return fixturesOut{}, err
	}
	to, err := parseDay("to", in.To)
	if err != nil {
		return fixturesOut{}, err
	}
	list, err := v.Fixtures(fm.FixtureFilter{Team: in.TeamIndex, Competition: comp, From: from, To: to, Status: in.Status, Limit: in.Limit})
	if errors.Is(err, fm.ErrFixturesUnavailable) {
		return fixturesOut{}, errors.New("the save's fixtures could not be read; game_overview's warnings say why")
	}
	if err != nil {
		return fixturesOut{}, err
	}
	return fixturesOut{FixtureList: list}, nil
}

// parseDay parses a YYYY-MM-DD input named field; zero when empty.
func parseDay(field, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD, got %q", field, s)
	}
	return t, nil
}

type tablesOut struct {
	saveNotes
	Tables []fm.Table `json:"tables"`
}

func getTable(v fm.View, in tableIn) (tablesOut, error) {
	comp, err := in.competition(v)
	if err != nil {
		return tablesOut{}, err
	}
	tables, err := v.Tables(in.TeamIndex, comp)
	if errors.Is(err, fm.ErrTablesUnavailable) {
		return tablesOut{}, errors.New("the save's league tables could not be read; game_overview's warnings say why")
	}
	if err != nil {
		return tablesOut{}, err
	}
	return tablesOut{Tables: tables}, nil
}
