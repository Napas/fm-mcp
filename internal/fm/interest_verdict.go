package fm

import "fmt"

// InterestKind is the kind of approach a verdict is asked for.
type InterestKind string

// The kinds of approach FM's "Interested in joining" filter offers.
const (
	InterestTransfer       InterestKind = "transfer"
	InterestLoan           InterestKind = "loan"
	InterestTransferOrLoan InterestKind = "transfer_or_loan"
)

// Verdict is how likely a player is to consider joining the human's club.
type Verdict string

// The three verdicts; uncertain covers every factor the model does not decode.
const (
	VerdictOut       Verdict = "out"
	VerdictUncertain Verdict = "uncertain"
	VerdictLikely    Verdict = "likely"
)

// Codes of the interest factors.
const (
	FactorOnlyJustJoined    = "only_just_joined"
	FactorTransferArranged  = "transfer_arranged"
	FactorRecentlyLeftClub  = "recently_left_your_club"
	FactorRetiring          = "retiring"
	FactorOnlyOneClub       = "only_one_club"
	FactorClubReputationGap = "club_reputation_gap"
	FactorDependsOnRole     = "depends_on_role"
)

const (
	interestOutBelow    = -20_000 // a sum below this is out: every labelled player below it failed in game
	interestLikelyFrom  = 30_000  // a transfer sum from here is likely: every labelled confident pass was above it
	recentlyLeftDays    = 150     // days after joining another club that a former player still counts as just left
	roleDependenceLevel = 8       // the playing-time level whose sum shows the dependence on the offered role
	interestDateLayout  = "2 January 2006"
)

// InterestFactor is one reason, with a stable code and a short sentence.
type InterestFactor struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// Interest is the verdict on whether a player would consider joining the
// human's club, as a transfer and as a loan.
type Interest struct {
	Transfer        Verdict          `json:"transfer"`
	Loan            Verdict          `json:"loan"`
	TransferReason  *InterestFactor  `json:"transfer_reason,omitempty"`
	TransferFactors []InterestFactor `json:"transfer_factors,omitempty"`
	LoanReason      *InterestFactor  `json:"loan_reason,omitempty"`
	LoanFactors     []InterestFactor `json:"loan_factors,omitempty"`
}

// ReasonFor is the reason a player is out for kind k; nil when he is not.
// transfer_or_loan is out only when both sides are, so it reports the
// transfer's.
func (in Interest) ReasonFor(k InterestKind) *InterestFactor {
	if k == InterestLoan {
		return in.LoanReason
	}
	return in.TransferReason
}

// For is the verdict for kind k. transfer_or_loan is out only when both
// sides are, and likely when the transfer is.
func (in Interest) For(k InterestKind) Verdict {
	switch k {
	case InterestTransfer:
		return in.Transfer
	case InterestLoan:
		return in.Loan
	}
	switch {
	case in.Transfer == VerdictOut && in.Loan == VerdictOut:
		return VerdictOut
	case in.Transfer == VerdictLikely:
		return VerdictLikely
	}
	return VerdictUncertain
}

// interestTarget is the human's club the rules judge a player against.
type interestTarget struct {
	club       int64 // CLUB index
	name       string
	reputation int // Rt, its first team's reputation
}

// interestCase is a judged player with what the reason texts need.
type interestCase struct {
	p        *Player
	in       interestInputs
	to       interestTarget
	terms    interestTerms // zero when gated or a free agent
	gate     string        // the code of the first hard gate he meets; "" when none
	verdicts Interest
}

// interestOf is nil for players judgeInterest does not evaluate.
func (g *Game) interestOf(p *Player) *Interest {
	c, ok := g.judgeInterest(p)
	if !ok {
		return nil
	}
	return g.describeInterest(c)
}

// judgeInterest builds verdicts without texts. False for the human's own
// players, those moving to his club, the other gender than the manager's
// team, national-team-only players, and everyone when the club is unknown.
func (g *Game) judgeInterest(p *Player) (interestCase, bool) {
	w := g.world
	mine, ok := w.clubOf(g.Info.ManagerTeamIndex)
	to := interestTarget{club: mine, reputation: g.targetReputation()}
	if !ok || to.reputation == 0 || p.Female != w.Teams[g.Info.ManagerTeamIndex].Women {
		return interestCase{}, false
	}
	in := g.auxOf(p).interest
	var contractTeam int64 = noTeam
	if in.main != nil {
		contractTeam = in.main.TeamIndex
	}
	isMine := func(team int64) bool {
		club, ok := w.clubOf(team)
		return ok && club == mine
	}
	if isMine(p.TeamIndex) || isMine(contractTeam) || in.arranged != nil && isMine(int64(in.arranged.team)) {
		return interestCase{}, false
	}
	if _, contracted := w.clubOf(contractTeam); !contracted {
		if t := w.Teams[p.TeamIndex]; t != nil && t.National {
			return interestCase{}, false
		}
	}
	to.name = w.Clubs[mine].Name
	c := interestCase{p: p, in: in, to: to, verdicts: Interest{Transfer: VerdictUncertain, Loan: VerdictUncertain}}
	if c.gate = g.gate(in, to); c.gate != "" {
		c.verdicts = Interest{Transfer: VerdictOut, Loan: VerdictOut}
		return c, true
	}
	if in.main == nil {
		return c, true
	}
	c.terms = g.interestTerms(p, to.reputation)
	transfer := -c.terms.penaltyT + c.terms.gap + c.terms.pt10
	loan := -c.terms.penaltyL + c.terms.gapLoan + max(c.terms.pt7, 0)
	switch {
	case transfer < interestOutBelow:
		c.verdicts.Transfer = VerdictOut
	case transfer >= interestLikelyFrom:
		c.verdicts.Transfer = VerdictLikely
	}
	if loan < interestOutBelow {
		c.verdicts.Loan = VerdictOut
	}
	return c, true
}

// reasonCode is the code c is out for kind k, "" when not out.
// transfer_or_loan reports the transfer's, as ReasonFor does.
func (c interestCase) reasonCode(k InterestKind) string {
	switch {
	case c.verdicts.For(k) != VerdictOut:
		return ""
	case c.gate != "":
		return c.gate
	case k == InterestLoan:
		return sumReasonCode(c.terms.penaltyL, c.terms.gapLoan)
	}
	return sumReasonCode(c.terms.penaltyT, c.terms.gap)
}

func (g *Game) describeInterest(c interestCase) *Interest {
	out := c.verdicts
	if c.gate != "" {
		f := g.gateFactor(c)
		out.TransferReason, out.LoanReason = f, f
		if c.in.main != nil {
			c.terms = g.interestTerms(c.p, c.to.reputation)
		}
	} else {
		if out.Transfer == VerdictOut {
			out.TransferReason = g.sumReason(c, false)
		}
		if out.Loan == VerdictOut {
			out.LoanReason = g.sumReason(c, true)
		}
	}
	if c.in.main != nil {
		out.TransferFactors = g.transferFactors(c, out.Transfer != VerdictOut)
		out.LoanFactors = g.loanFactors(c, out.Loan != VerdictOut)
	}
	return &out
}

// gate is the code of the first hard gate met, "" when none.
func (g *Game) gate(in interestInputs, to interestTarget) string {
	switch {
	case in.arranged != nil:
		return FactorTransferArranged
	case in.retiring:
		return FactorRetiring
	case in.onlyOneClub:
		return FactorOnlyOneClub
	case in.main != nil && in.previousClub == to.club && gameDaysSince(g.Info.Date, in.joined) <= recentlyLeftDays:
		return FactorRecentlyLeftClub
	}
	return ""
}

func (g *Game) gateFactor(c interestCase) *InterestFactor {
	switch c.gate {
	case FactorTransferArranged:
		a := c.in.arranged
		return &InterestFactor{c.gate, fmt.Sprintf("Has agreed a move to %s on %s", g.world.teamName(int64(a.team)), a.date.Format(interestDateLayout))}
	case FactorRetiring:
		return &InterestFactor{c.gate, "Is about to retire"}
	case FactorOnlyOneClub:
		return &InterestFactor{c.gate, "Will only join one particular club"}
	}
	return &InterestFactor{c.gate, fmt.Sprintf("Left %s %d days ago", c.to.name, gameDaysSince(g.Info.Date, c.in.joined))}
}

// sumReasonCode names the most negative term of an out sum.
func sumReasonCode(penalty, gap int) string {
	if gap < -penalty {
		return FactorClubReputationGap
	}
	return FactorOnlyJustJoined
}

func (g *Game) sumReason(c interestCase, loan bool) *InterestFactor {
	penalty, gap := c.terms.penaltyT, c.terms.gap
	if loan {
		penalty, gap = c.terms.penaltyL, c.terms.gapLoan
	}
	if sumReasonCode(penalty, gap) == FactorClubReputationGap {
		return g.gapFactor(c, loan)
	}
	return g.justJoinedFactor(c, loan)
}

func (g *Game) justJoinedFactor(c interestCase, loan bool) *InterestFactor {
	club := g.clubNameOf(c.p, c.in)
	_, d, signed := justJoinedMode(c.p, c.in, g.Info.Date, loan)
	if signed {
		return &InterestFactor{FactorOnlyJustJoined, fmt.Sprintf("Signed a new contract with %s %d days ago; won't consider a move yet", club, d)}
	}
	return &InterestFactor{FactorOnlyJustJoined, fmt.Sprintf("Joined %s %d days ago; won't consider a move yet", club, d)}
}

// gapFactor describes c's club reputation gap by its more negative term:
// his contract club's against yours, or his world reputation against your
// club. A loan's gap is the world reputation term alone.
func (g *Game) gapFactor(c interestCase, loan bool) *InterestFactor {
	if !loan {
		if club, world := clubReputationGapTerms(c.p, c.in, g.world, c.to.reputation); club < 0 && club <= world {
			first := contractFirstTeam(c.in.main, g.world)
			return &InterestFactor{FactorClubReputationGap, fmt.Sprintf("%s is a bigger club than %s", g.world.Clubs[first.ClubIndex].Name, c.to.name)}
		}
	}
	pronoun := "His"
	if c.p.Female {
		pronoun = "Her"
	}
	return &InterestFactor{FactorClubReputationGap, fmt.Sprintf("%s reputation is above %s's", pronoun, c.to.name)}
}

// transferFactors are the terms of c that weigh against a transfer; kept
// when the transfer is not out.
func (g *Game) transferFactors(c interestCase, kept bool) []InterestFactor {
	var out []InterestFactor
	if c.terms.gap < 0 {
		out = append(out, *g.gapFactor(c, false))
	}
	if !kept {
		return out
	}
	if c.terms.penaltyT > 0 {
		out = append(out, *g.justJoinedFactor(c, false))
	}
	pt8, _ := playingTimeValue(c.p, c.in, g.world, c.to.reputation, roleDependenceLevel, false)
	if -justJoined(c.p, c.in, g.world, g.Info.Date, roleDependenceLevel, false)+c.terms.gap+pt8 < 0 {
		out = append(out, InterestFactor{FactorDependsOnRole, "Depends on the role you'd offer"})
	}
	return out
}

// loanFactors are the terms of c that weigh against a loan; kept when the
// loan is not out.
func (g *Game) loanFactors(c interestCase, kept bool) []InterestFactor {
	var out []InterestFactor
	if c.terms.gapLoan < 0 {
		out = append(out, *g.gapFactor(c, true))
	}
	if kept && c.terms.penaltyL > 0 {
		out = append(out, *g.justJoinedFactor(c, true))
	}
	return out
}

// clubNameOf is the name of the club p's contract is with, else his team's.
func (g *Game) clubNameOf(p *Player, in interestInputs) string {
	if in.main != nil {
		if club, ok := g.world.clubOf(in.main.TeamIndex); ok {
			return g.world.Clubs[club].Name
		}
	}
	return p.Club
}
