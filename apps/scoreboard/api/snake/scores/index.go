// Package scores is the scoreboard's /snake/scores, Lemon Snake's board, on
// Vercel; vercel.json sends /snake/scores here. See package fn.
package scores

import (
	"net/http"

	"github.com/thebanri/limoni/apps/scoreboard/fn"
)

// Handler is what Vercel calls.
func Handler(w http.ResponseWriter, r *http.Request) { fn.SnakeScores(w, r) }
