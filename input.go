package main

import (
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
)

// The current position and change since last frame of a point.
type drag struct {
	pos box2d.B2Vec2
	delta box2d.B2Vec2
}

// Global, single threaded maps for easier input consumption
var (
	kdown = make(map[ebiten.Key]int)
	mdown = make(map[ebiten.MouseButton]int)
	mdrag = make(map[ebiten.MouseButton]drag)
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
	for m, d := range mdrag {
		if !ebiten.IsMouseButtonPressed(m) {
			delete(mdrag, m)
		}
		x, y := ebiten.CursorPosition()
		mdrag[m] = drag{
			pos:   box2d.B2Vec2{
				X: float64(x),
				Y: float64(y),
			},
			delta: box2d.B2Vec2{
				X: float64(x) - d.pos.X,
				Y: float64(y) - d.pos.Y,
			},
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
	f, ok := mdown[m]
	if f == iframe {
		return true
	}
	if ok {
		return false
	}
	mdown[m] = iframe
	return true
}

// Reports the distance the mouse button was dragged since last frame in pixels.
func MouseDrag(m ebiten.MouseButton) box2d.B2Vec2 {
	if !ebiten.IsMouseButtonPressed(m) {
		return box2d.B2Vec2{}
	}
	d, ok := mdrag[m]
	if !ok {
		x, y := ebiten.CursorPosition()
		mdrag[m] = drag{
			pos:   box2d.B2Vec2{
				X: float64(x),
				Y: float64(y),
			},
			delta: box2d.B2Vec2{},
		}
		return box2d.B2Vec2{}
	}
	return d.delta
}

