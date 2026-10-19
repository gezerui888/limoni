package main

import (
	"fmt"
	"image"
	"image/color"

	"github.com/thebanri/limoni/graphics"
)

// drawBlocks appends img drawn in half blocks over cols×rows cells at col,
// top: two pixels a cell, the upper in the foreground colour of ▀, the lower
// in the background. Rows off the screen are skipped; a transparent pixel
// leaves the terminal's own background.
func drawBlocks(b []byte, img image.Image, col, top, cols, rows, screenRows int) []byte {
	if cols <= 0 || rows <= 0 {
		return b
	}
	px := graphics.ResizeImage(img, cols, rows*2)
	o := px.Bounds().Min
	at := func(x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(px.At(o.X+x, o.Y+y)).(color.NRGBA)
	}
	for r := range rows {
		y := top + r
		if y < 0 || y >= screenRows {
			continue
		}
		b = cup(b, col, y)
		for x := range cols {
			hi, lo := at(x, 2*r), at(x, 2*r+1)
			switch {
			case hi.A < 128 && lo.A < 128:
				b = append(b, "\x1b[0m "...)
			case lo.A < 128:
				b = fmt.Appendf(b, "\x1b[0;38;2;%d;%d;%dm▀", hi.R, hi.G, hi.B)
			case hi.A < 128:
				b = fmt.Appendf(b, "\x1b[0;38;2;%d;%d;%dm▄", lo.R, lo.G, lo.B)
			default:
				b = fmt.Appendf(b, "\x1b[0;38;2;%d;%d;%d;48;2;%d;%d;%dm▀", hi.R, hi.G, hi.B, lo.R, lo.G, lo.B)
			}
		}
		b = append(b, "\x1b[0m"...)
	}
	return b
}
