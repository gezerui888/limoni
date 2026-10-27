package limoni

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thebanri/limoni/core/driver"
	"github.com/thebanri/limoni/core/terminal"
	"github.com/thebanri/limoni/widgets"
)

func TestLimoniColorsAndStyles(t *testing.T) {
	// 1. Hex Color parsing
	c1 := Hex("#FF0000")
	r, g, b := c1.RGB()
	if r != 255 || g != 0 || b != 0 {
		t.Fatalf("Hex '#FF0000' RGB mismatch: %d, %d, %d", r, g, b)
	}

	// 2. Short hex parsing
	c2 := Hex("#0F0")
	r, g, b = c2.RGB()
	if r != 0 || g != 255 || b != 0 {
		t.Fatalf("Hex '#0F0' RGB mismatch: %d, %d, %d", r, g, b)
	}

	// 3. Style fluent chaining
	st := NewStyle().
		WithFg(RGB(100, 150, 200)).
		WithBg(ColorBlack).
		Bold().
		Underline()

	if !st.HasModifier(ModifierBold) {
		t.Fatalf("expected bold modifier")
	}
	if !st.HasModifier(ModifierUnderline) {
		t.Fatalf("expected underline modifier")
	}

	// 4. Standalone style helpers
	bSt := Bold()
	if !bSt.HasModifier(ModifierBold) {
		t.Fatalf("expected bold from Bold()")
	}

	fgSt := Fg(ColorRed)
	if fgSt.Fg != ColorRed {
		t.Fatalf("expected red foreground from Fg()")
	}
}

func TestLimoniLayoutSplits(t *testing.T) {
	area := NewRect(0, 0, 100, 50)

	// Horizontal split
	hChunks := SplitHorizontal(area, Percentage(30), Percentage(70))
	if len(hChunks) != 2 {
		t.Fatalf("expected 2 horizontal chunks, got %d", len(hChunks))
	}
	if hChunks[0].Width != 30 || hChunks[1].Width != 70 {
		t.Fatalf("unexpected horizontal widths: %d, %d", hChunks[0].Width, hChunks[1].Width)
	}

	// Vertical split
	vChunks := SplitVertical(area, Fixed(5), Fill())
	if len(vChunks) != 2 {
		t.Fatalf("expected 2 vertical chunks, got %d", len(vChunks))
	}
	if vChunks[0].Height != 5 || vChunks[1].Height != 45 {
		t.Fatalf("unexpected vertical heights: %d, %d", vChunks[0].Height, vChunks[1].Height)
	}
}

func TestLimoniWidgetBuilders(t *testing.T) {
	// Block
	blk := NewBlock().
		WithTitle("Başlık").
		WithTitleAlign(AlignCenter).
		Rounded().
		WithPadding(1, 2, 1, 2)

	if blk.Title != "Başlık" || blk.BorderSymbols != SymbolsRounded {
		t.Fatalf("unexpected block configuration: %+v", blk)
	}

	// Paragraph
	p := NewParagraph("Türkçe metin ve emoji 🚀").WithWrap(true)
	if p.Text != "Türkçe metin ve emoji 🚀" || !p.Wrap {
		t.Fatalf("unexpected paragraph: %+v", p)
	}

	// List
	lst := NewList("Öğe 1", "Öğe 2").WithHighlightSymbol("> ")
	if len(lst.Items) != 2 || lst.HighlightSymbol != "> " {
		t.Fatalf("unexpected list: %+v", lst)
	}

	// Table
	tbl := NewTable().
		WithHeaders("İsim", "Durum").
		WithRow("Giriş", "Aktif")
	if tbl.Header == nil || len(tbl.Rows) != 1 {
		t.Fatalf("unexpected table: %+v", tbl)
	}

	// TextInput
	inp := NewTextInput("username").WithPlaceholder("Kullanıcı adı")
	if inp.ID != "username" || inp.Placeholder != "Kullanıcı adı" {
		t.Fatalf("unexpected text input: %+v", inp)
	}

	// Markdown
	md := NewMarkdown("# Başlık\n- Madde 1")
	if md.Content != "# Başlık\n- Madde 1" {
		t.Fatalf("unexpected markdown: %+v", md)
	}
}

func TestLimoniUnicodeWidths(t *testing.T) {
	// Turkish characters should have width 1
	turkish := "çğışöüÇĞİŞÖÜ"
	if w := StringWidth(turkish); w != 12 {
		t.Fatalf("StringWidth(%q) = %d; want 12", turkish, w)
	}

	// BMP Wide Emojis should have width 2
	emojis := []rune{'✅', '❌', '⚡', '✨', '⭐', '☕', '⏰', '⏳', '♿'}
	for _, em := range emojis {
		if w := RuneWidth(em); w != 2 {
			t.Fatalf("RuneWidth(%c) = %d; want 2", em, w)
		}
	}

	// BMP Symbols with East Asian Width Neutral/Narrow should have width 1
	narrowSymbols := []rune{'✓', '✔', '⚠', '★', '☆', '⚙', '│', '─'}
	for _, sym := range narrowSymbols {
		if w := RuneWidth(sym); w != 1 {
			t.Fatalf("RuneWidth(%c) = %d; want 1", sym, w)
		}
	}

	// SMP Emojis should have width 2
	smpEmojis := []rune{'🚀', '🔥', '🎉', '🔴', '🟢', '🔘', '💡', '📦'}
	for _, em := range smpEmojis {
		if w := RuneWidth(em); w != 2 {
			t.Fatalf("RuneWidth(%c) = %d; want 2", em, w)
		}
	}
}

func TestLimoniComposableLego(t *testing.T) {
	// Test creating composable components directly via root package `limoni`
	label := Label("Hello Lego", Bold())
	padded := PadAll(label, 1)
	bordered := Border(padded, SymbolsRounded, Fg(ColorCyan))

	table := NewTable().WithHeaders("A", "B").WithRow("1", "2")
	adaptedTable := AsComponent(table)

	stack := VStack(
		FixedSize(20, 3, bordered),
		Flex(1, adaptedTable),
		Center(Label("Centered Footer")),
		AlignComponent(Label("Right aligned"), AlignRight, AlignMiddle),
		AlignComponent(Label("AlignRight test"), AlignRight, AlignBottom),
	).WithGap(0)

	props := stack.LayoutInfo(NewRect(0, 0, 80, 24))
	if props.MinWidth == 0 {
		t.Fatalf("expected non-zero min width for composable stack")
	}
}

func TestLimoniComposableAdvancedExtensions(t *testing.T) {
	// 1. ZStack
	z := ZStack(
		Label("Background Layer"),
		Center(FixedSize(10, 2, Label("Modal Dialog"))),
	)
	if z == nil {
		t.Fatalf("expected non-nil ZStack")
	}

	// 2. StackLayout with Justify and AlignItems
	v := VStack(Label("A"), Label("B")).
		WithJustify(JustifySpaceBetween).
		WithAlignItems(AlignItemsCenter)
	if v == nil {
		t.Fatalf("expected non-nil VStack")
	}

	h := HStack(Label("X"), Label("Y")).
		WithJustify(JustifySpaceEvenly).
		WithAlignItems(AlignItemsEnd)
	if h == nil {
		t.Fatalf("expected non-nil HStack")
	}

	// 3. Declarative Conditionals (When, Match, Empty)
	whenTrue := When(true, Label("Yes"), Label("No"))
	whenFalse := When(false, Label("Yes"), Label("No"))
	matched := Match("mode", map[string]Component{
		"mode": Label("Matched"),
	}, Label("Default"))
	empty := Empty()

	if whenTrue == nil || whenFalse == nil || matched == nil || empty == nil {
		t.Fatalf("expected non-nil conditional components")
	}

	// 4. Style Context Cascading
	styled := WithStyle(Bold(),
		WithForeground(ColorGreen,
			WithBackground(ColorBlack, Label("Cascaded")),
		),
	)
	if styled == nil {
		t.Fatalf("expected non-nil styled component")
	}

	// 5. Interactive Event Propagation
	clicked := false
	btn := OnClick(Label("Click"), func(ev MouseEvent) {
		clicked = true
	})

	ctx := NewContext(NewRect(0, 0, 10, 3), NewStyle())
	ev := &Event{
		Type:  EventMouse,
		Mouse: MouseEvent{X: 2, Y: 1, Button: MouseLeft},
	}
	if !DispatchEvent(btn, ctx, ev) {
		t.Fatalf("expected click to be handled")
	}
	if !clicked {
		t.Fatalf("expected clicked to be true")
	}
}

// Inline mode has to stay out of the alternate screen and address the frame
// relatively, or it paints over the user's scrollback. Asserting on the bytes
// the driver actually wrote is the only way to know.
func TestInlineModeEmitsRelativeFrames(t *testing.T) {
	io := driver.NewMemoryTerminalIO(nil, 60, 20)
	backend := driver.NewPortableBackend(io)
	backend.SetInline(5)
	if err := backend.Setup(); err != nil {
		t.Fatalf("setup: %v", err)
	}
	term, err := terminal.New(backend)
	if err != nil {
		t.Fatalf("terminal: %v", err)
	}
	term.SetInline(5)

	var stop atomic.Bool
	var frames atomic.Int32
	app := testApp(term, appConfig{catchCtrlC: true, inlineHeight: 5})
	done := make(chan error, 1)
	go func() {
		done <- app.Run(context.Background(), func(f *Frame, ev *Event) bool {
			if frames.Add(1) > 1 && stop.Load() {
				return false
			}
			f.RenderWidget(widgets.NewLabel("inline frame"), NewRect(0, 0, 20, 1))
			return true
		})
	}()

	// Let the first frame render, then ask the loop to finish.
	stop.Store(true)
	app.Wakeup()
	if err := <-done; err != nil {
		t.Fatalf("run loop: %v", err)
	}
	_ = term.Close()

	out := string(string(io.Output()))

	if strings.Contains(out, "\x1b[?1049h") {
		t.Error("inline mode switched to the alternate screen")
	}
	if strings.Contains(out, "\x1b[?1049l") {
		t.Error("inline mode left the alternate screen it never entered")
	}
	// A full-screen clear in inline mode wipes the user's scrollback.
	if strings.Contains(out, "\x1b[2J") {
		t.Errorf("inline mode cleared the whole screen: %q", out)
	}
	// Absolute addressing would land on whatever row the terminal has scrolled
	// to, not the application's band.
	if strings.Contains(out, "\x1b[1;1H") {
		t.Errorf("inline frame used absolute cursor addressing: %q", out)
	}
	// Space is reserved by printing rows and stepping back up.
	if !strings.Contains(out, "\x1b[5A") {
		t.Errorf("inline setup did not reserve and rewind 5 rows: %q", out)
	}
	if !strings.Contains(out, "inline frame") {
		t.Errorf("frame content missing from output: %q", out)
	}
	// On exit the cursor is parked below the band so a prompt lands after it.
	if !strings.Contains(out, "\x1b[5B") {
		t.Errorf("inline teardown did not move past the frame: %q", out)
	}
	if frames.Load() == 0 {
		t.Error("application never rendered")
	}
}

// Full-screen mode must keep using the alternate screen.
func TestFullScreenModeStillUsesTheAlternateScreen(t *testing.T) {
	io := driver.NewMemoryTerminalIO(nil, 60, 20)
	backend := driver.NewPortableBackend(io)
	if err := backend.Setup(); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.Contains(string(io.Output()), "\x1b[?1049h") {
		t.Errorf("full-screen setup did not enter the alternate screen: %q", string(io.Output()))
	}
	_ = backend.Close()
	if !strings.Contains(string(io.Output()), "\x1b[?1049l") {
		t.Errorf("full-screen teardown did not leave the alternate screen: %q", string(io.Output()))
	}
}

// WithTitle must reach every entry point, not only the package-level Run:
// NewApp(...).Run and RunWithContext go through App.Run, which is where the
// title is applied. The saved title is popped on the way out, so quitting does
// not leave the application's name on the user's terminal.
func TestWithTitleSetsAndRestoresTheTitle(t *testing.T) {
	t.Setenv("LIMONI_PROBE", "0")
	io := driver.NewMemoryTerminalIO(nil, 40, 10)
	b := driver.NewPortableBackend(io)
	if err := b.Setup(); err != nil {
		t.Fatal(err)
	}
	term, err := terminal.New(b)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()

	var cfg appConfig
	WithTitle("zest — app.log")(&cfg)
	app := testApp(term, cfg)
	if err := app.Run(context.Background(), func(*Frame, *Event) bool { return false }); err != nil {
		t.Fatalf("run: %v", err)
	}

	out := io.Output()
	save, set, restore := []byte("\x1b[22;2t"), []byte("\x1b]2;zest — app.log\x07"), []byte("\x1b[23;2t")
	i, j, k := bytes.Index(out, save), bytes.Index(out, set), bytes.Index(out, restore)
	if i < 0 || j < 0 || k < 0 {
		t.Fatalf("save=%d set=%d restore=%d in %q", i, j, k, out)
	}
	if !(i < j && j < k) {
		t.Errorf("order should be save, set, restore: %d, %d, %d", i, j, k)
	}
}
