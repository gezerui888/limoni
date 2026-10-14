package board

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type snakeAnswer struct {
	Rank  int          `json:"rank"`
	Board []SnakeEntry `json:"board"`
	Error string       `json:"error"`
}

func (f *fixture) snake(method, body string, header ...string) (int, snakeAnswer) {
	f.t.Helper()
	w, _ := f.doRaw(method, "/snake/scores", body, header...)
	var a snakeAnswer
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
			f.t.Fatalf("%s: %v", w.Body.String(), err)
		}
	}
	return w.Code, a
}

// snakeRun is a run of Lemon Snake as the game would send it.
func snakeRun(name string, secs int, levels ...[2]int) string {
	ls, _ := json.Marshal(levels)
	if levels == nil {
		ls = []byte("[]")
	}
	return fmt.Sprintf(`{"name":%q,"secs":%d,"levels":%s}`, name, secs, ls)
}

func TestSnakeRunsAreScoredByTheServerAndRanked(t *testing.T) {
	forEachStore(t, func(t *testing.T, open func() Store) {
		f := newFixture(t, open(), epoch)
		// Level 1: 7 lemons, its monster's 4 hits; level 2: 3 lemons, no
		// monster yet.
		code, a := f.snake("POST", snakeRun("Ada", 90, [2]int{7, 4}, [2]int{3, 0}))
		want := 1*(70+20) + 100 + 2*30
		if code != 200 || a.Rank != 1 || len(a.Board) != 1 {
			t.Fatalf("%d %+v", code, a)
		}
		if e := a.Board[0]; e.Score != want || e.Level != 2 || e.Lemons != 10 || e.Secs != 90 || e.Name != "Ada" {
			t.Fatalf("scored %+v, want %d points, level 2, 10 lemons", e, want)
		}
		f.now = f.now.Add(1)
		if _, a := f.snake("POST", snakeRun("Bo", 20, [2]int{2, 0})); a.Rank != 2 {
			t.Errorf("a lower run ranked %d", a.Rank)
		}
		if _, a := f.snake("POST", snakeRun("Cy", 300, [2]int{9, 4}, [2]int{8, 5}, [2]int{6, 3})); a.Rank != 1 {
			t.Errorf("a higher run ranked %d", a.Rank)
		}
		if _, a := f.snake("GET", ""); len(a.Board) != 3 || a.Board[0].Name != "Cy" || a.Board[2].Name != "Bo" {
			t.Errorf("the board: %+v", a.Board)
		}
		// The other games' boards are their own.
		if _, a := f.drop("GET", ""); len(a.Board) != 0 {
			t.Errorf("Lemon Snake's runs reached Lemon Drop's board: %+v", a.Board)
		}
	})
}

func TestSnakeReturnsTopTen(t *testing.T) {
	forEachStore(t, func(t *testing.T, open func() Store) {
		f := newFixture(t, open(), epoch)
		for i := 1; i <= 14; i++ {
			code, a := f.snake("POST", snakeRun(fmt.Sprintf("P%02d", i), 60, [2]int{i, 0}))
			if code != 200 || len(a.Board) != min(i, 10) {
				t.Fatalf("POST %d: status %d, %d scores", i, code, len(a.Board))
			}
		}
		code, a := f.snake("GET", "")
		if code != 200 || len(a.Board) != 10 || a.Board[0].Name != "P14" || a.Board[9].Name != "P05" {
			t.Fatalf("GET: %d %+v", code, a)
		}
	})
}

func TestSnakeRunsTheGameCannotProduceAreRefused(t *testing.T) {
	m, _ := OpenMem("")
	f := newFixture(t, m, epoch)
	for _, c := range []struct{ why, body string }{
		{"no name", snakeRun(" ", 60, [2]int{3, 0})},
		{"no levels", snakeRun("A", 60)},
		{"no time", snakeRun("A", 0, [2]int{3, 0})},
		{"a level passed with too few lemons for its monster", snakeRun("A", 60, [2]int{4, 4}, [2]int{0, 0})},
		{"a level passed without its monster's health in hits", snakeRun("A", 60, [2]int{5, 3}, [2]int{0, 0})},
		{"more hits than the monster has", snakeRun("A", 60, [2]int{9, 5}, [2]int{0, 0})},
		{"the last monster beaten but no level after it", snakeRun("A", 60, [2]int{9, 4})},
		{"hits before the monster came", snakeRun("A", 60, [2]int{4, 1})},
		{"more seeds than the lemons gave", tooManySeeds()},
		{"lemons faster than the snake can eat", snakeRun("A", 1, [2]int{40, 0})},
		{"a field the game does not send", `{"name":"A","secs":60,"levels":[[3,0]],"score":999999}`},
	} {
		if code, a := f.snake("POST", c.body); code/100 != 4 || a.Error == "" {
			t.Errorf("%s: %d %+v", c.why, code, a)
		}
	}
	if _, a := f.snake("GET", ""); len(a.Board) != 0 {
		t.Errorf("a refused run reached the board: %+v", a.Board)
	}
}

// tooManySeeds is fourteen levels passed on five lemons each: 140 seeds,
// against monsters that take 4 + 5 + … + 17 = 147.
func tooManySeeds() string {
	var lv [][2]int
	for l := 1; l <= 14; l++ {
		lv = append(lv, [2]int{5, 3 + l})
	}
	return snakeRun("A", 600, append(lv, [2]int{0, 0})...)
}

// One address may send six runs a minute, of every game together.
func TestTheLimitCoversLemonSnake(t *testing.T) {
	m, _ := OpenMem("")
	f := newFixture(t, m, epoch)
	from := []string{"X-Forwarded-For", "10.9.9.8"}
	for i := 0; i < 3; i++ {
		if code, _ := f.drop("POST", run("A", 60, 10, 0), from...); code != 200 {
			t.Fatalf("Lemon Drop run %d: %d", i, code)
		}
		if code, _ := f.snake("POST", snakeRun("A", 60, [2]int{3, 0}), from...); code != 200 {
			t.Fatalf("Lemon Snake run %d: %d", i, code)
		}
	}
	if code, _ := f.snake("POST", snakeRun("A", 60, [2]int{3, 0}), from...); code != http.StatusTooManyRequests {
		t.Errorf("a seventh run in a minute: %d", code)
	}
}

func TestTheSnakeBoardSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	m, _ := OpenMem(path)
	f := newFixture(t, m, epoch)
	if code, _ := f.snake("POST", snakeRun("Ada", 60, [2]int{3, 0})); code != 200 {
		t.Fatal(code)
	}
	again, err := OpenMem(path)
	if err != nil {
		t.Fatal(err)
	}
	top, _ := again.TopSnake(context.Background(), SnakeShown)
	if len(top) != 1 || top[0].Name != "Ada" || top[0].Score != 30 {
		t.Fatalf("after a restart: %+v", top)
	}
}

func TestTheSnakeBoardAnswersTheBrowserAndApiPaths(t *testing.T) {
	m, _ := OpenMem("")
	f := newFixture(t, m, epoch)
	w, _ := f.doRaw("OPTIONS", "/api/snake/scores", "", "Origin", "https://thebanri.github.io")
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "https://thebanri.github.io" {
		t.Errorf("OPTIONS from the playground: %d %v", w.Code, w.Header())
	}
	w, _ = f.doRaw("GET", "/api/snake/scores", "", "Origin", "https://elsewhere.example")
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("GET from another page: %d %v", w.Code, w.Header())
	}
}
