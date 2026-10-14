# Lemon Snake

A snake that eats lemons, and a monster that wants the snake.

```bash
go install github.com/thebanri/limoni/apps/lemonsnake@latest

lemonsnake            # play
lemonsnake -fps 30    # fewer frames, for a slow terminal or link
lemonsnake -mute      # no sound
lemonsnake -board off # keep the scores on this machine
```

Or [play it in the browser](https://thebanri.github.io/limoni/?app=lemonsnake): the same code,
compiled to WebAssembly and drawn by xterm.js, with sound and the same leaderboard.

Each lemon is worth ten points times the level, grows the snake a segment,
and brings two seeds (twenty at most; the panel counts them). Every five
lemons a monster comes out of the corner farthest from the head and hunts
it. `space` spits a seed straight ahead, at twice the snake's speed, as
fast as you can press it; with none left it thuds. Lemons keep coming
during the fight, so eat to reload. Where the monster steps on the body it
bites off everything behind; on the head, the run is over. So are the walls
and the snake's own body — but a step into either waits half a step first,
so a turn pressed a moment late still counts.

A seed that strikes scores five times the level; the monster's last health
scores a hundred times. Then the level goes up: the snake is faster, the
next monster takes one more seed (3 + the level) and steps more often, and
more often still once it is down to a third.

The pace was set against a simulated player of ordinary reactions — a look
every 0.28 s, a turn now and then pressed up to 0.2 s late — who beats the
first monster about four runs in five and plays a minute or two: from 6
tiles a second at the start to about 13 at the top.

| Key | |
| :-- | :-- |
| `←↑→↓` / `wasd` | turn (two quick turns are both kept) |
| `space` | spit a seed |
| `t` | next colour theme: Lemon Grove, Neon Night, Deep Sea, Retro CRT, Paper |
| `m` | sound on and off |
| `n` | (on the title) change your name |
| `p` / `esc` | pause |
| `q` | quit (in a run, pause first) |

## Leaderboard and sound

A finished run goes to a shared leaderboard, the scoreboard's
`/snake/scores` ([apps/scoreboard](../scoreboard)), under the name asked for
at the first start. The game does not send its score: it sends each level's
lemons and seed hits, and the server works the score out with the rules
above and turns away a run the game could not have played. Without the
server, or with `-board off`, the game keeps its own top ten. The name, the
theme and that top ten live in `~/.config/limoni/lemonsnake.json` (the
user's configuration directory). `-board` or `LEMONSNAKE_BOARD` points it
at another server.

The sounds are synthesised at start-up, as Lemon Drop's are: soft sines and
triangles on a pentatonic scale, a note higher for each lemon on the way to
the monster. On Linux they play through pw-play, pacat, aplay or sox's
play; on macOS and Windows through the system's audio.

## Drawing

The board is drawn in half-block pixels, a tile four by four, and sized to
the window when a run starts: it needs 70×18, and takes up to 30×18 tiles.
Between steps the snake is drawn part of the way to where it is going, the
head sliding in and the tail out, with edges blended by how much of a pixel
they cover; at 60 frames a second it glides rather than jumps. A frame
allocates nothing; the tests hold a frame, through Limoni's diff, to zero,
in play and in every dialog. The dialogs open from their centre with a
small overshoot, and two lights run round their rounded borders.
