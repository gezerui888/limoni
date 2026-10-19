package main

import (
	"image"
	"image/color"
	"image/draw"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/thebanri/limoni/graphics"
	"github.com/thebanri/limoni/internal/overlay"
)

// A virtual placement (U=1) is shown wherever the program writes kitty's
// placeholder, U+10EEEE, in its cells: the foreground colour names the
// image, the underline colour the placement, and diacritics give the row and
// column of the picture each cell shows (and the image id's top byte). The
// program's text is the picture's position, so it scrolls, moves and goes
// with the text; tmux and editors use it for that.
const placeholderRune = 0x10EEEE

type kittyVirtual struct {
	image, placement uint32
	anim             *anim
	part             image.Rectangle // of the frames, as x, y, w, h chose
	cols, rows       int
	// canvas is the picture fitted into cols×rows cells of cellW×cellH,
	// made from src; parts are its pieces shown, kept so a piece shown
	// again is the same value and the overlay need not send it again.
	src          image.Image
	cellW, cellH int
	canvas       *image.RGBA
	parts        map[image.Rectangle]image.Image
}

var diacriticIndex = func() map[rune]int {
	m := map[rune]int{}
	for _, r := range diacritics {
		m[r] = len(m)
	}
	return m
}()

// placeholderCell is what a placeholder cell says.
type placeholderCell struct {
	image, placement uint32
	row, col, msb    int
}

// readPlaceholder decodes a cell, prev being the cell to its left when that
// is a placeholder (ok). As kitty has it, a missing row is prev's, a missing
// column the one after prev's, and a missing top byte prev's, when prev has
// the same colours.
func readPlaceholder(c *uv.Cell, prev placeholderCell, ok bool) (placeholderCell, bool) {
	r, n := utf8.DecodeRuneInString(c.Content)
	if r != placeholderRune {
		return placeholderCell{}, false
	}
	p := placeholderCell{image: colorID(c.Style.Fg), placement: colorID(c.Style.UnderlineColor)}
	d, j := [3]int{-1, -1, -1}, 0
	for _, m := range c.Content[n:] {
		v, found := diacriticIndex[m]
		if !found || j == len(d) {
			break
		}
		d[j], j = v, j+1
	}
	p.row, p.col, p.msb = d[0], d[1], d[2]
	same := ok && prev.image&0xffffff == p.image && prev.placement == p.placement
	if p.row < 0 {
		p.row = 0
		if same {
			p.row = prev.row
		}
	}
	if p.col < 0 {
		p.col = 0
		if same && p.row == prev.row {
			p.col = prev.col + 1
		}
	}
	if p.msb < 0 {
		p.msb = 0
		if same && p.row == prev.row && p.col == prev.col+1 {
			p.msb = prev.msb
		}
	}
	p.image |= uint32(p.msb) << 24
	return p, true
}

// colorID is the number a colour stands for: a 256-colour index, or the 24
// bits of a true colour; 0 for none.
func colorID(c color.Color) uint32 {
	switch c := c.(type) {
	case nil:
		return 0
	case ansi.IndexedColor:
		return uint32(c)
	case ansi.BasicColor:
		return uint32(c)
	}
	r, g, b, _ := c.RGBA()
	return r>>8<<16 | g>>8<<8 | b>>8
}

// placeholders appends to pics the pieces of virtual placements that the
// screen's placeholders show: each run of cells showing consecutive columns
// of one row, runs on consecutive lines and rows joined into one piece.
func (s *session) placeholders(pics []overlay.Picture) []overlay.Picture {
	type piece struct {
		v              *kittyVirtual
		x, y, n, h     int // in screen cells
		col, row, rows int // in the picture's cells
	}
	var pieces []piece
	join := func(p piece) {
		for i := range pieces {
			q := &pieces[i]
			if q.v == p.v && q.x == p.x && q.n == p.n && q.col == p.col && q.y+q.h == p.y && q.row+q.h == p.row {
				q.h++
				return
			}
		}
		pieces = append(pieces, p)
	}
	for y := range s.rows {
		var prev placeholderCell
		var ok bool
		var run piece
		for x := 0; x <= s.cols; x++ {
			var p placeholderCell
			var v *kittyVirtual
			if c := s.emu.CellAt(x, y); x < s.cols && c != nil {
				if p, ok = readPlaceholder(c, prev, ok); ok {
					v = s.virtualFor(p)
				}
			} else {
				ok = false
			}
			prev = p
			if run.v != nil && v == run.v && p.row == run.row && p.col == run.col+run.n && x == run.x+run.n {
				run.n++
				continue
			}
			if run.v != nil {
				join(run)
			}
			run = piece{}
			if v != nil && p.row < v.rows && p.col < v.cols {
				run = piece{v: v, x: x, y: y, n: 1, h: 1, col: p.col, row: p.row}
			}
		}
	}
	for _, p := range pieces {
		n := min(p.n, p.v.cols-p.col) // a run past the picture's right edge shows nothing there
		h := min(p.h, p.v.rows-p.row)
		img := s.piece(p.v, image.Rect(p.col*s.cellW, p.row*s.cellH, (p.col+n)*s.cellW, (p.row+h)*s.cellH))
		pics = append(pics, overlay.Picture{Img: img, X: p.x * s.cellW, Y: p.y * s.cellH, W: n * s.cellW, H: h * s.cellH})
	}
	return pics
}

// virtualFor is the virtual placement a placeholder names; placement 0 is
// the image's first.
func (s *session) virtualFor(p placeholderCell) *kittyVirtual {
	for _, v := range s.kitty.virtual {
		if v.image == p.image && (p.placement == 0 || v.placement == p.placement) {
			return v
		}
	}
	return nil
}

// piece is the part r of v's canvas, which is made again when the frame or
// the cell size has changed: the picture fitted into its cells, centred, as
// kitty shows it.
func (s *session) piece(v *kittyVirtual, r image.Rectangle) image.Image {
	if src := v.anim.frames[v.anim.cur]; src != v.src || v.cellW != s.cellW || v.cellH != s.cellH {
		v.src, v.cellW, v.cellH, v.parts = src, s.cellW, s.cellH, map[image.Rectangle]image.Image{}
		w, h := v.cols*s.cellW, v.rows*s.cellH
		img := subImage(src, v.part)
		fw, fh := graphics.ContainSize(img, w, h)
		fit := graphics.ResizeImage(img, fw, fh)
		v.canvas = image.NewRGBA(image.Rect(0, 0, w, h))
		at := image.Pt((w-fw)/2, (h-fh)/2)
		draw.Draw(v.canvas, image.Rect(0, 0, fw, fh).Add(at), fit, fit.Bounds().Min, draw.Src)
	}
	img := v.parts[r]
	if img == nil {
		img = v.canvas.SubImage(r)
		v.parts[r] = img
	}
	return img
}
