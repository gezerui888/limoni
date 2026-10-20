package overlay

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"net"
	"os"
	"slices"
	"testing"
)

func TestDisplayNumber(t *testing.T) {
	for in, want := range map[string]string{":0": "0", ":1.0": "1", ":12": "12"} {
		if got, err := displayNumber(in); err != nil || got != want {
			t.Errorf("displayNumber(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "localhost:10.0", ":", ":x"} {
		if _, err := displayNumber(in); err == nil {
			t.Errorf("displayNumber(%q) accepted a display that is not local", in)
		}
	}
}

func TestParseXauth(t *testing.T) {
	entry := func(number, name, data string) []byte {
		var b []byte
		b = binary.BigEndian.AppendUint16(b, 256)
		for _, f := range []string{"host", number, name, data} {
			b = binary.BigEndian.AppendUint16(b, uint16(len(f)))
			b = append(b, f...)
		}
		return b
	}
	file := append(entry("1", "MIT-MAGIC-COOKIE-1", "wrong"), entry("0", "MIT-MAGIC-COOKIE-1", "cookie")...)
	if name, data := parseXauth(file, "0"); name != "MIT-MAGIC-COOKIE-1" || string(data) != "cookie" {
		t.Fatalf("got %q %q", name, data)
	}
	if name, _ := parseXauth(file[:len(file)-3], "0"); name != "" {
		t.Fatalf("a truncated entry was used: %q", name)
	}
}

func TestParseSetup(t *testing.T) {
	b := make([]byte, 32+4+2*8+40)
	binary.LittleEndian.PutUint32(b[4:], 0x400000)  // id base
	binary.LittleEndian.PutUint32(b[8:], 0x1fffff)  // id mask
	binary.LittleEndian.PutUint16(b[16:], 3)        // vendor length, padded to 4
	binary.LittleEndian.PutUint16(b[18:], 65535)    // max request, in words
	b[21] = 2                                       // formats
	copy(b[36:], []byte{1, 1, 32, 0, 0, 0, 0, 0})   // depth 1
	copy(b[44:], []byte{24, 32, 32, 0, 0, 0, 0, 0}) // depth 24, 32 bpp
	screen := 52
	binary.LittleEndian.PutUint32(b[screen:], 0x3c5)
	var x xconn
	if err := x.parseSetup(b); err != nil {
		t.Fatal(err)
	}
	if x.root != 0x3c5 || x.maxReq != 4*65535 || !x.deep[24] || x.deep[1] {
		t.Fatalf("parsed root %#x maxReq %d deep %v", x.root, x.maxReq, x.deep)
	}
	if id := x.id(); id != 0x400001 {
		t.Fatalf("first id %#x", id)
	}
}

// requests reads X11 requests off the other end of a pipe and reports the
// ones the tests look at; opcode 127, which X11 leaves for extensions, marks
// the end of what a test sent.
func requests(c net.Conn) <-chan string {
	out := make(chan string, 1024)
	go func() {
		defer close(out)
		head := make([]byte, 4)
		for {
			if _, err := io.ReadFull(c, head); err != nil {
				return
			}
			body := make([]byte, 4*int(binary.LittleEndian.Uint16(head[2:]))-4)
			if _, err := io.ReadFull(c, body); err != nil {
				return
			}
			i16 := func(off int) int { return int(int16(binary.LittleEndian.Uint16(body[off:]))) }
			switch head[0] {
			case opCreateWindow:
				out <- fmt.Sprintf("create in %#x at %d,%d", binary.LittleEndian.Uint32(body[4:]), i16(8), i16(10))
			case opConfigureWindow:
				out <- fmt.Sprintf("move to %d,%d", i16(8), i16(12))
			case opDestroyWindow:
				out <- "destroy"
			case opCreatePixmap:
				out <- "upload"
			case opChangeWindowAttributes:
				out <- "background"
			case opFreePixmap:
				out <- "free"
			case 127:
				out <- "end"
			}
		}
	}()
	return out
}

// pipeOverlay is an Overlay talking to a pipe, its terminal window 1000×500
// around a grid of 800×400.
func pipeOverlay(t *testing.T) (*Overlay, <-chan string) {
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close() })
	o := &Overlay{x: &xconn{c: a, mask: 0xffff, maxReq: 1 << 18, term: 0x77, depth: 24}, pix: make(map[picKey]*pixmap), maxBytes: maxPixBytes}
	o.x.size.Store(1000<<16 | 500)
	return o, requests(b)
}

// shower returns a function that shows pics and reports what was sent.
func shower(t *testing.T, o *Overlay, got <-chan string) func(...Picture) []string {
	return func(pics ...Picture) []string {
		if err := o.Show(pics, 800, 400); err != nil {
			t.Fatal(err)
		}
		o.x.c.Write([]byte{127, 0, 1, 0})
		var all []string
		for s := range got {
			if s == "end" {
				break
			}
			all = append(all, s)
		}
		return all
	}
}

// An animation's frames follow each other in one window, as its background;
// on the next loop the pictures are already in the server.
func TestShowPlaysFramesInOneWindow(t *testing.T) {
	o, got := pipeOverlay(t)
	show := shower(t, o, got)
	frame := func(c uint8) Picture {
		return Picture{Img: image.NewUniform(color.RGBA{c, 0, 0, 255}), X: 0, Y: 0, W: 4, H: 2}
	}
	f1, f2 := frame(1), frame(2)
	if r := show(f1); !slices.Equal(r, []string{"upload", "create in 0x77 at 100,50"}) {
		t.Fatalf("frame 1 sent %q", r)
	}
	if r := show(f2); !slices.Equal(r, []string{"upload", "background"}) {
		t.Fatalf("frame 2 sent %q, want an upload and a new background", r)
	}
	if r := show(f1); !slices.Equal(r, []string{"background"}) {
		t.Fatalf("frame 1 again sent %q, want only a new background", r)
	}
	if len(o.wins) != 1 {
		t.Fatalf("%d windows for one animation", len(o.wins))
	}
}

// A picture of another shape than its cells gets a window of its own shape,
// centred, not one filling the cells with bands of its corner colour.
func TestShowFitsTheWindowToThePicture(t *testing.T) {
	o, got := pipeOverlay(t)
	show := shower(t, o, got)
	wide := image.NewRGBA(image.Rect(0, 0, 40, 10)) // 4:1 in cells of 40×40
	if r := show(Picture{Img: wide, X: 0, Y: 0, W: 40, H: 40}); !slices.Equal(r, []string{"upload", "create in 0x77 at 100,65"}) {
		t.Fatalf("sent %q, want the window 15 px down", r)
	}
	if w := o.wins[0]; w.key.w != 40 || w.key.h != 10 {
		t.Fatalf("window %d×%d, want 40×10", w.key.w, w.key.h)
	}
}

// Past maxPixBytes the least recently shown pictures are freed, never the
// one on screen.
func TestShowEvictsOldPictures(t *testing.T) {
	o, got := pipeOverlay(t)
	show := shower(t, o, got)
	side := 2048 // 16 MiB a picture: nine of them pass 128 MiB
	var first image.Image
	for i := range 9 {
		img := image.NewUniform(color.RGBA{uint8(i), 0, 0, 255})
		if i == 0 {
			first = img
		}
		r := show(Picture{Img: img, W: side, H: side})
		if i < 8 && slices.Contains(r, "free") || i == 8 && !slices.Contains(r, "free") {
			t.Fatalf("picture %d: sent %q", i, r)
		}
	}
	if _, ok := o.pix[picKey{first, side, side}]; ok || o.pixBytes > maxPixBytes {
		t.Fatalf("the oldest picture was kept, %d bytes held", o.pixBytes)
	}
}

func TestShowReusesMovesAndRemovesWindows(t *testing.T) {
	o, got := pipeOverlay(t)
	pic := Picture{Img: image.NewUniform(color.RGBA{255, 0, 0, 255}), X: 10, Y: 20, W: 4, H: 2}

	show := shower(t, o, got)

	if r := show(pic); !slices.Equal(r, []string{"upload", "create in 0x77 at 110,70"}) {
		t.Fatalf("first show sent %q", r)
	}
	pic.X = 30
	if r := show(pic); !slices.Equal(r, []string{"move to 130,70"}) {
		t.Fatalf("a moved picture was not just moved: %q", r)
	}
	if r := show(pic); len(r) != 0 {
		t.Fatalf("an unchanged frame sent %q", r)
	}
	pic.W = 8 // a resized picture: the new window is made before the old goes
	if r := show(pic); !slices.Equal(r, []string{"upload", "create in 0x77 at 130,70", "destroy"}) {
		t.Fatalf("a resized picture sent %q", r)
	}
	if r := show(); !slices.Equal(r, []string{"destroy"}) || len(o.wins) != 0 {
		t.Fatalf("a removed picture: sent %q, %d windows left", r, len(o.wins))
	}
	o.x.size.Store(0) // the terminal's window was destroyed
	if r := show(pic); len(r) != 0 {
		t.Fatalf("pictures were made for a window that is gone: %q", r)
	}
}

// The terminal window's size reaches CellSize through a ConfigureNotify, and
// a size the terminal's cells cannot account for (the window not resized
// yet) leaves the last good cell size.
func TestCellSizeFromTheTerminalWindow(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	o := &Overlay{x: &xconn{c: a, term: 0x42}}
	go o.x.read()
	configure := func(win uint32, w, h uint16) {
		ev := make([]byte, 32)
		ev[0] = 22
		binary.LittleEndian.PutUint32(ev[8:], win)
		binary.LittleEndian.PutUint16(ev[20:], w)
		binary.LittleEndian.PutUint16(ev[22:], h)
		b.Write(ev)
		b.Write(make([]byte, 32)) // an error packet: by the time it is read, ev was handled
	}

	configure(0x42, 1920, 1048) // Alacritty: 192×45 cells of 10×23, 13 px of padding
	if w, h := o.CellSize(192, 45); w != 10 || h != 23 {
		t.Fatalf("CellSize = %d×%d, want 10×23", w, h)
	}
	configure(0x99, 300, 300) // some other window
	if w, h := o.CellSize(192, 45); w != 10 || h != 23 {
		t.Fatalf("another window's size was taken: %d×%d", w, h)
	}
	if w, h := o.CellSize(150, 45); w != 10 || h != 23 { // 1920/150 leaves 120 px: stale
		t.Fatalf("a stale window size was taken: %d×%d", w, h)
	}
	configure(0x42, 1500, 1048)
	if w, h := o.CellSize(150, 45); w != 10 || h != 23 {
		t.Fatalf("after the resize: %d×%d", w, h)
	}
	configure(0x42, 1500, 1048-45) // a smaller font: 150×45 cells of 10×22
	if w, h := o.CellSize(150, 45); w != 10 || h != 22 {
		t.Fatalf("after a font change: %d×%d", w, h)
	}
}

func TestBGRX(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{10, 20, 30, 255})
	img.Set(1, 0, color.RGBA{40, 50, 60, 255})
	px := bgrx(img, 2, 1, color.RGBA{A: 255})
	if want := []byte{30, 20, 10, 255, 60, 50, 40, 255}; string(px) != string(want) {
		t.Fatalf("bgrx = %v, want %v", px, want)
	}
	// Transparent pixels show the background, half transparent ones half.
	img.Set(0, 0, color.NRGBA{200, 0, 0, 128})
	img.Set(1, 0, color.RGBA{})
	px = bgrx(img, 2, 1, color.RGBA{0, 0, 200, 255})
	if want := []byte{99, 0, 100, 255, 200, 0, 0, 255}; string(px) != string(want) {
		t.Fatalf("over a background: %v, want %v", px, want)
	}
	// A translucent terminal's background (premultiplied, alpha 230) shows
	// through transparent pixels as see-through as the terminal draws it.
	px = bgrx(img, 2, 1, color.RGBA{0, 0, 180, 230})
	if want := []byte{89, 0, 100, 242, 180, 0, 0, 230}; string(px) != string(want) {
		t.Fatalf("over a translucent background: %v, want %v", px, want)
	}
}

// SetBackground keeps the opacity measured from the terminal's window.
func TestSetBackgroundKeepsAlpha(t *testing.T) {
	o := &Overlay{bg: color.RGBA{A: 230}}
	o.SetBackground(color.RGBA{32, 36, 53, 255})
	if want := (color.RGBA{28, 32, 47, 230}); o.bg != want {
		t.Fatalf("bg %v, want %v", o.bg, want)
	}
}

func TestAncestors(t *testing.T) {
	if s := ancestors(1); len(s) != 0 {
		t.Fatalf("ancestors(1) = %v", s)
	}
	if s := ancestors(os.Getpid()); len(s) < 2 || s[0] != os.Getpid() || s[1] != os.Getppid() {
		t.Fatalf("own ancestors %v do not start with this process and its parent", s)
	}
}

// With no cache a picture's copy in the server goes as soon as it is off
// screen, as a video frame should.
func TestShowWithoutCacheFreesAtOnce(t *testing.T) {
	o, got := pipeOverlay(t)
	o.SetCache(0)
	show := shower(t, o, got)
	frame := func(c uint8) Picture {
		return Picture{Img: image.NewUniform(color.RGBA{c, 0, 0, 255}), W: 4, H: 2}
	}
	show(frame(1))
	if r := show(frame(2)); !slices.Equal(r, []string{"upload", "background", "free"}) {
		t.Fatalf("the next frame sent %q, want the old picture freed", r)
	}
	if len(o.pix) != 1 {
		t.Fatalf("%d pictures kept, want only the one on screen", len(o.pix))
	}
}

// Trim frees what is kept but not on screen, and leaves the cache as it was.
func TestTrimFreesPicturesOffScreen(t *testing.T) {
	o, got := pipeOverlay(t)
	show := shower(t, o, got)
	frame := func(c uint8) Picture {
		return Picture{Img: image.NewUniform(color.RGBA{c, 0, 0, 255}), W: 4, H: 2}
	}
	show(frame(1))
	show(frame(2))
	if len(o.pix) != 2 {
		t.Fatalf("%d pictures kept before Trim, want 2", len(o.pix))
	}
	if err := o.Trim(); err != nil {
		t.Fatal(err)
	}
	if len(o.pix) != 1 || o.maxBytes != maxPixBytes {
		t.Fatalf("after Trim: %d pictures, cache %d", len(o.pix), o.maxBytes)
	}
	o.x.c.Write([]byte{127, 0, 1, 0})
	if r := <-got; r != "free" {
		t.Fatalf("Trim sent %q, want a free", r)
	}
}
