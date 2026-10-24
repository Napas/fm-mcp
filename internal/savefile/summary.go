package savefile

import "fmt"

// maxSummaryLeagues is a sanity cap on the league-name count.
const maxSummaryLeagues = 1000

// Summary is the start of save_game_summary.dat: what the game's load
// screen shows about a save.
type Summary struct {
	Build         string // e.g. "26.2.0+2276383"
	ManagerName   string // the human manager
	ClubShortName string // e.g. "Spurs"
	ClubUID       uint32 // database uid of the manager's club
	GameDate      Date
}

// ReadSummary decodes save_game_summary.dat. Layout:
//
//	string nation list ("765,MALE,3,0,…"), string build,
//	u32 n + n league names, u8, u32 unknown, u16 database version, u8,
//	string manager name, string club short name, u32 club uid, date, …
func ReadSummary(a *Archive) (Summary, error) {
	f, err := a.ReadFile("save_game_summary.dat")
	if err != nil {
		return Summary{}, err
	}
	r := newReader(f.Body)
	r.str()
	s := Summary{Build: r.str()}
	n := r.u32()
	if n > maxSummaryLeagues {
		return Summary{}, fmt.Errorf("%w: %d leagues in save_game_summary.dat", ErrCorrupt, n)
	}
	for range n {
		r.str()
	}
	r.u8()
	r.u32()
	r.u16()
	r.u8()
	s.ManagerName = r.str()
	s.ClubShortName = r.str()
	s.ClubUID = r.u32()
	s.GameDate = r.date()
	if r.err != nil {
		return Summary{}, fmt.Errorf("save_game_summary.dat: %w", r.err)
	}
	return s, nil
}
