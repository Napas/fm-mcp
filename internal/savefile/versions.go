package savefile

// game_db.dat database versions, one per build, each the version first seen
// in that build. The game writes a constant per build (game_db.dat header).
const (
	dbVersion26_2 = 3727 // 26.2.0
	dbVersion26_3 = 3733 // 26.3.2
	dbVersion27_0 = 3754 // 27.0.0
)

// DatabaseVersion27 is FM 27.0.0's database version, exported for fm's
// build-dependent rules.
const DatabaseVersion27 = dbVersion27_0

// verifiedDatabaseVersions are the database versions whose whole save layout
// was checked against real saves. A build joins only when every structure
// fm reads is verified for it.
var verifiedDatabaseVersions = map[uint16]bool{dbVersion26_2: true, dbVersion26_3: true, dbVersion27_0: true}

// VerifiedDatabaseVersion reports whether saves with this database version
// were checked. Other versions may still parse but can decode wrong values.
func VerifiedDatabaseVersion(v uint16) bool { return verifiedDatabaseVersions[v] }
