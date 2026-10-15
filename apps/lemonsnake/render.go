package main

import (
	"math"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// The board is drawn in half blocks: a cell is two pixels, '▀' with the
// upper as the foreground and the lower as the background, and pixels come
// out square. A tile is tilePx×tilePx pixels, four cells by two rows.
//
// Between steps the snake is drawn where it is going: the head slides into
// the next tile and the tail out of the last one, by the fraction of the
// step that has passed. Shapes are painted with their coverage of each
// pixel, so a pixel half covered is half blended, and at 60 frames a second
// the motion reads as continuous rather than a tile at a time.

const (
	tilePx = 4
	panelW = 16
	panelH = 18
	margin = 1 // cells kept clear at the window's left and right edges
	gap    = 2
	pxW    = maxCols * tilePx // the pixel buffer's stride
)

type rgb struct{ r, g, b float64 }

func hex(v uint32) rgb {
	return rgb{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}
}

func (c rgb) mix(o rgb, t float64) rgb {
	return rgb{c.r + (o.r-c.r)*t, c.g + (o.g-c.g)*t, c.b + (o.b-c.b)*t}
}

func to8(v float64) uint8 { return uint8(math.Round(max(0, min(1, v)) * 255)) }

func (c rgb) color() cell.Color { return cell.NewColorRGB(to8(c.r), to8(c.g), to8(c.b)) }

func style(fg, bg rgb) cell.Style { return cell.Style{Fg: fg.color(), Bg: bg.color()} }

var (
	black = rgb{}
	red   = hex(0xff2a2a)
)

type theme struct {
	name                     string
	bg, tileA, tileB, border rgb
	text, label, accent      rgb
	head, tail, eye          rgb
	lemon, lemonHi, leaf     rgb
	seed                     rgb
	boss, bossEye, bossTooth rgb
}

var themes = [...]theme{
	{
		name: "Lemon Grove",
		bg:   hex(0x101410), tileA: hex(0x172019), tileB: hex(0x1b261e), border: hex(0x6b8f3a),
		text: hex(0xe8ecd8), label: hex(0x8a9a7a), accent: hex(0xf5d000),
		head: hex(0x9be15d), tail: hex(0x2f6e32), eye: hex(0x0b1a0b),
		lemon: hex(0xffd51c), lemonHi: hex(0xfff6a8), leaf: hex(0x5fd35a), seed: hex(0xf5ecc2),
		boss: hex(0x9a45d1), bossEye: hex(0xffe14d), bossTooth: hex(0xffffff),
	},
	{
		name: "Neon Night",
		// Soft rather than neon: saturated magenta, cyan and violet on near
		// black tired the eyes, so the same night in muted colours.
		bg: hex(0x13111f), tileA: hex(0x1a1729), tileB: hex(0x1e1b2f), border: hex(0x7c6bb0),
		text: hex(0xe4e0f0), label: hex(0x8e88a8), accent: hex(0x8fd3e0),
		head: hex(0x8fd3e0), tail: hex(0x8a74d0), eye: hex(0x13111f),
		lemon: hex(0xf5dc5a), lemonHi: hex(0xfff1b0), leaf: hex(0x7dd3a0), seed: hex(0xe8b4e0),
		boss: hex(0xd0607a), bossEye: hex(0xa8f0d0), bossTooth: hex(0xf4f0ff),
	},
	{
		name: "Deep Sea",
		bg:   hex(0x06121c), tileA: hex(0x0b1d2b), tileB: hex(0x0e2434), border: hex(0x2e86ab),
		text: hex(0xdff3ff), label: hex(0x6f93a8), accent: hex(0xffb347),
		head: hex(0xffb347), tail: hex(0xd1495b), eye: hex(0x06121c),
		lemon: hex(0xffe14a), lemonHi: hex(0xfff7c0), leaf: hex(0x4cd08a), seed: hex(0xbfe9ff),
		boss: hex(0x1b998b), bossEye: hex(0xff595e), bossTooth: hex(0xe0fff9),
	},
	{
		name: "Retro CRT",
		bg:   hex(0x020a02), tileA: hex(0x041204), tileB: hex(0x061806), border: hex(0x33ff33),
		text: hex(0x7dff7d), label: hex(0x2e9e2e), accent: hex(0xb6ff00),
		head: hex(0xb6ff00), tail: hex(0x1f7a1f), eye: hex(0x020a02),
		lemon: hex(0xe8ff5a), lemonHi: hex(0xfaffd0), leaf: hex(0x33ff33), seed: hex(0x9bff9b),
		boss: hex(0x18a018), bossEye: hex(0xe8ff5a), bossTooth: hex(0xcfffcf),
	},
	{
		name: "Paper",
		bg:   hex(0xf4efe1), tileA: hex(0xebe4d0), tileB: hex(0xe3dbc3), border: hex(0x8a7a50),
		text: hex(0x2b2618), label: hex(0x7a6f55), accent: hex(0xc0392b),
		head: hex(0x2e7d32), tail: hex(0x9ccc65), eye: hex(0xffffff),
		lemon: hex(0xf2c400), lemonHi: hex(0xfff0a0), leaf: hex(0x388e3c), seed: hex(0x5d4037),
		boss: hex(0x6a1b9a), bossEye: hex(0xffeb3b), bossTooth: hex(0xffffff),
	},
}

// ── cells ───────────────────────────────────────────────────────────────

type canvas struct {
	b    *buffer.Buffer
	w, h int
}

func (c canvas) set(x, y int, r rune, s cell.Style) {
	if x >= 0 && y >= 0 && x < c.w && y < c.h {
		c.b.Content[y*c.w+x] = cell.Cell{Content: r, Style: s}
	}
}

func (c canvas) text(x, y int, s string, st cell.Style) int {
	for _, r := range s {
		c.set(x, y, r, st)
		x++
	}
	return x
}

// number writes n in decimal without allocating.
func (c canvas) number(x, y, n int, st cell.Style) int {
	var d [20]byte
	i := len(d)
	for {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	for _, ch := range d[i:] {
		c.set(x, y, rune(ch), st)
		x++
	}
	return x
}

func (c canvas) fill(x0, y0, w, h int, st cell.Style) {
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			c.set(x, y, ' ', st)
		}
	}
}

func digits(n int) int {
	w := 1
	for n >= 10 {
		n /= 10
		w++
	}
	return w
}

func runes(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func (c canvas) centre(cx, y int, s string, st cell.Style) {
	c.text(cx-runes(s)/2, y, s, st)
}

func keyStyle(th *theme) cell.Style {
	st := style(th.accent, th.bg)
	st.Modifier |= cell.ModifierBold
	return st
}

// ── pixels ──────────────────────────────────────────────────────────────

type pix struct {
	p    *[maxCols * tilePx * maxRows * tilePx]rgb
	w, h int
}

func (q pix) blend(x, y int, c rgb, a float64) {
	if x >= 0 && y >= 0 && x < q.w && y < q.h && a > 0 {
		i := y*pxW + x
		q.p[i] = q.p[i].mix(c, min(1, a))
	}
}

// rect paints [x0,x1)×[y0,y1), each pixel by how much of it is covered.
func (q pix) rect(x0, y0, x1, y1 float64, c rgb, a float64) {
	for y := max(0, int(math.Floor(y0))); y < min(q.h, int(math.Ceil(y1))); y++ {
		cy := min(y1, float64(y+1)) - max(y0, float64(y))
		for x := max(0, int(math.Floor(x0))); x < min(q.w, int(math.Ceil(x1))); x++ {
			cx := min(x1, float64(x+1)) - max(x0, float64(x))
			q.blend(x, y, c, a*cx*cy)
		}
	}
}

// box paints a w×h rectangle centred on (x, y).
func (q pix) box(x, y, w, h float64, c rgb, a float64) {
	q.rect(x-w/2, y-h/2, x+w/2, y+h/2, c, a)
}

// span paints a band of width w from (ax, ay) to (bx, by), which share a
// row or a column.
func (q pix) span(ax, ay, bx, by, w float64, c rgb) {
	q.rect(min(ax, bx)-w/2, min(ay, by)-w/2, max(ax, bx)+w/2, max(ay, by)+w/2, c, 1)
}

func centreOf(p pos) (float64, float64) {
	return (float64(p.x) + 0.5) * tilePx, (float64(p.y) + 0.5) * tilePx
}

// The lemon, six pixels by four over a tile four wide: y rind, Y flesh,
// H shine, L leaf.
var lemonArt = [4]string{
	"...LL.",
	".yYHy.",
	"yYYYYy",
	".yyyy.",
}

// The monster, two tiles square: B body, W eye, M mouth, T tooth. The
// pupils are drawn on top, looking at the head.
var monsterArt = [8]string{
	"B.BBBB.B",
	"BBBBBBBB",
	"BWWBBWWB",
	"BWWBBWWB",
	"BBBBBBBB",
	"BMTMMTMB",
	"BMMMMMMB",
	"B.B..B.B",
}

func (g *game) render(b *buffer.Buffer) {
	W, H := int(b.Area.Width), int(b.Area.Height)
	if len(b.Content) < W*H {
		return
	}
	b.IsDirty = true
	g.lastW, g.lastH = W, H
	th := &themes[g.theme]
	cv := canvas{b: b, w: W, h: H}
	cv.fill(0, 0, W, H, style(th.text, th.bg))
	// Out of a run the board takes the window's size; a run, or its end,
	// keeps the board it was played on.
	if c, r, ok := fitBoard(W, H); ok && (g.phase == phTitle || g.phase == phName) {
		g.cols, g.rows = c, r
	}
	bw, bh := g.cols*tilePx+2, g.rows*tilePx/2+2
	if W < bw+gap+panelW || H < max(bh, panelH) {
		g.paused = g.paused || g.phase == phPlay
		cv.centre(W/2, H/2-1, "Lemon Snake needs a larger window:", style(th.text, th.bg))
		cv.centre(W/2, H/2, "70×18 or more", style(th.label, th.bg))
		return
	}
	left, top := (W-bw-gap-panelW)/2, (H-max(bh, panelH))/2
	g.drawBorder(cv, th, left, top, bw, bh)
	g.paint(th)
	g.pack(cv, left+1, top+1)
	if g.banner != bnNone && g.t-g.bannerT < 2.5 && g.phase == phPlay {
		g.drawBanner(cv, th, left+bw/2, top+2)
	}
	g.drawPanel(cv, th, left+bw+gap, top)
	g.drawDialogs(cv, th, left+bw/2, top+bh/2)
}

func (g *game) drawBorder(cv canvas, th *theme, x, y, w, h int) {
	st := style(th.border, th.bg)
	for i := 1; i < w-1; i++ {
		cv.set(x+i, y, '─', st)
		cv.set(x+i, y+h-1, '─', st)
	}
	for j := 1; j < h-1; j++ {
		cv.set(x, y+j, '│', st)
		cv.set(x+w-1, y+j, '│', st)
	}
	cv.set(x, y, '╭', st)
	cv.set(x+w-1, y, '╮', st)
	cv.set(x, y+h-1, '╰', st)
	cv.set(x+w-1, y+h-1, '╯', st)
}

// pack turns pixel pairs into half-block cells.
func (g *game) pack(cv canvas, x0, y0 int) {
	for y := range g.rows * tilePx / 2 {
		for x := range g.cols * tilePx {
			top, bot := g.px[2*y*pxW+x].color(), g.px[(2*y+1)*pxW+x].color()
			if top == bot {
				cv.set(x0+x, y0+y, ' ', cell.Style{Fg: top, Bg: bot})
			} else {
				cv.set(x0+x, y0+y, '▀', cell.Style{Fg: top, Bg: bot})
			}
		}
	}
}

// paint draws the board into the pixel buffer.
func (g *game) paint(th *theme) {
	q := pix{p: g.px, w: g.cols * tilePx, h: g.rows * tilePx}
	for y := range g.rows {
		for x := range g.cols {
			c := th.tileA
			if (x+y)%2 == 1 {
				c = th.tileB
			}
			q.rect(float64(x*tilePx), float64(y*tilePx), float64((x+1)*tilePx), float64((y+1)*tilePx), c, 1)
		}
	}
	if g.phase == phTitle {
		return
	}
	// f is how far the current step has gone.
	f := 0.0
	if g.phase == phPlay {
		f = min(1, g.acc/g.interval())
	}
	g.paintLemon(q, th)
	g.paintSeeds(q, th, f)
	g.paintSnake(q, th, f)
	if g.boss.on {
		g.paintMonster(q, th, f)
	}
	for _, p := range g.parts {
		if p.life > 0 {
			q.box(p.x, p.y, 1, 1, p.c, 2*p.life/p.total)
		}
	}
}

func (g *game) paintLemon(q pix, th *theme) {
	if !g.inside(g.lemon) {
		return
	}
	cx, cy := centreOf(g.lemon)
	// A round glow that breathes, so the lemon is the first thing seen.
	glow := 0.4 + 0.15*math.Sin(g.t*5)
	const r = 5.5
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			if d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) / r; d < 1 {
				q.blend(x, y, th.lemon, glow*(1-d)*(1-d))
			}
		}
	}
	x0, y0 := int(cx)-3, int(cy)-2
	rind := th.lemon.mix(black, 0.22)
	for j, row := range lemonArt {
		for i := 0; i < len(row); i++ {
			var c rgb
			switch row[i] {
			case 'y':
				c = rind
			case 'Y':
				c = th.lemon
			case 'H':
				c = th.lemonHi
			case 'L':
				c = th.leaf
			default:
				continue
			}
			q.blend(x0+i, y0+j, c, 1)
		}
	}
}

func (g *game) paintSeeds(q pix, th *theme, f float64) {
	for _, s := range g.seeds {
		if !s.live {
			continue
		}
		cx, cy := centreOf(s.p)
		d := steps[s.d]
		cx += float64(d.x) * f * 2 * tilePx
		cy += float64(d.y) * f * 2 * tilePx
		q.box(cx-float64(d.x)*2, cy-float64(d.y)*2, 2, 2, th.seed, 0.35)
		q.box(cx, cy, 2, 2, th.seed, 1)
	}
}

func (g *game) paintSnake(q pix, th *theme, f float64) {
	L := g.length
	if L == 0 {
		return
	}
	dead := g.phase == phOver && !g.won
	flash := dead && g.t-g.diedT < 1 && int((g.t-g.diedT)*8)%2 == 0
	col := func(i int) rgb {
		if flash {
			return th.text
		}
		return th.head.mix(th.tail, float64(i)/float64(max(1, L-1)))
	}
	// The head slides into the tile it is about to enter, and the tail
	// towards the segment before it, unless the snake grows.
	hx, hy := g.headPx(f)
	d := steps[g.next()]
	if g.phase == phOver {
		d = steps[g.dir]
	}
	tx, ty := centreOf(g.at(L - 1))
	if L > 1 && g.grow == 0 {
		nx, ny := centreOf(g.at(L - 2))
		tx, ty = tx+(nx-tx)*f, ty+(ny-ty)*f
	}

	// The body is a tube: a darker rim the width of a tile, then its own
	// colour down the middle, each pass over the whole length so the rim
	// does not show between segments.
	for _, pass := range [2]struct{ w, dark float64 }{{tilePx, 0.35}, {tilePx / 2, 0}} {
		for i := L - 1; i >= 1; i-- {
			ax, ay := centreOf(g.at(i))
			if i == L-1 {
				ax, ay = tx, ty
			}
			bx, by := centreOf(g.at(i - 1))
			q.span(ax, ay, bx, by, pass.w, col(i).mix(black, pass.dark))
		}
		cx, cy := centreOf(g.at(0))
		q.span(cx, cy, hx, hy, pass.w, col(0).mix(black, pass.dark))
	}
	q.box(hx, hy, tilePx, tilePx, col(0).mix(black, 0.35), 1)
	q.box(hx, hy, tilePx, tilePx-2, col(0), 1)
	q.box(hx, hy, tilePx-2, tilePx, col(0), 1)

	// Eyes, ahead of the centre and either side of the way it faces.
	ex, ey := float64(d.x)*0.6, float64(d.y)*0.6
	sx, sy := float64(-d.y)*1.1, float64(d.x)*1.1
	eye := th.eye
	if dead {
		eye = red
	}
	q.box(hx+ex+sx, hy+ey+sy, 1, 1, eye, 1)
	q.box(hx+ex-sx, hy+ey-sy, 1, 1, eye, 1)
}

// headPx is where the head is drawn, in pixels, f of the way through a
// step: from the centre of its tile towards the next, short of a wall.
func (g *game) headPx(f float64) (float64, float64) {
	hx, hy := centreOf(g.at(0))
	d := steps[g.next()]
	if g.phase != phPlay || g.deadly(g.at(0).add(d)) {
		return hx, hy
	}
	return hx + float64(d.x)*f*tilePx, hy + float64(d.y)*f*tilePx
}

func (g *game) paintMonster(q pix, th *theme, f float64) {
	b := &g.boss
	// It glides over the step after it moves.
	x, y := float64(b.p.x*tilePx), float64(b.p.y*tilePx)
	if b.movedAt == g.ticks {
		fx, fy := float64(b.from.x*tilePx), float64(b.from.y*tilePx)
		// Whole pixels: a sprite blended between two places looks smeared.
		x, y = math.Round(fx+(x-fx)*f), math.Round(fy+(y-fy)*f)
	}
	body := th.boss
	if g.enraged() {
		body = body.mix(red, 0.3+0.2*math.Sin(g.t*14))
	}
	eye := th.bossEye
	if g.t-b.hitT < 0.12 {
		body, eye = th.bossTooth, th.bossTooth
	}
	mouth := body.mix(black, 0.7)
	for j, row := range monsterArt {
		for i := 0; i < len(row); i++ {
			var c rgb
			switch row[i] {
			case 'B':
				c = body
			case 'W':
				c = eye
			case 'M':
				c = mouth
			case 'T':
				c = th.bossTooth
			default:
				continue
			}
			q.rect(x+float64(i), y+float64(j), x+float64(i+1), y+float64(j+1), c, 1)
		}
	}
	// Pupils, in the corner of each eye nearest the head.
	h := g.at(0)
	px, py := 0.0, 0.0
	if h.x > b.p.x {
		px = 1
	}
	if h.y > b.p.y {
		py = 1
	}
	for _, ex := range [2]float64{1, 5} {
		q.rect(x+ex+px, y+2+py, x+ex+px+1, y+3+py, black, 1)
	}
}

// ── text ────────────────────────────────────────────────────────────────

func (g *game) drawBanner(cv canvas, th *theme, cx, y int) {
	st := style(th.accent, th.bg)
	switch g.banner {
	case bnLevel, bnWin:
		s := " LEVEL   "
		if g.banner == bnWin {
			s = " MONSTER DOWN! LEVEL   "
		}
		x := cx - runes(s)/2
		cv.text(x, y, s, st)
		cv.number(x+runes(s)-2, y, g.level, st)
	case bnBoss:
		cv.centre(cx, y, " A MONSTER APPEARS! ", st)
		cv.centre(cx, y+1, " space spits seeds ", style(th.text, th.bg))
	}
}

func (g *game) drawPanel(cv canvas, th *theme, x, y int) {
	lbl, txt, acc := style(th.label, th.bg), style(th.text, th.bg), style(th.accent, th.bg)
	cv.text(x, y, "LEMON SNAKE", acc)
	cv.text(x, y+2, "Score", lbl)
	cv.number(x+8, y+2, g.score, txt)
	cv.text(x, y+3, "Best", lbl)
	cv.number(x+8, y+3, max(g.best, g.score), txt)
	cv.text(x, y+4, "Level", lbl)
	cv.number(x+8, y+4, max(1, g.level), txt)
	cv.text(x, y+5, "Length", lbl)
	cv.number(x+8, y+5, g.length, txt)

	if g.boss.on {
		cv.text(x, y+7, "MONSTER", style(th.boss.mix(th.text, 0.3), th.bg))
		nx := x + panelW - digits(g.boss.hp) - 1 - digits(g.boss.maxHP)
		nx = cv.number(nx, y+7, g.boss.hp, txt)
		nx = cv.text(nx, y+7, "/", lbl)
		cv.number(nx, y+7, g.boss.maxHP, lbl)
		full := g.boss.hp * panelW / max(1, g.boss.maxHP)
		for i := range panelW {
			c := th.label.mix(th.bg, 0.5)
			if i < full {
				c = th.boss
			}
			cv.set(x+i, y+8, '▄', style(c, th.bg))
		}
	} else {
		cv.text(x, y+7, "Next monster", lbl)
		for i := range lemonsPerLevel {
			c := th.label.mix(th.bg, 0.5)
			if i < g.eaten {
				c = th.lemon
			}
			cv.text(x+3*i, y+8, "◖◗", style(c, th.bg))
		}
	}

	cv.text(x, y+9, "Seeds", lbl)
	seeds := txt
	if g.ammo == 0 && g.boss.on {
		seeds = style(red, th.bg)
	}
	cv.number(x+8, y+9, g.ammo, seeds)

	cv.text(x, y+10, "Theme", lbl)
	cv.text(x, y+11, th.name, acc)

	sound := "sound"
	if g.muted || g.audio == nil {
		sound = "sound off"
	}
	keys := [...][2]string{
		{"←↑→↓", "move"},
		{"space", "spit seed"},
		{"t", "theme"},
		{"m", sound},
		{"p", "pause"},
		{"q", "quit"},
	}
	for i, k := range keys {
		cv.text(x, y+12+i, k[0], keyStyle(th))
		cv.text(x+7, y+12+i, k[1], lbl)
	}
}

// ── dialogs ─────────────────────────────────────────────────────────────

type dialogKind uint8

const (
	dlgNone dialogKind = iota
	dlgTitle
	dlgOver
	dlgPause
	dlgName
)

const (
	dialogW    = 40
	dialogOpen = 0.32 // seconds a dialog takes to open
	dialogLap  = 2.4  // seconds the light takes round its border
)

// box is the inside of an open dialog, where its lines go.
type box struct {
	cv   canvas
	th   *theme
	x, y int // the first line's left edge
	w    int
	bg   rgb
}

func (b box) st(fg rgb) cell.Style { return style(fg, b.bg) }

func (b box) centre(dy int, s string, fg rgb) {
	b.cv.centre(b.x+b.w/2, b.y+dy, s, b.st(fg))
}

func (b box) hints(dy int, h ...string) {
	x := b.x + b.w/2
	w := 0
	for i, s := range h {
		w += runes(s) + 2
		if i%2 == 1 && i < len(h)-1 {
			w += 2
		}
	}
	x -= (w - 2) / 2
	key := b.st(b.th.accent)
	key.Modifier |= cell.ModifierBold
	for i, s := range h {
		st := b.st(b.th.label)
		if i%2 == 0 {
			st = key
		}
		x = b.cv.text(x, b.y+dy, s, st) + 2
		if i%2 == 1 {
			x += 2
		}
	}
}

// openDialog draws a dialog of w×h cells centred on (cx, cy), age seconds
// after it opened: it grows from its centre with a little overshoot, casts
// a shadow, and two lights run round its rounded border. It reports
// whether it is open far enough for its lines.
func (g *game) openDialog(cv canvas, th *theme, cx, cy, w, h int, title string) (box, bool) {
	age := g.t - g.dialogT
	e := min(1, age/dialogOpen)
	// Ease out, overshooting a little before it settles.
	const c1 = 1.4
	e = 1 + (c1+1)*math.Pow(e-1, 3) + c1*math.Pow(e-1, 2)
	ww, hh := max(4, int(math.Round(float64(w)*e))), max(3, int(math.Round(float64(h)*e)))
	x0 := max(0, min(cv.w-ww-1, cx-ww/2))
	y0 := max(0, min(cv.h-hh-1, cy-hh/2))

	shadow := th.bg.mix(black, 0.55)
	for x := x0 + 1; x <= x0+ww; x++ {
		cv.set(x, y0+hh, ' ', style(shadow, shadow))
	}
	for y := y0 + 1; y < y0+hh; y++ {
		cv.set(x0+ww, y, ' ', style(shadow, shadow))
	}
	bg := th.bg.mix(th.border, 0.07)
	cv.fill(x0+1, y0+1, ww-2, hh-2, style(th.text, bg))

	// The border, clockwise from the top-left corner, lit by two lights
	// half a lap apart, each with a fading tail.
	per := 2*(ww-1) + 2*(hh-1)
	head := math.Mod(g.t/dialogLap, 1) * float64(per)
	for k := range per {
		var x, y int
		var r rune
		switch {
		case k < ww-1:
			x, y, r = x0+k, y0, '─'
		case k < ww-1+hh-1:
			x, y, r = x0+ww-1, y0+k-(ww-1), '│'
		case k < 2*(ww-1)+hh-1:
			x, y, r = x0+ww-1-(k-(ww-1+hh-1)), y0+hh-1, '─'
		default:
			x, y, r = x0, y0+hh-1-(k-(2*(ww-1)+hh-1)), '│'
		}
		switch {
		case x == x0 && y == y0:
			r = '╭'
		case x == x0+ww-1 && y == y0:
			r = '╮'
		case x == x0+ww-1 && y == y0+hh-1:
			r = '╯'
		case x == x0 && y == y0+hh-1:
			r = '╰'
		}
		lit := 0.0
		for _, h := range [2]float64{head, head + float64(per)/2} {
			d := math.Mod(h-float64(k)+2*float64(per), float64(per))
			lit = max(lit, 1-d/(float64(per)*0.2))
		}
		c := th.border.mix(th.accent, lit)
		if lit > 0.92 {
			c = c.mix(th.text, 0.6)
		}
		st := style(c, bg)
		st.Modifier |= cell.ModifierBold
		cv.set(x, y, r, st)
	}
	if age < dialogOpen {
		return box{}, false
	}
	// The title, set into the top border.
	tw := runes(title) + 4
	tx := x0 + (ww-tw)/2
	cv.set(tx, y0, '┤', style(th.border, bg))
	ts := style(th.accent, bg)
	ts.Modifier |= cell.ModifierBold
	cv.text(tx+2, y0, title, ts)
	cv.set(tx+1, y0, ' ', ts)
	cv.set(tx+tw-2, y0, ' ', ts)
	cv.set(tx+tw-1, y0, '├', style(th.border, bg))
	return box{cv: cv, th: th, x: x0 + 2, y: y0 + 1, w: ww - 4, bg: bg}, true
}

func (g *game) drawDialogs(cv canvas, th *theme, cx, cy int) {
	kind := dlgNone
	switch {
	case g.phase == phName:
		kind = dlgName
	case g.phase == phTitle:
		kind = dlgTitle
	case g.phase == phOver:
		// The board shows the crash a moment before the dialog opens.
		if g.t-g.diedT > 0.6 {
			kind = dlgOver
		}
	case g.paused:
		kind = dlgPause
	}
	if kind != g.dialog {
		g.dialog, g.dialogT = kind, g.t
	}
	// The leaderboard's rows: as many as it has, at least three, and what
	// the window holds.
	entries, _, _ := g.shownBoard()
	rows := max(3, min(boardSize, cv.h-10, len(entries)))
	switch kind {
	case dlgTitle:
		b, ok := g.openDialog(cv, th, cx, cy, dialogW, rows+10, "LEMON SNAKE")
		if !ok {
			return
		}
		b.centre(0, "Eat lemons. Every five, a monster", th.text)
		b.centre(1, "comes: spit seeds at it.", th.text)
		g.drawLeaders(b, 3, rows, false)
		b.hints(rows+5, "enter", "play", "n", "name")
		b.hints(rows+6, "t", "theme", "m", "sound", "q", "quit")
		x := b.cv.text(b.x+(b.w-runes("playing as ")-runes(g.name))/2, b.y+rows+7, "playing as ", b.st(th.label))
		b.cv.text(x, b.y+rows+7, g.name, b.st(th.text))
	case dlgOver:
		title := "GAME OVER"
		if g.won {
			title = "YOU WIN!"
		}
		b, ok := g.openDialog(cv, th, cx, cy, dialogW, rows+8, title)
		if !ok {
			return
		}
		x := b.x + b.w/2 - 11
		b.cv.text(x, b.y, "Score", b.st(th.label))
		big := b.st(th.accent)
		big.Modifier |= cell.ModifierBold
		b.cv.number(x+6, b.y, g.score, big)
		b.cv.text(x+15, b.y, "Level", b.st(th.label))
		b.cv.number(x+21, b.y, g.level, big)
		g.drawRank(b, 1)
		g.drawLeaders(b, 3, rows, true)
		b.hints(rows+5, "enter", "again", "esc", "menu", "q", "quit")
	case dlgPause:
		b, ok := g.openDialog(cv, th, cx, cy, 30, 7, "PAUSED")
		if !ok {
			return
		}
		b.hints(1, "p", "go on", "r", "restart")
		b.hints(2, "m", "sound", "q", "quit")
		state := g.soundState()
		x := b.cv.text(b.x+(b.w-runes("sound: ")-runes(state))/2, b.y+3, "sound: ", b.st(th.label))
		b.cv.text(x, b.y+3, state, b.st(th.text))
	case dlgName:
		b, ok := g.openDialog(cv, th, cx, cy, 34, 8, "YOUR NAME")
		if !ok {
			return
		}
		b.centre(0, "for the leaderboard", th.label)
		fw := nameMax + 2
		fx := b.x + (b.w-fw)/2
		field := th.bg.mix(black, 0.3)
		b.cv.fill(fx, b.y+2, fw, 1, style(th.text, field))
		x := b.cv.text(fx+1, b.y+2, g.typing, style(th.text, field))
		if int(g.t*2.5)%2 == 0 {
			b.cv.set(x, b.y+2, '▏', style(th.accent, field))
		}
		if g.name != "" {
			b.hints(4, "enter", "keep", "esc", "back")
		} else {
			b.hints(4, "enter", "keep")
		}
	}
}

// soundState says whether sound is on, off, or not there, and why.
func (g *game) soundState() string {
	switch {
	case g.audio == nil:
		return g.noSound
	case g.muted:
		return "off"
	}
	return "on"
}

// drawRank says where the run that ended landed.
func (g *game) drawRank(b box, dy int) {
	th := b.th
	switch {
	case g.score == 0:
		b.centre(dy, "no lemons this time", th.label)
	case g.worldRank < 0:
		b.centre(dy, "sending to the world board…", th.label)
	case g.worldState == worldOK && g.worldRank > 0:
		x := b.x + b.w/2 - 8
		st := b.st(th.accent)
		st.Modifier |= cell.ModifierBold
		x = b.cv.text(x, b.y+dy, "#", st)
		x = b.cv.number(x, b.y+dy, g.worldRank, st)
		b.cv.text(x, b.y+dy, " in the world!", st)
	case g.localRank == 1:
		b.centre(dy, "a new best!", th.accent)
	case g.worldState == worldOffline:
		b.centre(dy, "offline: kept on this machine", th.label)
	default:
		b.centre(dy, "not in the top ten, this time", th.label)
	}
}

// drawLeaders draws the board the game shows, n rows of it from line dy,
// with the run that just ended picked out when mark is set.
func (g *game) drawLeaders(b box, dy, n int, mark bool) {
	th := b.th
	entries, at, world := g.shownBoard()
	head := "YOUR TOP TEN"
	if world {
		head = "WORLD TOP TEN"
	}
	x := b.x + (b.w-32)/2
	hs := b.st(th.label)
	hs.Modifier |= cell.ModifierBold
	b.cv.text(x, b.y+dy, head, hs)
	switch g.worldState {
	case worldLoading:
		b.cv.text(x+32-runes("connecting…"), b.y+dy, "connecting…", b.st(th.label))
	case worldOffline:
		b.cv.text(x+32-runes("offline"), b.y+dy, "offline", b.st(th.label))
	}
	if len(entries) == 0 {
		b.centre(dy+2, "no runs yet: be the first!", th.text)
		return
	}
	for i := 0; i < n && i < len(entries); i++ {
		e := &entries[i]
		y := b.y + dy + 1 + i
		fg := th.text
		if i < 3 {
			fg = th.text.mix(th.accent, 0.35)
		}
		if mark && i+1 == at {
			fg = th.accent
			b.cv.set(x-2, y, '▸', b.st(th.accent))
		}
		st := b.st(fg)
		if i+1 < 10 {
			b.cv.number(x+1, y, i+1, b.st(th.label))
		} else {
			b.cv.number(x, y, i+1, b.st(th.label))
		}
		b.cv.text(x+3, y, e.Name, st)
		b.cv.number(x+26-digits(e.Score), y, e.Score, st)
		b.cv.text(x+28, y, "L", b.st(th.label))
		b.cv.number(x+29, y, e.Level, b.st(th.label))
	}
}
