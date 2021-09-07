package main

import "github.com/hajimehoshi/ebiten/v2"


// Global, single threaded maps for easier input consumption
var (
	kdown = make(map[ebiten.Key]int)
	mdown = make(map[ebiten.MouseButton]int)
	iframe = 1
)

func InputsUpdate() {
	iframe++
	for k := range kdown {
		if !ebiten.IsKeyPressed(k) {
			delete(kdown, k)
		}
	}
	for m := range mdown {
		if !ebiten.IsMouseButtonPressed(m) {
			delete(mdown, m)
		}
	}
}

// Returns true if a given k has just started to be pressed
func Clicked(k ebiten.Key) bool {
	if !ebiten.IsKeyPressed(k) {
		return false
	}
	f, ok := kdown[k]
	if f == iframe {
		return true
	}
	if ok {
		return false
	}
	kdown[k] = iframe
	return true
}

// Returns true if a given button has just started to be pressed
func MouseClicked(m ebiten.MouseButton) bool {
	if !ebiten.IsMouseButtonPressed(m) {
		return false
	}
	f, om := mdown[m]
	if f == iframe {
		return true
	}
	if om {
		return false
	}
	mdown[m] = iframe
	return true
}

