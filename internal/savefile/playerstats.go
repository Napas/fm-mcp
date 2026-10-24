package savefile

import (
	"encoding/binary"
	"fmt"
)

// Layout of rgman/player_stats.dat, version 15 (docs/format.md).
const (
	playerStatsVersion = 15
	statsRecordHead    = 16 // unknown bytes after the person index
	statsRecordMiddle  = 20 // unknown bytes after the scope slots
	statsScopes        = 8  // scope slots per player
	statsTeamScopes    = 6  // scope slots per earlier team
	seasonScope        = 5  // club competitions of the current season
	statsScopeVersion  = 6  // first byte of every scope block
	statsTotalsSize    = 17
	statsDetailSize    = 120
	statsScopeSize     = 1 + statsTotalsSize + statsDetailSize
	statsSummarySize   = 8 // u8 2, u16, 5 × u8
	statsSummaryTag    = 2
	recentRatings      = 16 // match ratings kept per player
	recentRatingSize   = 3
	statsTrailerSize   = 16
)

// Scope slots kept per player. Slots 6 and 7 (club and international matches
// of the calendar year) are not kept.
const (
	friendliesScope     = 0 // friendlies
	leagueScope         = 1
	cupsScope           = 2 // domestic cups
	continentalScope    = 3 // all continental competitions together
	internationalsScope = 4 // senior national team; this international season (inference)
)

// MatchStats are a player's totals over the matches of one scope.
type MatchStats struct {
	RatingSum        uint16 // match ratings × 10, summed over rated matches
	Minutes          uint16
	Starts           uint8
	SubApps          uint8
	Rated            uint8 // matches with a rating; guess: late substitutes get none
	Goals            uint8
	Assists          uint8
	PenaltiesTaken   uint8
	PenaltiesScored  uint8
	PlayerOfTheMatch uint8
	RedCards         uint8
	YellowCards      uint8
	GoalsConceded    uint8 // goalkeepers only
	CleanSheets      uint8 // goalkeepers only
	Detail           MatchDetail
}

// MatchDetail holds the decoded i16 values of a scope's detail block. The game
// writes some slots with other meanings for goalkeepers (docs/format.md): they
// are named by the outfield meaning, with the keeper meaning in a comment.
type MatchDetail struct {
	TeamGoals, TeamConceded          int16 // in the player's matches
	MatchesWon, MatchesLost          int16
	Passes, PassesCompleted          int16
	Tackles                          int16 // keepers: penalties faced
	TacklesWon                       int16 // keepers: penalties saved
	Headers                          int16 // keepers: saves held
	HeadersWon                       int16 // keepers: saves parried
	HeadersLost                      int16 // 0 for keepers
	Dribbles, Fouls, FoulsAgainst    int16
	Shots, ShotsOnTarget             int16
	ShotsOutsideBox, GoalsOutsideBox int16
	FreeKickShots                    int16
	GoalsFromDirectFreeKicks         int16
	MistakesLeadingToGoal            int16
	Distance                         int16 // tenths of a km
	Offsides                         int16
	KeyPasses, OpenPlayKeyPasses     int16
	KeyTackles, KeyHeaders           int16
	Interceptions, ChancesCreated    int16
	PossessionLost, PossessionWon    int16
	ExpectedGoals, ExpectedAssists   int16 // × 100
	NonPenaltyXG                     int16 // × 100
	Clearances, Sprints              int16
	ProgressivePasses                int16
	Crosses, CrossesCompleted        int16
	OpenPlayCrosses                  int16 // attempted, every player
	OpenPlayCrossesAttempted         int16 // keepers: saves tipped
	OpenPlayCrossesCompleted         int16 // keepers: expected goals on target faced × 100
	Blocks                           int16 // keepers: shots on target faced (inference)
	ShotsBlocked                     int16
	PressuresAttempted               int16
	PressuresCompleted               int16
}

// SeasonRecord holds one player's scopes of the current season; a scope the
// player has no match in is nil.
type SeasonRecord struct {
	Club           *MatchStats // all club competitions ("Overall (Club)")
	Friendlies     *MatchStats
	League         *MatchStats
	Cups           *MatchStats // domestic cups
	Continental    *MatchStats // all continental competitions together
	Internationals *MatchStats // senior national team; this international season (inference)
}

// detailField is a decoded MatchDetail field and its i16 index in the detail
// block.
type detailField struct {
	index int
	field *int16
}

// detailFields lists d's decoded fields with their i16 indices. It returns an
// array so that callers do not allocate.
func detailFields(d *MatchDetail) [46]detailField {
	return [...]detailField{
		{0, &d.TeamGoals}, {1, &d.TeamConceded}, {2, &d.MatchesWon}, {3, &d.MatchesLost}, {4, &d.Passes},
		{5, &d.PassesCompleted}, {6, &d.Tackles}, {7, &d.TacklesWon}, {8, &d.Headers}, {9, &d.HeadersWon},
		{10, &d.Dribbles}, {11, &d.Fouls}, {12, &d.FoulsAgainst}, {13, &d.Shots}, {14, &d.ShotsOnTarget},
		{20, &d.MistakesLeadingToGoal}, {23, &d.Distance}, {24, &d.Offsides}, {25, &d.OpenPlayCrossesAttempted},
		{26, &d.OpenPlayCrossesCompleted}, {27, &d.KeyPasses}, {28, &d.KeyTackles}, {29, &d.KeyHeaders},
		{30, &d.Interceptions}, {31, &d.ChancesCreated}, {32, &d.Blocks}, {33, &d.GoalsFromDirectFreeKicks},
		{34, &d.PossessionLost}, {35, &d.PossessionWon}, {36, &d.ExpectedGoals}, {37, &d.ExpectedAssists},
		{38, &d.Clearances}, {40, &d.FreeKickShots}, {41, &d.GoalsOutsideBox}, {45, &d.OpenPlayCrosses},
		{46, &d.ShotsOutsideBox}, {48, &d.Sprints}, {50, &d.ProgressivePasses},
		{51, &d.PressuresAttempted}, {52, &d.PressuresCompleted}, {53, &d.OpenPlayKeyPasses},
		{54, &d.NonPenaltyXG}, {56, &d.Crosses}, {57, &d.CrossesCompleted}, {58, &d.ShotsBlocked},
		{59, &d.HeadersLost},
	}
}

// ReadSeasonStats decodes the club, friendlies, league, cup, continental and
// international scopes of rgman/player_stats.dat, by player person index.
// Players with none of them are absent.
func ReadSeasonStats(a *Archive) (map[uint32]SeasonRecord, error) {
	f, err := a.ReadFile("rgman/player_stats.dat")
	if err != nil {
		return nil, err
	}
	if f.Header.Version != playerStatsVersion {
		return nil, fmt.Errorf("%w: player_stats.dat version %d", ErrUnsupported, f.Header.Version)
	}
	stats, err := parseSeasonStats(f.Body)
	if err != nil {
		return nil, fmt.Errorf("player_stats.dat: %w", err)
	}
	return stats, nil
}

// parseSeasonStats walks every record of player_stats.dat and keeps scopes 0
// to 5.
func parseSeasonStats(body []byte) (map[uint32]SeasonRecord, error) {
	r := newReader(body)
	out := map[uint32]SeasonRecord{}
	seen := map[uint32]bool{}
	for r.err == nil {
		if more := r.u8(); more == 0 {
			break
		} else if more != 1 {
			r.fail("record flag %d", more)
			break
		}
		person := r.u32()
		if seen[person] && r.err == nil {
			r.fail("person %d listed twice", person)
		}
		seen[person] = true
		r.take(statsRecordHead)
		var rec SeasonRecord
		for i := range statsScopes {
			s, ok := r.statsScope()
			if !ok {
				continue
			}
			switch i {
			case seasonScope:
				v := s
				rec.Club = &v
			case friendliesScope:
				v := s
				rec.Friendlies = &v
			case leagueScope:
				v := s
				rec.League = &v
			case cupsScope:
				v := s
				rec.Cups = &v
			case continentalScope:
				v := s
				rec.Continental = &v
			case internationalsScope:
				v := s
				rec.Internationals = &v
			}
		}
		if rec != (SeasonRecord{}) {
			out[person] = rec
		}
		r.take(statsRecordMiddle)
		for n := r.u32(); n > 0 && r.err == nil; n-- {
			if ref := r.u8(); ref != 0 {
				r.fail("team reference kind %d", ref)
			}
			r.u32()
			for range statsTeamScopes {
				r.statsScope()
			}
		}
		for n := r.u32(); n > 0 && r.err == nil; n-- {
			if tag := r.u8(); tag != statsSummaryTag {
				r.fail("season summary tag %d", tag)
			}
			r.take(statsSummarySize - 1)
		}
		if n := r.u8(); n != recentRatings {
			r.fail("%d recent ratings", n)
		}
		r.take(recentRatingSize * recentRatings)
	}
	r.take(statsTrailerSize)
	if r.err != nil {
		return nil, r.err
	}
	if r.remaining() != 0 {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrCorrupt, r.remaining())
	}
	return out, nil
}

// statsScope reads one scope slot: u8 present, then the block when present.
func (r *reader) statsScope() (MatchStats, bool) {
	switch present := r.u8(); {
	case r.err != nil || present == 0:
		return MatchStats{}, false
	case present != 1:
		r.fail("scope flag %d", present)
		return MatchStats{}, false
	}
	if v := r.u8(); v != statsScopeVersion {
		r.fail("scope version %d", v)
	}
	t := r.take(statsTotalsSize)
	detail := r.take(statsDetailSize)
	if r.err != nil {
		return MatchStats{}, false
	}
	s := MatchStats{
		RatingSum: binary.LittleEndian.Uint16(t),
		Minutes:   binary.LittleEndian.Uint16(t[2:]),
		Starts:    t[4], SubApps: t[5], Rated: t[6], Goals: t[7], Assists: t[8], GoalsConceded: t[9],
		PenaltiesTaken: t[10], PenaltiesScored: t[11], PlayerOfTheMatch: t[12],
		RedCards: t[13], YellowCards: t[14], CleanSheets: t[15],
	}
	for _, f := range detailFields(&s.Detail) {
		*f.field = int16(binary.LittleEndian.Uint16(detail[2*f.index:])) //nolint:gosec // bit reinterpretation of i16
	}
	if int(s.Rated) > int(s.Starts)+int(s.SubApps) {
		r.fail("%d rated matches in %d appearances", s.Rated, int(s.Starts)+int(s.SubApps))
		return MatchStats{}, false
	}
	return s, true
}
