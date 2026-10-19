package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBackgroundReply(t *testing.T) {
	c, ok, rest, done := backgroundReply([]byte("a\x1b]11;rgb:2828/2c2c/3434\x1b\\b\x1b[?62;22cc"))
	if !done || !ok || c != (color.RGBA{0x28, 0x2c, 0x34, 0xff}) || string(rest) != "abc" {
		t.Fatalf("got %v %v %q %v", c, ok, rest, done)
	}
	if _, ok, rest, done := backgroundReply([]byte("x\x1b[?1;2c")); !done || ok || string(rest) != "x" {
		t.Fatalf("no OSC 11 answer: %v %q %v", ok, rest, done)
	}
	if _, _, _, done := backgroundReply([]byte("\x1b]11;rgb:ff/ff/ff\a\x1b[?6")); done {
		t.Fatal("done before the DA1 answer ended")
	}
	if c, ok := parseXColor("rgb:f/80/ffff"); !ok || c != (color.RGBA{255, 128, 255, 255}) {
		t.Fatalf("parseXColor: %v %v", c, ok)
	}
}

// A kitty animation: frames added on the root, played in order, its gaps
// kept, and stopped at the end of its loops.
func TestKittyAnimation(t *testing.T) {
	s, _ := testSession(t)
	s.apc([]byte("Ga=T,f=100,i=3,q=2;" + pngOf(t, 10, 20)))
	rgba := base64.StdEncoding.EncodeToString([]byte{0, 255, 0, 255})
	s.apc([]byte("Ga=f,i=3,f=32,s=1,v=1,x=1,y=1,c=1,z=100,q=2;" + rgba))
	s.apc([]byte("Ga=f,i=3,f=32,s=1,v=1,X=1,q=2;" + rgba))
	a := s.kitty.images[3]
	if len(a.frames) != 3 || a.gaps[1] != 100*time.Millisecond || a.gaps[2] != defaultGap {
		t.Fatalf("%d frames, gaps %v", len(a.frames), a.gaps)
	}
	f1 := a.frames[1].(*image.RGBA)
	if f1.RGBAAt(0, 0) != (color.RGBA{255, 0, 0, 255}) || f1.RGBAAt(1, 1) != (color.RGBA{0, 255, 0, 255}) {
		t.Fatal("frame 2 is not the root with a green pixel at 1,1")
	}
	if f2 := a.frames[2].(*image.RGBA); f2.RGBAAt(0, 0) != (color.RGBA{0, 255, 0, 255}) || f2.RGBAAt(1, 1) != (color.RGBA{}) {
		t.Fatal("frame 3 is not a green pixel on transparency")
	}

	s.apc([]byte("Ga=a,i=3,s=3,v=2")) // play, and loop once more
	start := a.due.Add(-a.gaps[0])
	for _, c := range []struct {
		after time.Duration
		frame int
	}{{39, 0}, {40, 1}, {139, 1}, {140, 2}, {180, 0}, {360, 2}, {1000, 2}} {
		a.step(start.Add(c.after * time.Millisecond))
		if a.cur != c.frame {
			t.Fatalf("at %dms frame %d, want %d", c.after, a.cur, c.frame)
		}
	}
	if a.run {
		t.Fatal("still running after its loops")
	}

	// The picture shows the frame its animation is at.
	s.apc([]byte("Ga=a,i=3,c=2"))
	s.animate()
	for _, pic := range s.pics.byID {
		if pic.src != a.frames[1] {
			t.Fatal("the picture did not follow the animation")
		}
	}
	s.apc([]byte("Ga=c,i=3,r=3,c=1,w=1,h=1,x=5,y=5,C=1"))
	if a.frames[0].(*image.RGBA).RGBAAt(5, 5) != (color.RGBA{0, 255, 0, 255}) {
		t.Fatal("a=c did not compose frame 3 onto frame 1")
	}
	if s.timer != nil {
		s.timer.Stop()
	}
}

func TestKittyImageNumbers(t *testing.T) {
	s, r := testSession(t)
	s.apc([]byte("Ga=t,f=100,I=13;" + pngOf(t, 10, 20)))
	if got := replyOf(t, r); !strings.HasPrefix(got, "\x1b_Gi=4294967295,I=13;OK") {
		t.Fatalf("reply %q", got)
	}
	s.apc([]byte("Ga=p,I=13,q=2"))
	if len(s.pics.byID) != 1 {
		t.Fatal("a placement by number showed nothing")
	}
	s.apc([]byte("Ga=d,d=N,I=13"))
	if len(s.pics.byID) != 0 || len(s.kitty.images) != 0 || len(s.kitty.numbers) != 0 {
		t.Fatal("d=N left something")
	}
}

// A placement relative to another goes H, V cells from it, leaves the cursor,
// and goes with it.
func TestKittyRelativePlacement(t *testing.T) {
	s, r := testSession(t)
	s.apc([]byte("Ga=t,f=100,i=1,q=2;" + pngOf(t, 10, 20)))
	s.text([]byte("\x1b[3;5H"))
	s.apc([]byte("Ga=p,i=1,p=1,q=2,X=3,Y=4"))
	at := s.emu.CursorPosition()
	s.apc([]byte("Ga=p,i=1,p=2,P=1,Q=1,H=2,V=-1,q=2"))
	if s.emu.CursorPosition() != at {
		t.Fatal("a relative placement moved the cursor")
	}
	s.pics.update()
	var child *picture
	for _, k := range s.kitty.shown {
		if k.placement == 2 {
			child = k.pic
		}
		if k.placement == 1 && (k.pic.ox != 3 || k.pic.oy != 4) {
			t.Fatalf("offset %d,%d, want 3,4", k.pic.ox, k.pic.oy)
		}
	}
	if child == nil || child.col != 6 || child.top != 1 {
		t.Fatalf("child at %+v, want 6,1", child)
	}
	s.apc([]byte("Ga=p,i=1,p=3,P=9,i=1"))
	if got := replyOf(t, r); !strings.Contains(got, "ENOPARENT") {
		t.Fatalf("reply to a missing parent %q", got)
	}
	s.apc([]byte("Ga=d,d=i,i=1,p=1"))
	if len(s.pics.byID) != 0 {
		t.Fatal("the child outlived its parent")
	}
}

func TestKittyDeleteByPosition(t *testing.T) {
	s, _ := testSession(t)
	s.apc([]byte("Ga=t,f=100,i=1,q=2;" + pngOf(t, 20, 40))) // 2×2 cells
	place := func(row, col, z int) {
		s.text([]byte("\x1b[" + strconv.Itoa(row) + ";" + strconv.Itoa(col) + "H"))
		s.apc([]byte("Ga=p,i=1,C=1,q=2,z=" + strconv.Itoa(z)))
	}
	place(1, 1, 0)
	place(1, 11, 5)
	place(5, 1, 0)
	for _, c := range []struct {
		del  string
		left int
	}{
		{"d=p,x=12,y=2", 2}, // a cell of the second
		{"d=z,z=5", 2},      // nothing left at z=5
		{"d=y,y=6", 1},      // a row of the third
		{"d=x,x=1", 0},
	} {
		s.apc([]byte("Ga=d," + c.del))
		if n := len(s.pics.byID); n != c.left {
			t.Fatalf("after %s: %d pictures, want %d", c.del, n, c.left)
		}
	}
	if s.kitty.images[1] == nil {
		t.Fatal("lower case freed the image")
	}
	place(1, 1, 0)
	s.text([]byte("\x1b[2;2H"))
	s.apc([]byte("Ga=d,d=C"))
	if len(s.pics.byID) != 0 || s.kitty.images[1] != nil {
		t.Fatal("d=C at the picture left it or its image")
	}
}

// Placeholders written as text show the virtual placement's cells: one
// piece for a block of them, wherever the text is.
func TestKittyPlaceholders(t *testing.T) {
	s, _ := testSession(t)
	s.apc([]byte("Ga=T,U=1,f=100,i=42,c=3,r=2,q=2;" + pngOf(t, 30, 40)))
	if len(s.pics.byID) != 0 || len(s.kitty.virtual) != 1 {
		t.Fatal("a virtual placement was shown at the cursor")
	}
	// Image 42 in the 256 colours; rows by diacritic, columns inferred.
	s.text([]byte("\x1b[2;5H\x1b[38;5;42m\U0010EEEE̅\U0010EEEE\U0010EEEE"))
	s.text([]byte("\x1b[3;5H\U0010EEEE̍\U0010EEEE\U0010EEEE\x1b[m"))
	pics := s.placeholders(nil)
	if len(pics) != 1 {
		t.Fatalf("%d pieces, want 1: %+v", len(pics), pics)
	}
	p := pics[0]
	if p.X != 40 || p.Y != 20 || p.W != 30 || p.H != 40 || p.Img.Bounds().Dx() != 30 {
		t.Fatalf("piece %+v", p)
	}
	if again := s.placeholders(nil); again[0].Img != p.Img {
		t.Fatal("the same piece is a new picture each time")
	}
	// Only the second row, starting at its second column.
	s.text([]byte("\x1b[2;1H\x1b[2K\x1b[3;1H\x1b[2K\x1b[38;2;0;0;42m\U0010EEEE̍̍\U0010EEEE"))
	pics = s.placeholders(nil)
	if len(pics) != 1 || pics[0].W != 20 || pics[0].H != 20 || pics[0].Img.Bounds().Min != image.Pt(10, 20) {
		t.Fatalf("part of a row: %+v", pics)
	}
}

func TestITermMultipartAndGIF(t *testing.T) {
	s, _ := testSession(t)
	data := pngOf(t, 10, 20)
	s.osc([]byte("1337;MultipartFile=inline=1"))
	s.osc([]byte("1337;FilePart=" + data[:8]))
	s.osc([]byte("1337;FilePart=" + data[8:]))
	s.osc([]byte("1337;FileEnd"))
	if len(s.pics.byID) != 1 {
		t.Fatal("a file in parts was not shown")
	}

	g := &gif.GIF{}
	pal := color.Palette{color.Black, color.White}
	for i := range 3 {
		f := image.NewPaletted(image.Rect(0, 0, 4, 4), pal)
		f.SetColorIndex(i, 0, 1)
		g.Image, g.Delay = append(g.Image, f), append(g.Delay, 5)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	if n := gifFrames(buf.Bytes()); n != 3 {
		t.Fatalf("gifFrames = %d, want 3", n)
	}
	s.osc([]byte("1337;File=inline=1:" + base64.StdEncoding.EncodeToString(buf.Bytes())))
	animated := 0
	for _, pic := range s.pics.byID {
		if pic.anim != nil && len(pic.anim.frames) == 3 && pic.anim.run {
			animated++
		}
	}
	if animated != 1 {
		t.Fatal("the GIF does not play")
	}
}
