package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addPrompts(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "squad_review",
		Description: "Review the manager's squad: depth by position, injuries, contracts and priorities.",
	}, textPrompt(func(map[string]string) (string, error) {
		return `Act as my assistant manager and review my squad.
1. Call game_overview, then get_squad and squad_depth for my team.
2. Assess each position: starter quality (current ability), depth, and positions with fewer than two capable players.
3. List injured players and when they are expected back.
4. List contracts ending within 12 months of the game date, with age, current/potential ability, wage and value.
5. Point out the highest-potential young players who should get minutes.
Finish with my three most urgent priorities.`, nil
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "scout_position",
		Description: "Find and compare transfer targets for a position.",
		Arguments: []*mcp.PromptArgument{
			{Name: "position", Description: "GK SW DL DC DR DM ML MC MR AML AMC AMR ST WBL WBR", Required: true},
			{Name: "max_age", Description: "oldest acceptable age"},
			{Name: "max_value", Description: "budget per player in the currency game_overview reports"},
		},
	}, textPrompt(func(args map[string]string) (string, error) {
		pos := args["position"]
		if pos == "" {
			return "", errors.New("position is required")
		}
		return fmt.Sprintf(`Act as my chief scout and find a %[1]s for my team.
1. Call get_squad for my team to see who plays %[1]s now and how good they are.
2. Call search_players with position %[1]s and exclude_team_index set to my team%[2]s, sorted by current_ability; run it again sorted by potential_ability for younger options.
3. Pick the five most promising candidates and call compare_players on them.
Recommend a first choice and a cheaper alternative, with the attributes and contract details (wage, contract end) that support each.
If fog of war hides ability, value or wage, judge on what is visible (attributes, reputation) and list the players who need scouting.`,
			pos, optionalFilters(args, "max_age", "max_value")), nil
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "contract_review",
		Description: "Decide whom to renew, sell or release among expiring contracts.",
		Arguments: []*mcp.PromptArgument{
			{Name: "expires_by", Description: "YYYY-MM-DD; defaults to one year after the game date"},
		},
	}, textPrompt(func(args map[string]string) (string, error) {
		by := cmp.Or(args["expires_by"], "one year after the game date from game_overview")
		return fmt.Sprintf(`Act as my director of football and review contracts.
1. Call game_overview, then search_players with contracted_team_index set to my manager_team_index (this includes players out on loan), contract_expires_by %s and limit 100.
2. For each player weigh age, current and potential ability, position depth (squad_depth), wage and value.
3. Recommend renew, sell now, or let go for each, and flag release clauses from get_player.`, by), nil
	}))

	srv.AddPrompt(&mcp.Prompt{
		Name:        "scout_opponent",
		Description: "Scout an opponent: form, likely lineup and players to watch (FM27 saves).",
		Arguments: []*mcp.PromptArgument{
			{Name: "opponent", Description: "club name, or team_index (a non-negative number is team_index); empty means my next opponent"},
		},
	}, textPrompt(func(args map[string]string) (string, error) {
		subject, step := opponentTarget(args["opponent"])
		return fmt.Sprintf(`Act as my opposition analyst and scout %s.
1. Call game_overview and pass on its warnings.
2. %s
3. Call get_fixtures with the opponent's team_index, status played and limit 5.
4. Call get_match with the match_id of each result that has one: every such match lists its goals; match_data stored adds players (rating, xG) and team_stats.
5. Call get_last_lineup with the opponent's team_index for formation, roles and who played.
6. Pick three to five key players (goals, assists, high ratings). Call get_player_matches with their uid from get_match; skip unnamed players, which have no uid.
7. Call get_tactic for my current tactic.
Report its form (results, goals for and against), how it plays (formation, team_stats), likely lineup, players to watch, and what my tactic should change to face them.
Matches with match_data pruned or goals_only have goals only; quick_simulated ones only the score. Name them. What is not stored is unknown, not zero.`,
			subject, step), nil
	}))
}

// opponentTarget returns the subject and the step that finds the opponent's
// team_index. A non-negative number is a team_index, anything else a name.
func opponentTarget(opponent string) (subject, step string) {
	opponent = strings.TrimSpace(opponent)
	if opponent == "" {
		return "my next opponent", "Call get_fixtures for my team with status upcoming and limit 1; the opponent is the other team. If there is none, ask me for the opponent."
	}
	if n, err := strconv.ParseInt(opponent, 10, 64); err == nil && n >= 0 {
		team := "team_index " + strconv.FormatInt(n, 10)
		return team, "The opponent is " + team + "."
	}
	return opponent, fmt.Sprintf(`Call find_clubs with name %q for the opponent's team_index; prefer a club's first team (no U19, U21 or B in the name) and ask me only if several clubs match.`, opponent)
}

func textPrompt(render func(map[string]string) (string, error)) mcp.PromptHandler {
	return func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		text, err := render(req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}, nil
	}
}

func optionalFilters(args map[string]string, names ...string) string {
	var b strings.Builder
	for _, n := range names {
		if v := args[n]; v != "" {
			fmt.Fprintf(&b, ", %s %s", n, v)
		}
	}
	return b.String()
}
