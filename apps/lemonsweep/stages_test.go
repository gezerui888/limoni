package main

import (
	"math"
	"strings"
	"testing"

	limoni "github.com/thebanri/limoni"
	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// Every map is closed, the player starts on floor, bosses arrive on floor,
// and all of it can be walked to from the start: nothing spawns sealed in.
func TestEveryMapIsWhole(t *testing.T) {
	defer useArena(0)
	for i := range arenas {
		a := &arenas[i]
		for y, row := range a.rows {
			if len(row) != mapW {
				t.Fatalf("%s row %d is %d wide", a.name, y, len(row))
			}
		}
		useArena(i)
		for k := 0; k < mapW; k++ {
			for _, p := range [4][2]int{{k, 0}, {k, mapH - 1}, {0, k}, {mapW - 1, k}} {
				if tile(p[0], p[1]) == '.' {
					t.Fatalf("%s is open at %v", a.name, p)
				}
			}
		}
		for _, p := range []vec{{16, 16.5}, {16, 4}, {16, 28}} {
			if !free(p.x, p.y, 0.6) {
				t.Fatalf("%s: %v is not floor", a.name, p)
			}
		}
		if len(portals) < 6 {
			t.Fatalf("%s has %d spawn points", a.name, len(portals))
		}
		// Flood the floor from the start; every floor tile must be reached.
		var seen [mapW * mapH]bool
		todo := [][2]int{{16, 16}}
		seen[16*mapW+16] = true
		for len(todo) > 0 {
			p := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				x, y := p[0]+d[0], p[1]+d[1]
				if tile(x, y) == '.' && !seen[y*mapW+x] {
					seen[y*mapW+x] = true
					todo = append(todo, [2]int{x, y})
				}
			}
		}
		for y := 0; y < mapH; y++ {
			for x := 0; x < mapW; x++ {
				if tile(x, y) == '.' && !seen[y*mapW+x] {
					t.Fatalf("%s: floor at (%d, %d) cannot be reached", a.name, x, y)
				}
			}
		}
	}
}

// Each map on the title is a run of its own: ten minutes bring its warden,
// and sweeping it ends the run won, counted on that map.
func TestEachMapIsARun(t *testing.T) {
	defer useArena(0)
	for stage := 0; stage < nStages; stage++ {
		g := newGame(5)
		g.mapSel = stage
		g.titlePick(0)
		if g.stage != stage || cur != &arenas[stage] || g.endless || g.phase != phPlay {
			t.Fatalf("map %d chosen, playing %d (%s), endless %v", stage, g.stage, cur.name, g.endless)
		}
		if g.heat() != stageLead*float64(stage) {
			t.Fatalf("map %d starts at heat %.0f", stage, g.heat())
		}
		g.stageT = wardenAt - tick/2
		g.step(tick)
		w := -1
		for i := range g.enemies {
			if e := &g.enemies[i]; e.alive && e.warden {
				w = i
			}
		}
		if w < 0 || g.enemies[w].k != wardens[stage] {
			t.Fatalf("map %d: no warden at ten minutes", stage)
		}
		g.kill(w)
		if g.clearT <= 0 {
			t.Fatalf("map %d: the warden swept, but the run goes on", stage)
		}
		for g.phase != phDead {
			if g.phase == phCards {
				g.take(0)
			}
			g.hp = g.st.maxHP
			g.step(tick)
		}
		if !g.won || g.best.runs != 1 || g.best.endless.runs != 0 || g.best.maps[stage].swept != 1 {
			t.Fatalf("map %d after its warden: won %v, runs %d/%d, swept %d", stage, g.won, g.best.runs, g.best.endless.runs, g.best.maps[stage].swept)
		}
	}
}

// A mini-boss comes every two minutes on the map's clock: elite, grown,
// shown at the top, and worth most of a level once its bytes are in.
func TestAMiniBossEveryTwoMinutes(t *testing.T) {
	g := playing()
	minis := func() (n, at int) {
		for i := range g.enemies {
			if e := &g.enemies[i]; e.alive && e.mini {
				n, at = n+1, i
			}
		}
		return
	}
	g.stageT = miniEvery - tick/2
	g.step(tick)
	n, i := minis()
	if n != 1 || !g.enemies[i].elite || g.enemies[i].hp < miniHP*0.5 {
		t.Fatalf("%d mini-bosses at two minutes", n)
	}
	b := buffer.NewBuffer(cell.Rect{Width: 140, Height: 44})
	g.render(b)
	if !strings.Contains(snapshot(b), "GIANT") {
		t.Error("no mini-boss bar at the top")
	}
	lv := g.level
	g.kill(i)
	g.hp = g.st.maxHP
	for s := 0; s < 60; s++ {
		for k := range g.bytes {
			if g.bytes[k].alive {
				g.pos = g.bytes[k].pos // walk over them
				break
			}
		}
		g.stepBytes(tick)
	}
	if g.level <= lv && g.xp < need(g.level)*0.6 {
		t.Fatalf("the mini-boss paid %.1f of %.1f", g.xp, need(g.level))
	}
	g.stageT = 2*miniEvery - tick/2
	g.step(tick)
	if n, _ := minis(); n != 1 {
		t.Fatalf("%d mini-bosses at four minutes", n)
	}
}

// Standing in one spot is camping: after campAfter the heap opens the
// spawn window nearest the player half the time, not only the far ones.
func TestCampingBringsThemClose(t *testing.T) {
	g := playing()
	g.pos = vec{2.5, 2.5} // THE HEAP's top-left corner, between two windows
	nearest := func() int {
		n := 0
		for k := 0; k < 200; k++ {
			if d := g.portal().dist2(g.pos); d < 7*7 {
				n++
			}
		}
		return n
	}
	moving := nearest()
	for s := 0; s < int(campAfter/tick)+5; s++ {
		g.stepCamp(tick)
	}
	if camping := nearest(); camping < moving+70 {
		t.Fatalf("%d of 200 spawns close to a camping player, %d to one on the move", camping, moving)
	}
}

// ENDLESS: /dev/zero, chosen on the title, has no warden and its own
// records, and the next run from the death card is endless again.
func TestEndlessIsDevZero(t *testing.T) {
	g := newGame(3)
	defer useArena(0)
	g.mapSel = len(arenas) - 1
	g.titlePick(0)
	if !g.endless || cur != &arenas[len(arenas)-1] || g.phase != phPlay {
		t.Fatalf("ENDLESS gave %s, endless %v", cur.name, g.endless)
	}
	g.stageT = wardenAt - tick/2
	g.step(tick)
	for i := range g.enemies {
		if g.enemies[i].alive && g.enemies[i].warden {
			t.Fatal("a warden came to /dev/zero")
		}
	}
	g.die()
	if g.best.endless.runs != 1 || g.best.runs != 0 {
		t.Fatalf("endless run counted as %d/%d", g.best.runs, g.best.endless.runs)
	}
	g.reset()
	if !g.endless || cur != &arenas[len(arenas)-1] {
		t.Fatal("run again left /dev/zero")
	}
}

// On the title A and D change the map, and a click on a map's card picks
// it without starting the run.
func TestTheTitlePicksTheMap(t *testing.T) {
	g := newGame(1)
	g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'd'})
	g.key(limoni.KeyEvent{Type: limoni.KeyRight})
	if g.mapSel != 2 {
		t.Fatalf("D twice chose map %d", g.mapSel)
	}
	g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'a'})
	if g.mapSel != 1 {
		t.Fatalf("A chose map %d", g.mapSel)
	}
	b := buffer.NewBuffer(cell.Rect{Width: 140, Height: 44})
	g.render(b)
	x0 := (140 - (4*mapCardW + 3*2)) / 2
	g.mouse(limoni.MouseEvent{X: uint16(x0 + 3*(mapCardW+2) + 5), Y: uint16(g.mapY + 2), Button: limoni.MouseLeft})
	if g.mapSel != 3 || g.phase != phTitle {
		t.Fatalf("a click on /dev/zero's card chose %d, phase %v", g.mapSel, g.phase)
	}
	g.key(limoni.KeyEvent{Type: limoni.KeyEnter})
	if !g.endless || g.phase != phPlay {
		t.Fatal("Enter with /dev/zero chosen did not start it")
	}
}

// An enemy with a wall across its line walks round it: on THE STACK a
// boba straight above a frame's wall reaches a player straight below it.
func TestEnemiesWalkRoundWalls(t *testing.T) {
	g := playing()
	defer useArena(0)
	g.enter(1)
	g.pos = vec{8.5, 9.5}
	i := g.addEnemy(kBoba, vec{8.5, 3.5})
	if g.sees(g.enemies[i].pos) {
		t.Fatal("the wall is not between them")
	}
	for s := 0; s < 30*20 && g.enemies[i].pos.dist2(g.pos) > 1; s++ {
		g.hurtT = 1 // it may bite; the test is whether it arrives
		g.stepEnemies(tick)
	}
	if d := g.enemies[i].pos.dist2(g.pos); d > 1 {
		t.Fatalf("the boba is still %.1f away, at %v", math.Sqrt(d), g.enemies[i].pos)
	}
}

// Each map keeps its own records: a sweep counts with how long the warden
// stood, a death on it counts too, and the fastest stays the fastest.
func TestMapRecords(t *testing.T) {
	g := playing()
	defer useArena(0)
	g.kills = 100
	g.enter(0)
	g.kills += 40
	g.stageT = wardenAt + 50
	g.swept()
	m := g.best.maps[0]
	if m.runs != 1 || m.swept != 1 || m.fastest != 50 || m.kills != 40 {
		t.Fatalf("after a sweep: %+v", m)
	}
	g.best.maps[0].add(true, 80, 10)
	if g.best.maps[0].fastest != 50 || g.best.maps[0].kills != 40 {
		t.Fatalf("a slower sweep changed the best: %+v", g.best.maps[0])
	}
	// Dying in the moment after a sweep does not count the map again.
	g.die()
	if g.best.maps[0].runs != 2 || g.best.maps[1].runs != 0 {
		t.Fatalf("a death after the sweep counted: %+v", g.best.maps)
	}
	g = playing()
	g.enter(1)
	g.kills = 7
	g.die()
	if m := g.best.maps[1]; m.runs != 1 || m.swept != 0 || m.kills != 7 {
		t.Fatalf("a death on map 2: %+v", m)
	}
}

// The title shows every map's records, and drawing it allocates nothing.
func TestTitleShowsTheMaps(t *testing.T) {
	g := newGame(1)
	g.best.bests = bests{runs: 4, secs: 1931, level: 66, kills: 3120}
	g.best.maps = [nStages]mapBest{{runs: 4, swept: 3, fastest: 41, kills: 820}, {runs: 3, swept: 1, fastest: 77, kills: 1100}, {runs: 1, kills: 300}}
	g.best.endless = bests{runs: 2, secs: 1250, level: 58, kills: 2900}
	for _, sz := range [][2]uint16{{140, 44}, {110, 32}, {80, 30}} {
		b := buffer.NewBuffer(cell.Rect{Width: sz[0], Height: sz[1]})
		g.render(b)
		screen := snapshot(b)
		for _, want := range []string{"THE HEAP", "THE STACK", "KERNEL SPACE", "/dev/zero", "warden down", "0:41", "20:50"} {
			if !strings.Contains(screen, want) {
				t.Errorf("%dx%d: the title has no %q\n%s", sz[0], sz[1], want, screen)
			}
		}
		if avg := testing.AllocsPerRun(20, func() { g.render(b) }); avg != 0 {
			t.Errorf("%dx%d: the title allocates %.1f a frame", sz[0], sz[1], avg)
		}
	}
}

// Every map's line under its name fits inside its card.
func TestMapAboutFitsItsCard(t *testing.T) {
	for _, a := range arenas {
		if w := width(a.about); w > mapCardW-4 {
			t.Errorf("%s: %q is %d wide, the card holds %d", a.name, a.about, w, mapCardW-4)
		}
	}
}

// A boss is wider than a tile: started anywhere it fits on any map, it must
// come within its fighting range of the player, not stand against a wall
// it was led into. Walks are counted from many starts, and every boss
// kind; without its own count (flow[1]) some stuck in the gaps.
func TestBossesDoNotStickInWalls(t *testing.T) {
	defer useArena(0)
	for stage := range arenas {
		g := playing()
		g.enter(stage)
		g.endless = stage == len(arenas)-1
		r := rng(uint64(stage) + 7)
		for try := 0; try < 60; try++ {
			k := bosses[try%len(bosses)]
			kd := &kinds[k]
			var at, me vec
			for {
				at = vec{1 + r.f()*(mapW-2), 1 + r.f()*(mapH-2)}
				me = vec{1 + r.f()*(mapW-2), 1 + r.f()*(mapH-2)}
				if free(at.x, at.y, kd.r*0.8) && free(me.x, me.y, playerR) && at.dist2(me) > 12*12 {
					break
				}
			}
			g.enemies = [maxEnemies]enemy{}
			g.pos = me
			i := g.addEnemy(k, at)
			e := &g.enemies[i]
			e.cd = 1e9 // no shots or hazards: only the walk
			for s := 0; s < 30*60 && e.pos.dist2(g.pos) > 7.5*7.5; s++ {
				g.hurtT = 1
				g.stepEnemies(tick)
			}
			if d := math.Sqrt(e.pos.dist2(g.pos)); d > 7.5 {
				t.Errorf("%s: %s from %v to the player at %v stopped %.1f away, at %v",
					cur.name, kd.name, at, me, d, e.pos)
			}
		}
	}
}

// A boss that keeps its distance keeps it in sight: one at its range with
// a wall between comes round the wall, rather than waiting behind it.
func TestABossBehindAWallComesRound(t *testing.T) {
	defer useArena(0)
	g := playing()
	g.enter(0)
	g.pos = vec{10.5, 11.5} // under the foot of the HEAP's L, row 9
	i := g.addEnemy(kQueen, vec{10.5, 6.5})
	e := &g.enemies[i]
	e.cd = 1e9
	if g.sees(e.pos) {
		t.Fatal("the wall is not between them")
	}
	for s := 0; s < 30*20 && !g.sees(e.pos); s++ {
		g.hurtT = 1
		g.stepEnemies(tick)
	}
	if !g.sees(e.pos) {
		t.Fatalf("the Queen waited behind the wall, at %v", e.pos)
	}
}
