package terminal

import (
	"strings"
	"testing"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

func textFrame(w, h uint16) *Frame {
	return &Frame{Buffer: buffer.NewBuffer(cell.NewRect(0, 0, w, h))}
}

func row(f *Frame, y int) string {
	snap := strings.Split(f.Buffer.Snapshot(), "\n")
	return strings.TrimRight(snap[y], " ")
}

func TestFrameText(t *testing.T) {
	f := textFrame(10, 2)
	red := cell.Style{Fg: cell.NewColorANSI(1)}
	x := f.Text(0, 0, "err: ", red)
	if x = f.Text(x, 0, "disk full", cell.Style{}); x != 14 {
		t.Fatalf("Text returned %d, want 14 (where the text ends, drawn or not)", x)
	}
	if got := row(f, 0); got != "err: disk" { // the last column is the space after "disk"
		t.Fatalf("row 0 = %q", got)
	}
	if f.Buffer.Get(0, 0).Style != red || f.Buffer.Get(5, 0).Style != (cell.Style{}) {
		t.Fatal("each run keeps its own style")
	}

	// Off the left edge: the part inside is drawn; a wide character the
	// edge would halve is left out, and its second column stays blank.
	f = textFrame(10, 1)
	f.Text(-2, 0, "abcdef", cell.Style{})
	f.Text(4, 0, "日本", cell.Style{})
	if got := row(f, 0); got != "cdef日 本" { // a snapshot shows a wide character's second column as a blank
		t.Fatalf("clipped left: %q", got)
	}
	f = textFrame(10, 1)
	if x := f.Text(-1, 0, "日x", cell.Style{}); x != 2 || row(f, 0) != " x" {
		t.Fatalf("halved wide character: %q, end %d", row(f, 0), x)
	}

	// Off the frame entirely: nothing, no panic.
	f.Text(0, 5, "below", cell.Style{})
	f.Text(0, -1, "above", cell.Style{})
	f.Text(20, 0, "right", cell.Style{})
	f.Text(-30, 0, "left", cell.Style{})
}

func TestFrameTextFit(t *testing.T) {
	f := textFrame(12, 3)
	f.Text(0, 0, "a long old label", cell.Style{})
	if x := f.TextFit(0, 0, 6, "ok", cell.Style{}); x != 6 {
		t.Fatalf("TextFit returned %d", x)
	}
	f.TextFit(0, 1, 6, "too long here", cell.Style{})
	f.TextFit(0, 2, 6, "🇹🇷🇹🇷🇹🇷🇹🇷", cell.Style{}) // flags are cut whole, never into letters
	for y, want := range []string{"ok     old l", "too l…", "🇹🇷 🇹🇷 …"} {
		if got := row(f, y); got != want {
			t.Errorf("row %d = %q, want %q", y, got, want)
		}
	}
}

func TestFrameTextDoesNotAllocate(t *testing.T) {
	f := textFrame(40, 2)
	if n := testing.AllocsPerRun(100, func() {
		f.Text(-1, 0, "héllo 日本 🇹🇷", cell.Style{})
		f.TextFit(0, 1, 8, "a label too long", cell.Style{})
	}); n != 0 {
		t.Fatalf("%v allocations a call", n)
	}
}
