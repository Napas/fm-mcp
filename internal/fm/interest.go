package fm

import (
	"slices"
	"time"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// interestInputs are the save fields the interest model reads.
type interestInputs struct {
	joined       savefile.Date // JoinedClubPerson
	previousClub int64
	arranged     *arrangedMove // a linked status-19 transfer; nil when none
	retiring     bool
	onlyOneClub  bool
	main         *interestContract // nil for free agents
	loyalty      uint8             // hidden Loyalty
}

// interestContract is the small copy of the main contract the interest rules read.
type interestContract struct {
	Events                 []savefile.ContractEvent
	ClauseTypes            []uint8
	TeamIndex              int64
	End, Signed            savefile.Date
	Kind                   uint8 // FULL header byte 0
	CoreFlags              uint8
	PlayingTime            savefile.PlayingTime
	OriginalPlayingTime    savefile.PlayingTime
	PlayingTimeScore       int8
	RecentPlayingTimeScore int8
}

func newInterestContract(c *savefile.Contract) *interestContract {
	if c == nil {
		return nil
	}
	var clauses []uint8
	for _, cl := range c.Clauses {
		clauses = append(clauses, cl.Type)
	}
	return &interestContract{
		Events: c.Events, ClauseTypes: clauses, TeamIndex: c.TeamIndex, End: c.End, Signed: c.Signed,
		Kind: c.Kind, CoreFlags: c.CoreFlags, PlayingTime: c.PlayingTime, OriginalPlayingTime: c.OriginalPlayingTime,
		PlayingTimeScore: c.PlayingTimeScore, RecentPlayingTimeScore: c.RecentPlayingTimeScore,
	}
}

// arrangedMove is a move the player has agreed to.
type arrangedMove struct {
	team int32
	date time.Time
}

// newInterestInputs collects r's interest inputs. byID indexes the save's
// transfers; nil when they could not be read.
func newInterestInputs(r savefile.Player, byID map[uint32]savefile.Transfer) interestInputs {
	in := interestInputs{
		joined: r.JoinedClubPerson, previousClub: r.PreviousClub,
		retiring: r.Retiring, onlyOneClub: r.OnlyOneClub, main: newInterestContract(r.Contract), loyalty: r.Hidden[2],
	}
	for _, id := range r.TransferIDs {
		if t, ok := byID[id]; ok && t.Person == r.Index && t.Status == savefile.TransferArranged {
			in.arranged = &arrangedMove{team: t.Team, date: knownDate(t.Date)}
			break
		}
	}
	return in
}

// Constants of the interest rules (game_plugin 26.3.2).
const (
	transferLevel = 10 // the best playing-time level the human's club can offer (kind 4)
	loanLevel     = 7  // the level offered with a loan (kind 3)

	justJoinedDays       = 250 // T of the just-joined penalty, counted from joining
	justSignedDays       = 100 // T counted from signing, after a renewal
	expiryExemptFromDays = 61  // from here an expiring contract exempts
	expiryDays           = 300 // a contract ending sooner is expiring
	loanJoinedMaxAge     = 25  // up to this age a loan always counts from joining
	justSignedReducedAge = 23  // up to this age "just signed" takes a quarter (the lenient choice)
	loanMinLevel         = 5   // 0x32715dc: the loan penalty needs this playing-time level

	contractKindExempt   = 3 // FULL header byte 0 values the penalty reads
	contractKindExpiring = 4
	contractKindTenfold  = 2 // multiplies the penalty by 1000 instead of 100
	penaltyScale         = 100
	tenfoldPenaltyScale  = 1000
	// coreFlagsExempt are core byte 23 bits 0 and 3: they exempt him from the
	// just-joined penalty unless his club's first team has a manager and his
	// original playing time counts.
	coreFlagsExempt = 1 | 8

	// 0x128c898, the club reputation gap.
	targetBoost       = 1.15 // the target's reputation with a boost clause, under targetBoostMaxAge
	targetBoostMaxAge = 30
	lowStatusDiscount = 0.8  // the current club's reputation at his level fringe or below, prospect excepted
	worldRepStepBelow = -500 // a world reputation term below this counts worldRepStep times
	worldRepStep      = 8

	ptKeeperRating   = 15   // 0x12ad438 counts a GK rating from here as a keeper
	loanWantFromAge  = 21   // 0x12ad438: from this age a loan wants at least a fringe level
	unknownClubRep   = 1500 // the club reputation the reference model uses without a first team or with reputation 0
	reputationCap    = 36000
	unsetDays        = 99999 // days since an unset date
	interestLevelCap = 8     // an open contract event caps the playing-time level here
)

// Clause types the rules read; their names are not decoded.
var (
	exemptClauses      = []uint8{55, 56} // 0x128f550: no just-joined penalty
	targetBoostClauses = []uint8{54, 63} // 0x128c898: the target counts × targetBoost
)

// gapSteps multiplies a club reputation gap at or below upTo by factor;
// the first match applies (0x128c898).
var gapSteps = [...]struct{ upTo, factor int }{{-4001, 6}, {-3001, 5}, {-2001, 4}, {-1001, 2}}

// interestTerms are the decoded factors of the interest model for one
// player: just-joined penalties P, club reputation gaps and playing-time
// values, at the transfer level (L 10, kind 4) and the loan level (L 7,
// kind 3).
type interestTerms struct{ penaltyT, gap, pt10, penaltyL, pt7, gapLoan int }

// interestTerms evaluates p against the human's club, whose first team has
// reputation rt.
func (g *Game) interestTerms(p *Player, rt int) interestTerms {
	in, today := g.auxOf(p).interest, g.Info.Date
	pt10, _ := playingTimeValue(p, in, g.world, rt, transferLevel, false)
	pt7, _ := playingTimeValue(p, in, g.world, rt, loanLevel, true)
	return interestTerms{
		penaltyT: justJoined(p, in, g.world, today, transferLevel, false),
		gap:      clubReputationGap(p, in, g.world, rt),
		pt10:     pt10,
		penaltyL: justJoined(p, in, g.world, today, loanLevel, true),
		pt7:      pt7,
		gapLoan:  clubReputationGapLoan(p, rt),
	}
}

// targetReputation is Rt, the reputation of the human club's first team; 0
// when the manager's club is unknown.
func (g *Game) targetReputation() int { return targetReputation(g.world, g.Info.ManagerTeamIndex) }

// targetReputation is the first-team reputation of the club of team
// managerTeam; 0 when there is none.
func targetReputation(w World, managerTeam int64) int {
	club, ok := w.clubOf(managerTeam)
	if !ok {
		return 0
	}
	if t := clubFirstTeam(w, club); t != nil {
		return t.Reputation
	}
	return 0
}

// interestWarning explains why no player carries interest when the club of
// team managerTeam has no first team with a reputation; "" otherwise.
func interestWarning(w World, managerTeam int64) string {
	if targetReputation(w, managerTeam) != 0 {
		return ""
	}
	return "interest unavailable: your club's first team is not known"
}

// justJoined is 0x128f550: the penalty for a player who joined or renewed
// recently, at offered level L; loan selects kind 3.
func justJoined(p *Player, in interestInputs, w World, today time.Time, L int, loan bool) int {
	c := in.main
	if c == nil || p.Loan != nil {
		return 0
	}
	// 0x129fcf8 and 0x32715dc. The club +0x16a test applies only with flag
	// bit 3, which the filter's call does not set.
	if loan && (p.NationIndex != loanClubNation(p, c, w) || statusLevel(c.PlayingTime) < loanMinLevel) {
		return 0
	}
	if eventsExempt(c.Events) || hasClause(c, exemptClauses...) || c.Kind == contractKindExempt {
		return 0
	}
	joined := gameDaysSince(today, in.joined)
	due, ends := daysUntil(today, c.End)
	if joined >= expiryExemptFromDays && (c.Kind == contractKindExpiring || ends && due < expiryDays) {
		return 0
	}
	T, d, justSigned := justJoinedMode(p, in, today, loan)
	if T-d <= 0 {
		return 0
	}
	first := contractFirstTeam(c, w)
	managed := first != nil && first.HasManager && originalPlayingTimeCounts(c.OriginalPlayingTime)
	if !managed && c.CoreFlags&coreFlagsExempt != 0 {
		return 0
	}
	scale := penaltyScale
	if c.Kind == contractKindTenfold {
		scale = tenfoldPenaltyScale
	}
	penalty := (11 - L) * (T - d + 1) * int(in.loyalty) * scale
	if justSigned && p.Age <= justSignedReducedAge {
		penalty /= 4
	}
	return penalty
}

// justJoinedMode picks the window of the just-joined penalty: "just signed"
// (T 100, days since signing) after a renewal with at least 300 days left,
// else "just joined" (T 250, days since joining). The loan kind counts from
// joining up to age 25.
func justJoinedMode(p *Player, in interestInputs, today time.Time, loan bool) (T, d int, justSigned bool) {
	c := in.main
	joined, signed := gameDaysSince(today, in.joined), gameDaysSince(today, c.Signed)
	due, ends := daysUntil(today, c.End)
	if (!loan || p.Age > loanJoinedMaxAge) && signed < joined && ends && due >= expiryDays {
		return justSignedDays, signed, true
	}
	return justJoinedDays, joined, false
}

// clubReputationGap is 0x128c898 (reason 2) for a transfer: the sum of
// clubReputationGapTerms.
func clubReputationGap(p *Player, in interestInputs, w World, rt int) int {
	club, world := clubReputationGapTerms(p, in, w, rt)
	return club + world
}

// clubReputationGapTerms are the two terms of the transfer gap: the
// target's first-team reputation against his contract club's, stepped, and
// the world reputation term. Both 0 without a contract, playing time or club
// first team.
func clubReputationGapTerms(p *Player, in interestInputs, w World, rt int) (club, world int) {
	c := in.main
	if c == nil || c.PlayingTime == 0 {
		return 0, 0
	}
	first := contractFirstTeam(c, w)
	if first == nil {
		return 0, 0
	}
	target, current := min(4*rt, reputationCap), min(4*first.Reputation, reputationCap)
	if p.Age < targetBoostMaxAge && hasClause(c, targetBoostClauses...) {
		target = int(float64(target) * targetBoost)
	}
	if m := playingTimeLevel(p, c); m <= levelFringe && m != levelProspect {
		current = int(float64(current) * lowStatusDiscount)
	}
	d := target - current
	for _, s := range gapSteps {
		if d <= s.upTo {
			d *= s.factor
			break
		}
	}
	return d, worldReputationTerm(target, p.Reputation.World)
}

// clubReputationGapLoan is 0x128c898 for a loan: the world reputation term
// alone.
func clubReputationGapLoan(p *Player, rt int) int {
	return worldReputationTerm(min(4*rt, reputationCap), p.Reputation.World)
}

// worldReputationTerm compares the target T with 4 × his world reputation;
// only a negative result counts, × 8 below -500.
func worldReputationTerm(target, world int) int {
	v := target - min(4*world, reputationCap)
	if v < worldRepStepBelow {
		v *= worldRepStep
	}
	return min(v, 0)
}

// playingTimeValue is 0x12ad438 for a player at another club: the value of
// offering level L. ok is false for free agents, and below his wanted level
// when L ≤ 6 (those multipliers are not modelled). The division-tier term is
// not modelled.
func playingTimeValue(p *Player, in interestInputs, w World, rt, L int, loan bool) (int, bool) {
	c := in.main
	if c == nil {
		return 0, false
	}
	m := playingTimeLevel(p, c)
	if eventsCapLevel(c.Events) {
		m = min(m, interestLevelCap)
	}
	want := m
	switch m {
	case levelFringe:
		if gkRating(p) >= ptKeeperRating {
			want = levelSquad
		}
	case levelProspect:
		want = levelSquad
	}
	if loan && p.Age >= loanWantFromAge {
		want = max(want, levelFringe)
	}
	d := L - want
	cr := p.Reputation.Current
	clubRep := unknownClubRep
	if first := contractFirstTeam(c, w); first != nil && first.Reputation != 0 {
		clubRep = first.Reputation
	}
	gap := 5 * (rt - cr)
	if gap < 0 && rt < clubRep {
		gap += 10 * (rt - clubRep)
	}
	switch {
	case d >= 1:
		return max(0, gap+d*cr), true
	case d == 0:
		return 0, true
	case L <= 6:
		return 0, false
	default:
		return min(d*cr-gap, 0), true
	}
}

// playingTimeLevel is the better of his playing time's level and his score's.
func playingTimeLevel(p *Player, c *interestContract) int {
	score := scoreLevel(c.PlayingTime, c.PlayingTimeScore, c.RecentPlayingTimeScore, gkRating(p) >= keeperRating, false)
	return int(max(statusLevel(c.PlayingTime), score))
}

func gkRating(p *Player) int {
	if len(p.Positions) <= savefile.GKPosition {
		return 0
	}
	return p.Positions[savefile.GKPosition].Rating
}

func hasClause(c *interestContract, types ...uint8) bool {
	return slices.ContainsFunc(c.ClauseTypes, func(t uint8) bool { return slices.Contains(types, t) })
}

// contractFirstTeam is the first team of the club c is with; nil for
// national or unknown teams and clubs without a first team.
func contractFirstTeam(c *interestContract, w World) *Team {
	club, ok := w.clubOf(c.TeamIndex)
	if !ok {
		return nil
	}
	return clubFirstTeam(w, club)
}

func clubFirstTeam(w World, club int64) *Team {
	c, ok := w.Clubs[club]
	if !ok {
		return nil
	}
	return w.Teams[c.FirstTeam]
}

// originalPlayingTimeCounts reports whether an original playing time keeps
// the +0x57 exemption from applying: 1..6 or the keeper statuses 15..20.
func originalPlayingTimeCounts(s savefile.PlayingTime) bool {
	return s >= 1 && s <= 6 || s >= 15 && s <= 20
}

// loanClubNation is 0x129fcf8's club nation: his current team's club's,
// else his contract team's club's; noTeam when neither is a club team.
func loanClubNation(p *Player, c *interestContract, w World) int64 {
	for _, team := range []int64{p.TeamIndex, c.TeamIndex} {
		if _, ok := w.clubOf(team); ok {
			return w.Teams[team].NationIndex
		}
	}
	return noTeam
}

// gameDaysSince counts days from d to today as the game does: 365 a year
// plus the day-of-year difference. unsetDays when d has no year.
func gameDaysSince(today time.Time, d savefile.Date) int {
	if d.IsZero() {
		return unsetDays
	}
	t := d.Time()
	return (today.Year()-t.Year())*365 + today.YearDay() - t.YearDay()
}

// daysUntil counts calendar days from today to d; false when d is unset.
func daysUntil(today time.Time, d savefile.Date) (int, bool) {
	t := knownDate(d)
	if t.IsZero() {
		return 0, false
	}
	return int(t.Sub(today) / (24 * time.Hour)), true
}

// Contract events the interest rules skip (0x3755478).
func skippedEvent(id uint16) bool {
	return id == 0 || id >= 900 && id <= 930 || id == 1100 || id == 1104 || id == 1106
}

// Ring item lists of 0x12ae024 and 0x12ad438.
var (
	statusGatedEventTypes = []uint8{19, 20}                 // count only at status -2 (0x12ae024), never for the level cap
	excludedEventTypes    = []uint8{8, 37}                  // never count
	blockingEventU14      = []uint16{2, 18, 34, 39, 42, 60} // with 238–240: an open one blocks the exemption
)

// eventsExempt is 0x12ae024 (0x12ae048…0x12ae160): an open qualifying event
// exempts him from the just-joined penalty unless an open event has u14 2,
// 18, 34, 39, 42, 60 or 238–240.
func eventsExempt(events []savefile.ContractEvent) bool {
	qualifies := slices.ContainsFunc(events, func(e savefile.ContractEvent) bool {
		if !e.Open() || e.Status < -2 || skippedEvent(e.ID) {
			return false
		}
		return (e.Status == -2 || !slices.Contains(statusGatedEventTypes, e.Type)) && !slices.Contains(excludedEventTypes, e.Type)
	})
	return qualifies && !slices.ContainsFunc(events, func(e savefile.ContractEvent) bool {
		return e.Open() && (slices.Contains(blockingEventU14, e.U14) || e.U14 >= 238 && e.U14 <= 240)
	})
}

// eventsCapLevel reports whether an open event with status 2 or more caps
// the playing-time level (0x12ad438).
func eventsCapLevel(events []savefile.ContractEvent) bool {
	return slices.ContainsFunc(events, func(e savefile.ContractEvent) bool {
		return e.Open() && !skippedEvent(e.ID) && e.Status >= 2 &&
			!slices.Contains(statusGatedEventTypes, e.Type) && !slices.Contains(excludedEventTypes, e.Type)
	})
}
