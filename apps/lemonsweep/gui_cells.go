package main

import (
	"image/color"

	"github.com/thebanri/limoni/core/cell"
)

// -gui draws the same cells the terminal is sent, as a terminal would: what
// is decided here has no window in it, so it is tested everywhere.

// A cell is cellW×cellH window points: a terminal's shape, so the game lays
// itself out as it does in one.
const cellW, cellH = 8, 16

var (
	guiFg = color.RGBA{0xd0, 0xd0, 0xd0, 0xff} // the default colours, where a cell names none
	guiBg = color.RGBA{0x00, 0x00, 0x00, 0xff}
)

// cellColors is a cell's foreground and background, reverse video applied.
func cellColors(c *cell.Cell) (fg, bg color.RGBA) {
	fg, bg = resolveColor(c.Style.Fg, guiFg), resolveColor(c.Style.Bg, guiBg)
	if c.Style.Modifier&cell.ModifierReverse != 0 {
		fg, bg = bg, fg
	}
	return fg, bg
}

func resolveColor(c cell.Color, def color.RGBA) color.RGBA {
	switch c.Type() {
	case cell.ColorRGB:
		r, g, b := c.RGB()
		return color.RGBA{r, g, b, 0xff}
	case cell.ColorANSI:
		return ansiColor(c.ANSI())
	}
	return def
}

// ansiColor is xterm's 256-colour palette.
func ansiColor(n uint8) color.RGBA {
	base := [16]color.RGBA{
		{0, 0, 0, 255}, {205, 0, 0, 255}, {0, 205, 0, 255}, {205, 205, 0, 255},
		{0, 0, 238, 255}, {205, 0, 205, 255}, {0, 205, 205, 255}, {229, 229, 229, 255},
		{127, 127, 127, 255}, {255, 0, 0, 255}, {0, 255, 0, 255}, {255, 255, 0, 255},
		{92, 92, 255, 255}, {255, 0, 255, 255}, {0, 255, 255, 255}, {255, 255, 255, 255},
	}
	switch {
	case n < 16:
		return base[n]
	case n < 232:
		n -= 16
		step := func(v uint8) uint8 {
			if v == 0 {
				return 0
			}
			return 55 + 40*v
		}
		return color.RGBA{step(n / 36), step(n / 6 % 6), step(n % 6), 255}
	}
	v := 8 + 10*(n-232)
	return color.RGBA{v, v, v, 255}
}

// cellHalves is the colour of a cell's top and bottom half, and whether a
// glyph still has to be drawn over them: the world is half blocks, so most
// cells are two pixels and nothing more.
func cellHalves(c *cell.Cell) (top, bot color.RGBA, glyph bool) {
	fg, bg := cellColors(c)
	switch c.Content {
	case '▀':
		return fg, bg, false
	case '▄':
		return bg, fg, false
	case '█':
		return fg, fg, false
	case ' ', 0:
		return bg, bg, false
	}
	return bg, bg, true
}

// quadrants maps ▖…▟ to the quarters they fill: 1 upper left, 2 upper
// right, 4 lower left, 8 lower right.
var quadrants = map[rune]uint8{
	'▖': 4, '▗': 8, '▘': 1, '▙': 1 | 4 | 8, '▚': 1 | 8,
	'▛': 1 | 2 | 4, '▜': 1 | 2 | 8, '▝': 2, '▞': 2 | 4, '▟': 2 | 4 | 8,
}

// blockRect is the part of the cell a block element fills, in eighths of
// its width and height (x, y, w, h), and how much of the colour it takes
// (the shades); ok is false for anything else.
func blockRect(r rune) (x, y, w, h int, alpha float32, ok bool) {
	switch {
	case r >= '▁' && r <= '▇': // lower n eighths
		n := int(r-'▁') + 1
		return 0, 8 - n, 8, n, 1, true
	case r >= '▉' && r <= '▏': // left n eighths, 7 down to 1
		return 0, 0, 7 - int(r-'▉'), 8, 1, true
	case r == '▐':
		return 4, 0, 4, 8, 1, true
	case r == '▔':
		return 0, 0, 8, 1, 1, true
	case r == '▕':
		return 7, 0, 1, 8, 1, true
	case r >= '░' && r <= '▓':
		return 0, 0, 8, 8, 0.25 * float32(r-'░'+1), true
	}
	return 0, 0, 0, 0, 0, false
}
