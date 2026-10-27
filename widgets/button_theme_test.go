package widgets_test

import (
	"testing"

	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
	"github.com/thebanri/limoni/core/terminal"
	"github.com/thebanri/limoni/testkit"
	"github.com/thebanri/limoni/widgets"
)

func TestButtonFrameTheme(t *testing.T) {
	custom := cell.Style{Fg: cell.NewColorRGB(240, 80, 120), Bg: cell.NewColorRGB(40, 50, 60)}
	for _, preset := range []struct {
		name  string
		theme widgets.Theme
	}{
		{"dark", widgets.DarkTheme()}, {"light", widgets.LightTheme()},
	} {
		t.Run(preset.name, func(t *testing.T) {
			surface := preset.theme.RoleStyle("surface")
			for _, tc := range []struct {
				name    string
				focused bool
				button  widgets.Button
				want    cell.Style
			}{
				{"normal", false, widgets.Button{}, surface},
				{"focused", true, widgets.Button{}, surface.Merge(preset.theme.Focus)},
				{"explicit base", true, widgets.Button{Style: custom}, surface.Merge(custom).Merge(cell.Style{Modifier: cell.ModifierReverse | cell.ModifierBold})},
				{"bold base", true, widgets.Button{Style: cell.Style{Fg: custom.Fg, Modifier: cell.ModifierBold}}, surface.Merge(cell.Style{Fg: custom.Fg, Modifier: cell.ModifierReverse | cell.ModifierBold})},
				{"explicit focus", true, widgets.Button{FocusedStyle: custom}, surface.Merge(custom)},
				{"both explicit", true, widgets.Button{Style: cell.Style{Fg: cell.NewColorRGB(1, 2, 3)}, FocusedStyle: custom}, surface.Merge(custom)},
				{"disabled", true, widgets.Button{Disabled: true}, surface.Merge(cell.Style{Modifier: cell.ModifierDim})},
			} {
				t.Run(tc.name, func(t *testing.T) {
					term := testkit.NewTerminal(20, 1)
					tc.button.ID, tc.button.Label = "save", "Save"
					term.Draw(func(f *terminal.Frame) {
						f.SetTheme(preset.theme)
						if tc.focused {
							f.FocusManager.SetFocused("save")
						}
						f.RenderWidget(tc.button, term.Area())
					})
					if got := term.Frame().Buffer.CellAt(2, 0).Style; got != tc.want {
						t.Fatalf("button style = %+v; want %+v", got, tc.want)
					}
				})
			}
		})
	}
}

func TestButtonEmptyFocusTheme(t *testing.T) {
	for _, themed := range []bool{false, true} {
		term := testkit.NewTerminal(20, 1)
		term.Draw(func(f *terminal.Frame) {
			if themed {
				f.SetTheme(widgets.Theme{})
			}
			f.FocusManager.SetFocused("save")
			f.RenderWidget(widgets.Button{ID: "save", Label: "Save"}, term.Area())
		})
		want := cell.Style{Modifier: cell.ModifierReverse | cell.ModifierBold}
		if got := term.Frame().Buffer.CellAt(2, 0).Style; got != want {
			t.Fatalf("themed=%v: style = %+v; want %+v", themed, got, want)
		}
	}
}

func BenchmarkButtonDrawFocusedTheme(b *testing.B) {
	area := cell.Rect{Width: 20, Height: 1}
	buf := buffer.NewBuffer(area)
	theme := widgets.DarkTheme()
	ctx := cell.NewContext(area, theme.RoleStyle("surface"))
	ctx.ThemeStyle = theme.RoleStyle
	ctx.FocusedID = "save"
	button := widgets.Button{ID: "save", Label: "Save"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		button.Draw(ctx, buf)
	}
}
