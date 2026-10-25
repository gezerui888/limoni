//go:build !((darwin || windows || lemonsweep_gui) && !js)

package main

import "errors"

// Linux's terminals draw the game well and its windows need cgo and X11
// headers, so -gui is left out there unless asked for (gui.go).
func runGUI(*game) error {
	return errors.New("-gui is in the macOS and Windows builds; on Linux build with -tags lemonsweep_gui")
}
