package main

import "math"

// The playing field is up to maxCols×maxRows tiles, sized to the window
// when a run starts. The snake lives in a ring buffer the size of the
// largest board, and occ counts its segments on each tile, so a step is a
// few array writes and allocates nothing.
const (
	maxCols        = 30
	maxRows        = 18
	minCols        = 12
	minRows        = 8
	lemonsPerLevel = 5
	maxSeeds       = 12 // in flight at once: enough that a held or hammered space is never refused for want of one
	maxParticles   = 64
	startLen       = 4
	seedsPerLemon  = 2
	maxAmmo        = 20 // seeds the snake can carry
)

type pos struct{ x, y int }

func (p pos) add(q pos) pos { return pos{p.x + q.x, p.y + q.y} }

func idx(p pos) int { return p.y*maxCols + p.x }

type dir uint8

const (
	dUp dir = iota
	dRight
	dDown
	dLeft
)

var steps = [4]pos{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

type phase uint8

const (
	phTitle phase = iota
	phPlay
	phOver
	phName // the player's name being typed
)

type bannerKind uint8

const (
	bnNone bannerKind = iota
	bnLevel
	bnBoss
	bnWin
)

type seed struct {
	p    pos
	d    dir
	live bool
}

// monster is two tiles by two; p is its top-left tile.
type monster struct {
	on        bool
	p, from   pos  // where it is, and where it stepped from, for the glide
	movedAt   uint // the tick it stepped on
	hp, maxHP int
	wait      int     // ticks until its next step
	hitT      float64 // when a seed last struck it, for the flash
}

// particle is a spark, in pixels.
type particle struct {
	x, y, vx, vy float64
	life, total  float64
	c            rgb
}

// session is what outlives a run: the player, the boards, the sound and
// the screen. start keeps it and makes the rest anew.
type session struct {
	rng          uint64
	t            float64 // the clock
	theme        int
	best         int
	lastW, lastH int // the window, as last drawn
	quit         bool

	name   string
	typing string // the name being typed
	back   phase  // where the name entry returns to
	store  store
	remote *remote

	local             [boardSize]scoreEntry // the game's own board
	nLocal, localRank int
	world             [boardSize]scoreEntry // the shared board, as last fetched
	nWorld, worldRank int                   // worldRank: -1 while the run is being sent
	worldState        int

	audio   speaker // nil: no sound
	muted   bool
	noSound string // why there is no sound, when there is none

	dialog  dialogKind // the dialog on screen, and since when, for its opening
	dialogT float64

	px *[maxCols * tilePx * maxRows * tilePx]rgb // the board's pixels
}

type game struct {
	session
	phase  phase
	paused bool

	cols, rows int // this run's board

	body   [maxCols * maxRows]pos // ring buffer; body[head] is the head
	head   int
	length int
	grow   int
	occ    [maxCols * maxRows]uint8
	dir    dir
	queue  [2]dir // turns pressed faster than the snake steps
	nq     int

	lemon  pos
	eaten  int // lemons this level, towards the monster
	level  int
	score  int
	lemons int                 // in the whole run
	ammo   int                 // seeds to spit
	levels [maxLevels][2]int32 // each level's lemons and hits, for the leaderboard
	runT   float64             // time played, pauses left out

	boss  monster
	seeds [maxSeeds]seed
	parts [maxParticles]particle
	np    int // the next particle slot to reuse

	acc     float64 // time since the last step
	graced  bool    // held at a wall or the body for a late turn
	won     bool    // the run ended with the board full of snake
	ticks   uint
	banner  bannerKind
	bannerT float64
	diedT   float64
}

func newGame(seed uint64) *game {
	return &game{session: session{rng: seed | 1, px: new([maxCols * tilePx * maxRows * tilePx]rgb)}, cols: maxCols, rows: maxRows}
}

// rand returns a number in [0, n), from xorshift64*.
func (g *game) rand(n int) int {
	g.rng ^= g.rng >> 12
	g.rng ^= g.rng << 25
	g.rng ^= g.rng >> 27
	return int((g.rng * 2685821657736338717 >> 33) % uint64(n))
}

func (g *game) randf() float64 { return float64(g.rand(1<<20)) / (1 << 20) }

func (g *game) inside(p pos) bool { return p.x >= 0 && p.y >= 0 && p.x < g.cols && p.y < g.rows }

// fitBoard is the largest board a window holds, and whether it is at least
// the smallest playable one.
func fitBoard(w, h int) (cols, rows int, ok bool) {
	cols = min(maxCols, (w-panelW-gap-2-2*margin)/tilePx)
	rows = min(maxRows, (h-2)*2/tilePx)
	return cols, rows, cols >= minCols && rows >= minRows && h >= panelH
}

func (g *game) start() {
	cols, rows, ok := maxCols, maxRows, true
	if g.lastW > 0 {
		cols, rows, ok = fitBoard(g.lastW, g.lastH)
		if !ok {
			cols, rows = max(cols, minCols), max(rows, minRows)
		}
	}
	*g = game{session: g.session, cols: cols, rows: rows, phase: phPlay, level: 1, dir: dRight}
	g.sound(sfxStart, 0.8)
	for i := startLen - 1; i >= 0; i-- {
		g.push(pos{g.cols/2 - i, g.rows / 2})
	}
	g.placeLemon()
	g.show(bnLevel)
}

// at is the i'th segment, the head being 0.
func (g *game) at(i int) pos { return g.body[(g.head-i+len(g.body))%len(g.body)] }

func (g *game) push(p pos) {
	g.head = (g.head + 1) % len(g.body)
	g.body[g.head] = p
	g.length++
	g.occ[idx(p)]++
}

func (g *game) popTail() {
	g.occ[idx(g.at(g.length-1))]--
	g.length--
}

// cut leaves the snake its first n segments.
func (g *game) cut(n int) {
	for g.length > n {
		g.popTail()
	}
	g.grow = 0
}

// interval is the time between steps: a little shorter with every lemon,
// and more every level, down to about 13 tiles a second. The numbers were
// set with a simulated player of ordinary reactions (a look every 0.28 s,
// a wall now and then turned from too late), who should beat the first
// monster most of the time and play for a few minutes.
func (g *game) interval() float64 {
	return max(0.075, 0.16-0.008*float64(g.level-1)-0.0015*float64(g.eaten))
}

// monsterHP is how many seeds the monster of a level takes. The scoreboard
// checks a finished level against it: apps/scoreboard/board/snake.go.
func monsterHP(level int) int { return 3 + level }

// next is the way the snake will go on its next step.
func (g *game) next() dir {
	if g.nq > 0 {
		return g.queue[0]
	}
	return g.dir
}

func (g *game) update(dt float64) {
	g.t += dt
	g.pollRemote()
	if g.paused {
		return
	}
	for i := range g.parts {
		p := &g.parts[i]
		if p.life > 0 {
			p.life -= dt
			p.x += p.vx * dt
			p.y += p.vy * dt
			p.vy += 24 * dt
		}
	}
	if g.phase != phPlay {
		return
	}
	g.runT += dt
	iv := g.interval()
	// After a stall (a suspended process, a slow link) step once, not a burst.
	g.acc = min(g.acc+dt, 2*iv)
	for g.acc >= iv && g.phase == phPlay {
		g.acc -= iv
		g.tick()
		iv = g.interval()
	}
}

func (g *game) turn(d dir) {
	last := g.dir
	if g.nq > 0 {
		last = g.queue[g.nq-1]
	}
	if d == last || d == (last+2)%4 || g.nq == len(g.queue) {
		return
	}
	// A turn takes effect at once. The head is drawn sliding into the next
	// tile; past a third of the way there, that step is finished now and the
	// turn made from the new tile, and before it the head turns from the
	// tile it is on, a pixel or so to the side. Waiting for the step instead
	// felt late, or turned the head from a tile behind where it was seen.
	switch {
	case g.nq == 0 && g.phase == phPlay && !g.paused && g.graced:
		// Held at the edge: the turn is the step, at once.
		g.queue[0], g.nq = d, 1
		g.acc = 0
		g.sound(sfxTurn, 0.5)
		g.tick()
		return
	case g.nq == 0 && g.phase == phPlay && !g.paused && g.acc >= g.interval()/3:
		g.acc = 0
		g.tick()
		if g.phase != phPlay {
			return
		}
	}
	g.queue[g.nq] = d
	g.nq++
	g.sound(sfxTurn, 0.5)
}

// deadly is whether the head stepping onto p ends the run. The tail's tile
// is not, when the tail moves off it in the same step.
func (g *game) deadly(p pos) bool {
	if !g.inside(p) || g.bossCovers(p) {
		return true
	}
	return g.occ[idx(p)] > 0 && !(p == g.at(g.length-1) && g.grow == 0)
}

func (g *game) tick() {
	// Grace: a step into a wall or the body waits half a step first, for
	// the turn a player presses a moment late, which is how most runs of
	// ordinary players ended.
	if g.deadly(g.at(0).add(steps[g.next()])) && !g.graced {
		g.graced = true
		g.acc = g.interval() / 2
		return
	}
	g.graced = false
	g.ticks++
	g.dir = g.next()
	if g.nq > 0 {
		g.queue[0] = g.queue[1]
		g.nq--
	}
	n := g.at(0).add(steps[g.dir])
	if !g.inside(n) {
		g.die()
		return
	}
	// The tail leaves its tile this step unless the snake is growing, so
	// following it closely is allowed.
	if g.grow > 0 {
		g.grow--
	} else {
		g.popTail()
	}
	if g.occ[idx(n)] > 0 || g.bossCovers(n) {
		g.push(n) // drawn where it crashed
		g.die()
		return
	}
	g.push(n)
	if n == g.lemon {
		g.eat()
	}
	for i := range g.seeds {
		for k := 0; k < 2 && g.seeds[i].live; k++ {
			g.stepSeed(&g.seeds[i])
		}
	}
	if g.boss.on {
		g.moveBoss()
	}
}

func (g *game) eat() {
	g.burst(g.lemon, 0.5, 0.5, 10, themes[g.theme].lemonHi)
	g.grow++
	g.ammo = min(maxAmmo, g.ammo+seedsPerLemon)
	g.score += 10 * g.level
	g.logLemon()
	note := 2 // in a fight, the middle of the scale
	if !g.boss.on {
		g.eaten++
		note = min(g.eaten, lemonsPerLevel) - 1
	}
	g.sound(sfxEat+sfx(note), 0.8)
	// Every tile is snake, or will be once it has grown: the classic win,
	// before a monster could come for the last lemon and bite the body.
	if g.length+g.grow >= g.cols*g.rows {
		g.win()
		return
	}
	if !g.boss.on && g.eaten >= lemonsPerLevel {
		g.summon()
	}
	g.placeLemon()
}

// placeLemon puts the lemon on a free tile, or off the board if none is.
func (g *game) placeLemon() {
	free := func(p pos) bool { return g.occ[idx(p)] == 0 && !g.bossCovers(p) }
	for range 64 {
		if p := (pos{g.rand(g.cols), g.rand(g.rows)}); free(p) {
			g.lemon = p
			return
		}
	}
	for y := range g.rows {
		for x := range g.cols {
			if p := (pos{x, y}); free(p) {
				g.lemon = p
				return
			}
		}
	}
	g.lemon = pos{-1, -1}
}

func (g *game) die() {
	g.phase = phOver
	g.diedT = g.t
	g.burst(g.at(0), 0.5, 0.5, 20, themes[g.theme].text)
	g.sound(sfxOver, 0.9)
	g.finishRun()
}

// win ends the run with the board full.
func (g *game) win() {
	g.won = true
	g.phase = phOver
	g.diedT = g.t
	for i := 0; i < g.length; i += 5 {
		g.burst(g.at(i), 0.5, 0.5, 2, themes[g.theme].lemonHi)
	}
	g.sound(sfxWin, 1)
	g.finishRun()
}

// sound plays s, to the side of the board the head is on.
func (g *game) sound(s sfx, vol float32) {
	if g.audio == nil || g.muted {
		return
	}
	pan := float32(0)
	if g.length > 0 {
		pan = (float32(g.at(0).x)+0.5)/float32(g.cols)*1.2 - 0.6
	}
	g.audio.play(s, vol, pan)
}

// setPaused pauses a run or lets it go on.
func (g *game) setPaused(on bool) {
	if g.phase != phPlay || g.paused == on {
		return
	}
	g.paused = on
	g.sound(sfxPause, 0.7)
}

func (g *game) show(b bannerKind) { g.banner, g.bannerT = b, g.t }

// burst throws n sparks of colour c from a point of tile p, (fx, fy) of
// the way across it.
func (g *game) burst(p pos, fx, fy float64, n int, c rgb) {
	x, y := (float64(p.x)+fx)*tilePx, (float64(p.y)+fy)*tilePx
	for range n {
		a, v := g.randf()*2*math.Pi, 8+g.randf()*14
		life := 0.2 + g.randf()*0.25
		g.parts[g.np] = particle{x: x, y: y, vx: v * math.Cos(a), vy: v*math.Sin(a) - 6, life: life, total: life, c: c}
		g.np = (g.np + 1) % maxParticles
	}
}

// ── the monster ─────────────────────────────────────────────────────────

func (g *game) bossCovers(p pos) bool {
	b := &g.boss
	return b.on && p.x >= b.p.x && p.x <= b.p.x+1 && p.y >= b.p.y && p.y <= b.p.y+1
}

// summon brings the monster in at the corner farthest from the head.
func (g *game) summon() {
	h := g.at(0)
	corners := [4]pos{{0, 0}, {g.cols - 2, 0}, {0, g.rows - 2}, {g.cols - 2, g.rows - 2}}
	far, best := corners[0], -1
	for _, c := range corners {
		if d := abs(c.x-h.x) + abs(c.y-h.y); d > best {
			far, best = c, d
		}
	}
	hp := monsterHP(g.level)
	g.boss = monster{on: true, p: far, from: far, hp: hp, maxHP: hp, hitT: -1}
	g.boss.wait = g.bossPace()
	g.show(bnBoss)
	g.sound(sfxMonster, 0.9)
	g.burst(far, 1, 1, 24, themes[g.theme].boss)
	g.landed()
}

// enraged is a monster down to a third of its health.
func (g *game) enraged() bool { return g.boss.hp*3 <= g.boss.maxHP }

// bossPace is the ticks between the monster's steps: fewer each level, and
// fewer again once it is enraged.
func (g *game) bossPace() int {
	p := max(3, 8-g.level)
	if g.enraged() {
		p--
	}
	return p
}

func (g *game) moveBoss() {
	if g.boss.wait--; g.boss.wait > 0 {
		return
	}
	g.boss.wait = g.bossPace()
	h, b := g.at(0), g.boss.p
	// Distances to the head from the monster's centre, doubled to stay whole.
	ex, ey := 2*h.x-2*b.x-1, 2*h.y-2*b.y-1
	var d pos
	switch {
	case g.rand(4) == 0:
		d = steps[g.rand(4)]
	case abs(ex) >= abs(ey):
		d.x = sign(ex)
	default:
		d.y = sign(ey)
	}
	n := b.add(d)
	n.x = max(0, min(g.cols-2, n.x))
	n.y = max(0, min(g.rows-2, n.y))
	g.boss.from, g.boss.p, g.boss.movedAt = b, n, g.ticks
	g.landed()
}

// landed settles what the monster now stands on: the head ends the run,
// the body is bitten off there, the lemon moves and seeds strike.
func (g *game) landed() {
	for i := 0; i < g.length; i++ {
		if g.bossCovers(g.at(i)) {
			if i == 0 {
				g.die()
				return
			}
			g.burst(g.at(i), 0.5, 0.5, 10, themes[g.theme].tail)
			g.sound(sfxBite, 0.8)
			g.cut(i)
			break
		}
	}
	if g.bossCovers(g.lemon) {
		g.placeLemon()
	}
	for i := range g.seeds {
		if s := &g.seeds[i]; s.live && g.bossCovers(s.p) {
			s.live = false
			g.hit(s.p)
		}
	}
}

// fire spits a seed ahead of the head. Every lemon brings seedsPerLemon
// seeds; with none left it says so, with a soft thud, rather than nothing.
// Spitting once cost a tail segment, so a hammered space emptied the snake
// and then did nothing.
func (g *game) fire() {
	if !g.boss.on {
		return
	}
	if g.ammo == 0 {
		g.sound(sfxEmpty, 0.7)
		return
	}
	for i := range g.seeds {
		if s := &g.seeds[i]; !s.live {
			g.ammo--
			*s = seed{p: g.at(0), d: g.next(), live: true}
			g.sound(sfxSpit, 0.7)
			g.stepSeed(s)
			return
		}
	}
}

func (g *game) stepSeed(s *seed) {
	s.p = s.p.add(steps[s.d])
	switch {
	case !g.inside(s.p):
		s.live = false
	case g.bossCovers(s.p):
		s.live = false
		g.hit(s.p)
	}
}

func (g *game) hit(at pos) {
	if !g.boss.on {
		return
	}
	th := &themes[g.theme]
	g.boss.hp--
	g.boss.hitT = g.t
	g.score += 5 * g.level
	g.logHit()
	g.burst(at, 0.5, 0.5, 8, th.seed)
	if g.boss.hp > 0 {
		g.sound(sfxHit, 0.8)
		return
	}
	g.sound(sfxWin, 0.9)
	g.burst(g.boss.p, 1, 1, 40, th.boss)
	g.burst(g.boss.p, 1, 1, 16, th.bossEye)
	g.score += 100 * g.level
	g.boss.on = false
	g.seeds = [maxSeeds]seed{}
	if !g.inside(g.lemon) {
		g.placeLemon() // it had no room while the monster stood on the last free tiles
	}
	g.level++
	g.eaten = 0
	g.show(bnWin)
}

func abs(v int) int { return max(v, -v) }

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
