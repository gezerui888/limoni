//go:build unix

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/tiff" // chafa sends its iTerm2 pictures as TIFF

	"github.com/thebanri/limoni/graphics"
)

// osc takes iTerm2's inline pictures, ESC ] 1337 ; File = args : base64
// BEL: a whole picture file (PNG, JPEG, GIF, TIFF), shown at the cursor in the
// size the arguments ask for, a GIF animated. iTerm2 3.5 sends large ones
// in parts: MultipartFile = args, FilePart = base64 as many times as it
// takes, FileEnd. File transfers without inline=1 are dropped: there is
// nowhere to save them. Every other OSC goes on.
func (s *session) osc(body []byte) bool {
	rest, ok := bytes.CutPrefix(body, []byte("1337;"))
	if !ok {
		return false
	}
	key, val, _ := bytes.Cut(rest, []byte("="))
	switch string(key) {
	case "File":
		args, data, _ := bytes.Cut(val, []byte(":"))
		s.itermFile(string(args), data)
	case "MultipartFile":
		s.iterm.args, s.iterm.parts = string(val), s.iterm.parts[:0]
	case "FilePart":
		if len(s.iterm.parts)+len(val) <= maxStr {
			s.iterm.parts = append(s.iterm.parts, val...)
		}
	case "FileEnd":
		s.itermFile(s.iterm.args, s.iterm.parts)
		s.iterm.args, s.iterm.parts = "", nil
	default:
		return false
	}
	return true
}

// itermFile shows a file sent with its arguments, if inline.
func (s *session) itermFile(argStr string, data []byte) {
	args := map[string]string{}
	for _, kv := range strings.Split(argStr, ";") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			args[k] = v
		}
	}
	if args["inline"] != "1" {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return
	}
	img, format, err := decodeFile(raw)
	if err != nil {
		return
	}
	b := img.Bounds()
	var a *anim
	if n := gifFrames(raw); format == "gif" && n > 1 && n*4*b.Dx()*b.Dy() <= maxData {
		if g, err := graphics.DecodeGIF(bytes.NewReader(raw)); err == nil {
			a = &anim{frames: g.Frames, gaps: g.Delays, loops: -1}
			img = g.Frames[0]
		}
	}
	pw, ph := s.itermSize(args, b.Dx(), b.Dy())
	render := func(f image.Image) image.Image {
		if pw != b.Dx() || ph != b.Dy() {
			f = graphics.ResizeImage(f, pw, ph)
		}
		return f
	}
	cols := (pw + s.cellW - 1) / s.cellW
	rows := (ph + s.cellH - 1) / s.cellH
	if pic := s.put(render(img), cols, rows, moveAfter, false); pic != nil && a != nil {
		a.start(time.Now())
		pic.anim, pic.src, pic.render = a, img, render
	}
}

// decodeFile decodes a picture file, its size checked before its pixels
// are: the output may come from another machine.
func decodeFile(raw []byte) (image.Image, string, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", err
	}
	if cfg.Width > maxSide || cfg.Height > maxSide {
		return nil, "", errors.New("picture too large")
	}
	return image.Decode(bytes.NewReader(raw))
}

// gifFrames counts a GIF's frames without decoding any, so a small file of
// many frames cannot take a frame's worth of memory for each: -1 if it is
// not a GIF, the frames seen if it is cut short.
func gifFrames(b []byte) int {
	if len(b) < 13 || string(b[:3]) != "GIF" {
		return -1
	}
	i := 13
	if b[10]&0x80 != 0 { // global colour table
		i += 3 << (b[10]&7 + 1)
	}
	n := 0
	for i < len(b) {
		switch b[i] {
		case 0x3b: // trailer
			return n
		case 0x21: // extension: introducer, label, then sub-blocks
			i += 2
		case 0x2c: // image: descriptor, local colour table, LZW code size, sub-blocks
			if i+10 > len(b) {
				return n
			}
			if flags := b[i+9]; flags&0x80 != 0 {
				i += 3 << (flags&7 + 1)
			}
			i += 11
			n++
		default:
			return n
		}
		for i < len(b) && b[i] != 0 {
			i += int(b[i]) + 1
		}
		i++
	}
	return n
}

// itermSize is the size in pixels a picture of w×h is shown at: width and
// height are cells ("12"), pixels ("12px"), a share of the screen ("50%") or
// "auto"; with preserveAspectRatio (the default) the picture fits in the
// box they make, else it fills it. Nothing is shown wider than the screen.
func (s *session) itermSize(args map[string]string, w, h int) (int, int) {
	bw, wok := iterm2Dim(args["width"], s.cellW, s.cols*s.cellW)
	bh, hok := iterm2Dim(args["height"], s.cellH, s.rows*s.cellH)
	keep := args["preserveAspectRatio"] != "0"
	switch {
	case !wok && !hok:
		bw, bh = w, h
	case !wok:
		bw = w
		if keep {
			bw = w * bh / h
		}
	case !hok:
		bh = h
		if keep {
			bh = h * bw / w
		}
	case keep:
		bw, bh = graphics.ContainSize(image.Rect(0, 0, w, h), bw, bh)
	}
	if limit := s.cols * s.cellW; bw > limit {
		bw, bh = limit, bh*limit/bw
	}
	return max(bw, 1), max(bh, 1)
}

func iterm2Dim(spec string, cell, screen int) (int, bool) {
	var n int
	var err error
	switch {
	case spec == "" || spec == "auto":
		return 0, false
	case strings.HasSuffix(spec, "px"):
		n, err = strconv.Atoi(strings.TrimSuffix(spec, "px"))
	case strings.HasSuffix(spec, "%"):
		n, err = strconv.Atoi(strings.TrimSuffix(spec, "%"))
		n = screen * n / 100
	default:
		n, err = strconv.Atoi(spec)
		n *= cell
	}
	if err != nil || n <= 0 {
		return 0, false
	}
	return min(n, maxSide), true
}
