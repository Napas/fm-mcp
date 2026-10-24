//go:build race

package savefile

// raceEnabled is true when the race detector is on; timing checks skip then.
const raceEnabled = true
