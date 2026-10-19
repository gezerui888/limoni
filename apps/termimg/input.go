package main

import (
	"bytes"
	"slices"
)

// da1 rewrites the terminal's answer to DA1 (ESC [ ? Ps ; ... c) on its way
// to the program, adding 4: sixel graphics. Programs ask before they draw
// sixel, and the terminal itself has none to report.
//
// Everything else passes at once. Only the start of an answer cut off at
// the end of a read is held back, never a bare ESC or ESC [, so the Esc key
// and arrow keys are not delayed.
type da1 struct {
	held []byte
}

func (d *da1) filter(in []byte) []byte {
	b := append(d.held, in...)
	d.held = nil
	var out []byte
	for {
		i := bytes.Index(b, []byte("\x1b[?"))
		if i < 0 {
			return append(out, b...)
		}
		out = append(out, b[:i]...)
		b = b[i:]
		j := 3
		for j < len(b) && (b[j] >= '0' && b[j] <= '9' || b[j] == ';') {
			j++
		}
		switch {
		case j == len(b):
			d.held = slices.Clone(b) // the rest comes with the next read
			return out
		case b[j] != 'c':
			out = append(out, b[:j]...) // another ESC [ ? sequence: not ours
			b = b[j:]
			continue
		}
		params := bytes.Split(b[3:j], []byte(";"))
		out = append(out, b[:j]...)
		if !slices.ContainsFunc(params, func(p []byte) bool { return string(p) == "4" }) {
			out = append(out, ";4"...)
		}
		out = append(out, 'c')
		b = b[j+1:]
	}
}
