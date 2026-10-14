package main

import (
	"encoding/binary"
	"io"
	"math"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Sound works as in Lemon Drop (apps/lemondrop/sound.go), from which the
// mixer is taken: synthesised at start-up, no sample files, and streamed
// as raw 16-bit stereo PCM to whichever player the system has — pw-play
// (PipeWire), pacat (PulseAudio), aplay (ALSA) or sox's play. macOS and
// Windows use native audio (sound_system.go); browsers use Web Audio
// (sound_js.go). Without an audio output the game is silent and says so on exit.
//
// The game side of it allocates nothing: play hands a small value to the
// mixer's channel and never waits. The mixer runs on its own goroutine.

type sfx uint8

// The sounds are meant to be sweet rather than loud: sines and soft
// triangles on a pentatonic scale, short, with gentle attacks.
const (
	sfxEat sfx = iota // and the four after it: a note higher each lemon towards the monster
	_
	_
	_
	_
	sfxTurn
	sfxSpit
	sfxHit
	sfxMonster
	sfxBite
	sfxWin
	sfxStart
	sfxPause
	sfxOver
	sfxTheme
	sfxType
	sfxEmpty // space with no seed to spit
	nSfx
)

type speaker interface {
	play(s sfx, vol, pan float32)
	setVolume(v float32) // 0 … 1, for everything
}

const sampleRate = 22050

type sfxReq struct {
	s        sfx
	vol, pan float32
}

type mixer struct {
	req     chan sfxReq
	clips   [nSfx][]float32
	cmd     *exec.Cmd
	w       io.WriteCloser
	player  string
	master  atomic.Uint32 // float32 bits
	once    sync.Once
	done    chan struct{}
	cleanup func() // release native output after closing the PCM stream
}

type voice struct {
	clip   []float32
	pos    int
	gl, gr float32
}

var players = [...][]string{
	{"pw-play", "--raw", "--rate", "22050", "--channels", "2", "--format", "s16", "--latency", "40ms", "-"},
	{"pacat", "--raw", "--rate=22050", "--channels=2", "--format=s16le", "--latency-msec=40"},
	{"aplay", "-q", "-t", "raw", "-f", "S16_LE", "-r", "22050", "-c", "2", "-B", "60000"},
	{"play", "-q", "-t", "raw", "-r", "22050", "-e", "signed", "-b", "16", "-c", "2", "-"},
}

// newMixer prefers browser or native audio, then tries external players.
// It returns nil if no audio output can be started.
func newMixer() *mixer {
	if m := newBrowserMixer(); m != nil {
		return m
	}
	if m := newSystemMixer(); m != nil {
		return m
	}
	for _, p := range players {
		if _, err := exec.LookPath(p[0]); err != nil {
			continue
		}
		cmd := exec.Command(p[0], p[1:]...)
		w, err := cmd.StdinPipe()
		if err != nil {
			continue
		}
		if err := cmd.Start(); err != nil {
			_ = w.Close()
			continue
		}
		m := &mixer{req: make(chan sfxReq, 64), cmd: cmd, w: w, player: p[0], done: make(chan struct{})}
		m.setVolume(1)
		m.synth()
		go m.run()
		return m
	}
	return nil
}

func (m *mixer) play(s sfx, vol, pan float32) {
	select {
	case m.req <- sfxReq{s, vol, pan}:
	default: // the mixer is behind: drop the sound rather than the frame
	}
}

func (m *mixer) setVolume(v float32) { m.master.Store(math.Float32bits(v)) }

func (m *mixer) close() {
	m.once.Do(func() {
		close(m.done)
		if m.w != nil {
			_ = m.w.Close()
		}
		if m.cleanup != nil {
			m.cleanup()
		}
		if m.cmd == nil {
			return // native and browser audio have no process to wait for
		}
		done := make(chan struct{})
		go func() { _ = m.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(300 * time.Millisecond):
			_ = m.cmd.Process.Kill()
		}
	})
}

// run mixes 10 ms at a time. It keeps no more than about 60 ms ahead of the
// clock, or a pipe's worth of sound would queue up and every effect would
// arrive late.
func (m *mixer) run() {
	const chunk = sampleRate / 100
	var voices [24]voice
	out := make([]byte, chunk*4)
	start := time.Now()
	written := 0
	for {
		for {
			select {
			case r := <-m.req:
				for i := range voices {
					if voices[i].clip == nil {
						l := r.vol * float32(math.Min(1, 1-float64(r.pan)))
						rr := r.vol * float32(math.Min(1, 1+float64(r.pan)))
						voices[i] = voice{clip: m.clips[r.s], gl: l, gr: rr}
						break
					}
				}
				continue
			case <-m.done:
				return
			default:
			}
			break
		}
		master := math.Float32frombits(m.master.Load())
		for i := 0; i < chunk; i++ {
			var l, r float32
			for v := range voices {
				vc := &voices[v]
				if vc.clip == nil {
					continue
				}
				s := vc.clip[vc.pos]
				l += s * vc.gl
				r += s * vc.gr
				vc.pos++
				if vc.pos >= len(vc.clip) {
					vc.clip = nil
				}
			}
			binary.LittleEndian.PutUint16(out[i*4:], uint16(clip16(l*master)))
			binary.LittleEndian.PutUint16(out[i*4+2:], uint16(clip16(r*master)))
		}
		if _, err := m.w.Write(out); err != nil {
			return
		}
		written += chunk
		ahead := time.Duration(written)*time.Second/sampleRate - time.Since(start)
		if ahead > 60*time.Millisecond {
			time.Sleep(ahead - 50*time.Millisecond)
		}
	}
}

func clip16(v float32) int16 {
	v = float32(math.Tanh(float64(v))) // soft clip: many sounds at once stay musical
	return int16(v * 32000)
}

// ── synthesis ────────────────────────────────────────────────────────────

type synth struct {
	buf  []float32
	seed uint32
}

func newSynth(seconds float64) *synth {
	return &synth{buf: make([]float32, int(seconds*sampleRate)), seed: 12345}
}

func (s *synth) noise() float32 {
	s.seed ^= s.seed << 13
	s.seed ^= s.seed >> 17
	s.seed ^= s.seed << 5
	return float32(s.seed)/float32(math.MaxUint32)*2 - 1
}

// tone adds a sine sweeping from f0 to f1 Hz between t0 and t1 seconds,
// shaped by a quick attack and an exponential decay.
func (s *synth) tone(t0, t1, f0, f1, amp, decay float64, wave func(float64) float64) {
	i0, i1 := int(t0*sampleRate), int(t1*sampleRate)
	phase := 0.0
	for i := i0; i < i1 && i < len(s.buf); i++ {
		t := float64(i-i0) / float64(i1-i0)
		f := f0 * math.Pow(f1/f0, t)
		phase += f / sampleRate
		env := math.Min(1, t*(t1-t0)*400) * math.Exp(-t*decay)
		s.buf[i] += float32(wave(phase) * amp * env)
	}
}

// hiss adds noise between t0 and t1, low-passed at cutoff Hz (sweeping to
// cutoff1), shaped like tone.
func (s *synth) hiss(t0, t1, cutoff, cutoff1, amp, decay float64) {
	i0, i1 := int(t0*sampleRate), int(t1*sampleRate)
	var y float32
	for i := i0; i < i1 && i < len(s.buf); i++ {
		t := float64(i-i0) / float64(i1-i0)
		c := cutoff * math.Pow(cutoff1/cutoff, t)
		a := float32(1 - math.Exp(-2*math.Pi*c/sampleRate))
		y += (s.noise() - y) * a
		env := math.Min(1, t*(t1-t0)*300) * math.Exp(-t*decay)
		s.buf[i] += y * float32(amp*env)
	}
}

func sine(p float64) float64   { return math.Sin(2 * math.Pi * p) }
func square(p float64) float64 { return math.Copysign(0.6, math.Sin(2*math.Pi*p)) }
func saw(p float64) float64    { return 2*(p-math.Floor(p)) - 1 }

// tri is a triangle: rounder than a square, brighter than a sine.
func tri(p float64) float64 { return 2*math.Abs(2*(p-math.Floor(p+0.5))) - 1 }

func (m *mixer) synth() {
	mk := func(sec float64, f func(s *synth)) []float32 {
		s := newSynth(sec)
		f(s)
		return s.buf
	}
	// bell is a soft note with a quiet octave above it, which makes it ring.
	bell := func(s *synth, t, d, f, amp float64) {
		s.tone(t, t+d, f, f, amp, 5, sine)
		s.tone(t, t+d*0.6, f*2, f*2, amp*0.25, 7, sine)
	}

	// A lemon: a little "bloop" up to a bell, a step up the scale each time.
	for i, f := range [5]float64{784, 880, 1047, 1175, 1319} {
		m.clips[sfxEat+sfx(i)] = mk(0.3, func(s *synth) {
			s.tone(0, 0.06, f*0.6, f, 0.18, 2, tri)
			bell(s, 0.05, 0.25, f, 0.22)
		})
	}
	m.clips[sfxTurn] = mk(0.03, func(s *synth) {
		s.tone(0, 0.03, 1800, 1600, 0.05, 6, sine)
	})
	// A seed spat: a soft "pew".
	m.clips[sfxSpit] = mk(0.12, func(s *synth) {
		s.tone(0, 0.12, 1400, 500, 0.2, 4, tri)
	})
	// A seed striking the monster: a squishy "boing".
	m.clips[sfxHit] = mk(0.25, func(s *synth) {
		s.tone(0, 0.25, 420, 210, 0.3, 5, func(p float64) float64 { return sine(p) * (0.8 + 0.2*sine(p/9)) })
		s.hiss(0, 0.04, 3000, 1500, 0.08, 8)
	})
	// The monster comes: "uh-oh", in two notes down, and a low wobble.
	m.clips[sfxMonster] = mk(0.8, func(s *synth) {
		s.tone(0, 0.22, 392, 392, 0.22, 3, tri)
		s.tone(0.24, 0.6, 294, 280, 0.22, 3, tri)
		s.tone(0.1, 0.8, 98, 92, 0.25, 2, func(p float64) float64 { return sine(p) * (0.7 + 0.3*sine(p/12)) })
	})
	// The monster bites the body off: a soft chomp.
	m.clips[sfxBite] = mk(0.2, func(s *synth) {
		s.hiss(0, 0.08, 1800, 600, 0.35, 5)
		s.tone(0, 0.12, 180, 90, 0.35, 5, sine)
	})
	// The monster falls: bells running up, and a sparkle.
	m.clips[sfxWin] = mk(1.2, func(s *synth) {
		for i, f := range [...]float64{784, 988, 1175, 1568, 1976} {
			bell(s, float64(i)*0.08, 0.5, f, 0.2)
		}
		for i := range 6 {
			t := 0.4 + float64(i)*0.07
			s.tone(t, t+0.15, 3136+float64(i%3)*400, 3136+float64(i%3)*400, 0.05, 6, sine)
		}
	})
	m.clips[sfxStart] = mk(0.5, func(s *synth) {
		bell(s, 0, 0.2, 1047, 0.2)
		bell(s, 0.12, 0.38, 1568, 0.2)
	})
	m.clips[sfxPause] = mk(0.12, func(s *synth) {
		s.tone(0, 0.12, 1100, 880, 0.12, 5, sine)
	})
	// The end: a sad little "wah-wah" down, soft.
	m.clips[sfxOver] = mk(1.3, func(s *synth) {
		for i, f := range [...]float64{659, 587, 523} {
			t := float64(i) * 0.28
			s.tone(t, t+0.3, f, f*0.98, 0.2, 3, tri)
		}
		s.tone(0.84, 1.3, 440, 392, 0.2, 3, func(p float64) float64 { return tri(p) * (0.8 + 0.2*sine(p/40)) })
	})
	m.clips[sfxTheme] = mk(0.15, func(s *synth) {
		bell(s, 0, 0.15, 1568, 0.12)
	})
	m.clips[sfxEmpty] = mk(0.1, func(s *synth) {
		s.tone(0, 0.1, 160, 110, 0.3, 6, sine)
	})
	m.clips[sfxType] = mk(0.03, func(s *synth) {
		s.tone(0, 0.03, 2200, 2000, 0.06, 6, sine)
	})
}
