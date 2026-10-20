//go:build unix

package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"github.com/thebanri/limoni/internal/overlay"
)

// testSession is a session on an 80×24 shadow screen with cells of 10×20,
// tracking pictures as with an overlay, without showing them. Its replies
// to the program come out of the returned file.
func testSession(t *testing.T) (*session, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	emu := vt.NewEmulator(80, 24)
	go io.Copy(io.Discard, emu)
	s := &session{ptmx: w, emu: emu, cols: 80, rows: 24, cellW: 10, cellH: 20, pixels: true,
		ov: new(overlay.Overlay)} // never shown: only so place tracks pictures
	s.pics = pictures{emu: emu, byID: map[int]*picture{}}
	return s, r
}

func replyOf(t *testing.T, r *os.File) string {
	t.Helper()
	r.SetReadDeadline(time.Now().Add(time.Second))
	b := make([]byte, 256)
	n, err := r.Read(b)
	if err != nil {
		t.Fatalf("no reply: %v", err)
	}
	return string(b[:n])
}

func pngOf(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// Programs find kitty graphics by asking about a 1×1 RGB picture.
func TestKittyAnswersTheProbe(t *testing.T) {
	s, r := testSession(t)
	s.apc([]byte("Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA"))
	if got := replyOf(t, r); got != "\x1b_Gi=31;OK\x1b\\" {
		t.Fatalf("reply %q", got)
	}
	s.apc([]byte("Gi=32,a=q,f=100;bm90IGEgcGljdHVyZQ=="))
	if got := replyOf(t, r); got != "\x1b_Gi=32;EINVAL:bad PNG\x1b\\" {
		t.Fatalf("reply to a bad picture %q", got)
	}
}

// A PNG in two chunks, shown as it arrives: kept by its id, and the cursor
// past its right edge on its last row.
func TestKittyChunksAndCursor(t *testing.T) {
	s, r := testSession(t)
	data := pngOf(t, 20, 40) // 2×2 cells
	cut := len(data) / 8 * 4
	s.apc([]byte("Ga=T,f=100,i=5,m=1;" + data[:cut]))
	s.apc([]byte("Gm=0;" + data[cut:]))
	if got := replyOf(t, r); got != "\x1b_Gi=5;OK\x1b\\" {
		t.Fatalf("reply %q", got)
	}
	if s.kitty.images[5] == nil || len(s.pics.byID) != 1 {
		t.Fatalf("image kept: %v, pictures shown: %d", s.kitty.images[5] != nil, len(s.pics.byID))
	}
	if at := s.emu.CursorPosition(); at.X != 2 || at.Y != 1 {
		t.Fatalf("cursor at %v, want 2,1", at)
	}
}

func TestKittyRawCompressedRGBA(t *testing.T) {
	s, _ := testSession(t)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	zw.Close()
	img, err := s.kittyDecode(map[string]string{"f": "32", "s": "2", "v": "1", "o": "z"}, []byte(base64.StdEncoding.EncodeToString(z.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.(*image.NRGBA).Pix; !bytes.Equal(got, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("pixels %v", got)
	}
	if _, err := s.kittyDecode(map[string]string{"f": "24", "s": "4", "v": "4"}, []byte("AAAA")); err == nil {
		t.Fatal("too little data for its size was taken")
	}
}

// A temporary file is deleted after reading only where kitty would delete it.
func TestKittyTemporaryFiles(t *testing.T) {
	ours, err := os.CreateTemp("", "tty-graphics-protocol-*")
	if err != nil {
		t.Fatal(err)
	}
	ours.WriteString("pixels")
	ours.Close()
	other := filepath.Join(t.TempDir(), "tty-graphics-protocol-x") // not in the temp dir itself
	os.WriteFile(other, []byte("pixels"), 0o600)
	for _, c := range []struct {
		path string
		gone bool
	}{{ours.Name(), true}, {other, false}} {
		data, err := kittyRead("t", c.path, 0, 0)
		if err != nil || string(data) != "pixels" {
			t.Fatalf("%s: %q, %v", c.path, data, err)
		}
		if _, err := os.Stat(c.path); os.IsNotExist(err) != c.gone {
			t.Errorf("%s: deleted %v, want %v", c.path, os.IsNotExist(err), c.gone)
		}
	}
	if _, err := kittyRead("f", "/dev/null", 0, 0); err == nil {
		t.Error("a device was read")
	}
	if _, err := kittyRead("s", "../etc/passwd", 0, 0); err == nil {
		t.Error("a shared memory name with a slash was read")
	}
}

func TestKittyPlacementsAndDeletes(t *testing.T) {
	s, _ := testSession(t)
	data := pngOf(t, 10, 20)
	s.apc([]byte("Ga=t,f=100,i=7,q=2;" + data))
	s.apc([]byte("Ga=p,i=7,p=1,q=2"))
	s.apc([]byte("Ga=p,i=7,p=1,q=2,C=1")) // the same placement again replaces it
	if n := len(s.pics.byID); n != 1 {
		t.Fatalf("%d pictures after placing one twice, want 1", n)
	}
	s.apc([]byte("Ga=p,i=7,p=2,q=2"))
	s.apc([]byte("Ga=d,d=i,i=7,p=1"))
	if n := len(s.pics.byID); n != 1 {
		t.Fatalf("%d pictures after deleting one of two, want 1", n)
	}
	s.apc([]byte("Ga=d"))
	if n := len(s.pics.byID); n != 0 || s.kitty.images[7] == nil {
		t.Fatalf("after d=a: %d pictures, image kept %v; want none, kept", n, s.kitty.images[7] != nil)
	}
	s.apc([]byte("Ga=d,d=A"))
	if len(s.kitty.images) != 0 {
		t.Fatal("d=A kept the image")
	}
}

func TestITermSizes(t *testing.T) {
	s, _ := testSession(t) // 800×480 pixels of screen
	for _, c := range []struct {
		args  map[string]string
		w, h  int
		wantW int
		wantH int
	}{
		{map[string]string{}, 200, 100, 200, 100},
		{map[string]string{"width": "10"}, 200, 100, 100, 50},
		{map[string]string{"width": "50%"}, 200, 100, 400, 200},
		{map[string]string{"width": "10px", "height": "10px", "preserveAspectRatio": "0"}, 200, 100, 10, 10},
		{map[string]string{"width": "100px", "height": "100px"}, 200, 100, 100, 50},
		{map[string]string{}, 1600, 100, 800, 50}, // never wider than the screen
	} {
		if w, h := s.itermSize(c.args, c.w, c.h); w != c.wantW || h != c.wantH {
			t.Errorf("%v on %d×%d: %d×%d, want %d×%d", c.args, c.w, c.h, w, h, c.wantW, c.wantH)
		}
	}
}

func TestITermInlineOnly(t *testing.T) {
	s, _ := testSession(t)
	data := pngOf(t, 10, 20)
	if !s.osc([]byte("1337;File=name=eA==:"+data)) || len(s.pics.byID) != 0 {
		t.Fatal("a download (no inline=1) was shown, or passed on")
	}
	if !s.osc([]byte("1337;File=inline=1:"+data)) || len(s.pics.byID) != 1 {
		t.Fatal("an inline picture was not shown")
	}
	if s.osc([]byte("0;a title")) {
		t.Fatal("a title was taken")
	}
}

// Transparent padding (chafa fills whole cells with it) is cut off and the
// rest placed where it was; nothing at all is not shown.
func TestOpaqueCutsTransparentEdges(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 20))
	for y := 2; y < 15; y++ {
		for x := 1; x < 9; x++ {
			img.SetNRGBA(x, y, color.NRGBA{9, 9, 9, 255})
		}
	}
	got, dx, dy := opaque(img)
	if b := got.Bounds(); b.Dx() != 8 || b.Dy() != 13 || dx != 1 || dy != 2 {
		t.Fatalf("kept %v at %d,%d, want 8×13 at 1,2", b, dx, dy)
	}
	if got, _, _ := opaque(image.NewNRGBA(image.Rect(0, 0, 4, 4))); got != nil {
		t.Fatal("a fully transparent picture was kept")
	}
}

// POSIX shared memory names start with a slash (mpv's do); the memory is
// read and unlinked, as kitty does.
func TestKittySharedMemory(t *testing.T) {
	name := "termimg-test-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile("/dev/shm/"+name, []byte("pixels"), 0o600); err != nil {
		t.Skip("no /dev/shm:", err)
	}
	data, err := kittyRead("s", "/"+name, 0, 0)
	if err != nil || string(data) != "pixels" {
		t.Fatalf("read %q, %v", data, err)
	}
	if _, err := os.Stat("/dev/shm/" + name); !os.IsNotExist(err) {
		t.Fatal("the shared memory was left behind")
	}
}

// m=1 means chunks only for a direct transmission: mpv sends it with every
// shared-memory frame and never a last chunk.
func TestKittyIgnoresChunksOutsideDirect(t *testing.T) {
	s, _ := testSession(t)
	name := "termimg-test-m-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile("/dev/shm/"+name, []byte{1, 2, 3}, 0o600); err != nil {
		t.Skip("no /dev/shm:", err)
	}
	s.apc([]byte("Ga=T,t=s,f=24,s=1,v=1,C=1,q=2,m=1;" + base64.StdEncoding.EncodeToString([]byte(name))))
	if len(s.pics.byID) != 1 || s.kitty.first != nil {
		t.Fatalf("%d pictures, waiting for chunks %v; want one shown at once", len(s.pics.byID), s.kitty.first != nil)
	}
}
