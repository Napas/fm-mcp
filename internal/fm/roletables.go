package fm

import (
	"slices"
	"strings"

	"github.com/Napas/fm-mcp/internal/savefile"
)

// roleAttributes are the attributes an in-possession role weighs, as indices
// into savefile.AttributeNames: key ×16, preferable ×4 and negative ×3.
type roleAttributes struct{ key, preferable, negative []int }

// roleAttrs builds roleAttributes from space-separated attribute names.
func roleAttrs(key, preferable, negative string) roleAttributes {
	indices := func(names string) []int {
		var out []int
		for _, n := range strings.Fields(names) {
			i := slices.Index(savefile.AttributeNames[:], n)
			if i < 0 {
				panic("fm: unknown attribute " + n)
			}
			out = append(out, i)
		}
		return out
	}
	return roleAttributes{indices(key), indices(preferable), indices(negative)}
}

// roleAttributesByRole holds every in-possession role's attributes by role
// bit, from the classifier the selector 0x48c4a94 returns for it (address
// after the name; game_plugin 26.3.2). The game shades the key attributes
// green and the preferable ones blue (checked for six roles).
var roleAttributesByRole = map[uint64]roleAttributes{
	0x1: roleAttrs( // Goalkeeper, 0x48c4db8
		"Handling AerialReach CommandOfArea Communication Positioning Reflexes Agility Concentration",
		"Kicking Throwing Anticipation Decisions OneOnOnes",
		"Eccentricity"),
	0x2: roleAttrs( // Centre-Back, 0x48c50b0
		"Heading Marking Tackling Anticipation Positioning Strength JumpingReach",
		"Decisions Pace Bravery Aggression Composure Concentration",
		"Passing"),
	0x4: roleAttrs( // Full-Back, 0x48c51ac
		"Marking Tackling Anticipation Positioning Teamwork Acceleration Concentration",
		"Crossing Dribbling Passing Decisions Technique WorkRate Stamina Pace Agility",
		""),
	0x8: roleAttrs( // Wing-Back, 0x48c52a4
		"Crossing Marking Tackling Teamwork WorkRate Acceleration Stamina Pace",
		"Dribbling OffTheBall Passing Anticipation Decisions Positioning FirstTouch Technique Balance Agility Concentration",
		""),
	0x10: roleAttrs( // Defensive Midfielder, 0x48c539c
		"Tackling Anticipation Positioning Teamwork Concentration",
		"Marking Passing Decisions FirstTouch WorkRate Strength Stamina Aggression Composure",
		""),
	0x20: roleAttrs( // Central Midfielder, 0x48c533c
		"Passing Tackling Decisions FirstTouch Teamwork",
		"OffTheBall Vision Anticipation Positioning Technique WorkRate Stamina Composure Concentration",
		""),
	0x40: roleAttrs( // Wide Midfielder, 0x48c546c
		"Crossing Passing Decisions Technique Teamwork WorkRate Stamina Pace",
		"Dribbling OffTheBall Vision Anticipation FirstTouch Acceleration Agility Composure",
		""),
	0x80: roleAttrs( // Winger, 0x48c54d4
		"Crossing Dribbling Technique Teamwork Acceleration Pace Agility",
		"OffTheBall Passing Anticipation FirstTouch Flair WorkRate Stamina Balance",
		""),
	0x200: roleAttrs( // Attacking Midfielder, 0x48c55c0
		"LongShots OffTheBall Passing FirstTouch Technique Flair Composure",
		"Crossing Dribbling Finishing Vision Anticipation Decisions Acceleration Agility",
		""),
	0x400: roleAttrs( // Deep Lying Forward, 0x48c596c
		"Finishing OffTheBall FirstTouch Technique Strength Composure",
		"Dribbling Passing Vision Anticipation Decisions Teamwork Balance",
		""),
	0x800: roleAttrs( // Centre-Forward, 0x48c5aa4
		"Finishing Heading OffTheBall FirstTouch Technique Acceleration Strength Composure",
		"Dribbling Passing Anticipation Decisions Pace JumpingReach Balance Agility",
		""),
	0x1000: roleAttrs( // Ball-Playing Goalkeeper, 0x48c4e20
		"Handling AerialReach CommandOfArea Communication Kicking Positioning Reflexes Agility Concentration",
		"Passing Throwing Anticipation Decisions OneOnOnes Eccentricity Composure",
		""),
	0x4000: roleAttrs( // Advanced Centre-Back, 0x48c4ef8
		"Heading Marking Passing Tackling Anticipation Decisions Positioning Technique Teamwork Strength JumpingReach Composure",
		"Dribbling Vision FirstTouch Stamina Pace Bravery Aggression Concentration",
		""),
	0x8000: roleAttrs( // Deep Lying Playmaker, 0x48c5b0c
		"Passing Vision Decisions FirstTouch Technique Teamwork Composure",
		"Marking OffTheBall Tackling Anticipation Positioning WorkRate Stamina Balance Concentration",
		""),
	0x10000: roleAttrs( // Box To Box Midfielder, 0x48c5754
		"OffTheBall Passing Tackling Teamwork WorkRate Stamina",
		"Dribbling Finishing LongShots Anticipation Decisions Positioning FirstTouch Technique Acceleration Strength Pace Balance Aggression Composure",
		""),
	0x20000: roleAttrs( // Advanced Playmaker, 0x48c5bec
		"OffTheBall Passing Vision Decisions FirstTouch Technique Teamwork Composure",
		"Crossing Dribbling Anticipation Flair Acceleration Agility",
		""),
	0x40000: roleAttrs( // Target Forward, 0x48c5c50
		"Finishing Heading OffTheBall Strength JumpingReach Balance Bravery Aggression Composure",
		"Anticipation Decisions FirstTouch Teamwork",
		""),
	0x80000: roleAttrs( // Poacher, 0x48c5d38
		"Finishing Heading OffTheBall Anticipation Acceleration Composure Concentration",
		"Decisions FirstTouch Technique Balance",
		""),
	0x100000: roleAttrs( // Complete Forward, 0x48c5d98
		"Dribbling Finishing Heading OffTheBall Anticipation FirstTouch Technique Acceleration Strength Agility Composure",
		"LongShots Passing Vision Decisions Teamwork WorkRate Stamina Pace JumpingReach Balance",
		""),
	0x1000000: roleAttrs( // Ball-Playing Centre-Back, 0x48c4f64
		"Heading Marking Passing Tackling Anticipation Positioning Strength JumpingReach Composure",
		"Vision Decisions FirstTouch Technique Pace Bravery Aggression Concentration",
		""),
	0x8000000: roleAttrs( // Inside Forward, 0x48c589c
		"Dribbling OffTheBall Anticipation FirstTouch Technique Acceleration Agility Composure",
		"Crossing Finishing LongShots Passing Vision Flair WorkRate Stamina Pace Balance",
		""),
	0x10000000: roleAttrs( // Ball Winning Midfielder, 0x48c56f0
		"Tackling Anticipation Teamwork WorkRate Stamina Aggression",
		"Marking Positioning Strength Pace Bravery Agility Concentration",
		""),
	0x20000000: roleAttrs( // No-Nonsense Centre-Back, 0x48c5128
		"Heading Marking Tackling Anticipation Positioning Strength JumpingReach",
		"Pace Bravery Aggression Concentration",
		"Passing Composure"),
	0x40000000: roleAttrs( // Defensive Winger, 0x48c5538
		"Crossing OffTheBall Technique Teamwork WorkRate Stamina",
		"Dribbling Marking Passing Tackling Anticipation Decisions Positioning FirstTouch Acceleration Aggression Composure Concentration",
		"Vision Flair"),
	0x80000000: roleAttrs( // Channel Forward, 0x48c5dc8
		"Dribbling Finishing OffTheBall FirstTouch Technique WorkRate Acceleration Composure",
		"Crossing Heading Passing Anticipation Decisions Stamina Pace Balance Agility",
		""),
	0x100000000: roleAttrs( // Free Role, 0x48c5e30
		"Dribbling LongShots OffTheBall Passing Vision FirstTouch Technique Flair Composure",
		"Crossing Finishing Anticipation Decisions Acceleration Agility",
		""),
	0x200000000: roleAttrs( // Anchor, 0x48c5e94
		"Marking Tackling Anticipation Decisions Positioning Concentration",
		"Teamwork Strength Composure",
		""),
	0x800000000: roleAttrs( // Half Back, 0x48c5400
		"Heading Marking Tackling Anticipation Positioning Teamwork Strength JumpingReach Concentration",
		"Passing Decisions FirstTouch WorkRate Stamina Bravery Aggression Composure",
		""),
	0x1000000000: roleAttrs( // No-Nonsense Full-Back, 0x48c5214
		"Marking Tackling Anticipation Positioning Strength",
		"Heading Teamwork Bravery Aggression Concentration",
		"Dribbling Passing Technique Pace"),
	0x2000000000: roleAttrs( // Enganche, 0x48c5824
		"Passing Vision Decisions FirstTouch Technique Composure",
		"Dribbling OffTheBall Anticipation Flair Teamwork Agility",
		"WorkRate"),
	0x4000000000: roleAttrs( // Advanced Wing-Back, 0x48c530c
		"Crossing Dribbling OffTheBall Technique Teamwork WorkRate Acceleration Stamina Pace Agility",
		"Marking Passing Tackling Anticipation Decisions Positioning FirstTouch Flair Balance",
		""),
	0x8000000000: roleAttrs( // Regista, 0x48c5b74
		"OffTheBall Passing Vision Decisions FirstTouch Technique Flair Teamwork Composure",
		"Dribbling LongShots Anticipation Balance",
		"Positioning"),
	0x10000000000: roleAttrs( // False Nine, 0x48c59d0
		"Dribbling OffTheBall Passing Vision Decisions FirstTouch Technique Teamwork Acceleration Composure",
		"Finishing Anticipation Flair Balance Agility",
		""),
	0x20000000000: roleAttrs( // Second Striker, 0x48c5a38
		"Finishing OffTheBall Anticipation FirstTouch Acceleration Composure",
		"Dribbling LongShots Passing Decisions Technique WorkRate Stamina Pace Agility Concentration",
		""),
	0x40000000000: roleAttrs( // Wide Target Forward, 0x48c5cb0
		"Heading OffTheBall Strength JumpingReach Bravery",
		"Crossing Finishing Anticipation FirstTouch Teamwork WorkRate Stamina Balance",
		"Dribbling Vision Flair"),
	0x80000000000: roleAttrs( // Playmaking Winger, 0x48c5ef0
		"Crossing Dribbling OffTheBall Passing Vision Decisions FirstTouch Technique Teamwork Acceleration Composure",
		"Anticipation Flair WorkRate Stamina Pace Agility",
		""),
	0x100000000000: roleAttrs( // Inside Wing-Back, 0x48c5f54
		"Passing Tackling Anticipation Decisions Positioning Teamwork Acceleration Composure",
		"Marking FirstTouch Technique WorkRate Stamina Pace Agility Concentration",
		""),
	0x200000000000: roleAttrs( // Wide Forward, 0x48c608c
		"Dribbling OffTheBall Anticipation FirstTouch Technique Acceleration Pace Agility",
		"Crossing Finishing Passing Flair WorkRate Stamina Balance Composure",
		""),
	0x400000000000: roleAttrs( // Box To Box Playmaker, 0x48c60f4
		"OffTheBall Passing Vision Decisions FirstTouch Technique Teamwork WorkRate Stamina Composure",
		"Dribbling LongShots Marking Tackling Anticipation Positioning Acceleration Pace Balance Agility",
		""),
	0x800000000000: roleAttrs( // Mezzala, 0x48c57bc
		"Dribbling OffTheBall Passing Vision Decisions Technique WorkRate Acceleration",
		"Finishing LongShots Anticipation FirstTouch Flair Stamina Balance Composure",
		""),
	0x1000000000000: roleAttrs( // Wide Central Midfielder, 0x48c615c
		"Passing Tackling Decisions FirstTouch Teamwork",
		"Crossing Dribbling OffTheBall Vision Anticipation Positioning Technique WorkRate Stamina Agility Composure Concentration",
		""),
	0x2000000000000: roleAttrs( // Inside Winger, 0x48c5904
		"Dribbling FirstTouch Technique Teamwork Acceleration Agility Composure",
		"Crossing LongShots OffTheBall Passing Vision Anticipation Flair WorkRate Stamina Pace Balance",
		""),
	0x4000000000000: roleAttrs( // Segundo Volante, 0x48c61c0
		"Marking OffTheBall Passing Tackling Positioning WorkRate Stamina Pace",
		"Finishing LongShots Anticipation Decisions FirstTouch Acceleration Strength Balance Composure Concentration",
		""),
	0x8000000000000: roleAttrs( // Wide Centre-Back, 0x48c4fcc
		"Heading Marking Tackling Anticipation Positioning Strength JumpingReach",
		"Dribbling Decisions WorkRate Acceleration Stamina Pace Bravery Aggression Agility Composure Concentration",
		"Passing"),
	0x10000000000000: roleAttrs( // Inside Full-Back, 0x48c6024
		"Heading Marking Tackling Anticipation Positioning Strength",
		"Dribbling Decisions WorkRate Acceleration Stamina Pace JumpingReach Bravery Aggression Agility Composure Concentration",
		""),
	0x20000000000000: roleAttrs( // No Nonsense Goalkeeper, 0x48c4e80
		"Handling AerialReach CommandOfArea Communication Positioning Reflexes Agility Concentration",
		"Anticipation Decisions OneOnOnes",
		"Passing Eccentricity"),
	0x40000000000000: roleAttrs( // Overlapping Centre-Back, 0x48c5048
		"Crossing Heading Marking Tackling Anticipation WorkRate Strength Stamina Pace JumpingReach",
		"Dribbling OffTheBall Decisions Positioning Technique Acceleration Bravery Aggression Agility Composure Concentration",
		""),
	0x80000000000000: roleAttrs( // Playmaking Wing-Back, 0x48c5fb8
		"Passing Tackling Vision Decisions FirstTouch Technique Teamwork Acceleration Composure",
		"Crossing Dribbling Marking OffTheBall Anticipation Positioning WorkRate Stamina Pace Agility Concentration",
		""),
	0x100000000000000: roleAttrs( // Channel Midfielder, 0x48c5624
		"Crossing OffTheBall Passing FirstTouch Technique WorkRate Acceleration Composure",
		"Dribbling LongShots Vision Anticipation Decisions Flair Stamina Pace Agility",
		""),
	0x200000000000000: roleAttrs( // Midfield Playmaker, 0x48c568c
		"OffTheBall Passing Vision Decisions FirstTouch Technique Teamwork Composure",
		"Dribbling Tackling Anticipation Positioning Flair WorkRate Stamina Agility",
		""),
}
