package main

import "math"

// The walls of the maps past the first. Userland is the user's own files
// left out in the open: folders, documents, a trash can, boxes of archives.
// Kernel Space is the machine under them: CPUs, memory sticks, a board.
// Each is drawn once at start-up, in pixels, like the terminal windows.

func init() {
	wallTextures['f'] = makeFolders()
	wallTextures['d'] = makeDocuments()
	wallTextures['r'] = makeTrash()
	wallTextures['a'] = makeArchives()
	wallTextures['u'] = makeCPUs()
	wallTextures['m'] = makeRAM()
	wallTextures['k'] = makeBoard()
}

// fill paints the rectangle from (x0, y0) to (x1, y1), both inclusive.
func (w window) fill(x0, y0, x1, y1 int, c rgb) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			w.set(x, y, c)
		}
	}
}

// background starts a texture of one colour, without a window's frame.
func background(c rgb) window {
	w := newWindow(winW, winH, 4)
	w.fill(0, 0, winW-1, winH-1, c)
	return w
}

func label(w window, x, y int, s string, c rgb) {
	w.text(x, y, s, func(int) rgb { return c })
}

func makeFolders() *wallTex {
	w := background(hex(0x0b1519))
	for i, name := range [...]string{"src", "docs", "tmp"} {
		x := 6 + i*42
		w.fill(x, 9, x+12, 13, hex(0xc99419))  // the tab
		w.fill(x, 13, x+33, 40, hex(0xc99419)) // the back
		w.fill(x, 18, x+33, 40, hex(0xf2c443)) // the front
		w.fill(x, 18, x+33, 18, hex(0xffe08a)) // its lit edge
		w.fill(x, 40, x+33, 40, hex(0x9c7210)) // and its shadow
		label(w, x+17-width(name)*2, 44, name, hex(0xa9b8bf))
	}
	return w.done()
}

func makeDocuments() *wallTex {
	w := background(hex(0x0d1316))
	paper, fold := hex(0xe9e6dc), hex(0xb3afa4)
	for i, name := range [...]string{"a.go", "TODO", "b.md", "log"} {
		x := 5 + i*31
		w.fill(x, 7, x+21, 39, paper)
		for k := 0; k < 6; k++ { // the corner folded down
			w.fill(x+16+k, 7, x+21, 7+k, hex(0x0d1316))
			w.set(x+16+k, 7+k, fold)
			w.fill(x+16, 7+k, x+16+k-1, 7+k, fold)
		}
		for r := 0; r < 6; r++ {
			l := 8 + int(hash2(i, r)*10)
			c := hex(0x8a8d96)
			if r == 0 {
				c, l = hex(0x4a78c8), 10
			}
			w.fill(x+3, 15+r*4, x+3+l, 15+r*4, c)
		}
		label(w, x+11-width(name)*2, 44, name, hex(0xa9b4bc))
	}
	return w.done()
}

func makeTrash() *wallTex {
	w := background(hex(0x10161a))
	for _, x := range [...]int{14, 78} {
		w.fill(x+14, 5, x+22, 7, hex(0x9aa4ae)) // the handle
		w.fill(x-2, 8, x+38, 11, hex(0xb8c2cc)) // the lid
		w.fill(x-2, 11, x+38, 11, hex(0x6f7a84))
		for y := 12; y <= 44; y++ {
			in := (y - 12) / 6
			for xx := x + in; xx <= x+36-in; xx++ {
				c := hex(0x7f8a94)
				if (xx-x)%6 == 3 {
					c = hex(0x56606a) // the ribs
				}
				w.set(xx, y, c)
			}
		}
		label(w, x+6, 46, "Trash", hex(0x9aa6b0))
	}
	return w.done()
}

func makeArchives() *wallTex {
	w := background(hex(0x120f0c))
	box := func(x, y, bw, bh int, name string) {
		w.fill(x, y, x+bw, y+bh, hex(0xb98a52))
		w.fill(x, y, x+bw, y+1, hex(0x8e6838))             // the flaps
		w.fill(x+bw/2-1, y, x+bw/2+1, y+bh, hex(0xd8b880)) // the tape
		w.fill(x, y+bh, x+bw, y+bh, hex(0x7a5830))
		label(w, x+3, y+bh-7, name, hex(0x4a3418))
	}
	box(4, 28, 38, 20, "tar")
	box(45, 28, 38, 20, "zip")
	box(86, 28, 38, 20, "gz")
	box(22, 7, 38, 20, "7z")
	box(66, 7, 38, 20, "xz")
	return w.done()
}

func makeCPUs() *wallTex {
	w := background(hex(0x1a0d10))
	for i, x := range [...]int{8, 72} {
		w.fill(x, 8, x+46, 44, hex(0x2a2d33)) // the package
		for k := 0; k <= 46; k += 3 {         // its pins, all round
			w.set(x+k, 6, hex(0xb8bcc4))
			w.set(x+k, 46, hex(0xb8bcc4))
		}
		for k := 0; k <= 36; k += 3 {
			w.set(x-2, 8+k, hex(0xb8bcc4))
			w.set(x+48, 8+k, hex(0xb8bcc4))
		}
		w.fill(x+9, 14, x+37, 38, hex(0x8b929c)) // the heat spreader
		w.fill(x+9, 14, x+37, 15, hex(0xc8ced6))
		label(w, x+15, 24, [...]string{"CPU0", "CPU1"}[i], hex(0x3a3f46))
	}
	return w.done()
}

func makeRAM() *wallTex {
	w := background(hex(0x1a0d10))
	w.fill(0, 8, winW-1, 44, hex(0x1f6a3a)) // the stick's board
	w.fill(0, 8, winW-1, 8, hex(0x3a9a5a))
	for i := 0; i < 8; i++ {
		x := 4 + i*15
		w.fill(x, 13, x+11, 27, hex(0x18191c)) // a chip
		w.set(x+1, 14, hex(0xb8bcc4))
	}
	label(w, 50, 31, "DDR5", hex(0xd8e8dc))
	for x := 0; x < winW; x += 3 { // the gold contacts, notched
		if x >= 60 && x <= 66 {
			continue
		}
		w.fill(x, 39, x+1, 44, hex(0xd8b04a))
	}
	return w.done()
}

func makeBoard() *wallTex {
	bg := hex(0x16090c)
	w := background(bg)
	copper := hex(0xb0702e)
	// Traces: each runs along, then turns up or down, and ends in a via.
	for i := 0; i < 16; i++ {
		x0 := int(hash2(i, 1) * float32(winW))
		y0 := 4 + int(hash2(i, 2)*float32(winH-8))
		l := 10 + int(hash2(i, 3)*40)
		x1 := min(winW-3, x0+l)
		w.fill(x0, y0, x1, y0, copper)
		y1 := y0 + int((hash2(i, 4)-0.5)*30)
		y1 = max(3, min(winH-4, y1))
		w.fill(x1, min(y0, y1), x1, max(y0, y1), copper)
		w.fill(x1-1, y1-1, x1+1, y1+1, hex(0xe0a050)) // the via
		w.set(x1, y1, bg)
	}
	for i := 0; i < 10; i++ { // parts soldered on
		x := int(hash2(i, 9) * float32(winW-8))
		y := 4 + int(hash2(i, 10)*float32(winH-10))
		w.fill(x, y, x+5, y+2, hex(0x1c1c20))
		w.set(x, y+1, hex(0xc8ccd4))
		w.set(x+5, y+1, hex(0xc8ccd4))
	}
	w.fill(100, 40, 102, 42, hex(0x40ff70)) // a power light
	return w.done()
}

// ── floors and skies ─────────────────────────────────────────────────────

// noise is smooth value noise: hash2 at whole points, eased between.
func noise(x, y float64) float32 {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := float32(x-math.Floor(x)), float32(y-math.Floor(y))
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a := hash2(ix, iy) + (hash2(ix+1, iy)-hash2(ix, iy))*fx
	b := hash2(ix, iy+1) + (hash2(ix+1, iy+1)-hash2(ix, iy+1))*fx
	return a + (b-a)*fy
}

// groundAt is Userland's floor: open ground, darker and lighter in patches
// where the grid of a room's floor would be.
func groundAt(wx, wy, d float64) rgb {
	k := 0.15 + 0.6*noise(wx*0.25, wy*0.25) + 0.25*noise(wx*1.3, wy*1.3)
	return fogged(hex(cur.floor).mix(hex(cur.seam), k), d)
}

// skyAt is Userland's sky: from the haze at the horizon up into the night,
// with stars fixed on the turning view's angle, not on the ground.
func (g *game) skyAt(v *view, fy, rx, ry float64) rgb {
	t := float32(math.Min(1, (v.horizon-fy)/math.Max(1, v.horizon)*1.6))
	c := hex(cur.light).mix(hex(cur.ceil), t)
	ang := math.Atan2(ry, rx)
	if t > 0.3 && hash2(int(ang*90), int(fy)) > 0.998 {
		c = c.mix(rgb{1, 1, 1}, 0.3+0.15*float32(math.Sin(g.clock*3+ang*40)))
	}
	return c
}

// traceAt is Kernel Space's floor: the board, a copper trace across some
// tiles and along others, and a via where two meet.
func traceAt(wx, wy, fx, fy, d float64) rgb {
	ix, iy := int(math.Floor(wx)), int(math.Floor(wy))
	k := 0.0
	along, across := hash2(ix, iy) < 0.45, hash2(iy+31, ix) < 0.45
	if along {
		k = math.Max(k, seam(wy-0.5, fy, 0.06))
	}
	if across {
		k = math.Max(k, seam(wx-0.5, fx, 0.06))
	}
	if along && across {
		if dx, dy := wx-float64(ix)-0.5, wy-float64(iy)-0.5; dx*dx+dy*dy < 0.02 {
			k = 1
		}
	}
	k /= 1 + d*0.1
	return fogged(hex(cur.floor).mix(hex(cur.seam), float32(k)), d)
}
