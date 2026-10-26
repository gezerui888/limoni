package main

import (
	"image/color"
	"testing"

	"github.com/thebanri/limoni/core/cell"
)

// -gui reads the cells the way a terminal would: ▀ is the foreground over
// the background, ▄ the other way up, reverse video swaps them, and a
// glyph is left to draw only where a cell is not plain pixels.
func TestGUICells(t *testing.T) {
	red, blue := cell.NewColorRGB(255, 0, 0), cell.NewColorRGB(0, 0, 255)
	r, b := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	for _, tc := range []struct {
		ch        rune
		mod       cell.Modifier
		top, bot  color.RGBA
		wantGlyph bool
	}{
		{'▀', 0, r, b, false},
		{'▄', 0, b, r, false},
		{'█', 0, r, r, false},
		{' ', 0, b, b, false},
		{'▀', cell.ModifierReverse, b, r, false},
		{'A', 0, b, b, true},
	} {
		c := cell.Cell{Content: tc.ch, Style: cell.Style{Fg: red, Bg: blue, Modifier: tc.mod}}
		top, bot, glyph := cellHalves(&c)
		if top != tc.top || bot != tc.bot || glyph != tc.wantGlyph {
			t.Errorf("%q: %v %v %v, want %v %v %v", tc.ch, top, bot, glyph, tc.top, tc.bot, tc.wantGlyph)
		}
	}
	if got := ansiColor(196); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("colour 196 is %v", got)
	}
	if got := ansiColor(244); got != (color.RGBA{128, 128, 128, 255}) {
		t.Errorf("colour 244 is %v", got)
	}
	if x, y, w, h, _, _ := blockRect('▂'); x != 0 || y != 6 || w != 8 || h != 2 {
		t.Errorf("▂ fills %d,%d %dx%d", x, y, w, h)
	}
	if x, _, w, _, _, _ := blockRect('▏'); x != 0 || w != 1 {
		t.Errorf("▏ is %d wide", w)
	}
	if quadrants['▙'] != 1|4|8 {
		t.Error("▙ is not upper left and the bottom")
	}
}
