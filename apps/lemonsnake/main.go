// Lemon Snake: a snake that eats lemons and fights a monster.
//
//	go run .            # play
//	go run . -fps 30    # fewer frames, for a slow terminal or link
//	go run . -mute      # no sound
//	go run . -board off # keep the scores on this machine
//
// Every five lemons a monster comes out of the far corner and hunts the
// head. Space spits a seed at it, two to a lemon eaten; where it steps on
// the body it bites the rest off. Beat it and the next level is
// faster, and so is the next monster. T changes the colour theme at any
// time. Finished runs go to a shared leaderboard, as Lemon Drop's do.
//
// A frame allocates nothing: the snake lives in fixed arrays, and the tests
// hold a frame through Limoni's diff to zero.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/thebanri/limoni"
)

func main() {
	fps := flag.Int("fps", 60, "frames per second")
	mute := flag.Bool("mute", false, "no sound")
	board := flag.String("board", "", `the shared leaderboard's address; "off" keeps scores on this machine`)
	flag.Parse()

	var mix *mixer
	if !*mute {
		mix = newMixer()
	}

	term, err := limoni.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	g := newGame(uint64(time.Now().UnixNano()))
	g.store = newStore()
	g.restore()
	if g.remote = newRemote(boardURL(*board)); g.remote != nil {
		g.worldState = worldLoading
		g.remote.fetch()
	}
	if g.name == "" {
		g.askName()
	}
	switch {
	case mix != nil:
		g.audio = mix
	case *mute:
		g.noSound = "off (-mute)"
	default:
		g.noSound = "no player found"
	}
	last := time.Now()
	app := limoni.NewApp(term, limoni.WithFPS(*fps), limoni.WithTitle("Lemon Snake"))
	err = app.Run(context.Background(), func(f *limoni.Frame, ev *limoni.Event) bool {
		if ev == nil {
			now := time.Now()
			g.update(now.Sub(last).Seconds())
			last = now
		} else if ev.Type == limoni.EventKey {
			g.key(ev.Key)
		}
		g.render(f.Buffer)
		return !g.quit
	})
	term.Close()
	if mix != nil {
		mix.close()
	} else if !*mute {
		fmt.Fprintln(os.Stderr, "lemonsnake: no audio output available (on Linux, install pw-play, pacat, aplay or sox)")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// key handles a key. Q quits from the title, the pause and the end; in a
// run it pauses, so a slip of the finger does not lose the run.
func (g *game) key(k limoni.KeyEvent) {
	if g.phase == phName {
		g.nameKey(k)
		return
	}
	ch := k.Ch
	if ch >= 'A' && ch <= 'Z' {
		ch += 'a' - 'A'
	}
	rn := k.Type == limoni.KeyRune
	enter := k.Type == limoni.KeyEnter || k.Type == limoni.KeySpace
	switch {
	case rn && ch == 't':
		g.theme = (g.theme + 1) % len(themes)
		g.sound(sfxTheme, 0.7)
		g.persist()
		return
	case rn && ch == 'm':
		g.muted = !g.muted
		g.sound(sfxTheme, 0.7)
		return
	}
	switch {
	case g.phase == phTitle:
		switch {
		case rn && ch == 'q':
			g.quit = true
		case rn && ch == 'n':
			g.askName()
		case enter:
			g.start()
		}
	case g.phase == phOver:
		switch {
		case rn && ch == 'q':
			g.quit = true
		case enter || rn && ch == 'r':
			g.start()
		case k.Type == limoni.KeyEsc:
			g.phase = phTitle
		}
	case g.paused:
		switch {
		case rn && ch == 'q':
			g.quit = true
		case rn && ch == 'r':
			g.start()
		case k.Type == limoni.KeyEsc || rn && ch == 'p' || enter:
			g.setPaused(false)
		}
	case k.Type == limoni.KeyEsc || rn && (ch == 'p' || ch == 'q'):
		g.setPaused(true)
	case k.Type == limoni.KeyUp || rn && ch == 'w':
		g.turn(dUp)
	case k.Type == limoni.KeyRight || rn && ch == 'd':
		g.turn(dRight)
	case k.Type == limoni.KeyDown || rn && ch == 's':
		g.turn(dDown)
	case k.Type == limoni.KeyLeft || rn && ch == 'a':
		g.turn(dLeft)
	case k.Type == limoni.KeySpace:
		g.fire()
	}
}

// askName opens the name entry, with the name so far to edit.
func (g *game) askName() {
	g.typing = g.name
	g.back = phTitle
	g.phase = phName
}

// nameKey types the player's name: letters, digits and a few marks, up to
// nameMax; Backspace takes one back, Enter keeps it, and Esc goes back
// with the name there was, when there was one.
func (g *game) nameKey(k limoni.KeyEvent) {
	switch {
	case k.Type == limoni.KeyEnter:
		name := cleanName(g.typing)
		if name == "" {
			return
		}
		g.name = name
		g.persist()
		g.phase = g.back
		g.sound(sfxTheme, 0.7)
	case k.Type == limoni.KeyEsc:
		if g.name != "" {
			g.phase = g.back
		}
	case k.Type == limoni.KeyBackspace:
		if r := []rune(g.typing); len(r) > 0 {
			g.typing = string(r[:len(r)-1])
			g.sound(sfxType, 0.6)
		}
	case k.Type == limoni.KeySpace || k.Type == limoni.KeyRune:
		ch := k.Ch
		if k.Type == limoni.KeySpace {
			ch = ' '
		}
		if nameRune(ch) && len([]rune(g.typing)) < nameMax && (ch != ' ' || g.typing != "") {
			g.typing += string(ch)
			g.sound(sfxType, 0.6)
		}
	}
}
