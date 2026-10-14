package main

import (
	"math"
	"strings"
	"testing"

	"github.com/thebanri/limoni"
	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// playing is a run under way, in a window of 100×30.
func playing() *game {
	g := newGame(7)
	g.lastW, g.lastH = 100, 30
	g.start()
	return g
}

// step runs exactly one tick.
func step(g *game) { g.update(g.interval()) }

func TestMovesAndEats(t *testing.T) {
	g := playing()
	h := g.at(0)
	g.lemon = h.add(steps[dRight])
	step(g)
	if g.at(0) != g.lemon && g.score != 10 {
		t.Fatalf("head %v, score %d; want the lemon eaten for 10", g.at(0), g.score)
	}
	step(g) // the growth shows a step later
	if g.length != startLen+1 {
		t.Fatalf("length %d, want %d", g.length, startLen+1)
	}
}

func TestReversingIsIgnored(t *testing.T) {
	g := playing()
	g.turn(dLeft)
	step(g)
	if g.phase != phPlay || g.dir != dRight {
		t.Fatalf("phase %d dir %d: a reverse turn must be ignored", g.phase, g.dir)
	}
	// Two quick turns make a U-turn over two steps, not a reversal.
	g.turn(dUp)
	g.turn(dLeft)
	h := g.at(0)
	step(g)
	step(g)
	if want := h.add(pos{-1, -1}); g.at(0) != want || g.phase != phPlay {
		t.Fatalf("head %v, want %v", g.at(0), want)
	}
}

func TestWallAndSelfEndTheRun(t *testing.T) {
	g := playing()
	for i := 0; i < g.cols+2 && g.phase == phPlay; i++ {
		step(g)
	}
	if g.phase != phOver {
		t.Fatal("running into the wall did not end the run")
	}

	g = playing()
	g.grow = 3
	for _, d := range []dir{dDown, dLeft, dUp} {
		g.turn(d)
		step(g)
	}
	step(g) // past the grace
	if g.phase != phOver {
		t.Fatal("turning into its own body did not end the run")
	}
}

// A step into the wall waits half a step for a late turn, once: a turn
// then saves the run, and none ends it.
func TestALateTurnAtTheWallIsForgiven(t *testing.T) {
	g := playing()
	toWall := g.cols - 1 - g.at(0).x
	for range toWall {
		step(g)
	}
	if g.at(0).x != g.cols-1 || g.phase != phPlay {
		t.Fatalf("head %v, phase %d: not at the wall", g.at(0), g.phase)
	}
	step(g) // the step into the wall: held
	if g.phase != phPlay || !g.graced {
		t.Fatal("no grace at the wall")
	}
	g.update(g.interval() / 4) // a moment late
	g.turn(dDown)
	if g.phase != phPlay || g.at(0) != (pos{g.cols - 1, g.rows/2 + 1}) {
		t.Fatalf("the late turn did not save it: head %v, phase %d", g.at(0), g.phase)
	}

	// Without a turn, the grace runs out.
	g = playing()
	for range toWall + 1 {
		step(g)
	}
	g.update(g.interval() / 2)
	if g.phase != phOver {
		t.Fatal("the grace never ran out")
	}
}

func TestMonsterComesFightsAndFalls(t *testing.T) {
	g := playing()
	for range lemonsPerLevel {
		g.eat()
	}
	if !g.boss.on || g.boss.hp != 4 {
		t.Fatalf("monster %+v, want it on with 4 health at level 1", g.boss)
	}
	if g.bossCovers(g.at(0)) {
		t.Fatal("the monster landed on the head")
	}

	// Five lemons brought ten seeds; a spit spends one, not the tail.
	n := g.length
	if g.ammo != 2*lemonsPerLevel {
		t.Fatalf("%d seeds after %d lemons", g.ammo, lemonsPerLevel)
	}
	g.fire()
	if g.length != n || g.ammo != 2*lemonsPerLevel-1 || !g.seeds[0].live {
		t.Fatalf("length %d (was %d), %d seeds, seed %+v", g.length, n, g.ammo, g.seeds[0])
	}
	g.ammo = 0
	g.fire()
	if g.seeds[1].live {
		t.Fatal("a snake with no seeds spat one")
	}

	// Each hit takes one health; the last one ends the level.
	for g.boss.on {
		g.hit(g.boss.p)
	}
	if g.level != 2 || g.eaten != 0 {
		t.Fatalf("level %d eaten %d after the monster fell", g.level, g.eaten)
	}
	if g.interval() >= 0.16 {
		t.Fatal("level 2 is not faster")
	}
}

func TestSeedsFlyAndStrike(t *testing.T) {
	g := playing()
	g.boss = monster{on: true, p: g.at(0).add(pos{6, 0}), hp: 3, maxHP: 3, wait: 1 << 30}
	g.ammo = 1
	step(g)
	g.fire()
	for range 4 {
		step(g)
	}
	if g.boss.hp != 2 {
		t.Fatalf("monster health %d, want 2 after one seed", g.boss.hp)
	}
}

func TestMonsterBitesTheBody(t *testing.T) {
	g := playing()
	g.grow = 4
	for range 4 {
		step(g)
	}
	// The monster, two tiles wide, on segments 5 and 4 of a snake heading right.
	g.boss = monster{on: true, p: g.at(5), hp: 3, maxHP: 3}
	g.landed()
	if g.length != 4 || g.phase != phPlay {
		t.Fatalf("length %d phase %d: want the body bitten off at 4", g.length, g.phase)
	}
	g.boss.p = g.at(0)
	g.landed()
	if g.phase != phOver {
		t.Fatal("the monster on the head did not end the run")
	}
}

func TestThemeKeyCycles(t *testing.T) {
	g := newGame(1)
	for i := range len(themes) {
		if g.theme != i {
			t.Fatalf("theme %d, want %d", g.theme, i)
		}
		g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'T'})
	}
	if g.theme != 0 {
		t.Fatal("themes do not wrap round")
	}
}

// TestFramesAllocateNothing plays, renders and diffs frames, as the
// terminal would, in every theme, with the monster on the board.
func TestFramesAllocateNothing(t *testing.T) {
	g := newGame(3)
	r := cell.Rect{Width: 100, Height: 30}
	cur, prev := buffer.NewBuffer(r), buffer.NewBuffer(r)
	out := make([]byte, 0, 1<<16)
	n := 0
	frame := func() {
		if g.phase != phPlay {
			g.start()
		}
		switch n % 30 {
		case 0:
			g.turn(dir(g.rand(4)))
		case 10:
			g.eat()
		case 20:
			g.fire()
			g.theme = (g.theme + 1) % len(themes)
		}
		n++
		g.update(1.0 / 60)
		g.render(cur)
		out, _ = buffer.DiffWithOptions(cur, prev, out[:0], buffer.DiffOptions{TrueColor: true, EraseChar: true, RepeatChar: true})
		cur, prev = prev, cur
	}
	for range 600 {
		frame()
	}
	if avg := testing.AllocsPerRun(600, frame); avg != 0 {
		t.Errorf("%.2f allocations per frame, want 0", avg)
	}
}

func TestBoardFitsTheWindow(t *testing.T) {
	for _, c := range []struct {
		w, h, cols, rows int
		ok               bool
	}{
		{80, 24, 14, 11, true},
		{200, 60, maxCols, maxRows, true},
		{70, 18, minCols, minRows, true},
		{69, 18, 11, 8, false},
		{70, 17, 12, 7, false},
	} {
		cols, rows, ok := fitBoard(c.w, c.h)
		if cols != c.cols || rows != c.rows || ok != c.ok {
			t.Errorf("%d×%d: %d×%d %v, want %d×%d %v", c.w, c.h, cols, rows, ok, c.cols, c.rows, c.ok)
		}
	}
	// A run started in a window takes the board that fits it.
	g := newGame(1)
	g.lastW, g.lastH = 80, 24
	g.start()
	if g.cols != 14 || g.rows != 11 || g.at(0) != (pos{7, 5}) {
		t.Fatalf("board %d×%d head %v", g.cols, g.rows, g.at(0))
	}
}

// TestHeadSlidesBetweenSteps checks that the head is drawn moving between
// steps, a pixel or more ahead half-way through one, not a tile at a time.
func TestHeadSlidesBetweenSteps(t *testing.T) {
	g := playing()
	th := &themes[0]
	headAt := func() int { // the rightmost pixel of the head's colour on its row
		g.paint(th)
		_, cy := centreOf(g.at(0))
		last := -1
		for x := range g.cols * tilePx {
			if g.px[int(cy)*pxW+x].color() == th.head.color() {
				last = x
			}
		}
		return last
	}
	a := headAt()
	g.update(g.interval() / 2)
	b := headAt()
	if b-a < 1 || b-a > 3 {
		t.Fatalf("half a step moved the head %d pixels, want 1 to 3", b-a)
	}
}

// TestTurnsAreImmediateAndNeverJump follows the drawn head frame by frame
// through turns pressed at every point of a step. The head must start the
// new way on the very next frame, and never jump back: drawn sliding ahead
// while turns waited for the step, a late turn pulled it back a tile.
func TestTurnsAreImmediateAndNeverJump(t *testing.T) {
	g := newGame(7) // on the largest board, with room for every turn
	g.start()
	frame := func() float64 { return min(1, g.acc/g.interval()) }
	x, y := g.headPx(frame())
	turns := []dir{dUp, dLeft, dUp, dRight, dDown, dRight}
	for i := range 300 {
		turned := i%17 == 9
		if turned {
			g.turn(turns[i/17%len(turns)])
		}
		g.update(1.0 / 60)
		if g.phase != phPlay {
			t.Fatalf("died at frame %d", i)
		}
		nx, ny := g.headPx(frame())
		if d := math.Hypot(nx-x, ny-y); d > 2.9 {
			t.Fatalf("frame %d: the head jumped from (%.1f, %.1f) to (%.1f, %.1f)", i, x, y, nx, ny)
		}
		if want := steps[g.next()]; turned && (nx-x)*float64(want.x)+(ny-y)*float64(want.y) <= 0 {
			t.Fatalf("frame %d: turned %v, but the head moved (%.2f, %.2f)", i, want, nx-x, ny-y)
		}
		x, y = nx, ny
	}
}

// TestDialogsAllocateNothing holds each dialog, opening and open, with a
// full board on it, to zero allocations a frame through the diff.
func TestDialogsAllocateNothing(t *testing.T) {
	for _, sc := range []struct {
		name  string
		setup func(g *game)
	}{
		{"title", func(g *game) { g.phase = phTitle }},
		{"name", func(g *game) { g.askName(); g.typing = "Ada" }},
		{"paused", func(g *game) { g.paused = true; g.noSound = "no player found" }},
		{"over, world board", func(g *game) {
			g.worldState, g.worldRank = worldOK, 3
			g.die()
			g.diedT -= 5
		}},
		{"over, own board", func(g *game) { g.localRank = 2; g.die(); g.diedT -= 5 }},
		{"over, offline", func(g *game) { g.worldState = worldOffline; g.die(); g.diedT -= 5 }},
	} {
		t.Run(sc.name, func(t *testing.T) {
			g := playing()
			g.name = "Ada"
			for i := range boardSize {
				e := scoreEntry{Name: "Player", Score: 1000 - i*50, Level: 3}
				g.world[i], g.local[i] = e, e
			}
			g.nWorld, g.nLocal = boardSize, boardSize
			sc.setup(g)
			r := cell.Rect{Width: 100, Height: 30}
			cur, prev := buffer.NewBuffer(r), buffer.NewBuffer(r)
			out := make([]byte, 0, 1<<16)
			frame := func() {
				g.update(1.0 / 60)
				g.render(cur)
				out, _ = buffer.DiffWithOptions(cur, prev, out[:0], buffer.DiffOptions{TrueColor: true, EraseChar: true, RepeatChar: true})
				cur, prev = prev, cur
			}
			frame()
			if g.dialog == dlgNone {
				t.Fatal("no dialog on screen")
			}
			if avg := testing.AllocsPerRun(60, frame); avg != 0 { // opening
				t.Errorf("%.2f allocations per frame while opening, want 0", avg)
			}
			if avg := testing.AllocsPerRun(60, frame); avg != 0 { // open
				t.Errorf("%.2f allocations per frame when open, want 0", avg)
			}
		})
	}
}

// At first launch the game opens on the name dialog, on a board not yet
// fitted to the window: it must fit it, not ask for a larger window.
func TestTheFirstDialogFitsTheWindow(t *testing.T) {
	g := newGame(1)
	g.askName()
	g.typing = "Arslan"
	b := buffer.NewBuffer(cell.Rect{Width: 90, Height: 26})
	g.render(b)
	g.dialogT -= 10
	g.render(b)
	var sb strings.Builder
	for _, c := range b.Content {
		sb.WriteRune(c.Content)
	}
	if s := sb.String(); !strings.Contains(s, "YOUR NAME") || !strings.Contains(s, "Arslan") || strings.Contains(s, "larger window") {
		t.Fatal("the name dialog did not open in a 90×26 window")
	}
}

// TestSpaceCanBeHammered: with seeds to spit, every press spits one. Only
// four could fly at once, and each cost a tail segment, so a hammered
// space soon did nothing, and felt like a key that stuck.
func TestSpaceCanBeHammered(t *testing.T) {
	g := playing()
	for range lemonsPerLevel {
		g.eat()
	}
	g.boss.p, g.boss.wait = pos{0, 0}, 1<<30 // out of the way, and still
	have := g.ammo
	spat := 0
	for range have {
		g.key(limoni.KeyEvent{Type: limoni.KeySpace})
		g.update(1.0 / 60)
		spat = have - g.ammo
		if g.phase != phPlay {
			t.Fatal("the run ended")
		}
	}
	if spat != have || g.ammo != 0 {
		t.Fatalf("%d presses spat %d seeds", have, spat)
	}
}
