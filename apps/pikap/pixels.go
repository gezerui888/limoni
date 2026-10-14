package main

import (
	"image"
	"runtime"
	"sync"

	"github.com/thebanri/limoni/graphics"
)

// maxDeckPx bounds the picture's side in pixels. A Sixel picture is
// quantised and sent whole, so a record the size of a 4K screen would cost
// more per frame than the frame allows.
const maxDeckPx = 520

// pixelDeck is the deck drawn as an actual picture, for terminals that show
// Sixel: every pixel of the cell grid is sampled once, rather than two or
// six to a cell.
type pixelDeck struct {
	bufs [2]*image.RGBA // drawn in turn: the terminal compares pictures by pointer
	next int
	img  *image.RGBA // what is on show; nil when the deck is drawn in cells
	key  pixelKey
}

// pixelKey is everything that makes the picture what it is. While it stays
// the same the same picture is handed to the terminal, which then sends
// nothing at all.
type pixelKey struct {
	dv              deckView
	p               sceneParams
	cover           *art
	cellW, cellH    int
	topBg, bottomBg rgb
	help            bool
	valid           bool
}

// fitPixelDeck is fitDeck for a picture of at most maxDeckPx a side, kept
// in the middle of the room it was given.
func fitPixelDeck(x, y, w, h, cellW, cellH int) (deckView, bool) {
	mw, mh := max(1, maxDeckPx/cellW), max(1, maxDeckPx/cellH)
	if w > mw {
		x += (w - mw) / 2
		w = mw
	}
	if h > mh {
		y += (h - mh) / 2
		h = mh
	}
	return fitDeck(x, y, w, h, float64(cellW)/float64(cellH))
}

// draw returns the picture of the deck at dv, drawing it again only when
// something about it changed.
func (pd *pixelDeck) draw(s *scene, dv deckView, p sceneParams, cover *art, rows []rgb, cellW, cellH int, help bool) *image.RGBA {
	key := pixelKey{dv: dv, p: p, cover: cover, cellW: cellW, cellH: cellH, help: help, valid: true,
		topBg: rows[dv.y0], bottomBg: rows[dv.y0+dv.h-1]}
	if pd.img != nil && key == pd.key {
		return pd.img
	}
	pd.key = key
	w, h := dv.w*cellW, dv.h*cellH
	img := pd.bufs[pd.next]
	if img == nil || img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		img = image.NewRGBA(image.Rect(0, 0, w, h))
		pd.bufs[pd.next] = img
	} else {
		// Rewritten in place, so what is cached for it is the old picture.
		graphics.ForgetImage(img)
	}
	pd.next = 1 - pd.next

	// One unit of the record is R·cellW pixels across.
	s.begin(p, dv.R*float64(cellW))
	workers := min(runtime.GOMAXPROCS(0), h)
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for py := k; py < h; py += workers {
				bg := rows[dv.y0+py/cellH]
				row := img.Pix[py*img.Stride : py*img.Stride+w*4]
				for px := 0; px < w; px++ {
					u, v := dv.toUnits(float64(dv.x0)+(float64(px)+0.5)/float64(cellW),
						float64(dv.y0)+(float64(py)+0.5)/float64(cellH))
					c, _ := s.at(u, v, bg)
					row[px*4], row[px*4+1], row[px*4+2], row[px*4+3] = to8(c.r), to8(c.g), to8(c.b), 255
				}
			}
		}()
	}
	wg.Wait()
	pd.img = img
	return img
}

// forget says the deck is no longer drawn as a picture.
func (pd *pixelDeck) forget() {
	pd.img = nil
	pd.key = pixelKey{}
}
