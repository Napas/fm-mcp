package fm

// Media handling styles: the bits of the game's mask, named by 0x3697990.
const (
	mediaOutspoken       uint16 = 0x1
	mediaEvasive         uint16 = 0x2
	mediaVolatile        uint16 = 0x4
	mediaMindGames       uint16 = 0x8 // never set by the rule
	mediaUnflappable     uint16 = 0x10
	mediaFriendly        uint16 = 0x20
	mediaLevelHeaded     uint16 = 0x40
	mediaReserved        uint16 = 0x80
	mediaShortTempered   uint16 = 0x100
	mediaConfrontational uint16 = 0x200
)

// mediaStyleNames names the styles by bit number, the order the game joins them.
var mediaStyleNames = [...]string{
	"Outspoken", "Evasive", "Volatile", "Plays Mind Games", "Unflappable",
	"Media-friendly", "Level-headed", "Reserved", "Short-tempered", "Confrontational",
}

var attrControversy = mustAttribute("Controversy")

// mediaInput holds the media handling rule's inputs, hidden attributes 1-20.
type mediaInput struct {
	loyalty, pressure, professionalism, sportsmanship, temperament, controversy int

	negative bool // the save's flag bit 3, as for the personality
}

// mask is the game's media handling rule (game_plugin 26.3.2, 0x2f958d0,
// restrict set as for the personality). Without the negative flag, Volatile,
// Short-tempered and Confrontational give way to Outspoken; the game adds 1
// to the mask, so an Outspoken mask becomes Evasive.
func (in mediaInput) mask() uint16 {
	loy, pre, pro, spo, tem, con := in.loyalty, in.pressure, in.professionalism, in.sportsmanship, in.temperament, in.controversy
	var m uint16
	switch {
	case tem < 3:
		m = mediaShortTempered
	case tem < 7:
		m = mediaVolatile
	}
	switch {
	case pro >= 15 && tem <= 15 && pre >= 15 && (spo > 12 || con <= 14):
		m |= mediaEvasive
	case con >= 15:
		m |= mediaOutspoken
	case pro <= 12 && spo < 12 || loy <= 10:
		m |= mediaFriendly
	}
	switch {
	case spo <= 7 && tem <= 7:
		m |= mediaConfrontational
	case tem >= 15 && pre >= 15:
		m |= mediaUnflappable
	case con <= 5 && m&(mediaShortTempered|mediaVolatile) == 0 && pro > 14:
		m |= mediaReserved
	}
	if m == 0 {
		m = mediaLevelHeaded
	}
	if negative := mediaShortTempered | mediaConfrontational | mediaVolatile; !in.negative && m&negative != 0 {
		m = m&^(negative|mediaEvasive) + 1
	}
	return m
}

// mediaNames names the styles set in m, in bit order.
func mediaNames(m uint16) []string {
	var out []string
	for bit, name := range mediaStyleNames {
		if m&(1<<bit) != 0 {
			out = append(out, name)
		}
	}
	return out
}

// mediaHandling is FM's media handling, computed on demand.
func (p *Player) mediaHandling() []string {
	a := p.Attributes
	return mediaNames(mediaInput{
		loyalty: int(a[attrLoyalty]), pressure: int(a[attrPressure]), professionalism: int(a[attrProfessionalism]),
		sportsmanship: int(a[attrSportsmanship]), temperament: int(a[attrTemperament]), controversy: int(a[attrControversy]),
		negative: p.negativeLabels,
	}.mask())
}
