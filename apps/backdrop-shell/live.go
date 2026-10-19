package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/thebanri/limoni/core/cell"
)

// A live region: rows of the shell's output that a program goes on drawing
// after it has exited — a system fetch's spinning logo — and that scroll with
// the output like any other text.
//
// The program prints its picture as usual, then OSC 7799 ; live ; id ; w ; h
// (BEL or ST): the w×h cells ending on the row above the cursor are its,
// named id. It then streams frames for them over this wrapper's control
// socket, named in envSocket: "live id pid" as the first line, then what a
// -scene-cmd program writes. The sequence itself starts nothing. Anything
// printed to the screen can send it — a file being cat — but only a process
// of this user reaches the socket, so a stray one only marks rows that never
// move.
//
// The frames are shown while the rows are on screen and still hold what was
// printed there; otherwise the program is stopped, as a hidden scene is.
// After a resize the rows are found again, since a reflow moves lines. It is
// ended when its rows leave the history, when another region takes its
// place, when the window gets narrower than it (its rows wrap) or when the
// history is erased.
// One region at a time: a second fetch leaves the first one's picture
// as printed. A left click on its rows goes to the program too (clickLive).

const (
	oscLive   = 7799
	envSocket = "LIMONI_BACKDROP_SOCKET"
)

type liveRegion struct {
	id    string
	line  int // its first row, as a line of history and screen (spot.line)
	w, h  int
	print []cell.Cell // what was printed there, taken once the rows are written
	scene *cmdScene   // the program's frames; nil until it connects
	in    io.Writer   // to the program: clicks on the region
}

// liveConn is a program's connection, waiting for its region.
type liveConn struct {
	id  string
	pid int
	rw  io.ReadWriter
	c   io.Closer
}

// markLive takes OSC 7799: a region ending on the row above the cursor.
func (s *session) markLive(data []byte) bool {
	f := strings.Split(string(data), ";")
	if len(f) != 5 || f[1] != "live" || f[2] == "" || s.emu.IsAltScreen() {
		return true
	}
	w, errW := strconv.Atoi(f[3])
	h, errH := strconv.Atoi(f[4])
	if errW != nil || errH != nil || w < 1 || w > s.w || h < 1 || h > 500 {
		return true
	}
	// Lines this write scrolled off are still in the emulator's scrollback;
	// they join hist once it ends.
	line := len(s.hist) + s.emu.Scrollback().Len() + s.emu.CursorPosition().Y - h
	if line < 0 {
		return true
	}
	s.endLive()
	s.live = &liveRegion{id: f[2], line: line, w: w, h: h}
	if s.waiting != nil && s.waiting.id == s.live.id {
		s.attachLive(*s.waiting)
		s.waiting = nil
	}
	return true
}

// connectLive takes a program's connection: its region's frames if the
// region is marked, or it waits for the mark, which may come second.
func (s *session) connectLive(lc liveConn) {
	if s.live != nil && s.live.scene == nil && s.live.id == lc.id {
		s.attachLive(lc)
		return
	}
	if s.waiting != nil {
		s.waiting.c.Close()
	}
	s.waiting = &lc
}

func (s *session) attachLive(lc liveConn) {
	r := s.live
	c := &cmdScene{w: r.w, h: r.h, frame: make([]cell.Cell, r.w*r.h), changed: make(chan struct{}, 1),
		cellOf: convertCell, conn: lc.c}
	if lc.pid > 0 {
		c.proc, _ = os.FindProcess(lc.pid)
	}
	go c.read(0, lc.rw, r.w, r.h)
	r.scene, r.in = c, lc.rw
}

// clickLive passes a left click on the region's rows to its program, as an
// SGR mouse report in the region's own cells (1-based): a fetch can turn a
// click on a colour into the theme.
func (s *session) clickLive(x, y int) {
	if !s.liveSeen() || s.live.in == nil {
		return
	}
	r := s.live
	row := s.history() - s.scroll + y - r.line
	if row < 0 || row >= r.h || x >= r.w {
		return
	}
	report := fmt.Sprintf("\x1b[<0;%d;%dM", x+1, row+1)
	go func() { _, _ = io.WriteString(r.in, report) }() // never hold the screen up
}

func (s *session) endLive() {
	if s.live != nil && s.live.scene != nil {
		s.live.scene.Close()
	}
	s.live = nil
}

// liveCells is what the region's rows hold now.
func (s *session) liveCells(into []cell.Cell) []cell.Cell {
	r := s.live
	into = into[:0]
	for y := range r.h {
		for x := range r.w {
			into = append(into, convertCell(s.cellAt(spot{line: r.line + y, x: x})))
		}
	}
	return into
}

// liveIntact reports whether the region's rows still hold the print: not
// written over, cleared, or covered by a full-screen program.
func (s *session) liveIntact() bool {
	r := s.live
	return r != nil && r.print != nil && s.liveAt(r.line)
}

// liveAt reports whether the rows from line on hold the region's print.
func (s *session) liveAt(line int) bool {
	r := s.live
	if line < 0 || line+r.h > s.history()+s.h {
		return false
	}
	for y := range r.h {
		for x := range r.w {
			if convertCell(s.cellAt(spot{line: line + y, x: x})) != r.print[y*r.w+x] {
				return false
			}
		}
	}
	return true
}

// relocateLive finds the region again after a reflow has moved the lines:
// the rows holding its print nearest to where they were.
func (s *session) relocateLive() {
	r := s.live
	if r == nil {
		return
	}
	if r.print == nil || r.w > s.w {
		s.endLive()
		return
	}
	best := -1
	for line := range s.history() + s.h - r.h + 1 {
		if s.liveAt(line) && (best < 0 || abs(line-r.line) < abs(best-r.line)) {
			best = line
		}
	}
	if best < 0 {
		s.endLive()
		return
	}
	r.line = best
}

func abs(n int) int { return max(n, -n) }

// liveSeen reports whether any of the region's rows is on screen.
func (s *session) liveSeen() bool {
	r := s.live
	top := s.history() - s.scroll
	return s.liveOK && r.line+r.h > top && r.line < top+s.h
}

// drawLive puts the region's last frame over its rows on screen.
func (s *session) drawLive() {
	if !s.liveSeen() || s.live.scene == nil {
		return
	}
	r, c := s.live, s.live.scene
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready {
		return
	}
	top := s.history() - s.scroll
	w := min(r.w, s.w)
	for y := range s.h {
		if row := top + y - r.line; row >= 0 && row < r.h {
			copy(s.front.Content[y*s.w:y*s.w+w], c.frame[row*r.w:row*r.w+w])
		}
	}
}
