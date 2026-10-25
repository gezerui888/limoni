//go:build (darwin || windows || lemonsweep_gui) && !js

package main

import (
	"bytes"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"

	"github.com/thebanri/limoni"
	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// -gui plays in a window of the game's own, drawn with Ebitengine: the
// cells the terminal would be sent, painted as a terminal paints them, but
// with keys that say when they are let go and a mouse locked to the window.
// macOS and Windows build it without a C compiler; Linux, whose terminals
// do well, only with -tags lemonsweep_gui (it needs cgo and X11 headers).

type gui struct {
	g      *game
	buf    *buffer.Buffer
	pix    []byte        // two pixels a cell, its top and bottom half
	halves *ebiten.Image // pix, scaled up to the cells
	src    *text.GoTextFaceSource
	face   *text.GoTextFace
	top    text.DrawOptions

	scale          float64 // device pixels a window point
	sw, sh         int     // the screen in device pixels
	cw, ch         int     // a cell in device pixels, ch even
	ox, oy         int     // where the grid starts, centred
	cols, rows     int
	last           time.Time
	captured, skip bool
	cx, cy         int // the cursor where it was last seen
	focused        bool
	keys           []ebiten.Key
}

func runGUI(g *game) error {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
	if err != nil {
		return err
	}
	ebiten.SetWindowTitle("Lemon Sweep")
	ebiten.SetWindowSize(160*cellW, 50*cellH)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(ebiten.SyncWithFPS) // a step a frame, as fast as the screen shows them
	return ebiten.RunGame(&gui{g: g, src: src, last: time.Now(), focused: true})
}

// Layout asks for the screen in device pixels, so cells and letters are
// drawn sharp on a Retina screen rather than scaled up.
func (u *gui) Layout(w, h int) (int, int) {
	u.scale = ebiten.Monitor().DeviceScaleFactor()
	u.sw, u.sh = int(float64(w)*u.scale), int(float64(h)*u.scale)
	return u.sw, u.sh
}

func (u *gui) Update() error {
	g := u.g
	u.grid()
	u.input()
	now := time.Now()
	g.step(now.Sub(u.last).Seconds())
	u.last = now
	g.render(u.buf)
	g.countFrame(now)
	u.capture(g.phase == phPlay && u.focused)
	if g.quit {
		return ebiten.Termination
	}
	return nil
}

// grid fits as many cells as the window holds, and remakes the buffers when
// that changes.
func (u *gui) grid() {
	if u.scale == 0 {
		return
	}
	u.cw = max(1, int(math.Round(cellW*u.scale)))
	u.ch = max(2, 2*int(math.Round(cellH*u.scale/2)))
	cols, rows := max(20, u.sw/u.cw), max(8, u.sh/u.ch)
	u.ox, u.oy = max(0, (u.sw-cols*u.cw)/2), max(0, (u.sh-rows*u.ch)/2)
	if u.face == nil || u.face.Size != float64(u.cw)/0.6 {
		u.face = &text.GoTextFace{Source: u.src, Size: float64(u.cw) / 0.6} // Go Mono is 0.6 em wide
	}
	if cols == u.cols && rows == u.rows && u.buf != nil {
		return
	}
	u.cols, u.rows = cols, rows
	u.buf = buffer.NewBuffer(cell.Rect{Width: uint16(cols), Height: uint16(rows)})
	u.pix = make([]byte, cols*rows*2*4)
	if u.halves != nil {
		u.halves.Deallocate()
	}
	u.halves = ebiten.NewImage(cols, rows*2)
}

// capture locks the mouse to the window during a run and lets it go for
// the cards and menus, which are clicked.
func (u *gui) capture(want bool) {
	if want == u.captured {
		return
	}
	u.captured = want
	if want {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		u.skip = true // the first position after capture is not a movement
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
	u.g.locked = want
}

func (u *gui) input() {
	g := u.g
	if f := ebiten.IsFocused(); f != u.focused {
		u.focused = f
		g.held = [nAct]bool{}
		if !f && g.phase == phPlay {
			g.pauseRun()
		}
	}
	u.keys = inpututil.AppendJustPressedKeys(u.keys[:0])
	for _, k := range u.keys {
		if ev, ok := guiKey(k); ok {
			g.key(ev)
		}
	}
	u.keys = inpututil.AppendJustReleasedKeys(u.keys[:0])
	for _, k := range u.keys {
		if ev, ok := guiKey(k); ok {
			ev.Release = true
			g.key(ev)
		}
	}

	x, y := ebiten.CursorPosition()
	dx := x - u.cx
	u.cx, u.cy = x, y
	if u.captured {
		if u.skip {
			u.skip = false
		} else {
			g.lookDX += float64(dx) / u.scale // in points, as the browser's movement is
		}
		for _, b := range [...]struct {
			eb ebiten.MouseButton
			n  int
		}{{ebiten.MouseButtonLeft, 0}, {ebiten.MouseButtonRight, 2}} {
			if inpututil.IsMouseButtonJustPressed(b.eb) {
				g.pointerButton(b.n, true)
			} else if inpututil.IsMouseButtonJustReleased(b.eb) {
				g.pointerButton(b.n, false)
			}
		}
		return
	}
	if u.cw == 0 {
		return
	}
	m := limoni.MouseEvent{X: uint16(max(0, min(u.cols-1, (x-u.ox)/u.cw))), Y: uint16(max(0, min(u.rows-1, (y-u.oy)/u.ch)))}
	_, wheel := ebiten.Wheel()
	switch {
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		m.Button = limoni.MouseLeft
	case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
		m.Button = limoni.MouseRight
	case inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight):
		m.Button = limoni.MouseRelease
	case wheel > 0:
		m.Button = limoni.MouseScrollUp
	case wheel < 0:
		m.Button = limoni.MouseScrollDown
	case int(m.X) == g.mouseX && int(m.Y) == g.mouseY && g.mouseIn:
		return // nothing happened
	}
	g.mouse(m)
}

// guiKey is the key event a terminal would have sent for a key: keys by
// their place on the keyboard, so WASD is where it is on any layout.
func guiKey(k ebiten.Key) (limoni.KeyEvent, bool) {
	switch k {
	case ebiten.KeyArrowUp:
		return limoni.KeyEvent{Type: limoni.KeyUp}, true
	case ebiten.KeyArrowDown:
		return limoni.KeyEvent{Type: limoni.KeyDown}, true
	case ebiten.KeyArrowLeft:
		return limoni.KeyEvent{Type: limoni.KeyLeft}, true
	case ebiten.KeyArrowRight:
		return limoni.KeyEvent{Type: limoni.KeyRight}, true
	case ebiten.KeyEnter, ebiten.KeyNumpadEnter:
		return limoni.KeyEvent{Type: limoni.KeyEnter}, true
	case ebiten.KeyEscape:
		return limoni.KeyEvent{Type: limoni.KeyEsc}, true
	case ebiten.KeySpace:
		return limoni.KeyEvent{Type: limoni.KeySpace}, true
	case ebiten.KeyBackspace:
		return limoni.KeyEvent{Type: limoni.KeyBackspace}, true
	case ebiten.KeyTab:
		return limoni.KeyEvent{Type: limoni.KeyTab}, true
	}
	switch s := k.String(); {
	case len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z':
		return limoni.KeyEvent{Type: limoni.KeyRune, Ch: rune(s[0] - 'A' + 'a')}, true
	case len(s) == 6 && s[:5] == "Digit":
		return limoni.KeyEvent{Type: limoni.KeyRune, Ch: rune(s[5])}, true
	}
	return limoni.KeyEvent{}, false
}

func (u *gui) Draw(screen *ebiten.Image) {
	if u.buf == nil {
		return
	}
	// The pixels: each cell's two halves, one pixel each, scaled up.
	w := u.cols
	for i := range u.buf.Content {
		top, bot, _ := cellHalves(&u.buf.Content[i])
		x, y := i%w, i/w
		copy(u.pix[4*(2*y*w+x):], []byte{top.R, top.G, top.B, 255})
		copy(u.pix[4*((2*y+1)*w+x):], []byte{bot.R, bot.G, bot.B, 255})
	}
	u.halves.WritePixels(u.pix)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(float64(u.cw), float64(u.ch/2))
	op.GeoM.Translate(float64(u.ox), float64(u.oy))
	screen.DrawImage(u.halves, &op)
	// Then what is not pixels: letters, lines, shapes.
	for i := range u.buf.Content {
		c := &u.buf.Content[i]
		if _, _, glyph := cellHalves(c); glyph {
			fg, _ := cellColors(c)
			u.glyph(screen, u.ox+i%w*u.cw, u.oy+i/w*u.ch, c.Content, fg)
		}
	}
}

// glyph draws one character in the cell at (x, y): the block, box and
// shape characters by hand, as terminals do, so they meet their neighbours
// exactly; the rest from Go Mono.
func (u *gui) glyph(dst *ebiten.Image, x, y int, r rune, fg color.RGBA) {
	fx, fy, cw, ch := float32(x), float32(y), float32(u.cw), float32(u.ch)
	rect := func(x0, y0, x1, y1 float32) {
		vector.FillRect(dst, fx+x0*cw, fy+y0*ch, (x1-x0)*cw, (y1-y0)*ch, fg, false)
	}
	if bx, by, bw, bh, a, ok := blockRect(r); ok {
		c := fg
		if a < 1 {
			c = color.RGBA{uint8(float32(fg.R) * a), uint8(float32(fg.G) * a), uint8(float32(fg.B) * a), uint8(255 * a)}
		}
		vector.FillRect(dst, fx+float32(bx)*cw/8, fy+float32(by)*ch/8, float32(bw)*cw/8, float32(bh)*ch/8, c, false)
		return
	}
	if q, ok := quadrants[r]; ok {
		for i := uint8(0); i < 4; i++ {
			if q&(1<<i) != 0 {
				qx, qy := float32(i%2)/2, float32(i/2)/2
				rect(qx, qy, qx+0.5, qy+0.5)
			}
		}
		return
	}
	line := max(1, float32(math.Round(u.scale)))
	if seg, ok := cell.BoxDrawingSegments(r); ok || r == '━' || r == '┃' || r == '═' || r == '║' {
		wide := line
		switch r {
		case '━':
			seg, wide = cell.LineLeft|cell.LineRight, 2*line
		case '┃':
			seg, wide = cell.LineUp|cell.LineDown, 2*line
		case '═':
			vector.FillRect(dst, fx, fy+ch/2-2*line, cw, line, fg, false)
			vector.FillRect(dst, fx, fy+ch/2+line, cw, line, fg, false)
			return
		case '║':
			vector.FillRect(dst, fx+cw/2-2*line, fy, line, ch, fg, false)
			vector.FillRect(dst, fx+cw/2+line, fy, line, ch, fg, false)
			return
		}
		mx, my := float32(math.Floor(float64(cw/2-wide/2))), float32(math.Floor(float64(ch/2-wide/2)))
		if seg&cell.LineLeft != 0 {
			vector.FillRect(dst, fx, fy+my, mx+wide, wide, fg, false)
		}
		if seg&cell.LineRight != 0 {
			vector.FillRect(dst, fx+mx, fy+my, cw-mx, wide, fg, false)
		}
		if seg&cell.LineUp != 0 {
			vector.FillRect(dst, fx+mx, fy, wide, my+wide, fg, false)
		}
		if seg&cell.LineDown != 0 {
			vector.FillRect(dst, fx+mx, fy+my, wide, ch-my, fg, false)
		}
		return
	}
	var tri [6]float32 // a triangle's corners, in fractions of the cell
	switch r {
	case '▲':
		tri = [6]float32{0.5, 0.2, 0.9, 0.8, 0.1, 0.8}
	case '▼':
		tri = [6]float32{0.1, 0.2, 0.9, 0.2, 0.5, 0.8}
	case '▶', '►':
		tri = [6]float32{0.1, 0.2, 0.9, 0.5, 0.1, 0.8}
	case '◀', '◄':
		tri = [6]float32{0.9, 0.2, 0.1, 0.5, 0.9, 0.8}
	case '◢':
		tri = [6]float32{1, 0, 1, 1, 0, 1}
	case '◣':
		tri = [6]float32{0, 0, 1, 1, 0, 1}
	case '◤':
		tri = [6]float32{0, 0, 1, 0, 0, 1}
	case '◥':
		tri = [6]float32{0, 0, 1, 0, 1, 1}
	case '▮':
		rect(0.2, 0.15, 0.8, 0.85)
		return
	case '▯':
		vector.StrokeRect(dst, fx+0.2*cw, fy+0.15*ch, 0.6*cw, 0.7*ch, line, fg, false)
		return
	case '╱':
		vector.StrokeLine(dst, fx+cw, fy, fx, fy+ch, line, fg, true)
		return
	case '╲':
		vector.StrokeLine(dst, fx, fy, fx+cw, fy+ch, line, fg, true)
		return
	default:
		if r >= 0x2800 && r <= 0x28ff { // braille: two columns of four dots
			dots := [8][2]float32{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
			for i, d := range dots {
				if (r-0x2800)&(1<<i) != 0 {
					cx, cy := fx+(0.3+0.4*d[0])*cw, fy+(0.14+0.24*d[1])*ch
					vector.FillCircle(dst, cx, cy, max(line, cw/8), fg, true)
				}
			}
			return
		}
		u.top.GeoM.Reset()
		m := u.face.Metrics()
		u.top.GeoM.Translate(float64(x), float64(y)+(float64(u.ch)-(m.HAscent+m.HDescent))/2)
		u.top.ColorScale.Reset()
		u.top.ColorScale.ScaleWithColor(fg)
		text.Draw(dst, string(r), u.face, &u.top)
		return
	}
	var p vector.Path
	p.MoveTo(fx+tri[0]*cw, fy+tri[1]*ch)
	p.LineTo(fx+tri[2]*cw, fy+tri[3]*ch)
	p.LineTo(fx+tri[4]*cw, fy+tri[5]*ch)
	p.Close()
	var fo vector.FillOptions
	var do vector.DrawPathOptions
	do.ColorScale.ScaleWithColor(fg)
	vector.FillPath(dst, &p, &fo, &do)
}
