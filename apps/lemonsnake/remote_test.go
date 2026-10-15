// The shared board and the stores, against a real HTTP server and real
// files: not in a browser.

//go:build !js

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thebanri/limoni"
	"github.com/thebanri/limoni/core/buffer"
	"github.com/thebanri/limoni/core/cell"
)

// serverScore is the leaderboard's rule (apps/scoreboard/board/snake.go),
// written out again: the score it works out from what the run sent.
func serverScore(r remoteRun) int {
	s := 0
	for i, lv := range r.Levels {
		level := i + 1
		s += level * (10*int(lv[0]) + 5*int(lv[1]))
		if i < len(r.Levels)-1 {
			s += 100 * level
		}
	}
	return s
}

func (g *game) run() remoteRun {
	return remoteRun{Name: g.name, Secs: g.runSecs(), Levels: g.levels[:g.level]}
}

// fight plays a monster to the end: lemons to bring it, and seeds to fell
// it, each worth what a hit is.
func fight(g *game) {
	for range lemonsPerLevel {
		g.eat()
	}
	for g.boss.on {
		g.hit(g.boss.p)
	}
}

// Whatever a run scores, the leaderboard scores the same from its log:
// lemons and hits at every level, and each monster felled.
func TestTheLogScoresAsTheGameDoes(t *testing.T) {
	g := playing()
	fight(g)
	g.eat() // a lemon at level 2, then its monster
	fight(g)
	g.eat()
	g.eat()
	for range 3 { // three hits on level 3's monster, which stands
		g.hit(g.boss.p)
	}
	for range lemonsPerLevel {
		g.eat()
	}
	g.hit(g.boss.p)
	if g.level != 3 || !g.boss.on {
		t.Fatalf("the scripted run reached level %d, monster %v", g.level, g.boss.on)
	}
	if got := serverScore(g.run()); got != g.score {
		t.Fatalf("the leaderboard would score %d, the game scored %d", got, g.score)
	}
}

func TestTheNameIsTypedAndKept(t *testing.T) {
	g := newGame(1)
	g.store = store{path: filepath.Join(t.TempDir(), "lemonsnake.json")}
	g.askName()
	for _, r := range "Q tm-ü!" { // q, t and m are keys elsewhere; ! is not allowed
		g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: r})
	}
	g.key(limoni.KeyEvent{Type: limoni.KeyBackspace})
	g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 'x'})
	g.key(limoni.KeyEvent{Type: limoni.KeyEnter})
	if g.quit || g.muted || g.theme != 0 {
		t.Fatal("typing a name quit, muted or changed the theme")
	}
	if g.name != "Q tm-x" || g.phase != phTitle {
		t.Fatalf("name %q, phase %d", g.name, g.phase)
	}
	g.key(limoni.KeyEvent{Type: limoni.KeyRune, Ch: 't'})
	again := newGame(2)
	again.store = g.store
	again.restore()
	if again.name != "Q tm-x" || again.theme != 1 {
		t.Fatalf("after a restart: name %q, theme %d", again.name, again.theme)
	}
}

func TestRunsGoOnTheGamesOwnBoard(t *testing.T) {
	g := playing()
	g.store = store{path: filepath.Join(t.TempDir(), "lemonsnake.json")}
	g.name = "Ada"
	for _, n := range []int{3, 9, 1} {
		g.start()
		for range n {
			g.eat()
		}
		g.die()
	}
	if g.nLocal != 3 || g.local[0].Lemons != 9 || g.local[2].Lemons != 1 || g.localRank != 3 {
		t.Fatalf("the board: %+v, the last run's place %d", g.local[:g.nLocal], g.localRank)
	}
	again := newGame(3)
	again.store = g.store
	again.restore()
	if again.nLocal != 3 || again.best != g.local[0].Score || again.local[1].Name != "Ada" {
		t.Fatalf("after a restart: %+v, best %d", again.local[:again.nLocal], again.best)
	}
}

// The game against a stand-in for the leaderboard: it fetches the board on
// start, sends a run when it ends, and shows the world's board and its
// place, all without a frame waiting.
func TestTheSharedBoard(t *testing.T) {
	var mu sync.Mutex
	var got []remoteRun
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snake/scores" {
			http.NotFound(w, r)
			return
		}
		board := []scoreEntry{{Name: "Ece", Score: 5000, Level: 6, Lemons: 50, Secs: 200}}
		rank := 0
		if r.Method == "POST" {
			var run remoteRun
			if err := json.NewDecoder(r.Body).Decode(&run); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			mu.Lock()
			got = append(got, run)
			mu.Unlock()
			board = append(board, scoreEntry{Name: run.Name, Score: serverScore(run), Level: len(run.Levels)})
			rank = 2
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rank": rank, "board": board})
	}))
	defer srv.Close()

	g := playing()
	g.name = "Ada"
	g.remote = newRemote(srv.URL + "/")
	g.worldState = worldLoading
	g.remote.fetch()
	wait := func(what string, done func() bool) {
		t.Helper()
		for end := time.Now().Add(5 * time.Second); !done(); {
			if time.Now().After(end) {
				t.Fatalf("waited for %s", what)
			}
			g.pollRemote()
			time.Sleep(time.Millisecond)
		}
	}
	wait("the board", func() bool { return g.worldState == worldOK })
	if g.nWorld != 1 || g.world[0].Name != "Ece" {
		t.Fatalf("the world's board: %+v", g.world[:g.nWorld])
	}

	fight(g)
	g.eat()
	score := g.score
	g.die()
	if g.worldRank != -1 {
		t.Fatalf("the run was not sent: rank %d", g.worldRank)
	}
	wait("the run's place", func() bool { return g.worldRank > 0 })
	mu.Lock()
	sent := got[0]
	mu.Unlock()
	if sent.Name != "Ada" || len(sent.Levels) != 2 || serverScore(sent) != score {
		t.Fatalf("sent %+v for a run of %d", sent, score)
	}
	if g.worldRank != 2 || g.nWorld != 2 {
		t.Fatalf("place %d on a board of %d", g.worldRank, g.nWorld)
	}

	// The end dialog shows it.
	g.diedT -= 5
	s := opened(g, buffer.NewBuffer(cell.Rect{Width: 100, Height: 30}))
	for _, want := range []string{"GAME OVER", "#2 in the world!", "WORLD TOP TEN", "Ece", "Ada"} {
		if !strings.Contains(s, want) {
			t.Errorf("the end dialog has no %q:\n%s", want, s)
		}
	}
}

// A leaderboard that does not answer leaves the game its own board.
func TestAnAbsentBoardFallsBackToTheGamesOwn(t *testing.T) {
	g := newGame(4)
	g.name = "Ada"
	g.remote = newRemote("http://127.0.0.1:1")
	g.worldState = worldLoading
	g.remote.fetch()
	for end := time.Now().Add(5 * time.Second); g.worldState == worldLoading; {
		if time.Now().After(end) {
			t.Fatal("no answer from an address nobody listens on")
		}
		g.pollRemote()
		time.Sleep(time.Millisecond)
	}
	if g.worldState != worldOffline {
		t.Fatalf("state %d", g.worldState)
	}
	s := opened(g, buffer.NewBuffer(cell.Rect{Width: 100, Height: 30}))
	for _, want := range []string{"LEMON SNAKE", "YOUR TOP TEN", "offline", "playing as Ada"} {
		if !strings.Contains(s, want) {
			t.Errorf("the title has no %q:\n%s", want, s)
		}
	}
}
