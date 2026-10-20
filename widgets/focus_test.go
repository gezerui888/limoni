package widgets

import (
	"context"
	"slices"
	"testing"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// Every focusable widget looks different with the focus than without it,
// even when the application sets no focused style: a focus the tree reports
// and nobody can see is a bug (residual, #84).
func TestFocusAlwaysShows(t *testing.T) {
	checked := true
	choice := "a"
	tree := NewTreeViewState()
	tree.Select("x")
	listState := NewListState()
	listState.Select(0)
	rows := NewVirtualDataState()
	if err := rows.Refresh(context.Background(), virtualSource{}, 0, 3, 0); err != nil {
		t.Fatal(err)
	}
	rows.Select(RowID("row" + string(rune(0))))

	for _, tc := range []struct {
		name   string
		widget Widget
	}{
		{"button", Button{ID: "w", Label: "OK"}},
		{"checkbox", Checkbox{ID: "w", Label: "on", Checked: &checked}},
		{"radio", RadioButton{ID: "w", Label: "a", Value: "a", Selected: &choice}},
		{"select", Select{ID: "w", Options: []string{"one", "two"}, State: NewSelectState()}},
		{"slider", Slider{ID: "w", State: NewSliderState(5), Min: 0, Max: 10}},
		{"list", List{ID: "w", Items: []string{"one", "two"}, State: listState}},
		{"table", Table{ID: "w", Rows: []TableRow{NewRow("r1"), NewRow("r2")}, State: &TableState{Selected: 0}}},
		{"tree", TreeView{ID: "w", Roots: []TreeNode{{ID: "x", Label: "x"}, {ID: "y", Label: "y"}}, State: tree}},
		{"virtual rows", VirtualDataView{ID: "w", State: rows, Source: virtualSource{}}},
		{"split pane", SplitPane{ID: "w", First: List{Items: []string{"a"}}, Second: List{Items: []string{"b"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			area := cell.NewRect(0, 0, 12, 3)
			draw := func(focused string) (string, []cell.Style) {
				buf := buffer.NewBuffer(area)
				ctx := cell.NewContext(area, cell.Style{})
				ctx.FocusedID = focused
				tc.widget.Draw(ctx, buf)
				var styles []cell.Style
				for y := area.Y; y < area.Height; y++ {
					for x := area.X; x < area.Width; x++ {
						styles = append(styles, buf.Get(x, y).Style)
					}
				}
				return buf.Snapshot(), styles
			}
			text, styles := draw("")
			focusedText, focusedStyles := draw("w")
			if text == focusedText && slices.Equal(styles, focusedStyles) {
				t.Error("drawn the same with and without the focus")
			}
		})
	}
}
