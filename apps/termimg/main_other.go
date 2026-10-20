//go:build !unix

package main

import (
	"fmt"
	"os"
)

// termimg runs programs on a Unix pty, so elsewhere it only says so.
func main() {
	fmt.Fprintln(os.Stderr, "termimg: runs on Linux, macOS and the BSDs only, it needs a Unix pty.\nOn Windows, Windows Terminal 1.22 and later show sixel pictures by themselves.")
	os.Exit(1)
}
