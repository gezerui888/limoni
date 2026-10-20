package widgets

import (
	"strings"
	"testing"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

type styledRows []string

func (s styledRows) Len() int            { return len(s) }
func (s styledRows) ItemAt(i int) string { return s[i] }
func (s styledRows) StyleAt(i int) cell.Style {
	if i == 0 {
		return cell.Style{Modifier: cell.ModifierDim}
	}
	return cell.Style{}
}

// A provider can style its own rows, and the selected row keeps its own
// style with the selection's merged over it. With HighlightSpacing every row
// leaves room for the symbol, so selecting one does not move its text.
func TestListRowStylesAndHighlightSpacing(t *testing.T) {
	state := NewListState()
	state.Select(1)
	l := List{Provider: styledRows{"read", "new"}, State: state, HighlightSymbol: "> ", HighlightSpacing: true,
		SelectedStyle: cell.Style{Modifier: cell.ModifierBold}}
	area := cell.NewRect(0, 0, 10, 2)
	buf := buffer.NewBuffer(area)
	l.Draw(cell.NewContext(area, cell.Style{}), buf)
	rows := strings.Split(buf.Snapshot(), "\n")
	if rows[0] != "  read    " || rows[1] != "> new     " {
		t.Fatalf("rows %q", rows)
	}
	if buf.CellAt(2, 0).Style.Modifier&cell.ModifierDim == 0 {
		t.Error("the provider's row style was not drawn")
	}
	if buf.CellAt(2, 1).Style.Modifier&cell.ModifierBold == 0 {
		t.Error("the selected row lost the selection style")
	}
}

// A focused list without a FocusedStyle still shows its focus, on the
// selected row; a FocusedStyle of its own replaces that mark.
func TestFocusedListShowsItsFocus(t *testing.T) {
	state := NewListState()
	state.Select(1)
	area := cell.NewRect(0, 0, 10, 2)
	draw := func(l List, focused string) *buffer.Buffer {
		buf := buffer.NewBuffer(area)
		ctx := cell.NewContext(area, cell.Style{})
		ctx.FocusedID = focused
		l.Draw(ctx, buf)
		return buf
	}
	mark := cell.ModifierReverse | cell.ModifierBold
	l := List{ID: "l", Items: []string{"one", "two"}, State: state}

	if got := draw(l, "").CellAt(0, 1).Style.Modifier; got&mark != 0 {
		t.Errorf("unfocused selected row drawn with %b", got)
	}
	buf := draw(l, "l")
	if got := buf.CellAt(0, 1).Style.Modifier; got&mark != mark {
		t.Errorf("focused selected row drawn with %b, want reverse bold", got)
	}
	if got := buf.CellAt(0, 0).Style.Modifier; got&mark != 0 {
		t.Errorf("an unselected row took the focus mark: %b", got)
	}

	l.FocusedStyle = cell.Style{Modifier: cell.ModifierUnderline}
	if got := draw(l, "l").CellAt(0, 1).Style.Modifier; got&cell.ModifierReverse != 0 || got&cell.ModifierUnderline == 0 {
		t.Errorf("with a FocusedStyle the row is drawn with %b", got)
	}
}
