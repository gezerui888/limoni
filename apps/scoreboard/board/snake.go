package board

import (
	"errors"
	"time"
)

// Lemon Snake's board (apps/lemonsnake) sits beside the others, at
// /snake/scores, with its own runs and the same limit on how often one
// address may send one.
//
// A lemon scores ten points times the level, a seed that strikes the
// monster five times the level, and the monster's fall a hundred times the
// level; then the level goes up. The client sends what each level held, as
// [lemons, hits], and the score is worked out here from that:
//
//	score  Σ level × (10 × lemons + 5 × hits)  +  Σ 100 × level, for each level finished
//
// A level is finished when its monster falls: one that came after five
// lemons and took 3 + level hits. Every level but the last is
// finished; the last is where the run ended.
//
// As with the other boards, anyone can still send a run they did not play;
// the checks below only make sure it is one the game could have produced.
const (
	snakeLemonsToMonster = 5
	snakeMaxSecs         = 24 * 60 * 60
	snakeMaxLevels       = 200
	// The snake steps at most every 75 ms, and a turn can finish a step
	// early; nobody eats lemons faster than this, a second.
	snakeMaxLemonsPerSec = 30
	snakeMaxLemons       = 100000

	// SnakeBody is how large a run may be.
	SnakeBody = 8 << 10

	// SnakeShown is the number of runs returned by Lemon Snake's leaderboard.
	SnakeShown = 10
)

// SnakeRun is what the game sends when a run of Lemon Snake ends.
type SnakeRun struct {
	Name string `json:"name"`
	Secs int    `json:"secs"`
	// Levels is each level in turn, as [lemons, hits].
	Levels [][2]int `json:"levels"`
}

// SnakeEntry is a run on Lemon Snake's board.
type SnakeEntry struct {
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Level  int    `json:"level"`
	Lemons int    `json:"lemons"`
	Secs   int    `json:"secs"`
	At     int64  `json:"at"` // unix seconds
}

// snakeMonsterHP is how many hits the monster of a level takes.
func snakeMonsterHP(level int) int { return 3 + level }

// Check cleans the run's name and makes sure its numbers are ones the game
// could have produced.
func (r *SnakeRun) Check() error {
	r.Name = CleanName(r.Name)
	switch {
	case r.Name == "":
		return errors.New("a name is needed")
	case r.Secs < 1 || r.Secs > snakeMaxSecs:
		return errBadRun
	case len(r.Levels) < 1 || len(r.Levels) > snakeMaxLevels:
		return errBadRun
	}
	lemons, hits := 0, 0
	for i, lv := range r.Levels {
		level, l, h := i+1, lv[0], lv[1]
		last := i == len(r.Levels)-1
		switch {
		case l < 0 || h < 0 || l > snakeMaxLemons:
			return errBadRun
		// A finished level met its monster and took exactly its health; the
		// last one may not have, and its monster still stands.
		case !last && (l < snakeLemonsToMonster || h != snakeMonsterHP(level)):
			return errBadRun
		case last && (h >= snakeMonsterHP(level) || h > 0 && l < snakeLemonsToMonster):
			return errBadRun
		}
		lemons += l
		hits += h
		// Seeds come from lemons, two each; every hit was one of them.
		if hits > 2*lemons {
			return errBadRun
		}
	}
	if lemons > r.Secs*snakeMaxLemonsPerSec {
		return errBadRun
	}
	return nil
}

// Entry scores the run, as made at now.
func (r *SnakeRun) Entry(now time.Time) SnakeEntry {
	e := SnakeEntry{Name: r.Name, Secs: r.Secs, Level: len(r.Levels), At: now.Unix()}
	for i, lv := range r.Levels {
		level := i + 1
		e.Lemons += lv[0]
		e.Score += level * (10*lv[0] + 5*lv[1])
		if i < len(r.Levels)-1 {
			e.Score += 100 * level
		}
	}
	return e
}

// snakeBefore reports whether a ranks above b: the higher score, and in a
// tie, the run made first.
func snakeBefore(a, b SnakeEntry) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	return a.At < b.At
}
