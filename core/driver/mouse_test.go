package driver

import (
	"strings"
	"testing"
)

// Without the mouse, the setup sequence asks for no mouse reporting, and
// nothing else in it changes.
func TestSetupSequenceWithoutTheMouse(t *testing.T) {
	for _, inline := range []uint16{0, 3} {
		off := &mouseCapture{}
		off.off.Store(true)
		with, without := setupSequence(inline, &mouseCapture{}), setupSequence(inline, off)
		if !strings.Contains(with, "\x1b[?1003h") || strings.Contains(without, "\x1b[?1003h") || strings.Contains(without, "\x1b[?1006h") {
			t.Fatalf("inline %d: with %q, without %q", inline, with, without)
		}
		if strings.Replace(with, mouseOnSeq, "", 1) != without {
			t.Fatalf("inline %d: more than the mouse changed", inline)
		}
	}
}

// Before Setup, SetMouse decides what Setup sends; after it, the change is
// written at once, and only when it is a change.
func TestSetMouse(t *testing.T) {
	io := NewMemoryTerminalIO(nil, 20, 5)
	b := NewPortableBackend(io)
	if err := b.SetMouse(false); err != nil {
		t.Fatal(err)
	}
	if len(io.Output()) != 0 {
		t.Fatalf("SetMouse before Setup wrote %q", io.Output())
	}
	t.Setenv("LIMONI_PROBE", "0")
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	if out := string(io.Output()); strings.Contains(out, "\x1b[?1003h") {
		t.Fatalf("Setup asked for the mouse after SetMouse(false): %q", out)
	}
	before := len(io.Output())
	_ = b.SetMouse(false)
	if len(io.Output()) != before {
		t.Fatal("SetMouse wrote although nothing changed")
	}
	_ = b.SetMouse(true)
	if got := string(io.Output()[before:]); got != mouseOnSeq {
		t.Fatalf("SetMouse(true) after Setup wrote %q, want %q", got, mouseOnSeq)
	}
	if !b.MouseEnabled() {
		t.Fatal("MouseEnabled is false after SetMouse(true)")
	}
}

// Each mode asks for its own tracking and no other; a change after Setup
// clears the old mode first; with the mouse off a mode is only recorded; and
// teardown clears every mode, whichever was set.
func TestMouseModes(t *testing.T) {
	tracking := []string{"\x1b[?1000h", "\x1b[?1002h", "\x1b[?1003h"}
	for mode, want := range map[MouseMode]string{MouseMotion: "\x1b[?1003h", MouseDrag: "\x1b[?1002h", MouseClicks: "\x1b[?1000h"} {
		m := &mouseCapture{}
		m.mode.Store(uint32(mode))
		seq := setupSequence(0, m)
		for _, tr := range tracking {
			if strings.Contains(seq, tr) != (tr == want) {
				t.Errorf("mode %d: setup %q", mode, seq)
			}
		}
		if !strings.Contains(seq, "\x1b[?1006h") {
			t.Errorf("mode %d: no SGR coordinates", mode)
		}
	}

	t.Setenv("LIMONI_PROBE", "0")
	io := NewMemoryTerminalIO(nil, 20, 5)
	b := NewPortableBackend(io)
	if err := b.SetMouseMode(MouseClicks); err != nil || len(io.Output()) != 0 {
		t.Fatalf("SetMouseMode before Setup wrote %q (%v)", io.Output(), err)
	}
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	if out := string(io.Output()); !strings.Contains(out, "\x1b[?1000h") || strings.Contains(out, "\x1b[?1003h") {
		t.Fatalf("Setup after SetMouseMode(MouseClicks): %q", out)
	}

	written := func(fn func()) string {
		before := len(io.Output())
		fn()
		return string(io.Output()[before:])
	}
	if got := written(func() { _ = b.SetMouseMode(MouseClicks) }); got != "" {
		t.Errorf("an unchanged mode wrote %q", got)
	}
	if got := written(func() { _ = b.SetMouseMode(MouseDrag) }); got != mouseOffSeq+"\x1b[?1002h\x1b[?1006h" {
		t.Errorf("SetMouseMode(MouseDrag) wrote %q", got)
	}
	_ = b.SetMouse(false)
	if got := written(func() { _ = b.SetMouseMode(MouseMotion) }); got != "" {
		t.Errorf("with the mouse off SetMouseMode wrote %q", got)
	}
	if got := written(func() { _ = b.SetMouse(true) }); got != mouseOnSeq || b.MouseMode() != MouseMotion {
		t.Errorf("SetMouse(true) wrote %q in mode %d", got, b.MouseMode())
	}
	_ = b.SetMouseMode(MouseClicks)
	if got := written(func() { _ = b.Close() }); !strings.Contains(got, mouseOffSeq) {
		t.Errorf("Close did not clear every tracking mode: %q", got)
	}
}
