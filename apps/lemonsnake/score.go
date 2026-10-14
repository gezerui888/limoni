package main

import (
	"encoding/json"
	"math"
	"strings"
)

// A run is scored as it goes (game.go), and also written down level by
// level, so that the shared leaderboard (apps/scoreboard, /snake/scores)
// can work the score out again from what the run did rather than take it
// on trust:
//
//	score  Σ level × (10 × lemons + 5 × hits)  +  Σ 100 × level, for each level finished
//
// The leaderboard's rules are in apps/scoreboard/board/snake.go.

const (
	nameMax   = 12 // characters in a player's name
	boardSize = 10 // runs on a board
	maxLevels = 200
)

// scoreEntry is a run on a board, the game's own or the world's.
type scoreEntry struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Level  int    `json:"level"`
	Lemons int    `json:"lemons"`
	Secs   int    `json:"secs"`
	At     int64  `json:"at,omitempty"`
}

// saved is what the game keeps between runs: the name, the theme, and its
// own board.
type saved struct {
	Name  string       `json:"name"`
	Theme int          `json:"theme"`
	Board []scoreEntry `json:"board"`
}

func decodeSaved(data []byte) saved {
	var s saved
	if json.Unmarshal(data, &s) != nil {
		return saved{}
	}
	s.Name = cleanName(s.Name)
	if s.Theme < 0 || s.Theme >= len(themes) {
		s.Theme = 0
	}
	return s
}

func encodeSaved(s saved) []byte {
	data, _ := json.Marshal(s)
	return data
}

// nameRune is what a name may hold: what the leaderboard keeps.
func nameRune(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
		r == ' ' || r == '-' || r == '_' || r == '.' ||
		strings.ContainsRune("ÇĞİÖŞÜçğıöşü", r)
}

// cleanName keeps what a name may hold, trimmed and at most nameMax long,
// as the leaderboard does.
func cleanName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if n == nameMax {
			break
		}
		if nameRune(r) && (r != ' ' || n > 0) {
			b.WriteRune(r)
			n++
		}
	}
	return strings.TrimSpace(b.String())
}

// logLemon and logHit write the run down for the leaderboard, level by
// level. A run past maxLevels stays on this machine.
func (g *game) logLemon() {
	g.lemons++
	if g.level <= maxLevels {
		g.levels[g.level-1][0]++
	}
}

func (g *game) logHit() {
	if g.level <= maxLevels {
		g.levels[g.level-1][1]++
	}
}

func (g *game) runSecs() int { return max(1, int(math.Ceil(g.runT))) }

// finishRun puts the run that just ended on the game's own board, keeps
// it, and sends it to the shared one. It runs once a run, never in a frame
// of play.
func (g *game) finishRun() {
	g.best = max(g.best, g.score)
	g.localRank, g.worldRank = 0, 0
	if g.score == 0 {
		return
	}
	g.localRank = g.addLocal(scoreEntry{Name: g.name, Score: g.score, Level: g.level, Lemons: g.lemons, Secs: g.runSecs()})
	g.persist()
	g.sendRun()
}

// addLocal puts e on the game's own board and returns its place, 1-based,
// or 0 when it did not make it.
func (g *game) addLocal(e scoreEntry) int {
	i := 0
	for i < g.nLocal && g.local[i].Score >= e.Score {
		i++
	}
	if i >= boardSize {
		return 0
	}
	if g.nLocal < boardSize {
		g.nLocal++
	}
	copy(g.local[i+1:g.nLocal], g.local[i:g.nLocal-1])
	g.local[i] = e
	return i + 1
}

// restore reads the name, the theme and the game's own board from the store.
func (g *game) restore() {
	s := g.store.load()
	g.name, g.theme = s.Name, s.Theme
	g.nLocal = 0
	for _, e := range s.Board {
		if g.nLocal == boardSize {
			break
		}
		e.Name = cleanName(e.Name)
		g.local[g.nLocal] = e
		g.nLocal++
	}
	if g.nLocal > 0 {
		g.best = g.local[0].Score
	}
}

func (g *game) persist() {
	g.store.save(saved{Name: g.name, Theme: g.theme, Board: g.local[:g.nLocal]})
}
