package main

import (
	"image"
	"image/color"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

// recorder is a sink that keeps what it gets; it takes XTSMGRAPHICS,
// iTerm2 pictures and kitty commands.
type recorder struct {
	out    strings.Builder
	sixels []string
	strs   []string
}

func (r *recorder) text(b []byte)       { r.out.Write(b) }
func (r *recorder) csi(seq []byte) bool { return seq[len(seq)-1] == 'S' }
func (r *recorder) sixel(data []byte)   { r.sixels = append(r.sixels, string(data)) }
func (r *recorder) osc(body []byte) bool {
	r.strs = append(r.strs, "osc "+string(body))
	return strings.HasPrefix(string(body), "1337;")
}
func (r *recorder) apc(body []byte) bool {
	r.strs = append(r.strs, "apc "+string(body))
	return len(body) > 0 && body[0] == 'G'
}

// Text and other sequences pass through unchanged, a sixel picture and an
// answered query do not, whatever the reads are cut into.
func TestParserSplitsOutput(t *testing.T) {
	in := "a\x1b[31mb\x1bPq#0;2;0;0;100#0~\x1b\\c\x1bP1$r0m\x1b\\d\x1b[?2;1;0Se\x1b\x1b7f" +
		"\x1b]0;title\ag\x1b]1337;File=inline=1:AAAA\ah\x1b_Ga=q;AAAA\x1b\\i\x1b_other\x1b\\j"
	for _, step := range []int{1, 3, len(in)} {
		r := &recorder{}
		var p parser
		for i := 0; i < len(in); i += step {
			p.feed([]byte(in[i:min(i+step, len(in))]), r)
		}
		if want := "a\x1b[31mbc\x1bP1$r0m\x1b\\de\x1b\x1b7f\x1b]0;title\aghi\x1b_other\x1b\\j"; r.out.String() != want {
			t.Errorf("step %d: text %q, want %q", step, r.out.String(), want)
		}
		if len(r.sixels) != 1 || r.sixels[0] != "#0;2;0;0;100#0~" {
			t.Errorf("step %d: sixels %q", step, r.sixels)
		}
		if want := []string{"osc 0;title", "osc 1337;File=inline=1:AAAA", "apc Ga=q;AAAA", "apc other"}; !slices.Equal(r.strs, want) {
			t.Errorf("step %d: strings %q", step, r.strs)
		}
	}
}

func TestDA1GetsSixel(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"answer", "\x1b[?62;22c", "\x1b[?62;22;4c"},
		{"already sixel", "\x1b[?62;4;22c", "\x1b[?62;4;22c"},
		{"between keys", "a\x1b[?6cb", "a\x1b[?6;4cb"},
		{"DA2 is not DA1", "\x1b[>1;2c", "\x1b[>1;2c"},
		{"a mode report", "\x1b[?2026;2$y", "\x1b[?2026;2$y"},
		{"Esc key", "\x1b", "\x1b"},
		{"arrow", "\x1b[A", "\x1b[A"},
	}
	for _, c := range cases {
		var d da1
		if got := string(d.filter([]byte(c.in))); got != c.want {
			t.Errorf("%s: %q → %q, want %q", c.name, c.in, got, c.want)
		}
	}
	var d da1
	first := d.filter([]byte("x\x1b[?6"))
	second := d.filter([]byte("2c"))
	if string(first) != "x" || string(second) != "\x1b[?62;4c" {
		t.Errorf("an answer split over two reads: %q then %q", first, second)
	}
}

func newPictures(w, h int) (*vt.Emulator, *pictures) {
	emu := vt.NewEmulator(w, h)
	go io.Copy(io.Discard, emu)
	return emu, &pictures{emu: emu, byID: map[int]*picture{}}
}

func only(t *testing.T, p *pictures) *picture {
	t.Helper()
	p.update()
	if len(p.byID) != 1 {
		t.Fatalf("%d pictures, want 1", len(p.byID))
	}
	for _, pic := range p.byID {
		return pic
	}
	return nil
}

func TestPicturesFollowTheScreen(t *testing.T) {
	emu, p := newPictures(20, 6)
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	emu.WriteString("\x1b[2;3H")
	p.add(img, 2, 1, 4, 2, false)
	if pic := only(t, p); pic.col != 2 || pic.top != 1 {
		t.Fatalf("at %d,%d, want 2,1", pic.col, pic.top)
	}
	if at := emu.CursorPosition(); at.X != 2 || at.Y != 1 {
		t.Fatalf("marking moved the cursor to %v", at)
	}

	emu.WriteString("\x1b[5;1Hsomewhere else")
	only(t, p)

	emu.WriteString("\x1b[6;1H\n\n") // two lines scroll off the top
	if pic := only(t, p); pic.top != -1 {
		t.Fatalf("after scrolling two lines: top %d, want -1", pic.top)
	}

	emu.WriteString("\x1b[1;4H ") // a space over the picture's visible row
	p.update()
	if len(p.byID) != 0 {
		t.Fatal("a picture written over was kept")
	}
	if strings.ContainsRune(emu.String(), mark(1, 1)) {
		t.Fatal("its marks were left behind")
	}
}

func TestPicturesWaitForTheirScreen(t *testing.T) {
	emu, p := newPictures(20, 6)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	p.add(img, 0, 0, 2, 1, false)
	emu.WriteString("\x1b[?1049h") // a full-screen program starts
	only(t, p)
	emu.WriteString("hello\x1b[?1049l")
	if pic := only(t, p); pic.col != 0 || pic.top != 0 {
		t.Fatalf("back on the main screen at %d,%d", pic.col, pic.top)
	}
}

func TestDecodeSixel(t *testing.T) {
	// 3x6: red column via RGB, then green (HLS hue 240 = green) repeated twice, with a band break.
	img := decodeSixel([]byte(`"1;1;3;12#1;2;100;0;0#2;1;240;50;100#1~#2!2~-#1A`))
	if b := img.Bounds(); b.Dx() != 3 || b.Dy() != 12 {
		t.Fatalf("size %v", b)
	}
	want := map[[2]int]color.NRGBA{
		{0, 0}: {255, 0, 0, 255}, {0, 5}: {255, 0, 0, 255},
		{1, 0}: {0, 255, 0, 255}, {2, 5}: {0, 255, 0, 255},
		{0, 7}: {255, 0, 0, 255}, // 'A' sets bit 1 of the second band
		{0, 6}: {}, {1, 7}: {},
	}
	for p, c := range want {
		if got := img.NRGBAAt(p[0], p[1]); got != c {
			t.Errorf("pixel %v = %v, want %v", p, got, c)
		}
	}
}

// Text over part of a kitty picture leaves it, unlike a sixel one; clearing
// the screen takes both.
func TestStickyPicturesStayUnderText(t *testing.T) {
	emu, p := newPictures(20, 6)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	p.add(img, 2, 1, 4, 2, true)
	p.add(img, 10, 1, 4, 2, false)
	emu.WriteString("\x1b[2;3Hxx\x1b[2;11Hxx") // over the first cells of each
	p.update()
	if len(p.byID) != 1 {
		t.Fatalf("%d pictures, want only the sticky one", len(p.byID))
	}
	for _, pic := range p.byID {
		if !pic.sticky || pic.col != 2 || pic.top != 1 {
			t.Fatalf("kept %+v", pic)
		}
	}
	emu.WriteString("\x1b[2J")
	p.update()
	if len(p.byID) != 0 {
		t.Fatal("a cleared screen kept a picture")
	}
}

// CSI 0 C moves the cursor one column, as in the real terminal.
func TestZeroCountsAsOne(t *testing.T) {
	s, _ := testSession(t)
	s.parse.feed([]byte("\x1b[3;3H\x1b[0C\x1b[00B"), s)
	if at := s.emu.CursorPosition(); at.X != 3 || at.Y != 3 {
		t.Fatalf("cursor at %v, want 3,3", at)
	}
	if got := string(s.buf); got != "\x1b[3;3H\x1b[0C\x1b[00B" {
		t.Fatalf("passed on %q", got)
	}
}

// After a sixel picture the cursor is on its last row, as in xterm; text
// there cuts that row off and leaves the rows above.
func TestSixelCursorAndCut(t *testing.T) {
	s, _ := testSession(t)
	s.parse.feed([]byte("\x1b[3;5H\x1bP0;1;0q\"1;1;20;45#1;2;100;0;0#1!20~-!20~-!20~-!20~-!20~-!20~-!20~-!20~\x1b\\"), s)
	if at := s.emu.CursorPosition(); at.X != 4 || at.Y != 4 {
		t.Fatalf("cursor at %v, want its last row 4,4", at) // 45px in 20px cells: rows 2, 3, 4
	}
	pic := only(t, &s.pics)
	if pic.top != 2 || pic.rows != 3 {
		t.Fatalf("at top %d, %d rows", pic.top, pic.rows)
	}
	s.parse.feed([]byte("\r$ ls -la"), s) // over columns 4 and 5
	if pic = only(t, &s.pics); pic.rows != 2 || pic.top != 2 {
		t.Fatalf("after a prompt on its last row: top %d, %d rows, want 2 rows", pic.top, pic.rows)
	}
	if strings.ContainsRune(s.emu.String(), mark(s.pics.idOf(pic), 2)) {
		t.Fatal("the cut row's marks were left behind")
	}
	if got := pic.cropped(2 * s.cellH).Bounds().Dy(); got != 40 {
		t.Fatalf("cropped to %d pixel rows, want 40", got)
	}
	s.parse.feed([]byte("\x1b[3;5Hx"), s)
	s.pics.update()
	if len(s.pics.byID) != 0 {
		t.Fatal("written over in its first row, it was kept")
	}
}
