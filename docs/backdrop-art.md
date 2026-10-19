# Your own backgrounds: ASCII art, animation and pictures

A backdrop is whatever sits behind the text in your terminal. Limoni ships
four scenes (aurora, city, starfield, synthwave), and this guide is about
making your own: a picture, a piece of ASCII art, ASCII art that moves, or
a program that draws a scene of its own.

Everything here works in two places:

- **behind your shell**, with [`backdrop-shell`](../apps/backdrop-shell/README.md),
  in any terminal you open;
- **behind a Limoni application**, with `limoni.WithBackdrop(...)`.

> Türkçesi: [docs/tr/backdrop-art.md](tr/backdrop-art.md)

---

## 1. Try it in ten seconds

```bash
# The example art, if you installed with curl rather than from a checkout:
mkdir -p ~/.config/limoni/art && cd ~/.config/limoni/art
for f in cat rain bird clouds lemon; do
  curl -fsSLO https://raw.githubusercontent.com/thebanri/limoni/main/apps/backdrop-shell/art/$f.txt
done

backdrop-shell -art cat.txt                               # a cat that blinks
backdrop-shell -art rain.txt                              # rain
backdrop-shell -image ~/Pictures/wallpaper.jpg            # a picture
```

Type `exit` to leave. To keep one for every new terminal:

```bash
backdrop-shell enable -art ~/my-art.txt
backdrop-shell enable -image ~/Pictures/wallpaper.jpg -opacity 0.3
backdrop-shell enable -scene aurora                        # back to a built-in scene
```

`backdrop-shell status` shows what is set; `backdrop-shell disable` turns it
off. Changes apply at once to every open terminal. To make the background
quieter or stronger at any time:

```bash
backdrop-shell opacity 0.3     # or +0.1 / -0.1 to step it
```

---

## 2. Your first art file

An art file is a plain text file. The text **is** the picture:

```text
  /\_/\
 ( o.o )
  > ^ <
```

Save that as `cat.txt` and run `backdrop-shell -art cat.txt`. That is all
it takes; the rest of this guide is about making it look and move the way
you want.

Three rules to know from the start:

1. **A space is transparent.** Wherever the art has a space, you see the
   terminal's own background. The shape of your characters is the shape of
   the picture.
2. **One column per character.** Letters, digits, punctuation, box drawing
   (`─│┌┐└┘╭╮`), blocks (`▀▄█░▒▓`), Braille (`⣿⡇`) and most symbols are one
   column wide and work. Emoji and CJK characters are two columns wide; they
   are replaced with a space so the rows stay straight. Tabs jump to the next
   multiple of eight columns.
3. **Your shell's text is drawn over it.** Art shows only where nothing is
   written. Put it in a corner, or keep it quiet enough to read through.

---

## 3. Settings: lines that start with `@`

A line that starts, at its very first column, with one of the settings
below changes how the art is shown instead of being part of it. Any other
line is art — so `@@@@` or `@home` in your picture are drawn as they are.

| Setting | What it does | Example |
| :--- | :--- | :--- |
| `@align` | where the art sits: `center`, `top`, `bottom`, `left`, `right`, `top-left`, `top-right`, `bottom-left`, `bottom-right` | `@align bottom-right` |
| `@offset` | moves it from there, in columns and rows (negative moves left/up) | `@offset -3 -1` |
| `@color` | a colour, or several for a gradient | `@color #ffd23f` |
| `@gradient` | which way the gradient runs: `vertical`, `horizontal`, `diagonal` | `@gradient horizontal` |
| `@background` | a colour behind the whole screen | `@background #0b1020` |
| `@frame` | starts the next frame of an animation | `@frame` |
| `@fps` | frames a second (default 4) | `@fps 2` |
| `@scroll` | drifts the art, in columns and rows a second | `@scroll 5 0` |
| `@tile` | repeats the art to fill the screen; two numbers leave gaps | `@tile 10 3` |

Settings can go anywhere, but they read best at the top. A mistake is
reported with its line number — `line 2: @fps wants a number from 0 to 60`
— and backdrop-shell falls back to the aurora rather than leaving you
without a shell.

### Place it

```text
@align bottom-right
@offset -3 -1
      __
    .'  '.
   (  ()  )
    '.__.'
```

`bottom-right` puts the art in the corner; `@offset -3 -1` pulls it three
columns in from the right edge and one row up from the bottom, so it does
not touch the frame of the window.

### Colour it

```text
@color #fff6a8 #ffd23f #d99a00
@gradient vertical
```

One colour paints every character with it. Two or more make a gradient:
here from pale yellow at the top, through lemon, to a deep amber at the
bottom. `horizontal` runs it left to right, `diagonal` from the top-left
corner.

Without `@color`, art is drawn in a soft grey-blue — deliberately quieter
than your shell's text, so a picture is never mistaken for output.

`-opacity` (default `0.45`) then fades all of it into your terminal's own
background colour: `0.2` is a whisper, `1` is full strength.

---

## 4. Animation, part one: frames

A flipbook. Draw each picture, separated by `@frame`, and set how many are
shown a second:

```text
@fps 2
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( -.- )
```

That cat blinks — but half its life with its eyes shut, which looks sleepy
rather than alive. **Timing is made by repeating frames.** At `@fps 2`
every frame lasts half a second, so four frames with the closed one last
make the cat look for a second and a half and blink for half:

```text
@fps 2
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( o.o )
@frame
  /\_/\
 ( -.- )
```

`apps/backdrop-shell/art/cat.txt` does exactly this, and wags its tail on
the same beat.

Tips:

- Keep every frame the same size. Frames are lined up by their top-left
  corner, and the art is placed using the largest one, so a frame that is a
  column wider shifts nothing — but a picture drawn one column to the right
  in one frame will jump.
- Change as little as possible between frames. Only the characters that
  change are sent to the terminal, which is what keeps this cheap.
- `@fps 1` to `@fps 4` suits a background. Anything faster pulls the eye
  away from your work.

---

## 5. Animation, part two: movement

`@scroll` moves the whole art: columns a second to the right (negative:
left), rows a second down (negative: up). What leaves one side of the
screen comes back on the other.

```text
@scroll 5 0
@align top-left
@offset 0 3
  _/v\_
```

That bird crosses the screen at five columns a second. Give it two frames
and it flaps as it flies — frames and movement combine:

```text
@fps 4
@scroll 5 0
@align top-left
@offset 0 3
 __   __
   \v/
@frame
  _/v\_
```

(`apps/backdrop-shell/art/bird.txt`)

### Patterns that fill the screen: `@tile`

`@tile` repeats the art across the whole screen, like wallpaper. With
`@scroll` the whole pattern moves, which is how weather is made:

```text
@color #4a6fa5 #2c4466
@tile
@scroll 0 14
   |           '         |              .          |       '        |
         .         |              '           |          .
 '            |        .     |         .              '        |
       |            '                |        '    |                 .
            '    .        |      '                     .      |
   .       |          '        .          |     '                |    '
```

That is `rain.txt`: straight down, fourteen rows a second. Two things make
a tiled pattern look natural:

- **Make the tile wide and irregular.** The eye finds a repeat quickly; a
  pattern 60 or more columns wide, with drops at uneven distances, hides it.
- **Leave gaps with `@tile x y`.** `@tile 10 3` puts ten columns and three
  rows between copies — right for clouds, which should not touch:

```text
@color #5b6a99 #34406a
@tile 10 3
@scroll 1.5 0
      .--.
   .-(    ).
  (___.__)__)
```

Slow is better for a background: clouds at `1.5`, rain at `10`–`15`,
snow at `2`–`4` falling with a slight `@scroll 1 3`.

---

## 6. Art from other tools

Anything a program prints in colour can be saved and used as art, because
backdrop-shell reads the colour codes in the file (ANSI SGR: 16 colours, 256
colours and truecolor, for text and background). A colour from the text
wins over `@color`.

```bash
# A picture as coloured block art (chafa: https://hpjansson.org/chafa/)
chafa --size 100x30 --format symbols photo.jpg > photo.txt

# A picture as coloured letters
jp2a --colors --width=100 photo.jpg > photo.txt

# Big lettering, in rainbow colours
figlet -f slant "hello" | lolcat -f > hello.txt
toilet -f future --gay "hello" > hello.txt
```

Then add settings at the top with any editor — `@align bottom-right`, or
`@scroll` to make it drift — and use it like any other art.

Two things to know about tool output:

- Block art from `chafa` colours **spaces** with a background colour.
  Those spaces are drawn (they are part of the picture); only spaces
  without a colour are transparent.
- Choose `--size`/`--width` smaller than your terminal. Art that is larger
  than the screen is cut off at the edges.

---

## 7. A scene of your own: a program

Art is drawn once, at one size. A scene that fills the screen whatever its
size, and moves the way the built-in ones do, is a program — in any
language — named with `-scene-cmd`:

```bash
backdrop-shell -scene-cmd ~/.config/limoni/scenes/fireflies.py
backdrop-shell enable -scene-cmd ~/.config/limoni/scenes/fireflies.py   # in every terminal
```

A program kept in `~/.config/limoni/scenes` is also a scene by name, listed
with the built-in ones: `fireflies.py` there is `-scene fireflies`, and a
folder holding a program of its own name, `cosmos/cosmos`, is `-scene cosmos`.

A complete example is
[`apps/backdrop-shell/scenes/fireflies.py`](../apps/backdrop-shell/scenes/fireflies.py);
copy it and change what it draws. The smallest scene there is, a dot that
swings across the middle of the screen:

```python
#!/usr/bin/env python3
import math, os, sys, time

cols, rows = int(os.environ["LIMONI_COLS"]), int(os.environ["LIMONI_ROWS"])
t = 0.0
while True:
    x = int((math.sin(t) + 1) / 2 * (cols - 1))
    # A frame: clear, move to the middle row, draw a yellow dot.
    sys.stdout.write(f"\x1b[2J\x1b[{rows // 2 + 1};{x + 1}H\x1b[38;2;255;210;60m•")
    sys.stdout.flush()
    t += 0.1
    time.sleep(0.1)
```

Save it, `chmod +x` it, and give its path to `-scene-cmd`. The rules:

- **It is an executable file.** A script starts with `#!` and is made
  executable with `chmod +x`. To pass arguments, write a two-line script
  that calls the real thing with them.
- **It runs on a terminal the size of the screen.** `LIMONI_COLS` and
  `LIMONI_ROWS` hold the size; `stty size`, `tput cols` and Python's
  `shutil.get_terminal_size()` give it too.
- **It draws as on any terminal:** colours (`\x1b[38;2;R;G;Bm` for text,
  `\x1b[48;2;R;G;Bm` for background), moving the cursor
  (`\x1b[ROW;COLH`, counted from 1), clearing (`\x1b[2J`).
- **Each frame starts by clearing or going home** — `\x1b[2J` or
  `\x1b[H`. A frame is shown when the next one begins, so a half-drawn one
  never shows. When the program falls quiet between frames, what it drew
  shows too, so one that draws a single picture and sleeps works as well.
- **Flush after each frame** (`flush()`, `fflush(stdout)`), or the frame
  waits in the program's own buffer.
- **A cell without a background colour shows the terminal's own
  background**, like a space in art; a cell with one is part of the scene.
  `-opacity` fades both, as for any scene.

backdrop-shell looks after the rest. When the window changes size, the
program is started again with the new size. While the window is out of
focus, or a full-screen program covers the background, the program is
stopped, and costs nothing; it goes on where it was once the background
can be seen. When another background is chosen or the terminal closes, it
is ended, together with anything it started. `-fps` and `-still` do not
apply: the program sets its own pace.

Its error output is thrown away, since the screen belongs to your shell.
If the background stays empty, run the program on its own in a terminal —
`~/.config/limoni/scenes/fireflies.py` — and read what it says.

What reaches the terminal is only what changed from one frame to the next,
so redrawing everything every frame is fine. What costs is colour that
drifts smoothly: it changes every cell every frame. Round colours to a few
steps, as `fireflies.py` does, and a cell is sent again only when it
crosses one.

---

## 8. A picture as wallpaper

```bash
backdrop-shell -image ~/Pictures/wallpaper.jpg -opacity 0.3
```

PNG, JPEG and GIF (the first frame) work. The picture covers the screen —
scaled until no edge is bare, the overflow cropped evenly — and is drawn
with half blocks at two pixels per cell. It looks like pixel art, not a
photograph: a terminal cell can hold two colours, and that is the whole
resolution there is, in every terminal.

Dark, soft pictures work best: a starry sky, a blurred city at night, a
gradient. A busy, bright photograph fights with the text in front of it —
lower `-opacity` until it stops.

A picture is still, so it costs nothing while you work: it is drawn when
the terminal opens and again when the window changes size.

---

## 9. What it costs

What is sent to the terminal is only the characters that change, so a
background costs as much as it moves. Measured behind fish in kitty,
120×40, twenty seconds each, CPU time of all threads:

| Background | backdrop-shell | kitty |
| :--- | ---: | ---: |
| picture (`-image`) | 0.00% | 0.01% |
| aurora, `-still` | 0.01% | 1.82% |
| `cat.txt` (2 fps) | 0.03% | 0.29% |
| `rain.txt` (14 rows/s) | 0.22% | 2.30% |
| starfield | 0.33% | 1.25% |
| aurora | 0.54% | 1.26% |
| synthwave | 0.77% | 1.45% |

(Percent of one core. kitty's share moves with what else it is doing and
with the focus; the still aurora and the moving one cost it about the same.)

Everything stops while the window is out of focus, and while a full-screen
program such as btop covers the whole background. To make any background
cheaper: fewer frames (`@fps`, or `-fps 10` for a built-in scene), fewer
characters that change, or `-still`.

---

## 10. When something looks wrong

| You see | Why, and what to do |
| :--- | :--- |
| The art is frozen | `still = true` in the settings (`backdrop-shell status` says so). `backdrop-shell enable -still=false` makes it move; choosing a new background with `enable -art …` clears it too; `backdrop-shell reset` starts over. |
| Copying takes the art with it | That is the terminal's own selection (Shift+drag, or `select = false`). A plain drag selects in backdrop-shell and copies only what the shell wrote. |
| A setting is drawn as text | It must start at the first column; an indented `@fps` is part of the picture. Only the names in the table in section 3 are settings. |
| Rows are ragged | Tabs (they jump to multiples of eight) or two-column characters (replaced by a space). Use spaces and one-column characters. |
| Nothing at all | A 16-colour terminal, `LIMONI_BACKDROP=off`, or you are inside backdrop-shell already (`backdrop-shell status` says so). |
| A program's scene stays empty | Run the program on its own to see its errors; its error output is thrown away behind the shell. Check that it is `chmod +x`, starts with `#!`, and flushes after each frame. |
| The art hides behind a program | Programs that paint their own background (btop, a vim colour scheme) cover it. That is by design. |

---

## 11. For Go programmers

The same art and pictures work in any Limoni application:

```go
art, err := backdrop.LoadArt("cat.txt")   // or backdrop.ParseArt(reader)
if err != nil { ... }
limoni.Run(app, limoni.WithBackdrop(art))

img, err := backdrop.LoadImage("wallpaper.jpg")
limoni.Run(app, limoni.WithBackdrop(backdrop.Fade(img, bgColor, 0.3)))
```

### Writing a scene of your own

A scene is anything with two methods (`terminal.Backdrop`):

```go
type Backdrop interface {
    Render(dst *buffer.Buffer, t time.Duration) // paint every cell for time t
    Interval() time.Duration                    // how often it changes; 0 = still
}
```

Here is a complete one — a band of light that sweeps down the screen:

```go
type sweep struct{}

func (sweep) Interval() time.Duration { return time.Second / 15 }

func (sweep) Render(dst *buffer.Buffer, t time.Duration) {
    w, h := int(dst.Area.Width), int(dst.Area.Height)
    // Where the band is: one pass down the screen every four seconds.
    band := math.Mod(t.Seconds()/4, 1) * float64(h+8) - 4
    for y := 0; y < h; y++ {
        d := math.Abs(float64(y) - band)
        // Quantised to eight steps: a row changes only when the band
        // crosses a step, not on every frame.
        k := math.Floor(math.Max(0, 1-d/4)*8) / 8
        bg := cell.NewColorRGB(uint8(8+k*30), uint8(10+k*40), uint8(24+k*60))
        for x := 0; x < w; x++ {
            dst.Content[y*w+x] = cell.Cell{Content: ' ', Style: cell.Style{Bg: bg}}
        }
    }
}
```

The rules that keep a scene cheap, and why:

1. **Paint every cell.** `dst` still holds the previous frame.
2. **Be a function of `t`.** Draw from the time you are given, not from a
   counter you keep. The same moment may be drawn twice (once for the
   application, once for the background alone), and must look the same.
3. **Quantise what drifts.** A colour computed from a smooth curve is
   different in every frame, so every cell is resent every frame. Round it
   to a handful of steps and a cell changes only when it crosses one.
4. **Prefer background colour behind spaces** for large areas. The encoder
   sends a run of one colour as one colour and a repeat.
5. **Work out what does not move once per size**, and copy it each frame.
6. **Allocate nothing in `Render`.** Keep buffers on the scene; build them
   when the size changes.

To see what a scene costs, draw it into a buffer, diff it against the
previous frame with `buffer.DiffWithOptions`, and count the bytes a second —
`traffic` in `backdrop/backdrop_test.go` does exactly that, and
`TestTheGuidesExampleScene` runs it on the sweep above: 2.7 KB a second.
Compare that with a full repaint every frame too, but read the ratio with
care: a flat scene repaints almost for free (one colour and a run per row),
so even a cheap one can look like a large share of it.
