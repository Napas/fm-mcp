package fm

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// Profile is a player as the manager sees him. With fog of war on it carries
// his knowledge; values his scouting does not cover are nil (0 for
// attributes) and left out of the JSON. Its injuries, positions, national
// team index, season and favoured clubs are shared with the game: treat them
// as read-only.
type Profile struct {
	PlayerBasics
	// ReputationLevel is FM's reputation word, e.g. "National"; fog of war
	// never hides it. Empty when it is not computed (see World.reputationLevel).
	ReputationLevel  string     `json:"reputation_level,omitempty"`
	Knowledge        *int       `json:"knowledge,omitempty"` // 0..100, with fog of war on
	Scouting         Scouting   `json:"scouting,omitempty"`  // with fog of war on
	CurrentAbility   *int       `json:"current_ability,omitempty"`
	PotentialAbility *int       `json:"potential_ability,omitempty"`
	Attributes       Attributes `json:"attributes,omitzero"` // 1..20
	// Personality is FM's personality label, computed from the hidden
	// attributes; left out when they are hidden.
	Personality string `json:"personality,omitempty"`
	// MediaHandling is FM's media handling styles, computed from the hidden
	// attributes; left out with the personality.
	MediaHandling []string `json:"media_handling,omitempty"`
	// FavouredClubs are in save order; left out with the personality.
	FavouredClubs []FavouredClub `json:"favoured_clubs,omitempty"`
	// Traits are left out when attributes are hidden.
	Traits         []string      `json:"traits,omitempty"`
	Value          *int64        `json:"value,omitempty"` // in the user's currency
	NotForSale     bool          `json:"not_for_sale,omitempty"`
	Contract       *ContractInfo `json:"contract,omitempty"`        // nil for free agents
	Loan           *ContractInfo `json:"loan,omitempty"`            // set while on loan at TeamIndex
	FutureContract *ContractInfo `json:"future_contract,omitempty"` // pre-agreed move
	// PlayingTime is FM's Playing Time: the agreed squad status, shown in
	// squad lists and as "Agreed" on the contract screen. OriginalPlayingTime
	// was agreed at signing; the game does not show it. ActualPlayingTime is
	// the contract screen's headline. Left out when unknown or hidden by fog
	// of war, like the wage.
	PlayingTime         string `json:"playing_time,omitempty"`
	OriginalPlayingTime string `json:"original_playing_time,omitempty"`
	ActualPlayingTime   string `json:"actual_playing_time,omitempty"`
	// LoyaltyBonusRemaining is the main contract's unpaid loyalty bonus, in
	// the user's currency; nil when none or hidden.
	LoyaltyBonusRemaining *int64 `json:"loyalty_bonus_remaining,omitempty"`
	// SeasonDetail lists this season's totals per competition group. Fog of
	// war never hides it.
	SeasonDetail []SeasonDetail `json:"season_detail,omitempty"`
	// Interest (a single-player profile, or a search row with
	// Filter.Interest) is the verdict on whether the player would join the
	// manager's club; nil for players it does not evaluate.
	Interest *Interest `json:"interest,omitempty"`
	// RoleAbility (Search only) is the role ability in the search's role and
	// position; nil when the current ability is hidden.
	RoleAbility *int `json:"role_ability,omitempty"`
	// Per90 (Search only) holds the club per-90 rates the search sorts or
	// filters on.
	Per90 map[string]float64 `json:"per90,omitempty"`
	// BestRoles (Profile only) are the highest role abilities; nil when the
	// current ability is hidden.
	BestRoles []RoleFit `json:"best_roles,omitempty"`
}

// View answers player queries as the manager sees the save. With fog of war
// on, profiles leave out values his scouting does not cover, filters on them
// match only players whose value is visible, and sorts put players with a
// hidden key last, most reputable first.
type View struct {
	g   *Game
	fog bool
}

func (g *Game) View(fogOfWar bool) View { return View{g: g, fog: fogOfWar} }

func (v View) FogOfWar() bool { return v.fog }

func (v View) Info() Info { return v.g.Info }

func (v View) Money() Money { return v.g.Info.Money }

func (v View) PlayerCount() int { return v.g.PlayerCount() }

// Team returns the team with the given index, money in the user's currency.
// Fog of war hides the bank balance of teams outside the manager's club.
func (v View) Team(index int64) (*Team, bool) {
	t, ok := v.g.world.Teams[index]
	if !ok {
		return nil, false
	}
	return v.team(t), true
}

// FindTeams returns teams with a name word starting with each word of
// query, most reputable first, money in the user's currency as Team shows
// it. No words is an ErrInvalidFilter.
func (v View) FindTeams(query string, limit int) ([]*Team, error) {
	teams, err := v.g.findTeams(query, limit)
	for i, t := range teams {
		teams[i] = v.team(t)
	}
	return teams, err
}

func (v View) team(t *Team) *Team {
	out := *t
	out.Budgets = nil // GBP; only Budgets serves them, converted
	out.BankBalance = v.Money().Amount(t.BankBalance)
	if v.fog && !v.g.world.sameClub(t.Index, v.g.Info.ManagerTeamIndex) {
		out.BankBalance = 0
	}
	return &out
}

// Budgets returns the manager's club's budgets in the user's currency and
// wage period. Fog of war does not hide them. Nil when the manager's team is
// unknown or its club has no full finances.
func (v View) Budgets() *Budgets {
	t, ok := v.g.world.Teams[v.g.Info.ManagerTeamIndex]
	if !ok || t.Budgets == nil {
		return nil
	}
	m := v.Money()
	return &Budgets{
		TransferBudget: m.Amount(t.Budgets.TransferBudget),
		WageBudget:     m.Wage(t.Budgets.WageBudget),
	}
}

// seen is a player with what the viewer knows of him.
type seen struct {
	p *Player
	s sight
}

func (v View) see(p *Player) seen {
	if !v.fog {
		return seen{p, fullKnowledge}
	}
	return seen{p, sight(p.knowledge)}
}

// Profile returns the player with the given unique ID.
func (v View) Profile(uid uint32) (Profile, bool) {
	p, ok := v.g.byUID[uid]
	if !ok {
		return Profile{}, false
	}
	x := v.see(p)
	out := v.profile(x)
	out.Interest = v.g.interestOf(p)
	if x.s.full() {
		out.BestRoles = v.g.auxOf(p).roles.bestRoles()
	}
	return out, true
}

func (v View) profile(x seen) Profile {
	p, s := x.p, x.s
	out := Profile{PlayerBasics: p.PlayerBasics, SeasonDetail: seasonDetails(v.g.auxOf(p).season, p.hasKeeperStats()), ReputationLevel: v.g.world.reputationLevel(p.PlayerBasics)}
	if v.fog {
		out.Knowledge, out.Scouting = ptr(int(s)), s.scouting()
	}
	if s.full() {
		out.CurrentAbility, out.PotentialAbility = ptr(p.CurrentAbility), ptr(p.PotentialAbility)
		out.PlayingTime = playingTimeName(p.playingTime)
		out.OriginalPlayingTime = playingTimeName(p.originalPlayingTime)
		out.ActualPlayingTime = playingTimeName(p.actualPlayingTime)
		out.Personality, out.MediaHandling = p.personality(), p.mediaHandling()
		out.FavouredClubs = v.g.auxOf(p).favoured
		if p.LoyaltyBonusRemaining > 0 {
			out.LoyaltyBonusRemaining = ptr(v.Money().Amount(p.LoyaltyBonusRemaining))
		}
	}
	for i, a := range p.Attributes {
		if s.attribute(i) {
			out.Attributes[i] = a
		}
	}
	if s.partial() {
		out.NotForSale = p.NotForSale
		out.Traits = traitNames(p.traits)
	}
	if s.value(p) {
		out.Value = ptr(v.Money().Amount(p.Value))
	}
	out.Contract, out.Loan, out.FutureContract = v.contract(p.Contract, s), v.contract(p.Loan, s), v.contract(p.FutureContract, s)
	return out
}

// contract converts c to the user's currency and wage period.
func (v View) contract(c *Contract, s sight) *ContractInfo {
	if c == nil {
		return nil
	}
	out := &ContractInfo{TeamIndex: c.TeamIndex, Team: c.Team, Start: c.Start, Joined: c.Joined, End: c.End}
	if !s.full() {
		return out
	}
	m := v.Money()
	out.Wage = ptr(m.Wage(c.WeeklyWage))
	for _, cl := range c.Clauses {
		cl.Amount = m.Amount(cl.Amount)
		out.Clauses = append(out.Clauses, cl)
	}
	return out
}

func (v View) profiles(xs []seen) []Profile {
	out := make([]Profile, len(xs))
	for i, x := range xs {
		out[i] = v.profile(x)
	}
	return out
}

// FindPlayers returns players with a name word starting with each word of
// query, ignoring case and accents, most reputable first. No words is an
// ErrInvalidFilter.
func (v View) FindPlayers(query string, limit int) ([]Profile, error) {
	words, err := queryWords("name", query)
	if err != nil {
		return nil, err
	}
	out := v.where(func(x seen) bool { return matchesNameStarts(x.p.names, words) })
	slices.SortStableFunc(out, func(a, b seen) int { return cmp.Compare(b.p.Reputation.Current, a.p.Reputation.Current) })
	return v.profiles(truncate(out, limit)), nil
}

// Squad returns the players of teamIndex, highest current ability first: a
// club team's current players, or a national team's called-up players. A
// squad number belongs to the current contract's team, so rows of players
// contracted to another team leave it out: a first-team player at the
// reserves, and every row of a national team. An unknown team is an
// ErrInvalidFilter.
func (v View) Squad(teamIndex int64) ([]Profile, error) {
	if err := v.g.checkTeam(teamIndex); err != nil {
		return nil, err
	}
	xs := v.bestFirst(func(p *Player) bool { return p.plays(teamIndex) })
	out := v.profiles(xs)
	for i, x := range xs {
		c := x.p.Loan
		if c == nil {
			c = x.p.Contract
		}
		if c == nil || c.TeamIndex != teamIndex {
			out[i].SquadNumber = 0
		}
	}
	return out, nil
}

// LoanedOut returns the players contracted to teamIndex who are out on
// loan, highest current ability first, with the main contract's squad
// number: the one teamIndex's squad-number screen shows. An unknown team is
// an ErrInvalidFilter.
func (v View) LoanedOut(teamIndex int64) ([]Profile, error) {
	if err := v.g.checkTeam(teamIndex); err != nil {
		return nil, err
	}
	xs := v.bestFirst(func(p *Player) bool {
		return p.Loan != nil && p.Contract != nil && p.Contract.TeamIndex == teamIndex
	})
	out := v.profiles(xs)
	for i, x := range xs {
		out[i].SquadNumber = x.p.Contract.SquadNumber
	}
	return out, nil
}

func (v View) bestFirst(keep func(*Player) bool) []seen {
	out := v.where(func(x seen) bool { return keep(x.p) })
	sortByKey(out, currentAbility, false)
	return out
}

func (v View) where(keep func(seen) bool) []seen {
	var out []seen
	for _, p := range v.g.players {
		if x := v.see(p); keep(x) {
			out = append(out, x)
		}
	}
	return out
}

type PositionDepth struct {
	Position string    `json:"position"`
	Players  []Profile `json:"players"`
}

// Depth returns, for every position, the players at teamIndex who can play
// there, best rated first and then by current ability. An unknown team is an
// ErrInvalidFilter.
func (v View) Depth(teamIndex int64) ([]PositionDepth, error) {
	squad, err := v.Squad(teamIndex)
	if err != nil {
		return nil, err
	}
	out := make([]PositionDepth, len(savefile.PositionNames))
	for i, pos := range savefile.PositionNames {
		out[i].Position = pos
		for _, p := range squad {
			if p.Positions[i].Rating >= naturalRating {
				out[i].Players = append(out[i].Players, p)
			}
		}
		slices.SortStableFunc(out[i].Players, func(a, b Profile) int {
			return cmp.Compare(b.Positions[i].Rating, a.Positions[i].Rating)
		})
	}
	return out, nil
}

// InterestDropped counts the players Filter.Interest left out, by the code
// of the reason.
type InterestDropped struct {
	Total    int            `json:"total"`
	ByReason map[string]int `json:"by_reason"`
}

// SearchResult is a page of Search matches.
type SearchResult struct {
	Players []Profile
	Total   int // matches before the limit
	// NeedsScouting counts players who pass every filter the viewer can
	// judge, but fog of war hides a value another filter needs. They may
	// match once scouted.
	NeedsScouting int
	// InterestDropped counts the players who passed every filter but are out
	// for the interest kind; nil without Filter.Interest.
	InterestDropped *InterestDropped
	// NationalTeams have call-ups and match the club and gender filters.
	// Their players are not searched. Most reputable first.
	NationalTeams []*Team
	// Hint explains an empty club search: the club matched only teams
	// without players, or only teams of the other gender.
	Hint string
}

// Search returns the players matching f (up to f.Limit), the total, the
// scouting count and the matching national teams it does not search.
// Position, attribute and sort names ignore case.
func (v View) Search(f Filter) (SearchResult, error) {
	q, err := v.g.compile(f)
	if err != nil {
		return SearchResult{}, err
	}
	var res SearchResult
	for _, t := range q.national {
		res.NationalTeams = append(res.NationalTeams, v.team(t))
	}
	var out []seen
	if q.interest != "" {
		res.InterestDropped = &InterestDropped{ByReason: map[string]int{}}
	}
	for _, p := range v.g.players {
		x := v.see(p)
		verdict := q.judge(x)
		if verdict == rejected {
			continue
		}
		if q.interest != "" {
			if c, ok := v.g.judgeInterest(p); ok && c.verdicts.For(q.interest) == VerdictOut {
				if verdict == matched {
					res.InterestDropped.Total++
					res.InterestDropped.ByReason[c.reasonCode(q.interest)]++
				}
				continue
			}
		}
		if verdict == matched {
			out = append(out, x)
		} else {
			res.NeedsScouting++
		}
	}
	sortByKey(out, q.key, f.Ascending)
	page := truncate(out, f.Limit)
	res.Players, res.Total = v.profiles(page), len(out)
	if q.interest != "" {
		for i, x := range page {
			res.Players[i].Interest = v.g.interestOf(x.p)
		}
	}
	if q.roleAbility != nil {
		for i, x := range page {
			if a, ok := q.roleAbility(x.p, x.s); ok {
				res.Players[i].RoleAbility = &a
			}
		}
	}
	for i, x := range page {
		res.Players[i].Per90 = v.g.per90Row(x.p, q.per90)
	}
	if res.Total == 0 && res.NeedsScouting == 0 {
		res.Hint = v.g.emptyClubHint(q)
	}
	return res, nil
}

// emptyClubHint explains why a club filter found no players; "" when the
// club is not the cause. A gender mismatch takes precedence.
func (g *Game) emptyClubHint(q query) string {
	if q.Club == "" {
		return ""
	}
	teams := g.teamsByReputation(q.teams)
	if q.women != nil && !slices.ContainsFunc(teams, func(t *Team) bool { return t.Women == *q.women }) {
		gender, teamSide, suggest := "men", "women's", "women"
		if *q.women {
			gender, teamSide, suggest = "women", "men's", "men"
		}
		return fmt.Sprintf("club %q matched only %s teams (%s), but gender is %s. Set gender to %s or any.",
			q.Club, teamSide, teamList(teams), gender, suggest)
	}
	if slices.ContainsFunc(g.players, func(p *Player) bool { return q.teams[p.TeamIndex] }) {
		return ""
	}
	return fmt.Sprintf("club %q matched only teams with no players (%s). Use find_clubs with the full name; nicknames (PSG, Barça) are not in the save.",
		q.Club, teamList(teams))
}

// maxErrorTeams caps the teams named in the unknown-club error.
const maxErrorTeams = 3

// teamList names up to maxErrorTeams teams with team_index and counts the
// rest.
func teamList(teams []*Team) string {
	shown := teams[:min(len(teams), maxErrorTeams)]
	names := make([]string, 0, len(shown)+1)
	for _, t := range shown {
		names = append(names, fmt.Sprintf("%s (team_index %d)", t.Name, t.Index))
	}
	if rest := len(teams) - len(shown); rest > 0 {
		names = append(names, fmt.Sprintf("%d more", rest))
	}
	return strings.Join(names, ", ")
}

// exampleTeams lists teams as an error suffix.
func exampleTeams(teams []*Team) string {
	if len(teams) == 0 {
		return ""
	}
	return ", e.g. " + teamList(teams)
}

// query is a validated Filter.
type query struct {
	Filter
	money     Money
	pos       int         // position index, -1 for any
	minAttrs  map[int]int // attribute index → minimum
	women     *bool
	expiresBy time.Time
	joinedBy  time.Time      // latest joined_club date MinMonthsAtClub allows
	teams     map[int64]bool // allowed club team indexes; nil for any
	national  []*Team        // national teams with call-ups matching the club and gender filters
	nation    string
	key       sortKeyFunc
	// roleAbility is the role ability in the role filter's role; nil
	// without one.
	roleAbility sortKeyFunc
	interest    InterestKind       // "" without the interest option
	minPer90    func(*Player) bool // nil without MinPer90
	minMinutes  int                // MinSeasonMinutes, else per90DefaultMinutes in a per-90 search
	// per90 lists the per-90 stats rows carry: the sort's, then MinPer90's.
	per90         []string
	traits        uint64   // Traits as savefile.TraitNames bits
	personalities []string // Personalities as personalityLabels spell them
}

func (g *Game) compile(f Filter) (query, error) {
	q := query{Filter: f, money: g.Info.Money, pos: -1, minAttrs: make(map[int]int, len(f.MinAttributes)), nation: nationKey(f.Nation)}
	if f.Position != "" {
		q.pos = slices.Index(savefile.PositionNames[:], strings.ToUpper(f.Position))
		if q.pos < 0 {
			return query{}, fmt.Errorf("%w: position %q; valid codes: %s", ErrInvalidFilter, f.Position, strings.Join(savefile.PositionNames[:], " "))
		}
	}
	for name, v := range f.MinAttributes {
		i, ok := attributeOf(name)
		if !ok {
			return query{}, fmt.Errorf("%w: attribute %q; valid names: %s", ErrInvalidFilter, name, strings.Join(attributeNames[:], ", "))
		}
		if v < 1 || v > 20 {
			return query{}, fmt.Errorf("%w: %s minimum %d is outside 1-20", ErrInvalidFilter, attributeNames[i], v)
		}
		q.minAttrs[i] = v
	}
	if err := g.checkRefs(f); err != nil {
		return query{}, err
	}
	switch k := InterestKind(f.Interest); k {
	case "":
	case InterestTransfer, InterestLoan, InterestTransferOrLoan:
		if g.targetReputation() == 0 {
			return query{}, fmt.Errorf("%w: interest needs your club, but the manager's team is unknown", ErrInvalidFilter)
		}
		q.interest = k
	default:
		return query{}, fmt.Errorf("%w: interest %q; valid values: %s, %s, %s", ErrInvalidFilter, f.Interest, InterestTransfer, InterestLoan, InterestTransferOrLoan)
	}
	var err error
	if q.women, err = genderFilter(f.Gender); err != nil {
		return query{}, err
	}
	if f.Club != "" {
		words, err := queryWords("club", f.Club)
		if err != nil {
			return query{}, err
		}
		q.teams, q.national = g.clubTeamsNamed(words)
		if q.women != nil {
			q.national = slices.DeleteFunc(q.national, func(t *Team) bool { return t.Women != *q.women })
		}
		if len(q.teams) == 0 {
			return query{}, fmt.Errorf("%w: no club team matches %q; use find_clubs, or team_index for a national team%s", ErrInvalidFilter, f.Club, exampleTeams(q.national))
		}
	}
	if f.Gender == "" && f.TeamIndex == nil && f.ContractedTeamIndex == nil && f.Club == "" {
		q.women = g.managerWomen()
	}
	if f.ContractExpiresBy != "" {
		if q.expiresBy, err = time.Parse(time.DateOnly, f.ContractExpiresBy); err != nil {
			return query{}, fmt.Errorf("%w: contract_expires_by %q is not YYYY-MM-DD", ErrInvalidFilter, f.ContractExpiresBy)
		}
	}
	switch {
	case f.MinMonthsAtClub < 0 || f.MinMonthsAtClub > maxMonthsAtClub:
		return query{}, fmt.Errorf("%w: min_months_at_club %d is outside 0-%d", ErrInvalidFilter, f.MinMonthsAtClub, maxMonthsAtClub)
	case f.MinSeasonMinutes < 0:
		return query{}, fmt.Errorf("%w: min_season_minutes %d is negative", ErrInvalidFilter, f.MinSeasonMinutes)
	case f.MinMonthsAtClub > 0:
		q.joinedBy = monthsBefore(g.Info.Date, f.MinMonthsAtClub)
	}
	if f.Role != "" {
		if q.roleAbility, err = g.roleAbilityKey(f.Role, q.pos); err != nil {
			return query{}, err
		}
	}
	if err := compileProfileFilters(&q); err != nil {
		return query{}, err
	}
	if err := g.compilePer90(&q); err != nil {
		return query{}, err
	}
	if q.key == nil {
		if q.key, err = searchKey(f.SortBy, q.roleAbility); err != nil {
			return query{}, err
		}
	}
	if _, bySeason := seasonKeys[strings.ToLower(f.SortBy)]; !g.hasSeason && (f.MinSeasonMinutes > 0 || bySeason || len(q.per90) > 0) {
		return query{}, fmt.Errorf("%w: season stats could not be read (game_overview's warnings say why); drop the season_* and per90_* sort_by, min_season_minutes and min_per90", ErrInvalidFilter)
	}
	return q, nil
}

// maxMonthsAtClub caps MinMonthsAtClub at 100 years.
const maxMonthsAtClub = 1200

// monthsBefore is the date n months before t, on the last day of that month
// when t's day is past it.
func monthsBefore(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()-time.Month(n), 1, 0, 0, 0, 0, t.Location())
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

// compileProfileFilters validates the height, trait, personality and role
// ability filters.
func compileProfileFilters(q *query) error {
	switch {
	case q.MinHeightCM < 0:
		return fmt.Errorf("%w: min_height_cm %d is negative", ErrInvalidFilter, q.MinHeightCM)
	case q.MaxHeightCM < 0:
		return fmt.Errorf("%w: max_height_cm %d is negative", ErrInvalidFilter, q.MaxHeightCM)
	case q.MinRoleAbility > 0 && q.roleAbility == nil:
		return fmt.Errorf("%w: min_role_ability needs a role and a position", ErrInvalidFilter)
	}
	for _, name := range q.Traits {
		bit := slices.IndexFunc(savefile.TraitNames[:], func(n string) bool { return n != "" && strings.EqualFold(n, name) })
		if bit < 0 {
			return fmt.Errorf("%w: trait %q; valid names: %s", ErrInvalidFilter, name, strings.Join(traitNames(math.MaxUint64), ", "))
		}
		q.traits |= 1 << bit
	}
	for _, name := range q.Personalities {
		i := slices.IndexFunc(personalityLabels, func(l string) bool { return strings.EqualFold(l, name) })
		if i < 0 {
			return fmt.Errorf("%w: personality %q; valid labels: %s", ErrInvalidFilter, name, strings.Join(personalityLabels, ", "))
		}
		q.personalities = append(q.personalities, personalityLabels[i])
	}
	return nil
}

// compilePer90 validates the per90_ sort and MinPer90, and sets minMinutes:
// MinSeasonMinutes, else per90DefaultMinutes when either is set.
func (g *Game) compilePer90(q *query) error {
	if name, ok := strings.CutPrefix(strings.ToLower(q.SortBy), per90Prefix); ok {
		field, err := per90Field(name)
		if err != nil {
			return err
		}
		q.key, q.per90 = g.per90Key(field), append(q.per90, field)
	}
	minimums := make(map[string]float64, len(q.MinPer90))
	for _, name := range slices.Sorted(maps.Keys(q.MinPer90)) {
		field, err := per90Field(name)
		if err != nil {
			return err
		}
		minimums[field] = q.MinPer90[name]
		if !slices.Contains(q.per90, field) {
			q.per90 = append(q.per90, field)
		}
	}
	if len(minimums) > 0 {
		q.minPer90 = func(p *Player) bool { return g.meetsPer90(p, minimums) }
	}
	if q.minMinutes = q.MinSeasonMinutes; len(q.per90) > 0 && q.minMinutes == 0 {
		q.minMinutes = per90DefaultMinutes
	}
	return nil
}

// roleAbilityKey returns the sort key of role ability in the named
// in-possession role at position index pos, which must be set (≥ 0) and
// offer the role. Fog of war hides it with the current ability.
func (g *Game) roleAbilityKey(name string, pos int) (sortKeyFunc, error) {
	role, ok := inPossessionRoleNamed(name)
	if !ok {
		return nil, fmt.Errorf("%w: role %q; valid in-possession roles: %s", ErrInvalidFilter, name, inPossessionRoleNames(offeredAnywhere))
	}
	if pos < 0 {
		return nil, fmt.Errorf("%w: role %q needs a position, e.g. DC", ErrInvalidFilter, name)
	}
	code, offered := savefile.PositionNames[pos], positionRoles[pos]
	if offered == 0 {
		return nil, fmt.Errorf("%w: no role is offered at %s", ErrInvalidFilter, code)
	}
	if offered&role == 0 {
		return nil, fmt.Errorf("%w: role %q is not offered at %s; roles at %s: %s", ErrInvalidFilter, name, code, code, inPossessionRoleNames(offered))
	}
	at := placeAt(pos)
	return func(p *Player, s sight) (int, bool) { return g.roleAbility(p, role, at), s.full() }, nil
}

const sortByRoleAbility = "role_ability"

// searchKey returns the sort key named name: roleAbility (nil without a role
// filter) for sortByRoleAbility, and by default when it is set; else sortKey.
func searchKey(name string, roleAbility sortKeyFunc) (sortKeyFunc, error) {
	switch n := strings.ToLower(name); {
	case n == sortByRoleAbility && roleAbility == nil:
		return nil, fmt.Errorf("%w: sort key %q needs a role", ErrInvalidFilter, name)
	case n == sortByRoleAbility, n == "" && roleAbility != nil:
		return roleAbility, nil
	}
	return sortKey(name)
}

// verdict is how a player fares against a query.
type verdict uint8

const (
	rejected verdict = iota
	matched
	unjudged // passes the visible filters; fog of war hides a value another needs
)

// judge checks x against every filter. A filter on a value fog of war hides
// cannot be judged; one on a visible value the save lacks (a free agent's
// wage, an unknown value) fails.
func (q query) judge(x seen) verdict {
	p, s := x.p, x.s
	switch {
	case q.women != nil && p.Female != *q.women,
		q.pos >= 0 && p.Positions[q.pos].Rating < naturalRating,
		q.MinAge > 0 && p.Age < q.MinAge,
		q.MaxAge > 0 && p.Age > q.MaxAge,
		q.TeamIndex != nil && !p.plays(*q.TeamIndex),
		q.ContractedTeamIndex != nil && (p.Contract == nil || p.Contract.TeamIndex != *q.ContractedTeamIndex),
		q.ExcludeTeamIndex != nil && p.plays(*q.ExcludeTeamIndex),
		q.NationIndex != nil && p.NationIndex != *q.NationIndex,
		q.teams != nil && !q.teams[p.TeamIndex],
		q.nation != "" && !strings.Contains(p.nationKey, q.nation),
		!q.expiresBy.IsZero() && (p.Contract == nil || p.Contract.End.IsZero() || p.Contract.End.After(q.expiresBy)),
		q.MinHeightCM > 0 && (p.HeightCM == 0 || p.HeightCM < q.MinHeightCM),
		q.MaxHeightCM > 0 && (p.HeightCM == 0 || p.HeightCM > q.MaxHeightCM),
		q.ExcludeInjured && p.Injured(),
		q.FreeAgentsOnly && p.TeamIndex >= 0,
		!q.joinedBy.IsZero() && (p.JoinedClub == nil || p.JoinedClub.After(q.joinedBy)),
		q.minMinutes > 0 && (p.Season == nil || p.Season.Minutes < q.minMinutes),
		q.minPer90 != nil && !q.minPer90(p):
		return rejected
	}
	var j judgement
	if q.MinCurrentAbility > 0 {
		j.check(s.full(), p.CurrentAbility >= q.MinCurrentAbility)
	}
	if q.MaxCurrentAbility > 0 {
		j.check(s.full(), p.CurrentAbility <= q.MaxCurrentAbility)
	}
	if q.MinPotentialAbility > 0 {
		j.check(s.full(), p.PotentialAbility >= q.MinPotentialAbility)
	}
	if q.MaxValue > 0 {
		j.check(s.partial(), p.hasValue() && q.money.Amount(p.Value) <= q.MaxValue)
	}
	if q.MaxWage > 0 {
		j.check(!s.wageHidden(p), s.wage(p) && q.money.Wage(p.WeeklyWage()) <= q.MaxWage)
	}
	if q.MaxReleaseClause > 0 {
		j.check(s.full() || p.Contract == nil, p.Contract != nil && p.Contract.ReleaseFee > 0 && q.money.Amount(p.Contract.ReleaseFee) <= q.MaxReleaseClause)
	}
	for i, minimum := range q.minAttrs {
		j.check(s.attribute(i), int(p.Attributes[i]) >= minimum)
	}
	if q.traits != 0 {
		j.check(s.partial(), p.traits&q.traits == q.traits)
	}
	if len(q.personalities) > 0 {
		j.check(s.full(), slices.Contains(q.personalities, p.personality()))
	}
	if q.MinRoleAbility > 0 {
		a, visible := q.roleAbility(p, s)
		j.check(visible, a >= q.MinRoleAbility)
	}
	return j.verdict()
}

// judgement collects the results of filters on values fog of war may hide.
type judgement struct{ failed, hidden bool }

// check records one filter; pass is ignored when the value is hidden.
func (j *judgement) check(visible, pass bool) {
	switch {
	case !visible:
		j.hidden = true
	case !pass:
		j.failed = true
	}
}

func (j judgement) verdict() verdict {
	switch {
	case j.failed:
		return rejected
	case j.hidden:
		return unjudged
	}
	return matched
}

// sortKeyFunc returns a player's sort key and whether the viewer sees it
// (false also when he has none).
type sortKeyFunc func(*Player, sight) (int, bool)

func sortKey(name string) (sortKeyFunc, error) {
	switch strings.ToLower(name) {
	case "", "current_ability":
		return currentAbility, nil
	case "potential_ability":
		return func(p *Player, s sight) (int, bool) { return p.PotentialAbility, s.full() }, nil
	case "age":
		return func(p *Player, _ sight) (int, bool) { return p.Age, true }, nil
	case "reputation":
		return func(p *Player, _ sight) (int, bool) { return p.Reputation.Current, true }, nil
	case "value":
		return func(p *Player, s sight) (int, bool) { return int(p.Value), s.value(p) }, nil
	case "wage":
		return func(p *Player, s sight) (int, bool) { return int(p.WeeklyWage()), s.wage(p) }, nil
	}
	if i, ok := attributeOf(name); ok {
		return func(p *Player, s sight) (int, bool) { return int(p.Attributes[i]), s.attribute(i) }, nil
	}
	if k, ok := seasonKeys[strings.ToLower(name)]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: sort key %q", ErrInvalidFilter, name)
}

// seasonKeys sort by Player.Season. Fog of war never hides them. Without a
// match a player counts 0; without a rated match he comes last by average rating.
var seasonKeys = map[string]sortKeyFunc{
	"season_goals":               seasonCountKey(func(s *SeasonStats) int { return s.Goals }),
	"season_assists":             seasonCountKey(func(s *SeasonStats) int { return s.Assists }),
	"season_appearances":         seasonCountKey(func(s *SeasonStats) int { return s.Starts + s.SubApps }),
	"season_minutes":             seasonCountKey(func(s *SeasonStats) int { return s.Minutes }),
	"season_player_of_the_match": seasonCountKey(func(s *SeasonStats) int { return s.PlayerOfTheMatch }),
	"season_average_rating": func(p *Player, _ sight) (int, bool) {
		if p.Season == nil || p.Season.AverageRating == nil {
			return 0, false
		}
		return int(math.Round(*p.Season.AverageRating * 100)), true
	},
}

func seasonCountKey(count func(*SeasonStats) int) sortKeyFunc {
	return func(p *Player, _ sight) (int, bool) {
		if p.Season == nil {
			return 0, true
		}
		return count(p.Season), true
	}
}

func currentAbility(p *Player, s sight) (int, bool) { return p.CurrentAbility, s.full() }

// sortByKey orders xs by key, stably, computing each key once; players
// whose key is hidden come last, most reputable first.
func sortByKey(xs []seen, key sortKeyFunc, ascending bool) {
	type keyed struct {
		seen
		key   int
		shown bool
	}
	ks := make([]keyed, len(xs))
	for i, x := range xs {
		k, shown := key(x.p, x.s)
		ks[i] = keyed{x, k, shown}
	}
	slices.SortStableFunc(ks, func(a, b keyed) int {
		switch {
		case a.shown && b.shown && ascending:
			return cmp.Compare(a.key, b.key)
		case a.shown && b.shown:
			return cmp.Compare(b.key, a.key)
		case a.shown:
			return -1
		case b.shown:
			return 1
		}
		return cmp.Compare(b.p.Reputation.Current, a.p.Reputation.Current)
	})
	for i, k := range ks {
		xs[i] = k.seen
	}
}
