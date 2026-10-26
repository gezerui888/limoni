package main

import (
	"encoding/binary"
	"image"
	"image/png"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// TestPromo films Lemon Sweep's commercial: something calm, then cracks,
// explosions and the frameworks one after another, then the game itself,
// played by the bot. Every frame is a 160×45 cell buffer — what a terminal
// would be sent — painted to pixels here, and every sound comes from the
// game's synth.
//
//	PROMO=/tmp/lemonsweep.mp4 go test -run TestPromo -timeout 30m
//	PROMO_STILLS=dir … also writes every fifteenth frame as a PNG
func TestPromo(t *testing.T) {
	out := os.Getenv("PROMO")
	if out == "" {
		t.Skip("set PROMO to the .mp4 to write")
	}
	f := newFilm(t, out)
	for _, s := range script() {
		n := int(math.Round(s.dur * fps))
		for i := 0; i < n; i++ {
			f.shake, f.flash, f.invert = 0, 0, false
			s.draw(f, float64(i)/fps, i)
			f.emit()
		}
	}
	f.finish(out)
}

const (
	cols, rows = 160, 45
	pixW, pixH = 12, 24
	fps        = 30
	pxW, pxH   = cols, rows * 2 // the half-block picture
)

// ── the film ─────────────────────────────────────────────────────────────

type film struct {
	t      *testing.T
	b      *buffer.Buffer
	c      canvas
	img    *image.RGBA
	face   xfont.Face
	glyphs map[rune]*image.Alpha
	ff     *exec.Cmd
	enc    io.WriteCloser
	video  string
	stills string
	frame  int
	track  []float32 // stereo, interleaved, at sampleRate
	clips  [nSfx][]float32
	shake  float64 // screen pixels of jitter this frame
	flash  float32 // white over the frame
	invert bool
	rnd    *rand.Rand
	parts  []spark
	snap   []rgb // the picture a shatter breaks
	shards []shard
	boom   float64 // shake left from the game's last explosion
}

func newFilm(t *testing.T, out string) *film {
	ft, err := opentype.Parse(gomono.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: 20, DPI: 72, Hinting: xfont.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	b := buffer.NewBuffer(cell.NewRect(0, 0, cols, rows))
	f := &film{t: t, b: b, c: canvas{b, cols, rows}, face: face, glyphs: map[rune]*image.Alpha{},
		img: image.NewRGBA(image.Rect(0, 0, cols*pixW, rows*pixH)), stills: os.Getenv("PROMO_STILLS"),
		rnd: rand.New(rand.NewPCG(7, 11))}
	var m mixer
	m.synth()
	f.clips = m.clips
	f.video = strings.TrimSuffix(out, filepath.Ext(out)) + ".video.mp4"
	f.ff = exec.Command("ffmpeg", "-y", "-loglevel", "error", "-f", "rawvideo", "-pix_fmt", "rgba",
		"-s", strconv.Itoa(cols*pixW)+"x"+strconv.Itoa(rows*pixH), "-r", strconv.Itoa(fps), "-i", "-",
		"-c:v", "libx264", "-preset", "medium", "-crf", "17", "-pix_fmt", "yuv420p", f.video)
	f.ff.Stderr = os.Stderr
	if f.enc, err = f.ff.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	if err := f.ff.Start(); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *film) now() float64 { return float64(f.frame) / fps }

// emit paints the cells and sends the frame to ffmpeg.
func (f *film) emit() {
	f.paint()
	if _, err := f.enc.Write(f.img.Pix); err != nil {
		f.t.Fatal(err)
	}
	if f.stills != "" && f.frame%15 == 0 {
		w, _ := os.Create(filepath.Join(f.stills, strconv.Itoa(f.frame/15+1000)+".png"))
		_ = png.Encode(w, f.img)
		_ = w.Close()
	}
	f.frame++
}

// finish closes the video, writes the sound and puts them together.
func (f *film) finish(out string) {
	_ = f.enc.Close()
	if err := f.ff.Wait(); err != nil {
		f.t.Fatal(err)
	}
	if need := int(f.now()*sampleRate+1) * 2; need > len(f.track) {
		f.track = append(f.track, make([]float32, need-len(f.track))...)
	}
	peak := float32(0.01)
	for _, v := range f.track {
		peak = max(peak, float32(math.Abs(float64(v))))
	}
	wav := strings.TrimSuffix(out, filepath.Ext(out)) + ".wav"
	pcm := make([]byte, 44+len(f.track)*2)
	copy(pcm, "RIFF")
	binary.LittleEndian.PutUint32(pcm[4:], uint32(36+len(f.track)*2))
	copy(pcm[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(pcm[16:], 16)
	binary.LittleEndian.PutUint16(pcm[20:], 1)
	binary.LittleEndian.PutUint16(pcm[22:], 2)
	binary.LittleEndian.PutUint32(pcm[24:], sampleRate)
	binary.LittleEndian.PutUint32(pcm[28:], sampleRate*4)
	binary.LittleEndian.PutUint16(pcm[32:], 4)
	binary.LittleEndian.PutUint16(pcm[34:], 16)
	copy(pcm[36:], "data")
	binary.LittleEndian.PutUint32(pcm[40:], uint32(len(f.track)*2))
	for i, v := range f.track {
		binary.LittleEndian.PutUint16(pcm[44+i*2:], uint16(clip16(v*0.92/peak)))
	}
	if err := os.WriteFile(wav, pcm, 0o644); err != nil {
		f.t.Fatal(err)
	}
	mux := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", f.video, "-i", wav,
		"-c:v", "copy", "-af", "loudnorm=I=-14:TP=-1", "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-shortest", out)
	mux.Stderr = os.Stderr
	if err := mux.Run(); err != nil {
		f.t.Fatal(err)
	}
	_ = os.Remove(f.video)
	_ = os.Remove(wav)
	f.t.Logf("%s: %.1f s", out, f.now())
}

// ── painting cells into pixels ───────────────────────────────────────────

func colorOf(c cell.Color, fallback rgb) [3]uint8 {
	if c.Type() != cell.ColorRGB {
		return [3]uint8{to8(fallback.r), to8(fallback.g), to8(fallback.b)}
	}
	r, g, b := c.RGB()
	return [3]uint8{r, g, b}
}

func (f *film) paint() {
	pix := f.img.Pix
	stride := f.img.Stride
	clear(pix)
	ox, oy := 0, 0
	if f.shake > 0 {
		ox = int((f.rnd.Float64()*2 - 1) * f.shake)
		oy = int((f.rnd.Float64()*2 - 1) * f.shake)
	}
	W, H := cols*pixW, rows*pixH
	put := func(x, y int, c [3]uint8) {
		x, y = x+ox, y+oy
		if x < 0 || y < 0 || x >= W || y >= H {
			return
		}
		i := y*stride + x*4
		pix[i], pix[i+1], pix[i+2], pix[i+3] = c[0], c[1], c[2], 255
	}
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cols; cx++ {
			p := &f.b.Content[cy*cols+cx]
			fg := colorOf(p.Style.Fg, hex(0xd8d8d8))
			bg := colorOf(p.Style.Bg, hex(0))
			if p.Style.Modifier&cell.ModifierReverse != 0 {
				fg, bg = bg, fg
			}
			x0, y0 := cx*pixW, cy*pixH
			r := p.Content
			cov := coverage(r)
			var mask *image.Alpha
			if cov == nil && r != ' ' && r != 0 {
				mask = f.glyph(r)
			}
			for y := 0; y < pixH; y++ {
				for x := 0; x < pixW; x++ {
					a := 0.0
					switch {
					case cov != nil:
						a = cov((float64(x)+0.5)/pixW, (float64(y)+0.5)/pixH)
					case mask != nil:
						a = float64(mask.Pix[y*mask.Stride+x]) / 255
					}
					if a <= 0 {
						put(x0+x, y0+y, bg)
						continue
					}
					var c [3]uint8
					for k := range c {
						c[k] = uint8(float64(bg[k])*(1-a) + float64(fg[k])*a + 0.5)
					}
					put(x0+x, y0+y, c)
				}
			}
		}
	}
	if f.flash > 0 || f.invert {
		k := min(f.flash, 1)
		for i := 0; i < len(pix); i += 4 {
			for j := 0; j < 3; j++ {
				v := float32(pix[i+j])
				if f.invert {
					v = 255 - v
				}
				pix[i+j] = uint8(v + (255-v)*k)
			}
		}
	}
}

func (f *film) glyph(r rune) *image.Alpha {
	if m, ok := f.glyphs[r]; ok {
		return m
	}
	m := image.NewAlpha(image.Rect(0, 0, pixW, pixH))
	d := xfont.Drawer{Dst: m, Src: image.Opaque, Face: f.face, Dot: fixed.P(0, 18)}
	d.DrawString(string(r))
	f.glyphs[r] = m
	return m
}

// coverage is how much of a cell a block element or braille pattern
// covers at (x, y), both 0…1; nil for a character the font draws.
func coverage(r rune) func(x, y float64) float64 {
	in := func(b bool) float64 {
		if b {
			return 1
		}
		return 0
	}
	quad := func(tl, tr, bl, br bool) func(x, y float64) float64 {
		return func(x, y float64) float64 {
			l, t := x < 0.5, y < 0.5
			return in(t && l && tl || t && !l && tr || !t && l && bl || !t && !l && br)
		}
	}
	switch {
	case r == '▀':
		return func(_, y float64) float64 { return in(y < 0.5) }
	case r >= 0x2581 && r <= 0x2588:
		n := float64(r-0x2580) / 8
		return func(_, y float64) float64 { return in(y >= 1-n) }
	case r >= 0x2589 && r <= 0x258f:
		n := float64(0x2590-r) / 8
		return func(x, _ float64) float64 { return in(x < n) }
	case r == '▐':
		return func(x, _ float64) float64 { return in(x >= 0.5) }
	case r >= 0x2591 && r <= 0x2593:
		n := float64(r-0x2590) / 4
		return func(_, _ float64) float64 { return n }
	case r == '▔':
		return func(_, y float64) float64 { return in(y < 0.125) }
	case r == '▕':
		return func(x, _ float64) float64 { return in(x >= 0.875) }
	case r >= 0x2596 && r <= 0x259f:
		q := [...][4]bool{{false, false, true, false}, {false, false, false, true}, {true, false, false, false},
			{true, false, true, true}, {true, false, false, true}, {true, true, true, false}, {true, true, false, true},
			{false, true, false, false}, {false, true, true, false}, {false, true, true, true}}[r-0x2596]
		return quad(q[0], q[1], q[2], q[3])
	case r >= 0x2800 && r <= 0x28ff:
		bits := r - 0x2800
		dots := [8][2]float64{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
		return func(x, y float64) float64 {
			for i, d := range dots {
				if bits&(1<<i) != 0 {
					dx, dy := (x-(0.3+d[0]*0.4))*pixW, (y-(0.14+d[1]*0.24))*pixH
					if dx*dx+dy*dy < 5 {
						return 1
					}
				}
			}
			return 0
		}
	}
	return nil
}

// ── sound ────────────────────────────────────────────────────────────────

// mix adds a mono clip at at seconds.
func (f *film) mix(clip []float32, at float64, vol, pan float32) {
	l, r := vol*min(1, 1-pan), vol*min(1, 1+pan)
	i0 := int(at*sampleRate) * 2
	if need := i0 + len(clip)*2; need > len(f.track) {
		f.track = append(f.track, make([]float32, need-len(f.track))...)
	}
	for i, v := range clip {
		f.track[i0+i*2] += v * l
		f.track[i0+i*2+1] += v * r
	}
}

func (f *film) sfx(s sfx, vol float32) { f.mix(f.clips[s], f.now(), vol, 0) }

func clipOf(sec float64, fn func(s *synth)) []float32 {
	s := newSynth(sec)
	fn(s)
	return s.buf
}

// mic is the game's speaker, recording into the film.
type mic struct{ f *film }

func (m mic) play(s sfx, vol, pan float32) {
	m.f.mix(m.f.clips[s], m.f.now(), vol*0.55, pan)
	if s == sfxBoom || s == sfxFreeze {
		m.f.boom = 5 // the camera feels it too
	}
}
func (mic) setVolume(float32) {}

var (
	scratchSnd = clipOf(0.45, func(s *synth) { // a record scratch
		s.tone(0, 0.18, 300, 1400, 0.5, 1, saw)
		s.tone(0.18, 0.45, 1400, 120, 0.5, 2, saw)
		s.hiss(0, 0.45, 4000, 800, 0.4, 3)
	})
	kick  = clipOf(0.3, func(s *synth) { s.tone(0, 0.3, 160, 42, 1, 6, sine) })
	snare = clipOf(0.2, func(s *synth) {
		s.hiss(0, 0.2, 6000, 1500, 0.55, 9)
		s.tone(0, 0.08, 230, 180, 0.3, 10, sine)
	})
	hat    = clipOf(0.05, func(s *synth) { s.hiss(0, 0.05, 12000, 9000, 0.18, 18) })
	whoosh = clipOf(0.3, func(s *synth) { s.hiss(0, 0.3, 500, 7000, 0.35, 1) })
	ding   = clipOf(0.8, func(s *synth) {
		s.tone(0, 0.8, 1568, 1568, 0.3, 5, sine)
		s.tone(0, 0.8, 3136, 3136, 0.1, 8, sine)
	})
	boing = clipOf(0.6, func(s *synth) {
		for k := 0.0; k < 0.6; k += 0.1 {
			s.tone(k, k+0.1, 180+400*math.Exp(-k*4), 120+300*math.Exp(-k*4), 0.4*math.Exp(-k*3), 1, sine)
		}
	})
	click = clipOf(0.03, func(s *synth) { s.hiss(0, 0.03, 4000, 2000, 0.25, 30) })
)

func note(n float64) float64 { return 440 * math.Pow(2, (n-69)/12) }

// pad is a soft major chord, for the calm before.
func (f *film) pad(at, dur float64) {
	f.mix(clipOf(dur, func(s *synth) {
		for _, n := range []float64{60, 64, 67, 72} {
			s.tone(0, dur, note(n), note(n), 0.09, 0.6, sine)
		}
	}), at, 1, 0)
}

// beat lays bars of a 150 bpm beat from at: kick, snare, hats and a bass.
func (f *film) beat(at, dur float64) {
	step := 60.0 / 150 / 2
	bass := []float64{33, 33, 45, 33, 36, 36, 43, 31}
	for i := 0; float64(i)*step < dur; i++ {
		t := at + float64(i)*step
		if i%4 == 0 || i%16 == 14 {
			f.mix(kick, t, 0.9, 0)
		}
		if i%4 == 2 {
			f.mix(snare, t, 0.7, 0)
		}
		f.mix(hat, t, 0.6, float32(i%2)*0.4-0.2)
		n := bass[i%len(bass)]
		f.mix(clipOf(step, func(s *synth) { s.tone(0, step, note(n), note(n), 0.22, 3, square) }), t, 1, 0)
	}
}

// jingle is the sting under the name.
func (f *film) jingle(at float64) {
	for i, n := range []float64{72, 76, 79, 84, 88} {
		f.mix(clipOf(0.5, func(s *synth) { s.tone(0, 0.5, note(n), note(n), 0.18, 4, square) }), at+float64(i)*0.09, 1, 0)
	}
	f.mix(clipOf(1.6, func(s *synth) {
		for _, n := range []float64{60, 72, 76, 79, 84} {
			s.tone(0, 1.6, note(n), note(n), 0.1, 2, square)
		}
	}), at+0.5, 1, 0)
	f.mix(kick, at+0.5, 1, 0)
}

// ── pixels ───────────────────────────────────────────────────────────────

func (f *film) px(x, y int, c rgb) { f.c.px(x, y, c) }

func (f *film) get(x, y int) rgb {
	p := f.c.at(x, y/2)
	if p == nil {
		return rgb{}
	}
	t, b := halves(p)
	if y%2 == 0 {
		return t
	}
	return b
}

func (f *film) fill(c rgb) { f.c.fill(0, 0, cols, rows, ' ', style(c, c)) }

func (f *film) vgrad(top, bot rgb) {
	for y := 0; y < pxH; y++ {
		c := top.mix(bot, float32(y)/pxH)
		for x := 0; x < pxW; x++ {
			f.px(x, y, c)
		}
	}
}

// stripes fills with diagonal bands that move: a loud background.
func (f *film) stripes(a, b rgb, t float64) {
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			c := a
			if int(float64(x+y)+t*40)/10%2 == 0 {
				c = b
			}
			f.px(x, y, c)
		}
	}
}

// rays fills with n bands turning round (cx, cy).
func (f *film) rays(cx, cy float64, n int, rot float64, a, b rgb) {
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			ang := math.Atan2(float64(y)-cy, float64(x)-cx) + rot
			c := a
			if int(math.Floor(ang/(2*math.Pi)*float64(n)*2+100))%2 == 0 {
				c = b
			}
			d := math.Hypot(float64(x)-cx, (float64(y)-cy)*1.4)
			f.px(x, y, c.mix(a.mul(0.3), float32(math.Min(1, d/120))))
		}
	}
}

func (f *film) disc(cx, cy, rx, ry float64, c rgb) {
	for y := int(cy - ry - 1); y <= int(cy+ry+1); y++ {
		for x := int(cx - rx - 1); x <= int(cx+rx+1); x++ {
			dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
			if dx*dx+dy*dy <= 1 {
				f.px(x, y, c.mul(float32(1.1-0.3*(dx+dy+1)/2)))
			}
		}
	}
}

func (f *film) rect(x0, y0, x1, y1 int, c rgb) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			f.px(x, y, c)
		}
	}
}

func (f *film) line(x0, y0, x1, y1 float64, c rgb) {
	n := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
	for i := 0; i <= n; i++ {
		k := float64(i) / float64(n)
		f.px(int(math.Round(x0+(x1-x0)*k)), int(math.Round(y0+(y1-y0)*k)), c)
	}
}

// sprite draws s centred on (cx, cy), scale screen pixels a sprite pixel.
func (f *film) sprite(s *sprite, cx, cy, scale float64) {
	w, h := float64(s.w)*scale, float64(s.h)*scale
	for y := int(cy - h/2); y < int(cy+h/2); y++ {
		for x := int(cx - w/2); x < int(cx+w/2); x++ {
			sx, sy := int((float64(x)-cx+w/2)/scale), int((float64(y)-cy+h/2)/scale)
			if sx < 0 || sy < 0 || sx >= s.w || sy >= s.h || !s.on[sy*s.w+sx] {
				continue
			}
			f.px(x, y, s.px[sy*s.w+sx])
		}
	}
}

// words writes s in the 3×5 font, centred on column cx, with a shadow.
func (f *film) words(s string, cx, py, scale int, col rgb) {
	w := len([]rune(s))*(fontW+1)*scale - scale
	x := cx - w/2
	f.c.big(x+scale/2+1, py+scale/2+1, s, scale, hex(0x0a0610))
	f.c.big(x, py, s, scale, col)
}

// fit is the largest scale up to most at which s fits in w pixels.
func fit(s string, w, most int) int {
	return max(1, min(most, w/(len([]rune(s))*(fontW+1))))
}

// caption writes s in the 3×5 font on a band of bg, centred, from half
// row py.
func (f *film) caption(py int, s string, fg, bg rgb) {
	w := len([]rune(s))*(fontW+1) - 1
	x := (pxW - w) / 2
	f.rect(x-3, py-2, x+w+2, py+fontH+1, bg)
	f.c.big(x, py, s, 1, fg)
}

// typed is s as far as a typist gets in t seconds.
func typed(s string, t, perSec float64) string {
	r := []rune(s)
	return string(r[:min(len(r), max(0, int(t*perSec)))])
}

// ── particles, cracks, shards ────────────────────────────────────────────

type spark struct {
	x, y, vx, vy, life float64
	c                  rgb
	big                bool
}

func (f *film) burst(x, y float64, n int, speed float64, cs ...rgb) {
	for range n {
		a, v := f.rnd.Float64()*2*math.Pi, speed*(0.3+f.rnd.Float64())
		f.parts = append(f.parts, spark{x, y, math.Cos(a) * v, math.Sin(a)*v - speed*0.3, 0.6 + f.rnd.Float64()*0.8,
			cs[f.rnd.IntN(len(cs))], f.rnd.IntN(3) == 0})
	}
}

func (f *film) stepParts() {
	dt := 1.0 / fps
	live := f.parts[:0]
	for _, p := range f.parts {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		p.vy += 120 * dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		c := p.c.mul(float32(math.Min(1, p.life*2)))
		f.px(int(p.x), int(p.y), c)
		if p.big {
			f.px(int(p.x)+1, int(p.y), c)
			f.px(int(p.x), int(p.y)+1, c)
			f.px(int(p.x)+1, int(p.y)+1, c)
		}
		live = append(live, p)
	}
	f.parts = live
}

type seg struct{ x0, y0, x1, y1, d float64 }

// crack is lines running from (cx, cy) like glass hit there; each segment
// knows how far from the hit it starts, so it can spread.
func crack(rnd *rand.Rand, cx, cy float64, arms int) []seg {
	var segs []seg
	var grow func(x, y, a, d float64, n int)
	grow = func(x, y, a, d float64, n int) {
		for i := 0; i < n; i++ {
			l := 4 + rnd.Float64()*6
			nx, ny := x+math.Cos(a)*l, y+math.Sin(a)*l*0.6
			segs = append(segs, seg{x, y, nx, ny, d})
			d += l
			if rnd.IntN(5) == 0 {
				grow(nx, ny, a+(rnd.Float64()-0.5)*1.6, d, n/2)
			}
			x, y, a = nx, ny, a+(rnd.Float64()-0.5)*0.7
		}
	}
	for k := 0; k < arms; k++ {
		grow(cx, cy, float64(k)/float64(arms)*2*math.Pi+rnd.Float64()*0.5, 0, 14)
	}
	return segs
}

// cracks draws the segments up to reach pixels from the hit.
func (f *film) cracks(segs []seg, reach float64) {
	for _, s := range segs {
		if s.d > reach {
			continue
		}
		f.line(s.x0+1, s.y0+1, s.x1+1, s.y1+1, hex(0x101018))
		f.line(s.x0, s.y0, s.x1, s.y1, hex(0xf4f8ff))
	}
}

type shard struct {
	sx, sy, x, y, vx, vy, a, va float64
}

// shatter breaks the picture now on screen into pieces round seeds; draw
// with shards from then on, over whatever is behind.
func (f *film) shatter(cx, cy float64) {
	f.snap = make([]rgb, pxW*pxH)
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			f.snap[y*pxW+x] = f.get(x, y)
		}
	}
	f.shards = f.shards[:0]
	for range 40 {
		x, y := f.rnd.Float64()*pxW, f.rnd.Float64()*pxH
		dx, dy := x-cx, y-cy
		d := math.Hypot(dx, dy) + 1
		f.shards = append(f.shards, shard{x, y, x, y, dx / d * 60, dy/d*40 - 30, 0, (f.rnd.Float64() - 0.5) * 6})
	}
}

func (f *film) drawShards(t float64) {
	owner := func(x, y int) int {
		best, bd := 0, math.MaxFloat64
		for i, s := range f.shards {
			if d := (float64(x)-s.sx)*(float64(x)-s.sx) + (float64(y)-s.sy)*(float64(y)-s.sy); d < bd {
				best, bd = i, d
			}
		}
		return best
	}
	for y := 0; y < pxH; y++ {
		for x := 0; x < pxW; x++ {
			i := owner(x, y)
			s := f.shards[i]
			px, py := s.vx*t, s.vy*t+70*t*t
			a := s.va * t
			dx, dy := float64(x)-s.sx, float64(y)-s.sy
			nx := s.sx + dx*math.Cos(a) - dy*math.Sin(a) + px
			ny := s.sy + dx*math.Sin(a) + dy*math.Cos(a) + py
			c := f.snap[y*pxW+x]
			if owner(min(pxW-1, x+1), y) != i || owner(x, min(pxH-1, y+1)) != i {
				c = hex(0xe8f0ff) // the edge of the glass
			}
			f.px(int(nx), int(ny), c)
		}
	}
}

// ── the lemon ────────────────────────────────────────────────────────────

// lemon draws the hero centred on (cx, cy), r pixels wide each side, squashed
// by sq (1 is round), in sunglasses if cool.
func (f *film) lemon(cx, cy, r, sq float64, cool bool) {
	rx, ry := r*1.3/math.Sqrt(sq), r*math.Sqrt(sq)
	yel := hex(0xffe14a)
	f.disc(cx-rx*0.95, cy, rx*0.18+1, ry*0.25+1, yel.mul(0.9))
	f.disc(cx+rx*0.95, cy, rx*0.18+1, ry*0.25+1, yel.mul(0.8))
	f.disc(cx, cy, rx, ry, yel)
	f.disc(cx+r*0.25, cy-ry-r*0.05, r*0.35, r*0.15, hex(0x58c24a)) // the leaf
	if cool {
		gl := hex(0x0c0a12)
		w, h := int(r*0.5), int(math.Max(2, r*0.28))
		y := int(cy - ry*0.25)
		f.rect(int(cx)-w-2, y, int(cx)-2, y+h, gl)
		f.rect(int(cx)+2, y, int(cx)+w+2, y+h, gl)
		f.rect(int(cx)-3, y, int(cx)+3, y, gl)
		f.px(int(cx)-w, y+1, hex(0xffffff))
		f.px(int(cx)+4, y+1, hex(0xffffff))
		f.line(cx-r*0.2, cy+ry*0.45, cx+r*0.3, cy+ry*0.35, hex(0x5a3a08)) // the smirk
	} else {
		f.rect(int(cx-r*0.35), int(cy-ry*0.2), int(cx-r*0.35)+1, int(cy-ry*0.2)+1, hex(0x1a1020))
		f.rect(int(cx+r*0.25), int(cy-ry*0.2), int(cx+r*0.25)+1, int(cy-ry*0.2)+1, hex(0x1a1020))
		f.line(cx-r*0.15, cy+ry*0.3, cx+r*0.1, cy+ry*0.35, hex(0x5a3a08))
	}
}

// broom is the sweeper's: a handle from (x, y) up and to the right.
func (f *film) broom(x, y, ang float64) {
	dx, dy := math.Cos(ang), math.Sin(ang)
	for k := 0.0; k < 26; k++ {
		f.px(int(x+dx*k), int(y+dy*k), hex(0x8a5a2b))
		f.px(int(x+dx*k)+1, int(y+dy*k), hex(0x6a4020))
	}
	for k := -5.0; k <= 5; k++ {
		for j := 0.0; j < 7; j++ {
			f.px(int(x-dx*j-dy*k*0.8), int(y-dy*j+dx*k*0.8), hex(0xe0b050).mul(float32(1-j*0.06)))
		}
	}
}

// ── the game, filmed ─────────────────────────────────────────────────────

// take is a run warmed up by the bot, out of earshot, ready to film.
func take(mapSel int, seed uint64, taste []augID, warm float64) *game {
	g := newGame(seed)
	g.mapSel = mapSel
	g.reset()
	for t := 0.0; t < warm; t += 1.0 / fps {
		bot(g, taste)
		g.step(1.0 / fps)
		g.hp = max(g.hp, g.st.maxHP*0.7) // the camera crew does not die
	}
	return g
}

// play films one frame of g with the bot at the wheel, turning the view
// toward what it aims at rather than snapping, as a person would.
// then, if given, steers further after the bot: a dash, an active cast.
func (f *film) play(g *game, taste []augID, then ...func(g *game)) {
	g.audio, g.volume = mic{f}, 7
	from := g.ang
	bot(g, taste)
	d := math.Remainder(g.ang-from, 2*math.Pi)
	g.ang = from + d*0.3
	for _, fn := range then {
		fn(g)
	}
	g.step(1.0 / fps)
	if g.phase == phCards {
		bot(g, taste) // a level up: take the card before it is seen
	}
	g.hp = max(g.hp, g.st.maxHP*0.7)
	g.render(f.b)
	f.shake = max(f.shake, f.boom)
	f.boom *= 0.75
}

// crowd puts n enemies in front of the player, in a cone, where there is
// room.
func crowd(g *game, n int, ks ...kindID) {
	for i := 0; i < n; i++ {
		k := ks[i%len(ks)]
		p := g.pos.add(dir(g.ang + (g.rnd.f()-0.5)*1.3).scale(5 + g.rnd.f()*4))
		if free(p.x, p.y, kinds[k].r) {
			g.addEnemy(k, p)
		}
	}
}

// every is true on the frames t crosses a multiple of sec.
func every(t, sec float64) bool { return int(t/sec) != int((t-1.0/fps)/sec) || t == 0 }

// tag is a small label in the top left, so the game keeps the screen:
// its name in the big font on a chip of col, a line of text under it.
func (f *film) tag(label, sub string, col rgb, t float64) {
	w := len([]rune(label))*(fontW+1) + 3
	x := int(math.Min(2, -float64(w)+t*400))
	f.rect(x, 6, x+w, 6+fontH+3, col)
	f.c.big(x+2, 8, label, 1, hex(0x101010))
	if sub != "" && t > 0.15 {
		cx := max(0, x)
		f.c.fill(cx, 8, len([]rune(sub))+2, 1, ' ', style(hex(0xffffff), hex(0x101018)))
		f.c.text(cx+1, 8, sub, style(hex(0xffffff), hex(0x101018)))
	}
}

// boss puts the next boss in the order a few steps in front of the
// player, and lets the run go on a moment.
func boss(g *game, n int, taste []augID) {
	g.bossN = n
	g.spawnBoss()
	for i := range g.enemies {
		if e := &g.enemies[i]; e.alive && kinds[e.k].boss {
			for _, d := range []float64{4, 3.5, 3} {
				if p := g.pos.add(dir(g.ang).scale(d)); free(p.x, p.y, kinds[e.k].r) {
					e.pos = p
					break
				}
			}
		}
	}
	for range 20 {
		bot(g, taste)
		g.step(1.0 / fps)
	}
}

// ── the script ───────────────────────────────────────────────────────────

type scene struct {
	dur  float64
	draw func(f *film, t float64, i int)
}

func at(i int, sec float64) bool { return i == int(math.Round(sec*fps)) }

var (
	tasteStorm = []augID{augTesla, augChain, augStorm, augStatic, augIRQ, augConductor, augCron, augThunder, augRoundTrip, augShort, augDaemon, augSuper}
	tasteFire  = []augID{augSegfault, augIgnite, augBurnIn, augFirewall, augCron, augOverheat, augRoundTrip, augWildfire, augTesla, augIncinerate, augDaemon, augHotPath}
	tasteBroke = []augID{augRoundTrip, augFork, augCron, augPanic, augSegfault, augRecursion, augStorm, augCascade, augTesla, augScroll, augCompaction, augREP}
)

// calm is the meadow the heap was, with a happy little lemon.
func (f *film) calm(t float64) {
	f.vgrad(hex(0x9fd8ff), hex(0xffd6e8))
	for x := 0; x < pxW; x++ {
		h := 70 + int(3*math.Sin(float64(x)*0.08))
		f.rect(x, h, x, pxH-1, hex(0x6fcf5a).mul(0.9+0.1*float32(math.Sin(float64(x)*0.7))))
	}
	f.disc(130, 18, 9, 9, hex(0xfff3a0))
	for i, x := range []float64{20, 55, 95} {
		cx := math.Mod(x+t*4+float64(i)*7, 180) - 10
		f.disc(cx, 14+float64(i)*5, 9, 3, hex(0xffffff))
		f.disc(cx+6, 12+float64(i)*5, 6, 3, hex(0xffffff))
	}
	f.lemon(80, 52+2*math.Sin(t*4), 12, 1+0.05*math.Sin(t*8), false)
}

func script() []scene {
	var (
		cr    []seg
		games [8]*game
	)
	ink := hex(0x203040)
	var beatLen float64 // from the hero's scene to the last of the game's
	list := []scene{
		// 1. Calm. Too calm.
		{3.4, func(f *film, t float64, i int) {
			if i == 0 {
				f.pad(0, 3.4)
				for _, k := range []float64{0.4, 1.3, 2.2} {
					f.mix(clipOf(0.15, func(s *synth) { s.tone(0, 0.15, 2600, 3400, 0.08, 3, sine) }), k, 1, 0.5)
				}
			}
			f.calm(t)
			switch {
			case t < 1.6:
				f.caption(78, "This is your heap.", ink, hex(0xffffff))
			case t < 2.8:
				f.caption(78, "Calm. Tidy. Zero allocations.", ink, hex(0xffffff))
			default:
				f.caption(78, "Nothing could possibly go wr", ink, hex(0xffffff))
				f.shake = (t - 2.8) * 10
			}
		}},
		// 2. Smash: the screen cracks, and Bubble Tea comes through it.
		{2.4, func(f *film, t float64, i int) {
			if i == 0 {
				cr = crack(f.rnd, 112, 40, 9)
				f.mix(scratchSnd, f.now(), 0.8, 0)
				f.sfx(sfxBoom, 1)
			}
			if t < 0.45 {
				f.calm(3.4)
				f.caption(78, "Nothing could possibly go wr", ink, hex(0xffffff))
				f.cracks(cr, t*500)
				f.invert = i < 2
				f.shake = 14 * (1 - t*2)
				if at(i, 0.4) {
					f.shatter(112, 40)
				}
				return
			}
			if at(i, 0.46) {
				f.sfx(sfxRoar, 1)
				f.sfx(sfxBoom, 0.8)
			}
			k := t - 0.45
			f.stripes(hex(0xb0103a), hex(0x70081e), t)
			s := math.Min(4.2, 1+k*14)
			if k > 0.3 {
				s = 4.2 + 0.3*math.Sin(k*30)*math.Exp(-k*3)
			}
			f.sprite(kinds[kBoba].spr[int(t*12)%len(kinds[kBoba].spr)], 80, 46, s)
			col := hex(0xffe14a)
			if int(t*10)%2 == 1 {
				col = hex(0xffffff)
			}
			f.words("BUBBLE TEA", 80, 3, 3, col)
			f.caption(80, "has entered the heap", hex(0xffffff), hex(0x101018))
			f.shake = 8 * math.Exp(-k*3)
			f.flash = float32(math.Max(0, 1-k*6))
			if k < 1.2 {
				f.drawShards(k)
			}
		}},
		// 3. The rest of them, one cut each.
		{0.6, villain(kCrab, "RATATUI", "BROUGHT A CRAB", "nobody asked why", hex(0xf74c00), hex(0x2a1004), sfxSpit)},
		{0.6, villain(kModal, "TVIEW", "ARE YOU SURE?", "[ YES ]   [ ALSO YES ]", hex(0x3d7bff), hex(0x0a1430), sfxCharge)},
		{0.6, villain(kJSX, "INK", "RE-RENDERED", "your grandma. twice.", hex(0x61dafb), hex(0x082028), sfxZap)},
		{0.6, villain(kElder, "NCURSES", "SINCE 1993", "it has seen things", hex(0x33ff66), hex(0x041a08), sfxHurt)},
		{0.9, func(f *film, t float64, i int) {
			villain(kBomb, "NOTCURSES", "IS A BOMB", "this one is literal", hex(0xff6a1a), hex(0x2a0e02), sfxSpit)(f, t, i)
			if at(i, 0.45) {
				f.sfx(sfxBoom, 1)
				f.burst(80, 42, 260, 120, hex(0xffe14a), hex(0xff6a1a), hex(0xff2a2a), hex(0xffffff))
			}
			if t >= 0.45 {
				f.flash = float32(math.Max(0, 0.8-(t-0.45)*4))
				f.shake = 12
				f.stepParts()
			}
		}},
		{0.6, villain(kBar, "PROGRESS BAR", "99%", "forever.", hex(0xc06ef3), hex(0x1e0a2a), sfxPop)},
		// 4. Silence. A question.
		{2.2, func(f *film, t float64, i int) {
			f.parts = f.parts[:0]
			f.fill(hex(0))
			const ask = "who will clean up this mess?"
			q := typed(ask, t-0.3, 18)
			if n := len([]rune(q)); n > 0 && at(i, 0.3+float64(n-1)/18) {
				f.mix(click, f.now(), 0.6, 0)
			}
			x := (pxW - len(ask)*(fontW+1)) / 2
			x = f.c.big(x, 42, q, 1, hex(0xd8d8d8))
			if int(t*3)%2 == 0 {
				f.rect(x, 42, x+2, 46, hex(0xd8d8d8))
			}
		}},
		// 5. The hero lands.
		{3.6, func(f *film, t float64, i int) {
			if i == 0 {
				f.mix(whoosh, f.now(), 1, 0)
				f.beat(f.now()+0.35, beatLen-0.35)
			}
			f.rays(80, 50, 12, t*0.8, hex(0xffb020), hex(0xff7a10))
			y, sq := -40+t*290, 1.0
			if t >= 0.35 {
				k := t - 0.35
				y = 60
				sq = 1 - 0.45*math.Exp(-k*6)*math.Cos(k*25)
				f.shake = 10 * math.Exp(-k*5)
			}
			if at(i, 0.35) {
				f.sfx(sfxBoom, 1)
				f.mix(boing, f.now(), 0.8, 0)
				f.burst(80, 78, 90, 70, hex(0xd8c090), hex(0xffffff), hex(0xa08050))
				f.flash = 0.8
			}
			f.broom(104, 84, -1.2)
			f.lemon(80, y, 17, sq, true)
			f.stepParts()
			for _, when := range []float64{0.9, 1.3, 1.7} {
				if at(i, when) {
					f.sfx(sfxShot, 1)
					f.shake = 6
				}
			}
			if t >= 0.9 {
				f.words("THE", 80, 3, 2, hex(0xffffff))
			}
			if t >= 1.3 {
				f.words("THE GARBAGE", 80, 3, 2, hex(0xffffff))
			}
			if t >= 1.7 {
				f.words("COLLECTOR", 80, 15, 3, hex(0xffe14a))
			}
			if t >= 2.4 {
				if at(i, 2.4) {
					f.sfx(sfxPing, 1)
				}
				k := float32(math.Max(0, 1-(t-2.4)*3))
				for d := -3; d <= 3; d++ {
					f.px(70+d, 55, hex(0xffffff).mul(k))
					f.px(70, 55+d, hex(0xffffff).mul(k))
				}
				f.caption(82, "he's a lemon. he has a broom.", hex(0x101010), hex(0xffe14a))
			}
		}},
		// 6. Numbers, shouted.
		{0.8, stat("0 B/OP", "zero allocations. we measured.", hex(0x1a1a2e), hex(0x6ee07a))},
		{0.8, stat("144 FPS", "in a terminal. yes. a terminal.", hex(0x2e0a1a), hex(0xff5fd2))},
		{0.8, stat("79 CARDS", "most are a bad idea. take them.", hex(0x0a1e2e), hex(0x5ad1ff))},
		{0.8, stat("5 BOSSES", "one of them is a modal dialog.", hex(0x2e1e0a), hex(0xffa62b))},
		// 7. A review, from someone.
		{2.4, func(f *film, t float64, i int) {
			f.vgrad(hex(0x1a0a2e), hex(0x0a0414))
			for k := 0; k < 5; k++ {
				when := 0.15 + float64(k)*0.15
				if t < when {
					continue
				}
				if at(i, when) {
					f.mix(ding, f.now(), 0.25, float32(k-2)*0.3)
				}
				f.star(44+float64(k)*18, 28, 7*math.Min(1, (t-when)*8))
			}
			if t > 1 {
				f.caption(48, "\"I uninstalled my GPU.\"", hex(0xffffff), hex(0x2a1a40))
				f.caption(60, "- a lemon, probably", hex(0xa090c0), hex(0x140a24))
			}
		}},
		// 8. The game: what a build does, one thing a shot.
		{2.0, func(f *film, t float64, i int) {
			if i == 0 {
				games[0] = take(0, 3, tasteBroke, 70)
				cut(f)
			}
			f.play(games[0], tasteBroke)
			f.tag("REAL FOOTAGE", "a game. in a terminal. really.", hex(0xffe14a), t)
		}},
		{1.6, func(f *film, t float64, i int) {
			g := games[0]
			if i == 0 {
				cut(f)
				g.pending = 1
				g.openCards()
			}
			if t < 1.1 {
				g.cardHov = int(t*5) % g.nCards
				if i > 0 && every(t, 0.2) {
					f.sfx(sfxMenu, 0.6)
				}
				g.step(1.0 / fps)
				g.render(f.b)
			} else {
				f.play(g, tasteBroke)
			}
			f.tag("A CARD EVERY LEVEL", "build something broken.", hex(0x5ad1ff), t)
		}},
		{2.6, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(0, 13, tasteBroke, 60)
				pickN(g, augRoundTrip, 5)
				crowd(g, 10, kSpinner, kBoba, kCursor)
				games[1] = g
				cut(f)
			}
			f.play(games[1], tasteBroke, func(g *game) {
				if every(t, 0.45) {
					g.actT[augRoundTrip] = 0
				}
			})
			f.tag("ROUND TRIP", "a boomerang. it comes back. so do they.", hex(0xffa94a), t)
		}},
		{2.8, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(0, 17, tasteBroke, 80)
				for _, a := range []augID{augColdSnap, augAfterburn, augSparkGap, augRam} {
					pickN(g, a, 3)
				}
				pickN(g, augDoubleBuf, 2)
				crowd(g, 12, kBoba, kCrab, kJSX)
				games[2] = g
				cut(f)
			}
			f.play(games[2], tasteBroke, func(g *game) {
				if every(t, 0.55) {
					side := int(t/0.55)%2 == 0
					g.held[actLeft], g.held[actRight], g.held[actFwd] = side, !side, true
					g.dashN, g.dashT = g.st.dashN, 0
					g.dash()
				}
			})
			f.tag("DASH", "leaves fire. ends in ice. zaps the rest.", hex(0x9fd8ff), t)
		}},
		{2.4, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(0, 23, tasteBroke, 60)
				pickN(g, augCompaction, 5)
				pickN(g, augPanic, 3)
				crowd(g, 24, kSpinner, kBoba, kCursor, kJSX)
				games[3] = g
				cut(f)
			}
			f.play(games[3], tasteBroke, func(g *game) {
				if every(t, 1.1) {
					g.actT[augCompaction] = 0
				}
			})
			f.tag("COMPACTION", "pulls the heap together. then: bang.", hex(0xc06ef3), t)
		}},
		{3.0, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(2, 31, tasteBroke, 90)
				pickN(g, augPanic, 5)
				pickN(g, augCascade, 3)
				pickN(g, augNapalm, 3)
				pickN(g, augFork, 3)
				crowd(g, 26, kBoba, kSpinner, kCrab, kBomb, kJSX)
				games[5] = g
				cut(f)
			}
			f.play(games[5], tasteBroke, func(g *game) {
				if every(t, 0.6) {
					crowd(g, 8, kSpinner, kBoba, kBomb)
				}
			})
			f.tag("PANIC()", "every hit explodes. on purpose.", hex(0xff6a1a), t)
		}},
		{2.2, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(1, 11, tasteStorm, 150)
				pickN(g, augStorm, 5)
				pickN(g, augTesla, 5)
				pickN(g, augChain, 3)
				crowd(g, 18, kSpinner, kBoba, kBar)
				games[6] = g
				cut(f)
			}
			f.play(games[6], tasteStorm, func(g *game) {
				if every(t, 0.35) {
					g.actT[augStorm], g.actT[augTesla] = 0, 0
				}
			})
			f.tag("BROADCAST STORM", "lightning is a valid gc strategy.", hex(0xffe14a), t)
		}},
		{2.4, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(0, 5, tasteFire, 90)
				boss(g, 0, tasteFire)
				games[7] = g
				cut(f)
				f.sfx(sfxRoar, 1)
			}
			f.play(games[7], tasteFire)
			f.tag("BOBA PRIME", "a boss. made of tea. with a straw.", hex(0xff5fd2), t)
		}},
		{2.0, func(f *film, t float64, i int) {
			if i == 0 {
				g := take(2, 9, tasteStorm, 200)
				boss(g, 2, tasteStorm)
				games[1] = g
				cut(f)
				f.sfx(sfxRoar, 1)
			}
			f.play(games[1], tasteStorm)
			f.tag("THE MODAL QUEEN", "press ok. there is no cancel.", hex(0x3d7bff), t)
		}},
		{2.0, func(f *film, t float64, i int) {
			if i == 0 {
				games[4] = take(3, 8, tasteStorm, 300)
				crowd(games[4], 16, kSpinner, kBoba, kElder, kBar)
				cut(f)
			}
			f.play(games[4], tasteStorm)
			f.tag("/DEV/ZERO", "it does not end. that's the point.", hex(0x6ee07a), t)
		}},
		// 9. The last frame of the game cracks, and breaks.
		{1.6, func(f *film, t float64, i int) {
			if i == 0 {
				cr = crack(f.rnd, 80, 45, 11)
				f.mix(scratchSnd, f.now(), 0.6, 0)
			}
			if t < 0.5 {
				f.play(games[4], tasteStorm)
				f.cracks(cr, t*400)
				f.shake = 6
				if at(i, 0.2) {
					f.sfx(sfxBoom, 1)
				}
				if at(i, 0.48) {
					f.shatter(80, 45)
				}
				return
			}
			if at(i, 0.5) {
				f.sfx(sfxBoom, 1)
				f.mix(scratchSnd, f.now(), 0.4, 0)
			}
			f.fill(hex(0x03040a))
			f.drawShards(t - 0.5)
			f.flash = float32(math.Max(0, 0.7-(t-0.5)*3))
		}},
		// 10. The name, and how to get it.
		{5.4, func(f *film, t float64, i int) {
			if i == 0 {
				games[5] = newGame(42)
				f.jingle(f.now())
				f.flash = 1
			}
			g := games[5]
			g.step(1.0 / fps)
			g.render(f.b)
			bg := hex(0x101018)
			f.rect(0, 72, pxW-1, pxH-1, bg)
			if t > 2.4 {
				f.caption(75, "FREE. LIKE MEMORY SHOULD BE.", hex(0x101010), hex(0x6ee07a))
			}
			f.c.centered(rows-4, typed("$ go install github.com/thebanri/limoni/apps/lemonsweep@latest", t-0.4, 60), style(hex(0xffe14a), bg))
			if t > 1.6 {
				f.c.centered(rows-2, "or in a browser: thebanri.github.io/limoni/?app=lemonsweep", style(hex(0x9fd8ff), bg))
			}
		}},
		// 11. One more thing.
		{3.4, func(f *film, t float64, i int) {
			f.fill(hex(0))
			spr := kinds[kSpinner].spr[int(t*12)%len(kinds[kSpinner].spr)]
			x := 170 - math.Min(1, t*2)*36
			if t < 1.45 {
				f.sprite(spr, x, 50, 3)
			}
			if t > 0.6 && t < 1.45 {
				f.caption(24, "...psst. is the heap clean?", hex(0xff5fd2), hex(0x14060e))
			}
			if t >= 1.25 && t < 1.7 {
				f.broom(-30+(t-1.25)*400, 74, -0.4)
			}
			if at(i, 1.45) {
				f.sfx(sfxPop, 1)
				f.sfx(sfxShot, 0.8)
				f.burst(134, 50, 70, 120, hex(0xff5fd2), hex(0xffffff), hex(0xffe14a))
				f.shake = 6
			}
			f.stepParts()
			if t > 2 {
				f.caption(42, "it is now.", hex(0xffe14a), hex(0))
			}
		}},
	}
	// The hero is the tenth scene; the last three are the break, the name
	// and the post-credits.
	for _, sc := range list[9 : len(list)-3] {
		beatLen += sc.dur
	}
	return list
}

// villain is one fast cut: a creature, its framework, and what it did.
func villain(k kindID, name, what, sub string, col, bg rgb, s sfx) func(f *film, t float64, i int) {
	return func(f *film, t float64, i int) {
		if i == 0 {
			f.sfx(s, 1)
			f.mix(kick, f.now(), 0.8, 0)
			f.parts = f.parts[:0]
		}
		f.invert = i == 0
		f.stripes(bg, bg.mul(1.6), t)
		spr := kinds[k].spr
		scale := 42 / float64(max(spr[0].w, spr[0].h))
		f.sprite(spr[int(t*12)%len(spr)], 80+2*math.Sin(t*50), 41, scale*(1.15-0.15*math.Min(1, t*8)))
		f.words(name, 80, 2, fit(name, 156, 3), col)
		f.words(what, 80, 64, fit(what, 156, 2), hex(0xffffff))
		f.caption(80, sub, hex(0xffffff), bg.mul(0.5))
		f.shake = 4
	}
}

// stat is one number, punched in.
func stat(big, sub string, bg, col rgb) func(f *film, t float64, i int) {
	return func(f *film, t float64, i int) {
		if i == 0 {
			f.sfx(sfxLevel, 0.8)
			f.flash = 0.6
			f.parts = f.parts[:0]
			for range 40 {
				f.parts = append(f.parts, spark{f.rnd.Float64() * pxW, -f.rnd.Float64() * 40, 0, 20, 2, col.mul(0.5), false})
			}
		}
		f.rays(80, 40, 16, t*-1.2, bg.mul(1.8), bg)
		f.stepParts()
		s := fit(big, 150, 7)
		s += int((1 - math.Min(1, t*10)) * 2)
		f.words(big, 80, 36-s*fontH/2, s, col)
		f.caption(74, sub, hex(0x101010), col)
		f.shake = 5 * (1 - t)
	}
}

// star draws a five-pointed star of radius r.
func (f *film) star(cx, cy, r float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			a := math.Atan2(dy, dx) + math.Pi/2
			k := math.Mod(a+2*math.Pi, 2*math.Pi/5) / (2 * math.Pi / 5)
			if math.Hypot(dx, dy) <= r*(0.45+0.55*math.Abs(1-2*k)) {
				f.px(x, y, hex(0xffd43b))
			}
		}
	}
}

// cut is a hard cut between shots: a white frame and a whoosh.
func cut(f *film) {
	f.flash = 0.9
	f.mix(whoosh, f.now(), 0.6, 0)
	f.mix(kick, f.now(), 0.5, 0)
}
