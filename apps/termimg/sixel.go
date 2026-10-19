package main

import (
	"image"
	"image/color"
	"math"
)

// maxSide bounds the canvas: sixel data may come from a remote host, so don't trust its sizes.
const maxSide = 8192

// vt340 is the default sixel palette, in percent RGB.
var vt340 = [16][3]int{
	{0, 0, 0}, {20, 20, 80}, {80, 13, 13}, {20, 80, 20},
	{80, 20, 80}, {20, 80, 80}, {80, 80, 20}, {53, 53, 53},
	{26, 26, 26}, {33, 33, 60}, {60, 26, 26}, {33, 60, 33},
	{60, 33, 60}, {33, 60, 60}, {60, 60, 33}, {80, 80, 80},
}

// decodeSixel decodes sixel data (everything after the 'q') into an image.
// Pixels no sixel touched stay transparent so the terminal background shows through.
func decodeSixel(data []byte) *image.NRGBA {
	var pal [256]color.NRGBA
	for i, c := range vt340 {
		pal[i] = pct(c[0], c[1], c[2])
	}
	img := image.NewNRGBA(image.Rect(0, 0, 0, 0))
	w, h, x, y, cur := 0, 0, 0, 0, 0

	grow := func(nw, nh int) {
		b := img.Bounds()
		if nw <= b.Dx() && nh <= b.Dy() {
			return
		}
		next := image.NewNRGBA(image.Rect(0, 0, min(maxSide, max(nw, 2*b.Dx())), min(maxSide, max(nh, 2*b.Dy()))))
		for row := 0; row < b.Dy(); row++ {
			copy(next.Pix[row*next.Stride:], img.Pix[row*img.Stride:row*img.Stride+b.Dx()*4])
		}
		img = next
	}
	put := func(c byte, rep int) {
		bits := c - '?'
		rep = min(rep, maxSide-x)
		if bits != 0 && rep > 0 && y < maxSide {
			grow(x+rep, min(y+6, maxSide))
			for k := 0; k < 6 && y+k < maxSide; k++ {
				if bits&(1<<k) != 0 {
					for r := 0; r < rep; r++ {
						img.SetNRGBA(x+r, y+k, pal[cur])
					}
					h = max(h, y+k+1)
				}
			}
		}
		x += max(rep, 0)
		w = max(w, min(x, maxSide))
	}

	for i := 0; i < len(data); {
		c := data[i]
		switch {
		case c == '"':
			var n []int
			n, i = nums(data, i+1)
			if len(n) >= 4 {
				w, h = max(w, min(n[2], maxSide)), max(h, min(n[3], maxSide))
				grow(w, h)
			}
		case c == '#':
			var n []int
			n, i = nums(data, i+1)
			if len(n) > 0 {
				cur = n[0] & 255
				if len(n) >= 5 {
					pal[cur] = colorSpec(n[1], n[2], n[3], n[4])
				}
			}
		case c == '!':
			var n []int
			n, i = nums(data, i+1)
			if len(n) > 0 && i < len(data) && data[i] >= '?' && data[i] <= '~' {
				put(data[i], n[0])
				i++
			}
		case c == '$':
			x = 0
			i++
		case c == '-':
			x, y = 0, y+6
			i++
		case c >= '?' && c <= '~':
			put(c, 1)
			i++
		default:
			i++
		}
	}
	if img.Bounds().Dx() == w && img.Bounds().Dy() == h {
		return img
	}
	// The canvas grew by doubling; a SubImage of it would keep all of it
	// alive for as long as the picture is.
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		copy(out.Pix[y*out.Stride:y*out.Stride+4*w], img.Pix[y*img.Stride:])
	}
	return out
}

// nums parses "1;2;3" starting at i and returns the values and the index after them.
func nums(data []byte, i int) ([]int, int) {
	var out []int
	n, has := 0, false
	for ; i < len(data); i++ {
		c := data[i]
		switch {
		case c >= '0' && c <= '9':
			n, has = min(n*10+int(c-'0'), 1<<20), true
		case c == ';':
			out = append(out, n)
			n, has = 0, false
		default:
			if has {
				out = append(out, n)
			}
			return out, i
		}
	}
	if has {
		out = append(out, n)
	}
	return out, i
}

func colorSpec(pu, a, b, c int) color.NRGBA {
	switch pu {
	case 2:
		return pct(a, b, c)
	case 1:
		// DEC hue puts blue at 0°, red at 120°; shift to the usual red-at-0° wheel.
		r, g, bl := hls(float64((a+240)%360)/360, float64(min(b, 100))/100, float64(min(c, 100))/100)
		return color.NRGBA{uint8(r*255 + .5), uint8(g*255 + .5), uint8(bl*255 + .5), 255}
	}
	return color.NRGBA{0, 0, 0, 255}
}

func pct(r, g, b int) color.NRGBA {
	f := func(v int) uint8 { return uint8(min(v, 100) * 255 / 100) }
	return color.NRGBA{f(r), f(g), f(b), 255}
}

func hls(h, l, s float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	q := l * (1 + s)
	if l >= .5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		t -= math.Floor(t)
		switch {
		case t < 1./6:
			return p + (q-p)*6*t
		case t < .5:
			return q
		case t < 2./3:
			return p + (q-p)*(2./3-t)*6
		}
		return p
	}
	return hue(h + 1./3), hue(h), hue(h - 1./3)
}
