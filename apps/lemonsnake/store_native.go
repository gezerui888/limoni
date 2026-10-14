//go:build !js

package main

import (
	"os"
	"path/filepath"
)

// store keeps the name, the theme and the game's own board in a small file in the
// user's configuration directory: ~/.config/limoni/lemonsnake.json on
// Linux. Where there is none, they last the session; so do they for the
// zero store, which the tests use.
type store struct {
	path string
}

func newStore() store {
	dir, err := os.UserConfigDir()
	if err != nil {
		return store{}
	}
	return store{path: filepath.Join(dir, "limoni", "lemonsnake.json")}
}

func (s store) load() saved {
	if s.path == "" {
		return saved{}
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return saved{}
	}
	return decodeSaved(data)
}

// save writes a temporary file and renames it over the old one, so a crash
// halfway through never leaves the board cut short. It runs when a run
// ends or the name changes, never in a frame.
func (s store) save(sv saved) {
	if s.path == "" {
		return
	}
	data := encodeSaved(sv)
	if os.MkdirAll(filepath.Dir(s.path), 0o755) != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}
