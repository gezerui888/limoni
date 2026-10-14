//go:build js

package main

import "syscall/js"

// In a browser the name, the theme and the game's own board live in localStorage,
// under one key, so they are there the next time the page is opened, in
// that browser. The zero store keeps nothing.
type store struct {
	key string
}

func newStore() store { return store{key: "lemonsnake"} }

// localStorage throws in some private windows and when storage is blocked;
// syscall/js turns that into a panic, which here means "nothing stored".
func (s store) load() (sv saved) {
	if s.key == "" {
		return saved{}
	}
	defer func() {
		if recover() != nil {
			sv = saved{}
		}
	}()
	v := js.Global().Get("localStorage").Call("getItem", s.key)
	if v.Type() != js.TypeString {
		return saved{}
	}
	return decodeSaved([]byte(v.String()))
}

func (s store) save(sv saved) {
	if s.key == "" {
		return
	}
	data := encodeSaved(sv)
	defer func() { _ = recover() }()
	js.Global().Get("localStorage").Call("setItem", s.key, string(data))
}
