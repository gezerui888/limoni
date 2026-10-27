package limoni

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
	"github.com/thebanri/limoni/core/driver"
	"github.com/thebanri/limoni/core/terminal"
)

func TestAppOptions(t *testing.T) {
	// Default options
	var defaultCfg appConfig
	if defaultCfg.catchCtrlC {
		t.Fatalf("expected catchCtrlC to default to false")
	}

	// WithCatchCtrlC(true)
	var cfg1 appConfig
	opt1 := WithCatchCtrlC(true)
	opt1(&cfg1)
	if !cfg1.catchCtrlC {
		t.Fatalf("expected catchCtrlC to be true")
	}

	// WithoutDefaultQuitKeys()
	var cfg2 appConfig
	opt2 := WithoutDefaultQuitKeys()
	opt2(&cfg2)
	if !cfg2.catchCtrlC {
		t.Fatalf("expected WithoutDefaultQuitKeys to enable catchCtrlC")
	}
}

// testApp builds an App from an appConfig, the way the options would.
func testApp(term *Terminal, cfg appConfig) *App {
	return &App{term: term, cfg: cfg, wakeup: make(chan struct{}, 1)}
}

type pulse struct{ renders atomic.Int32 }

func (p *pulse) Interval() time.Duration { return 10 * time.Millisecond }

func (p *pulse) Render(dst *buffer.Buffer, t time.Duration) {
	p.renders.Add(1)
	v := uint8(t / time.Millisecond)
	for i := range dst.Content {
		dst.Content[i] = cell.Cell{Content: ' ', Style: cell.Style{Bg: cell.NewColorRGB(0, v, 0)}}
	}
}

// WithBackdrop keeps the scene moving in an App that draws only on events,
// without calling the application function for it.
func TestRunMovesTheBackdropWithoutCallingTheApplication(t *testing.T) {
	t.Setenv("LIMONI_PROBE", "0")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("LIMONI_BACKDROP", "")
	in := make(chan []byte, 4)
	b := driver.NewPortableBackend(&feedIO{in: in})
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(b)
	if err != nil {
		t.Fatal(err)
	}
	scene := &pulse{}
	var frames atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewApp(term, WithBackdrop(scene)).Run(ctx, func(f *Frame, ev *Event) bool {
			frames.Add(1)
			return true
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Run = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Run did not stop")
		}
		_ = term.Close()
	})

	waitFor(t, func() bool { return frames.Load() == 1 })
	time.Sleep(200 * time.Millisecond)
	if n := frames.Load(); n != 1 {
		t.Fatalf("the application drew %d frames; the backdrop should move without it", n)
	}
	if n := scene.renders.Load(); n < 5 {
		t.Fatalf("the backdrop was drawn %d times in 200ms at 100 fps", n)
	}
}

// Without WithFPS an App draws only when something happens, so it costs
// nothing while idle. One of those things is the terminal answering the
// capability probe after the first frame, over SSH especially: the frame on
// screen may have been encoded for another terminal, and without input
// nothing else would draw it again.
func TestRunIdlesAndDrawsWhenTheProbeIsAnswered(t *testing.T) {
	t.Setenv("LIMONI_PROBE", "")
	in := make(chan []byte, 4)
	b := driver.NewPortableBackend(&feedIO{in: in})
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(b)
	if err != nil {
		t.Fatal(err)
	}
	var frames atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewApp(term).Run(ctx, func(f *Frame, ev *Event) bool {
			frames.Add(1)
			return true
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Run = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Run did not stop")
		}
		_ = term.Close()
	})

	waitFor(t, func() bool { return frames.Load() == 1 })
	time.Sleep(150 * time.Millisecond)
	if n := frames.Load(); n != 1 {
		t.Fatalf("an idle App drew %d frames; want only the first", n)
	}
	in <- []byte("\x1b[?62;22c") // DA1, the sentinel
	waitFor(t, func() bool { return frames.Load() == 2 })
}

// feedIO delivers input when the test sends it.
type feedIO struct {
	in      chan []byte
	pending []byte
}

func (f *feedIO) Read(p []byte) (int, error) {
	if len(f.pending) == 0 {
		f.pending = <-f.in
	}
	n := copy(p, f.pending)
	f.pending = f.pending[n:]
	return n, nil
}

func (f *feedIO) Write(p []byte) (int, error)   { return len(p), nil }
func (f *feedIO) Size() (uint16, uint16, error) { return 40, 10, nil }

// Two applications in one process — two SSH sessions, say — each with its
// own terminal. Waking one must not redraw the other, and cancelling one must
// leave the other running. With the package-level wakeup channel both of
// these failed: a wakeup went to whichever loop read it first.
func TestAppsInOneProcessAreIsolated(t *testing.T) {
	type session struct {
		app    *App
		frames atomic.Int32
		cancel context.CancelFunc
		done   chan error
	}
	start := func() *session {
		t.Setenv("LIMONI_PROBE", "0")
		b := driver.NewPortableBackend(driver.NewMemoryTerminalIO(nil, 40, 10))
		if err := b.Setup(); err != nil {
			t.Fatal(err)
		}
		term, err := terminal.New(b)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = term.Close() })
		s := &session{app: NewApp(term), done: make(chan error, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go func() {
			s.done <- s.app.Run(ctx, func(f *Frame, ev *Event) bool {
				s.frames.Add(1)
				return true
			})
		}()
		waitFor(t, func() bool { return s.frames.Load() == 1 }) // initial frame
		return s
	}
	a, b := start(), start()

	// Alternate, so that a shared channel cannot pass by luck: the goroutine
	// that started waiting first would take every wakeup.
	for round := 0; round < 10; round++ {
		target, other := a, b
		if round%2 == 0 {
			target, other = b, a
		}
		wantTarget, wantOther := target.frames.Load()+1, other.frames.Load()
		target.app.Wakeup()
		waitFor(t, func() bool { return target.frames.Load() == wantTarget })
		time.Sleep(5 * time.Millisecond)
		if n := other.frames.Load(); n != wantOther {
			t.Fatalf("round %d: waking one app drew %d frames in the other", round, n-wantOther)
		}
	}

	wantA, wantB := a.frames.Load()+1, b.frames.Load()+1
	Wakeup() // the package-level call reaches every running app
	waitFor(t, func() bool { return a.frames.Load() == wantA && b.frames.Load() == wantB })

	a.cancel()
	select {
	case err := <-a.done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled app returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled app did not stop")
	}
	wantB = b.frames.Load() + 1
	b.app.Wakeup()
	waitFor(t, func() bool { return b.frames.Load() == wantB })

	b.cancel()
	<-b.done
	running.mu.RLock()
	n := len(running.apps)
	running.mu.RUnlock()
	if n != 0 {
		t.Fatalf("%d apps still registered after both returned", n)
	}
}

// A context that is already cancelled stops before drawing anything.
func TestAppRunWithCancelledContext(t *testing.T) {
	b := driver.NewPortableBackend(driver.NewMemoryTerminalIO(nil, 20, 5))
	term, err := terminal.New(b)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	drew := false
	err = NewApp(term).Run(ctx, func(*Frame, *Event) bool { drew = true; return true })
	if !errors.Is(err, context.Canceled) || drew {
		t.Fatalf("Run = %v, drew = %v", err, drew)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
