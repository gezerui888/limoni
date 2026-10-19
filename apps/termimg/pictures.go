package main

import (
	"image"
	"maps"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/charmbracelet/x/vt"
)

// Pictures are tracked in the shadow screen by marks: every cell a picture
// covers gets a private-use character saying which picture and which of its
// rows it is. The marks go to the shadow screen only, never to the real
// terminal. Scrolling, inserted lines and scroll regions carry them along,
// so a picture is wherever its marks are; anything written over a cell,
// even a space, replaces the mark, and the picture goes, as a sixel picture
// does in a terminal that has them. A screen switched away hides its marks,
// and with them its pictures.
const (
	markBase = 0x100000 // plane 16, private use: id<<8 | row
	maxRows  = 256
	maxID    = 0xed // marks stay below U+10EEEE, kitty's placeholder
)

func mark(id, row int) rune { return rune(markBase | id<<8 | row) }

func unmark(s string) (id, row int, ok bool) {
	r, _ := utf8.DecodeRuneInString(s)
	if r < markBase || r > markBase|maxID<<8|(maxRows-1) {
		return 0, 0, false
	}
	return int(r>>8) & 0xff, int(r) & 0xff, true
}

type picture struct {
	img        image.Image
	cols, rows int
	// col and top are where its marks were last seen; top may be above the
	// screen when the picture has partly scrolled away.
	col, top int
	// dx and dy place img within its cells: its transparent edges are cut
	// off, as the overlay cannot show through them (see opaque); ox and oy
	// are kitty's offset within the first cell (X, Y).
	dx, dy, ox, oy int
	// An animated picture shows anim's current frame, src, made into img
	// by render.
	anim   *anim
	src    image.Image
	render func(image.Image) image.Image
	// alt is the screen it is on: while the other one shows, so do not its
	// marks, and it is neither looked for nor shown.
	alt bool
	// sticky pictures (kitty's) stay when text is written over them, as in
	// kitty: any of their marks left places them; they go when none is.
	// shortcut: one written over in full goes, where kitty would keep it.
	sticky bool
}

// pictures are the pictures on the shadow screen, by id.
type pictures struct {
	emu  *vt.Emulator
	byID map[int]*picture
	last int
}

// add marks a picture of cols×rows cells at col, top (rows above the screen
// are skipped) and returns its id, 0 when it cannot be shown. The cursor is
// put back with CUP, not DECSC/DECRC, which would lose the program's own
// saved cursor.
func (p *pictures) add(img image.Image, col, top, cols, rows int, sticky bool) int {
	id := p.freeID()
	if id == 0 {
		return 0
	}
	rows = min(rows, maxRows)
	cols = min(cols, p.emu.Width()-col)
	if cols <= 0 || rows <= 0 {
		return 0
	}
	pic := &picture{img: img, cols: cols, rows: rows, col: col, top: top, alt: p.emu.IsAltScreen(), sticky: sticky}
	var b []byte
	for r := range rows {
		y := top + r
		if y < 0 || y >= p.emu.Height() {
			continue
		}
		b = cup(b, col, y)
		for range cols {
			b = utf8.AppendRune(b, mark(id, r))
		}
	}
	at := p.emu.CursorPosition()
	p.emu.Write(cup(b, at.X, at.Y))
	p.byID[id] = pic
	return id
}

// idOf returns the id pic is shown under, 0 if it is gone. Kept pictures
// are found by themselves, not by id: an id is used again once its picture
// has gone.
func (p *pictures) idOf(pic *picture) int {
	for id, q := range p.byID {
		if q == pic {
			return id
		}
	}
	return 0
}

// remove takes a picture away, its marks with it.
func (p *pictures) remove(id int) {
	if p.byID[id] != nil {
		delete(p.byID, id)
		p.blank(map[int]bool{id: true})
	}
}

func (p *pictures) freeID() int {
	for range maxID {
		p.last = p.last%maxID + 1
		if p.byID[p.last] == nil {
			return p.last
		}
	}
	return 0
}

// seen is where one row of a picture's marks was found: n cells from x0 on
// line y, or n < 0 if the row turned up in pieces.
type seen struct{ y, x0, n int }

// update finds every picture by its marks and drops those whose marks are
// broken: a cell written over, or a row missing from the middle of the
// screen. A dropped picture's leftover marks are blanked, so its id can be
// used again.
func (p *pictures) update() {
	if len(p.byID) == 0 {
		return
	}
	found := map[int]map[int]*seen{} // id → row → where
	w, h := p.emu.Width(), p.emu.Height()
	for y := range h {
		for x := range w {
			c := p.emu.CellAt(x, y)
			if c == nil {
				continue
			}
			id, row, ok := unmark(c.Content)
			if !ok {
				continue
			}
			rows := found[id]
			if rows == nil {
				rows = map[int]*seen{}
				found[id] = rows
			}
			if s := rows[row]; s == nil {
				rows[row] = &seen{y: y, x0: x, n: 1}
			} else if s.n > 0 && s.y == y && s.x0+s.n == x {
				s.n++
			} else {
				s.n = -1
			}
		}
	}
	stale := map[int]bool{}
	alt := p.emu.IsAltScreen()
	for id, pic := range p.byID {
		if pic.alt != alt {
			continue
		}
		if !pic.locate(found[id], h) {
			delete(p.byID, id)
			stale[id] = found[id] != nil
		}
	}
	for id := range found {
		if p.byID[id] == nil {
			stale[id] = true
		}
	}
	p.blank(stale)
}

// locate sets where pic is from the rows of marks found, and reports whether
// they make a whole picture: every row that should be on the screen is
// there, complete and in line with the others.
func (pic *picture) locate(rows map[int]*seen, h int) bool {
	if len(rows) == 0 {
		return false
	}
	if pic.sticky {
		// Any row gives the top; only a whole one gives the column, as text
		// may have covered a row's first cells.
		for r, s := range rows {
			pic.top = s.y - r
			if s.n == pic.cols {
				pic.col = s.x0
				break
			}
		}
		return true
	}
	col, top := -1, 0
	for r, s := range rows {
		if r >= pic.rows || s.n != pic.cols {
			return false
		}
		if col < 0 {
			col, top = s.x0, s.y-r
		} else if s.x0 != col || s.y-r != top {
			return false
		}
	}
	for r := range pic.rows {
		if y := top + r; y >= 0 && y < h && rows[r] == nil {
			return false
		}
	}
	pic.col, pic.top = col, top
	return true
}

// blank writes spaces over the marks of the ids that are true in ids.
func (p *pictures) blank(ids map[int]bool) {
	if !slices.Contains(slices.Collect(maps.Values(ids)), true) {
		return // nothing to blank: spare the scan
	}
	var b []byte
	for y := range p.emu.Height() {
		for x := range p.emu.Width() {
			if c := p.emu.CellAt(x, y); c != nil {
				if id, _, ok := unmark(c.Content); ok && ids[id] {
					b = append(cup(b, x, y), ' ')
				}
			}
		}
	}
	if len(b) > 0 {
		at := p.emu.CursorPosition()
		p.emu.Write(cup(b, at.X, at.Y))
	}
}

func cup(b []byte, x, y int) []byte {
	b = append(b, "\x1b["...)
	b = strconv.AppendInt(b, int64(y+1), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(x+1), 10)
	return append(b, 'H')
}
