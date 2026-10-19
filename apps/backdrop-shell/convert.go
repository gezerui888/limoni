package main

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/thebanri/limoni/core/cell"
)

// convertCell turns a cell of the emulated screen into a Limoni cell.
//
// The colours the shell asked for are kept as it asked for them: a palette
// colour stays a palette colour, so it is still drawn in the user's theme. A
// cell with no background colour stays without one, which is what lets the
// scene show through it (terminal.ComposeBackdrop). A link (OSC 8) is kept,
// for the terminal to show and for Ctrl+click (openLink).
func convertCell(c *uv.Cell) cell.Cell {
	if c == nil {
		return cell.Cell{Content: ' '}
	}
	out := cell.Cell{Style: cell.Style{
		Fg:       convertColor(c.Style.Fg),
		Bg:       convertColor(c.Style.Bg),
		Modifier: convertAttrs(c.Style),
	}}
	if c.Link.URL != "" {
		out.Style.Link = cell.Link(c.Link.URL)
	}
	switch {
	case c.Width == 0 && c.Content == "":
		// The second column of a wide character.
		out.Content = cell.RuneContinuation
	case c.Content == "" || c.Content == " ":
		out.Content = ' '
	default:
		out.Content = cell.ClusterContent(c.Content, max(1, c.Width))
	}
	return out
}

func convertColor(c color.Color) cell.Color {
	switch v := c.(type) {
	case nil:
		return cell.NewColorDefault()
	case ansi.BasicColor:
		return cell.NewColorANSI(uint8(v))
	case ansi.IndexedColor:
		return cell.NewColorANSI(uint8(v))
	}
	r, g, b, _ := c.RGBA()
	return cell.NewColorRGB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func convertAttrs(s uv.Style) cell.Modifier {
	var m cell.Modifier
	a := s.Attrs
	if a&uv.AttrBold != 0 {
		m |= cell.ModifierBold
	}
	if a&uv.AttrFaint != 0 {
		m |= cell.ModifierDim
	}
	if a&uv.AttrItalic != 0 {
		m |= cell.ModifierItalic
	}
	if a&uv.AttrBlink != 0 {
		m |= cell.ModifierBlink
	}
	if a&uv.AttrReverse != 0 {
		m |= cell.ModifierReverse
	}
	if a&uv.AttrConceal != 0 {
		m |= cell.ModifierHidden
	}
	if a&uv.AttrStrikethrough != 0 {
		m |= cell.ModifierStrikethrough
	}
	switch s.Underline {
	case uv.UnderlineNone:
	case uv.UnderlineCurly:
		m |= cell.ModifierUndercurl
	default:
		m |= cell.ModifierUnderline
	}
	return m
}
