package terminal

import (
	"testing"
	"time"

	"github.com/thebanri/limoni/core/driver"
	"github.com/thebanri/limoni/graphics"
)

// An overlay that cannot be opened must not leave pictures as blank cells:
// the frame is drawn in half blocks instead. A remote or in-memory terminal
// alone rules the overlay out: its pictures would go to the server's screen.
func TestOverlayFallsBackToHalfBlocks(t *testing.T) {
	t.Setenv("LIMONI_PROBE", "0")
	t.Setenv("LIMONI_GRAPHICS", "overlay")
	b := driver.NewPortableBackend(driver.NewMemoryTerminalIO(nil, 20, 6))
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	term, err := New(b)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()
	if got := term.Capabilities().GraphicsProto; got != graphics.ProtocolOverlay {
		t.Fatalf("LIMONI_GRAPHICS=overlay detected as %d", got)
	}

	var seen uint8
	err = term.Draw(func(f *Frame) {
		seen = f.ImageProtocol
	})
	if err != nil {
		t.Fatal(err)
	}
	if graphics.Protocol(seen) != graphics.ProtocolHalfBlock || term.Capabilities().GraphicsProto != graphics.ProtocolHalfBlock {
		t.Fatalf("frame saw protocol %d, terminal kept %d; want half blocks", seen, term.Capabilities().GraphicsProto)
	}
	if term.overlay != nil {
		t.Fatal("an overlay was opened for an in-memory terminal")
	}
}

// SetOverlayPictures waits for the terminal's answers, which may name a
// protocol of its own, and gives way to LIMONI_GRAPHICS.
func TestOverlayPicturesWaitForTheProbe(t *testing.T) {
	t.Setenv("LIMONI_PROBE", "")
	t.Setenv("LIMONI_GRAPHICS", "")
	b := driver.NewPortableBackend(driver.NewMemoryTerminalIO([]byte("\x1b[?62;22c"), 20, 6))
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	term, err := New(b)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()
	term.caps.GraphicsProto = graphics.ProtocolHalfBlock
	term.SetOverlayPictures(true)
	if term.overlayWanted() {
		t.Fatal("the overlay was wanted before the terminal answered")
	}
	term.StartEventLoop()
	select {
	case <-b.ProbeAnswered():
	case <-time.After(5 * time.Second):
		t.Fatal("the probe was never answered")
	}
	if !term.overlayWanted() {
		t.Fatal("half blocks after the answer did not ask for the overlay")
	}
	t.Setenv("LIMONI_GRAPHICS", "halfblock")
	if term.overlayWanted() {
		t.Fatal("LIMONI_GRAPHICS did not win")
	}
}
