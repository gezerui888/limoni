package main

import (
	"math"
	"testing"
	"time"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
	"github.com/thebanri/limoni/graphics"
)

func pixelUI(t testing.TB) (*ui, *buffer.Buffer) {
	u := newUI(&fakeSource{})
	u.glyph, u.pixels, u.cellW, u.cellH = sixelPixels, true, 10, 20
	b := buffer.NewBuffer(cell.Rect{Width: 120, Height: 40})
	u.tick(time.Now())
	return u, b
}

func TestSixelDeckIsAPicture(t *testing.T) {
	u, b := pixelUI(t)
	u.render(b)
	if u.pixImg == nil {
		t.Fatal("no picture in Sixel mode")
	}
	if got, want := u.pixImg.Bounds().Dx(), u.dv.w*u.cellW; got != want {
		t.Fatalf("picture is %d px wide, the deck is %d", got, want)
	}
	if u.pixImg.Bounds().Dx() > maxDeckPx+u.cellW || u.pixImg.Bounds().Dy() > maxDeckPx+u.cellH {
		t.Fatalf("picture %v is over the size limit", u.pixImg.Bounds())
	}
	first := u.pixImg
	u.render(b)
	if u.pixImg != first {
		t.Fatal("an unchanged deck was drawn again; the terminal would resend it")
	}
	u.angle += 0.1
	u.render(b)
	if u.pixImg == first {
		t.Fatal("a turned record kept the old picture")
	}
	u.pixels = false
	u.render(b)
	if u.pixImg != nil {
		t.Fatal("a terminal without Sixel was given a picture")
	}
}

// In cells of 6×16 rather than 1×2 the record was an ellipse. In pixels a
// unit of the record is as far down as across, and the cells under the
// picture are RuneImage, so the diff writes no blanks over it.
func TestSixelDeckIsRoundAndLeftAlone(t *testing.T) {
	u, b := pixelUI(t)
	u.cellW, u.cellH = 6, 16
	u.render(b)
	if across, down := u.dv.R*6, u.dv.Ry*16; math.Abs(across-down) > 1e-9 {
		t.Fatalf("a unit of the record is %.2f px across and %.2f px down", across, down)
	}
	W := int(b.Area.Width)
	for y := u.dv.y0; y < u.dv.y0+u.dv.h; y++ {
		for x := u.dv.x0; x < u.dv.x0+u.dv.w; x++ {
			if c := b.Content[y*W+x].Content; c != cell.RuneImage {
				t.Fatalf("cell %d,%d under the picture is %q", x, y, c)
			}
		}
	}
}

func TestGlyphCycle(t *testing.T) {
	u := newUI(&fakeSource{})
	var got []glyphStyle
	for range 3 {
		u.cycleGlyph()
		got = append(got, u.glyph)
	}
	if got[0] != asciiGlyphs || got[1] != halfBlocks || got[2] != asciiGlyphs {
		t.Fatalf("without Sixel: %v", got)
	}
	u.glyph, u.pixels = halfBlocks, true
	got = got[:0]
	for range 3 {
		u.cycleGlyph()
		got = append(got, u.glyph)
	}
	if got[0] != asciiGlyphs || got[1] != sixelPixels || got[2] != halfBlocks {
		t.Fatalf("with Sixel: %v", got)
	}
}

func BenchmarkSixelFrame(b *testing.B) {
	u, buf := pixelUI(b)
	for i := 0; i < b.N; i++ {
		u.angle += 0.01
		u.render(buf)
		graphics.EncodeSixel(u.pixImg, uint16(u.dv.w), uint16(u.dv.h), 10, 20, false)
	}
}
