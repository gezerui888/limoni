package main

import (
	"testing"

	"github.com/thebanri/limoni"
)

func TestQuitModalOpensAndCloses(t *testing.T) {
	u, _, b := setup(t)
	u.key(limoni.KeyEvent{Type: limoni.KeyEsc})
	if !u.qm.open || u.quit {
		t.Fatal("Esc did not ask")
	}
	// It grows over a fraction of a second, not at once.
	u.tick(clock.Add(0))
	if u.qm.p >= 1 {
		t.Fatal("opened without animating")
	}
	settle(u, b)
	if u.qm.p != 1 || u.qm.buttons[quitYes].w == 0 {
		t.Fatalf("not fully open: p %.2f, buttons %v", u.qm.p, u.qm.buttons)
	}
	// Keys do nothing else while it is up.
	u.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'f'})
	if u.full {
		t.Fatal("a key reached the player through the modal")
	}
	// Esc says no, and it shrinks away.
	u.key(limoni.KeyEvent{Type: limoni.KeyEsc})
	if u.quit || u.qm.open {
		t.Fatal("Esc did not close")
	}
	settle(u, b)
	if u.qm.visible() {
		t.Fatalf("still shown at p %.2f", u.qm.p)
	}
}

func TestQuitModalHoverAndClick(t *testing.T) {
	u, _, b := setup(t)
	u.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'q'})
	settle(u, b)
	yes := u.qm.buttons[quitYes]
	move := func(x, y int) {
		u.mouse(limoni.MouseEvent{Button: limoni.MouseNone, X: uint16(x), Y: uint16(y)})
	}
	move(yes.x+1, yes.y+1)
	settle(u, b)
	if u.qm.glow[quitYes] < 0.95 || u.qm.glow[quitNo] > 0.05 {
		t.Fatalf("glow under the pointer: %v", u.qm.glow)
	}
	move(0, 0)
	settle(u, b)
	// The highlight is also what Enter would answer, so it stays on the
	// button last pointed at, until a key moves it.
	if u.qm.glow[quitYes] < 0.95 || u.qm.glow[quitNo] > 0.05 {
		t.Fatalf("pointer gone, the last button keeps its glow: %v", u.qm.glow)
	}
	u.key(limoni.KeyEvent{Type: limoni.KeyRight})
	settle(u, b)
	if u.qm.glow[quitYes] > 0.05 || u.qm.glow[quitNo] < 0.95 {
		t.Fatalf("→ moves the glow to no: %v", u.qm.glow)
	}
	// A click on the record behind is not for the record.
	angle := u.angle
	press(u, uint16(u.dv.cx), uint16(u.dv.cy))
	release(u, uint16(u.dv.cx), uint16(u.dv.cy))
	if u.angle != angle || u.qm.open {
		t.Fatal("a click outside the card should only close it")
	}
	// Yes quits.
	u.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'q'})
	settle(u, b)
	yes = u.qm.buttons[quitYes]
	press(u, uint16(yes.x+2), uint16(yes.y+1))
	if !u.quit {
		t.Fatal("clicking yes did not quit")
	}
}

func TestQuitKeys(t *testing.T) {
	u, _, b := setup(t)
	u.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'q'})
	settle(u, b)
	// The answer to a stray Enter is no.
	u.key(limoni.KeyEvent{Type: limoni.KeyEnter})
	if u.quit || u.qm.open {
		t.Fatal("Enter on the default should be no")
	}
	settle(u, b)
	u.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'q'})
	u.key(limoni.KeyEvent{Type: limoni.KeyLeft})
	u.key(limoni.KeyEvent{Type: limoni.KeyEnter})
	if !u.quit {
		t.Fatal("← Enter should be yes")
	}
}
