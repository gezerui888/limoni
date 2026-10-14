package main

import (
	"math"

	"github.com/thebanri/limoni"
	"github.com/thebanri/limoni/core/cell"
)

// quitModal is the "Quit pikap?" question. It opens and closes by growing
// and shrinking from the middle of the window, dims what is behind it, and
// its two buttons warm up under the pointer.
type quitModal struct {
	open bool
	p    float64 // how open it is: 0 gone … 1 fully open, moved by tick

	sel     int // the button the keys are on: 0 yes, 1 no
	hovered int // the button under the pointer, -1 for none
	glow    [2]float64

	// Where it was drawn, for the mouse.
	card    cell.Rect
	buttons [2]span
	bh      int // the buttons' height in rows
}

const (
	quitYes = iota
	quitNo
)

const (
	quitOpenTime = 0.22 // seconds from closed to open, and back
	quitCardW    = 38
	quitCardH    = 9
	quitBtnW     = 12
)

// The yes button turns the red of a warning; the no button, the accent.
var quitRed = lin(0xe5484d)

func (q *quitModal) visible() bool { return q.open || q.p > 0 }

func (q *quitModal) show() {
	q.open, q.sel, q.hovered = true, quitNo, -1
}

func (q *quitModal) hide() { q.open = false }

// tick moves the animations dt seconds on.
func (q *quitModal) tick(dt float64) {
	if q.open {
		q.p = min(1, q.p+dt/quitOpenTime)
	} else {
		q.p = max(0, q.p-dt/quitOpenTime)
	}
	for i := range q.glow {
		target := 0.0
		if q.open && (i == q.hovered || (q.hovered < 0 && i == q.sel)) {
			target = 1
		}
		q.glow[i] += (target - q.glow[i]) * (1 - math.Exp(-dt/0.07))
	}
}

// ease is p shaped to overshoot a little and settle, so the card pops
// open; closing is quick and without a bounce.
func (q *quitModal) ease() float64 {
	t := q.p
	if q.open {
		const c1, c3 = 1.2, 2.2
		t--
		return 1 + c3*t*t*t + c1*t*t
	}
	return t * t * (3 - 2*t)
}

// key handles a key while the modal is up; every key is taken. It reports
// whether the answer was yes.
func (q *quitModal) key(k limoni.KeyEvent) (quit bool) {
	if !q.open {
		return false
	}
	switch k.Type {
	case limoni.KeyEsc:
		q.hide()
	case limoni.KeyLeft, limoni.KeyUp:
		q.sel, q.hovered = quitYes, -1
	case limoni.KeyRight, limoni.KeyDown:
		q.sel, q.hovered = quitNo, -1
	case limoni.KeyTab:
		q.sel, q.hovered = 1-q.sel, -1
	case limoni.KeyEnter, limoni.KeySpace:
		if q.sel == quitYes {
			return true
		}
		q.hide()
	case limoni.KeyRune:
		switch k.Ch {
		case 'y', 'Y':
			return true
		case 'n', 'N', 'q', 'Q':
			q.hide()
		case 'h', 'H':
			q.sel, q.hovered = quitYes, -1
		case 'l', 'L':
			q.sel, q.hovered = quitNo, -1
		}
	}
	return false
}

// button is which button is at (x, y), or -1.
func (q *quitModal) button(x, y int) int {
	for i, b := range q.buttons {
		if b.w > 0 && y >= b.y && y < b.y+q.bh && x >= b.x && x < b.x+b.w {
			return i
		}
	}
	return -1
}

// mouse handles the pointer while the modal is up; it takes every event.
// It reports whether the answer was yes.
func (q *quitModal) mouse(m limoni.MouseEvent) (quit bool) {
	if !q.open {
		return false
	}
	x, y := int(m.X), int(m.Y)
	b := q.button(x, y)
	switch {
	case m.Button == limoni.MouseNone || m.Drag:
		q.hovered = b
		if b >= 0 {
			q.sel = b
		}
	case m.Button == limoni.MouseLeft:
		switch {
		case b == quitYes:
			return true
		case b == quitNo, !q.inCard(x, y):
			q.hide()
		}
	}
	return false
}

func (q *quitModal) inCard(x, y int) bool {
	c := q.card
	return c.Width > 0 && x >= int(c.X) && x < int(c.X+c.Width) && y >= int(c.Y) && y < int(c.Y+c.Height)
}

// dimCells darkens every cell by f, 1 being as it was.
func dimCells(c canvas, f float64) {
	scale := func(col cell.Color) cell.Color {
		if col.Type() != cell.ColorRGB {
			return col
		}
		r, g, b := col.RGB()
		return cell.NewColorRGB(uint8(float64(r)*f), uint8(float64(g)*f), uint8(float64(b)*f))
	}
	for i := range c.b.Content[:c.w*c.h] {
		cl := &c.b.Content[i]
		cl.Style.Fg, cl.Style.Bg = scale(cl.Style.Fg), scale(cl.Style.Bg)
	}
}

// drawQuit puts the modal over the middle of the window.
func (u *ui) drawQuit(c canvas, accent rgb) {
	q := &u.qm
	q.card, q.buttons = cell.Rect{}, [2]span{}
	if !q.visible() || c.w < quitCardW+2 || c.h < quitCardH+2 {
		return
	}
	e := q.ease()
	dimCells(c, 1-0.6*min(1, q.p))

	// The card grows from the middle; only its width overshoots.
	w := max(2, int(math.Round(quitCardW*(0.55+0.45*e))))
	h := max(1, min(quitCardH, int(math.Round(quitCardH*e))))
	x0, y0 := (c.w-w)/2, (c.h-h)/2
	cardBg := func(y int) rgb { return u.rows[y].scale(0.3) }
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			c.set(x, y, ' ', style(textCol, cardBg(y)))
		}
	}
	// A thin rim in the accent, which fades in with the card.
	rim := func(x, y int, r rune) {
		bg := cardBg(y)
		c.set(x, y, r, style(bg.mix(accent, min(1, q.p)), bg))
	}
	if h >= 3 {
		for x := x0 + 1; x < x0+w-1; x++ {
			rim(x, y0, '─')
			rim(x, y0+h-1, '─')
		}
		for y := y0 + 1; y < y0+h-1; y++ {
			rim(x0, y, '│')
			rim(x0+w-1, y, '│')
		}
		rim(x0, y0, '╭')
		rim(x0+w-1, y0, '╮')
		rim(x0, y0+h-1, '╰')
		rim(x0+w-1, y0+h-1, '╯')
	}
	if !q.open || h < quitCardH || q.p < 0.6 {
		return // the contents come in with the card, and go before it
	}

	q.card = cell.Rect{X: uint16(x0), Y: uint16(y0), Width: uint16(w), Height: uint16(h)}
	fade := smoothstep(0.6, 1, q.p)
	say := func(y int, s string, fg rgb, bold bool) {
		st := style(cardBg(y).mix(fg, fade), cardBg(y))
		if bold {
			st = st.Bold()
		}
		c.text(x0+(w-cell.StringWidth(s))/2, y, s, st)
	}
	say(y0+2, "Quit pikap?", textCol, true)
	say(y0+3, "The record stops turning.", subCol, false)

	// Two buttons, three rows tall, side by side.
	q.bh = 3
	const gap = 3
	bx := x0 + (w-(2*quitBtnW+gap))/2
	by := y0 + 5
	labels := [2]string{"Yes", "No"}
	for i, label := range labels {
		g := q.glow[i]
		x := bx + i*(quitBtnW+gap)
		q.buttons[i] = span{x, by, quitBtnW}
		warm := accent
		if i == quitYes {
			warm = quitRed
		}
		fill := func(y int) rgb {
			base := cardBg(y).mix(textCol, 0.1)
			return cardBg(y).mix(base.mix(warm, 0.9*g), fade)
		}
		for y := by; y < by+q.bh; y++ {
			for xx := x; xx < x+quitBtnW; xx++ {
				c.set(xx, y, ' ', style(textCol, fill(y)))
			}
		}
		st := style(subCol.mix(textCol, g), fill(by+1))
		if g > 0.5 {
			st = st.Bold()
		}
		c.text(x+(quitBtnW-len(label))/2, by+1, label, st)
	}
}
