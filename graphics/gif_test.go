package graphics

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"
)

var (
	gifRed  = color.RGBA{255, 0, 0, 255}
	gifBlue = color.RGBA{0, 0, 255, 255}
)

func gifFrame(r image.Rectangle, c color.Color) *image.Paletted {
	f := image.NewPaletted(r, color.Palette{color.Transparent, gifRed, gifBlue})
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			f.Set(x, y, c)
		}
	}
	return f
}

func encodeGIF(t *testing.T, g *gif.GIF) *Animation {
	t.Helper()
	g.Config = image.Config{Width: 4, Height: 4}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	a, err := DecodeGIF(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// A frame that changes a corner keeps the rest of the one before; one that
// asks to be cleared leaves nothing behind; one that asks to be undone
// leaves what was there before it.
func TestDecodeGIFComposesFrames(t *testing.T) {
	a := encodeGIF(t, &gif.GIF{
		Image: []*image.Paletted{
			gifFrame(image.Rect(0, 0, 4, 4), gifRed),
			gifFrame(image.Rect(0, 0, 2, 2), gifBlue),
			gifFrame(image.Rect(2, 2, 4, 4), gifBlue),
			gifFrame(image.Rect(0, 2, 1, 3), gifBlue),
		},
		Delay:    []int{0, 0, 0, 0},
		Disposal: []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious, gif.DisposalNone},
	})
	at := func(frame, x, y int) color.RGBA {
		return a.Frames[frame].(*image.RGBA).RGBAAt(x, y)
	}
	if at(1, 0, 0) != gifBlue || at(1, 3, 3) != gifRed {
		t.Errorf("frame 2: corner %v, rest %v; want blue over red", at(1, 0, 0), at(1, 3, 3))
	}
	if at(2, 0, 0) != (color.RGBA{}) || at(2, 3, 3) != gifBlue || at(2, 3, 0) != gifRed {
		t.Errorf("frame 3: cleared corner %v, new corner %v, rest %v", at(2, 0, 0), at(2, 3, 3), at(2, 3, 0))
	}
	if at(3, 3, 3) != gifRed || at(3, 0, 2) != gifBlue {
		t.Errorf("frame 4: undone corner %v, new pixel %v", at(3, 3, 3), at(3, 0, 2))
	}
	if a.Frames[0] == a.Frames[1] {
		t.Error("two frames are the same image value; the caches would not tell them apart")
	}
}

func TestAnimationFrameTiming(t *testing.T) {
	a := encodeGIF(t, &gif.GIF{
		Image: []*image.Paletted{gifFrame(image.Rect(0, 0, 4, 4), gifRed), gifFrame(image.Rect(0, 0, 4, 4), gifBlue)},
		Delay: []int{5, 1}, // 50 ms, then 10 ms shown as browsers do: 100 ms
	})
	if a.Delays[0] != 50*time.Millisecond || a.Delays[1] != 100*time.Millisecond {
		t.Fatalf("delays %v", a.Delays)
	}
	for _, c := range []struct {
		at    time.Duration
		frame int
	}{{0, 0}, {49 * time.Millisecond, 0}, {50 * time.Millisecond, 1}, {149 * time.Millisecond, 1}, {150 * time.Millisecond, 0}, {-1, 1}} {
		if got := a.Frame(c.at); got != a.Frames[c.frame] {
			t.Errorf("Frame(%v) is not frame %d", c.at, c.frame)
		}
	}
	if n := testing.AllocsPerRun(100, func() { a.Frame(70 * time.Millisecond) }); n != 0 {
		t.Errorf("Frame allocates %v times", n)
	}
}
