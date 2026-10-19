package main

import (
	"net"
	"strings"
	"testing"
	"time"
)

// A live region's frames are drawn over its rows, follow them as the output
// scrolls, and stop showing once the rows hold something else.
func TestLiveRegionFollowsItsRows(t *testing.T) {
	s, _ := testSession(t, "")
	s.emu.SetScrollbackSize(100)
	s.emu.RegisterOscHandler(oscLive, s.markLive)
	// The write scrolls the screen before the mark: lines the emulator has
	// not handed to the history yet still count.
	s.write1([]byte("a\r\nb\r\nc\r\nd\r\n$ fetch\r\nLOGO1\r\nLOGO2\r\n\x1b]7799;live;ab12;5;2\x07$ "))
	if s.live == nil || s.live.line != 5 || s.live.print == nil {
		t.Fatalf("region %+v, want line 5 with its print", s.live)
	}

	ours, theirs := net.Pipe()
	defer theirs.Close()
	s.connectLive(liveConn{id: "ab12", rw: ours, c: ours})
	go func() { _, _ = theirs.Write([]byte("\x1b[HSPIN1\r\nSPIN2")) }()
	select {
	case <-s.live.scene.Changed():
	case <-time.After(5 * time.Second):
		t.Fatal("no frame")
	}

	// LOGOn rows show SPINn, wherever scrolling has put them.
	check := func(when string, want int) {
		t.Helper()
		s.copyScreen()
		s.draw()
		seen := 0
		for y := range s.h {
			if app := rowText(s.app, y); strings.HasPrefix(app, "LOGO") {
				seen++
				if front := rowText(s.front, y); front != "SPIN"+app[4:5] {
					t.Fatalf("%s: row %d is %q over %q", when, y, front, app)
				}
			}
		}
		if seen != want {
			t.Fatalf("%s: %d region rows on screen, want %d", when, seen, want)
		}
	}
	check("printed", 2)

	// A click on the region reaches the program in the region's cells; one
	// beside it does not.
	for y := range s.h {
		if strings.HasPrefix(rowText(s.app, y), "LOGO2") {
			s.clickLive(9, y) // past the region's width: nothing
			s.clickLive(2, y)
		}
	}
	_ = theirs.SetReadDeadline(time.Now().Add(5 * time.Second))
	got := make([]byte, 32)
	n, _ := theirs.Read(got)
	if string(got[:n]) != "\x1b[<0;3;2M" {
		t.Fatalf("the program got %q for a click on its second row", got[:n])
	}
	s.write1([]byte("\r\n\r\n\r\n\r\n"))
	check("scrolled by four", 1)
	s.write1([]byte("\r\n"))
	check("scrolled off", 0)
	if s.live == nil {
		t.Fatal("the region ended when it left the screen; it is still in the history")
	}
	s.scroll = 2
	check("scrolled back", 2)
	s.scroll = 0

	// A new region takes the old one's place; written over, it stops
	// showing; erased history ends it.
	s.write1([]byte("NEW1\r\n\x1b]7799;live;cd34;4;1\x07"))
	if s.live.id != "cd34" {
		t.Fatalf("region %q, want the new one", s.live.id)
	}
	s.copyScreen()
	if !s.liveOK {
		t.Fatal("a fresh region is not intact")
	}
	s.write1([]byte("\x1b[A\rXXXX"))
	s.copyScreen()
	if s.liveOK {
		t.Fatal("rows that no longer hold the print still show the frames")
	}
	s.write1([]byte("\x1b[3J"))
	if s.live != nil {
		t.Fatal("erasing the history left the region")
	}
}

// A connection may come before the mark; a stray mark, with no program
// behind it, changes nothing on screen.
func TestLiveConnectionWaitsForItsMark(t *testing.T) {
	s, _ := testSession(t, "")
	s.emu.RegisterOscHandler(oscLive, s.markLive)
	ours, theirs := net.Pipe()
	defer theirs.Close()
	s.connectLive(liveConn{id: "late", rw: ours, c: ours})
	s.write1([]byte("x\r\n\x1b]7799;live;other;3;1\x07"))
	if s.live == nil || s.live.scene != nil || s.waiting == nil {
		t.Fatal("a connection was given to another region")
	}
	s.write1([]byte("y\r\n\x1b]7799;live;late;3;1\x07"))
	if s.live == nil || s.live.scene == nil || s.waiting != nil {
		t.Fatal("the waiting connection did not meet its mark")
	}
	for _, bad := range []string{"7799;live;z;0;1", "7799;live;z;31;1", "7799;live;;3;1", "7799;die;z;3;1"} {
		s.markLive([]byte(bad))
		if s.live == nil || s.live.id != "late" {
			t.Fatalf("%q replaced the region", bad)
		}
	}
}

// A resize reflows the lines and moves the region's rows; it is found again,
// and ends only when the window is too narrow for it.
func TestLiveRegionIsFoundAgainAfterAResize(t *testing.T) {
	s, _ := testSession(t, "")
	s.emu.SetScrollbackSize(100)
	s.emu.RegisterOscHandler(oscLive, s.markLive)
	long := strings.Repeat("x", 25) // one row at 30 columns, two at 20
	s.write1([]byte(long + "\r\nLOGO1\r\nLOGO2\r\n\x1b]7799;live;ab12;5;2\x07$ "))
	if s.live == nil || s.live.line != 1 {
		t.Fatalf("region %+v, want line 1", s.live)
	}
	resize := func(w, h int) {
		s.reflow(w, h)
		s.w, s.h = w, h
		s.relocateLive()
	}
	resize(20, 6)
	if s.live == nil || s.live.line != 2 || !s.liveIntact() {
		t.Fatalf("narrower: region %+v, want it found on line 2", s.live)
	}
	resize(30, 6)
	if s.live == nil || s.live.line != 1 || !s.liveIntact() {
		t.Fatalf("back to 30: region %+v, want line 1", s.live)
	}
	resize(4, 6)
	if s.live != nil {
		t.Fatal("a window narrower than the region kept it")
	}
}

// A link the shell prints is kept for the terminal, and Ctrl+click opens a
// web address, never another kind.
func TestCtrlClickOpensWebLinks(t *testing.T) {
	s, _ := testSession(t, "\x1b]8;;https://kernel.org/\x1b\\7.2\x1b]8;;\x1b\\ \x1b]8;;file:///etc/passwd\x1b\\file\x1b]8;;\x1b\\")
	var opened []string
	defer func(was func(string)) { openURL = was }(openURL)
	openURL = func(u string) { opened = append(opened, u) }
	if got := s.app.Content[0].Style.LinkURL(); got != "https://kernel.org/" {
		t.Fatalf("the link on screen is %q", got)
	}
	s.mouse(mouseEvent{button: 0, x: 1, y: 0})  // a plain click: no
	s.mouse(mouseEvent{button: 16, x: 1, y: 0}) // Ctrl+click: yes
	s.mouse(mouseEvent{button: 16, x: 5, y: 0}) // a file: no
	if len(opened) != 1 || opened[0] != "https://kernel.org/" {
		t.Fatalf("opened %v", opened)
	}
}
