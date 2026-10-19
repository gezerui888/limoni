package main

// maxCSI and maxStr cap a buffered escape sequence: past them it is passed
// on as it is, so a program that never ends one cannot grow the buffer
// forever. Strings carry whole pictures, so theirs is large.
const (
	maxCSI = 256
	maxStr = 64 << 20
)

const (
	stText = iota
	stEsc
	stCSI
	stStr    // the body of a DCS, OSC or APC string
	stStrEsc // an ESC inside one: the start of ST, or not
)

// sink receives what the parser finds in the program's output, in order.
// Each method that returns false has the sequence passed on unchanged.
type sink interface {
	// text is output to pass on unchanged.
	text(b []byte)
	// csi gets a whole control sequence, ESC [ included.
	csi(seq []byte) bool
	// sixel gets the data of a sixel picture, after its 'q'.
	sixel(data []byte)
	// osc gets the body of an OSC string, between ESC ] and its end.
	osc(body []byte) bool
	// apc gets the body of an APC string, between ESC _ and ESC \.
	apc(body []byte) bool
}

// parser splits a program's output into text, control sequences and the
// strings pictures come in (DCS for sixel, APC for kitty, OSC for iTerm2).
// It keeps its state between writes, so a sequence may arrive in pieces.
// shortcut: only 7-bit introducers and ST are seen; nobody sends the 8-bit
// forms.
type parser struct {
	state int
	kind  byte // the string's introducer: 'P', ']' or '_'
	seq   []byte
}

func (p *parser) feed(b []byte, s sink) {
	start := 0 // start of the pending run of plain text in b
	for i, c := range b {
		switch p.state {
		case stText:
			if c == 0x1b {
				if i > start {
					s.text(b[start:i])
				}
				p.state = stEsc
			}
			continue
		case stEsc:
			switch c {
			case '[':
				p.state, p.seq = stCSI, append(p.seq[:0], 0x1b, '[')
			case 'P', ']', '_':
				p.state, p.kind, p.seq = stStr, c, p.seq[:0]
			case 0x1b:
				s.text([]byte{0x1b})
			default:
				s.text([]byte{0x1b, c})
				p.state = stText
			}
		case stCSI:
			p.seq = append(p.seq, c)
			if c >= 0x40 && c <= 0x7e {
				if !s.csi(p.seq) {
					s.text(p.seq)
				}
				p.state = stText
			} else if len(p.seq) > maxCSI {
				s.text(p.seq)
				p.state = stText
			}
		case stStr:
			switch {
			case c == 0x1b:
				p.state = stStrEsc
			case c == 0x07 && p.kind == ']': // OSC may end in BEL
				p.end(s, "\a")
			default:
				p.seq = append(p.seq, c)
			}
		case stStrEsc:
			switch c {
			case '\\':
				p.end(s, "\x1b\\")
			case 0x1b:
				p.seq = append(p.seq, 0x1b)
			default:
				p.seq = append(p.seq, 0x1b, c)
				p.state = stStr
			}
		}
		if p.state == stStr && len(p.seq) > maxStr {
			s.text(append([]byte{0x1b, p.kind}, p.seq...))
			p.state = stText
		}
		start = i + 1
	}
	if p.state == stText && start < len(b) {
		s.text(b[start:])
	}
}

// end hands on a finished string: to the sink by its kind, or unchanged,
// with the ending it came with.
func (p *parser) end(s sink, term string) {
	p.state = stText
	switch p.kind {
	case 'P':
		i := 0
		for i < len(p.seq) && (p.seq[i] >= '0' && p.seq[i] <= '9' || p.seq[i] == ';') {
			i++
		}
		if i < len(p.seq) && p.seq[i] == 'q' {
			s.sixel(p.seq[i+1:])
			return
		}
	case ']':
		if s.osc(p.seq) {
			return
		}
	case '_':
		if s.apc(p.seq) {
			return
		}
	}
	out := append([]byte{0x1b, p.kind}, p.seq...)
	s.text(append(out, term...))
}
