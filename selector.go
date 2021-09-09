package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"image/color"
	"math"
)

// the selection helper provides a UI for selecting arbitrary objects and applying transformations, such as moves,
// scales, and rotations, to them

type Selectable interface {
	// A transformation that moves a 1 unit square centered at the origin to the correct position in world units
	Transform() Mx
	// Updates the transformation based on user behavior. The supplied transformation would map a unit square centered
	// at 0,0 to the correct boundary.
	SetTransform(m Mx)
}

// Different states the selector UX can be in, depending on the location of the initial click, which change behavior
// of dragging
type selstate int
const (
	selidle selstate = iota
	selmoving
	selrotating
	selscaling
)

type Selector struct {
	// Current selection
	s Selectable
	// Camera to use for locating the mouse in the world, and rendering.
	C *Camera
	// All possible selectables.
	Selectables []Selectable

	state selstate
}

// determines if the given coordinates intersect with a 1x1 square transformed by the inverse of the given matrix.
func (s *Selector) hit(x, y float64, m Mx) bool {
	m.Invert()
	tx, ty := m.Apply(x, y)
	return tx >= -0.5 && tx <= 0.5 && ty >= -0.5 && ty <= 0.5
}

func (s *Selector) Update() {
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.state = selidle
		return
	}
	if MouseClicked(ebiten.MouseButtonLeft) {
		cx, cy := s.C.Cursor()
		if s.s == nil || !s.hit(cx, cy, s.s.Transform()) {
			s.s = nil
			// Select the clicked on object with the smallest square diagonal
			diagonal := math.MaxFloat64
			for _, se := range s.Selectables {
				if s.hit(cx, cy, se.Transform()) {
					m := se.Transform()
					blx, bly := m.Apply(-0.5, -0.5)
					tlx, tly := m.Apply(0.5, 0.5)
					sd := (tlx-blx)*(tlx-blx) + (tly-bly)*(tly-bly)
					if sd < diagonal {
						diagonal = sd
						s.s = se
					}
				}
			}
		}
		// If we had a hit, what type of dragging should we do?
		if s.s == nil {
			// Nothing left to do
			return
		}
		s.state = selmoving
		//t := s.s.Transform()
		//t.Invert()
		//tx, ty := t.Apply(cx, cy)
		//switch {
		//case math.Abs(tx) > 0.45 && math.Abs(ty) > 0.45:
		//	// corner was clicked, thats rotation
		//	s.state = selrotating
		//case math.Abs(tx) < 0.05 && math.Abs(ty) > 0.45:
		//	// middle scaler
		//	s.state = selrotating
		//
		//
		//}
	}
	if s.s != nil {
		d := MouseDrag(ebiten.MouseButtonLeft)
		wdx, wdy := 2 * s.C.hw * d.X / float64(s.C.sw), -2 * s.C.hh * d.Y / float64(s.C.sh)
		t := s.s.Transform()
		t.Translate(wdx, wdy)
		s.s.SetTransform(t)
	}
}

func (s *Selector) Draw(screen *ebiten.Image) {
	geom := s.C.ToScreen()
	// draws an outline in unit square coordinates
	outline := func(sblx, sbly, strx, stry float64) {
		t := s.s.Transform()
		blx, bly := t.Apply(sblx, sbly)
		trx, try := t.Apply(strx, stry)
		drawline(screen, blx, bly, trx, bly, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, trx, bly, trx, try, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, trx, try, blx, try, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, blx, try, blx, bly, 3, geom, color.RGBA{R: 255, A: 255})
	}
	if s.s != nil {
		outline(-0.5, -0.5, 0.5, 0.5)
	}
}

// Makes the spawn point of a level selectable
type SpawnSelector struct {
	C *Camera
	L *Level
}

func (s *SpawnSelector) Transform() Mx {
	// The spawn needs to scale with zoom, so we compute its scale transform based on the current camera dimensions
	// The spawn is 20px * 20px when rendered.
	side := 2 * s.C.hw * 20 / float64(s.C.sw)
	geo := Mx{}
	geo.Scale(side, side)
	geo.Translate(s.L.Spawn.X, s.L.Spawn.Y)
	return geo
}

func (s *SpawnSelector) SetTransform(m Mx) {
	// We ignore all other transformations besides translation for the spawn
	s.L.Spawn.X, s.L.Spawn.Y = m.Apply(0, 0)
}

// An editor that supports transforming arbitrary objects in the scene
type TransformEditor struct {
	s Selector
	e *Editor
}

func ActivateTransformEditor(r *Root, e *Editor) {
	var ss []Selectable
	ss = append(ss, &SpawnSelector{
		C: &e.c,
		L: &e.l,
	})
	for _, a := range e.l.Art {
		ss = append(ss, a)
	}
	for _, b := range e.l.Platforms {
		ss = append(ss, b)
	}
	r.a = &TransformEditor{
		s: Selector{
			C:           &e.c,
			Selectables: ss,
		},
		e: e,
	}
}

func (t *TransformEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return t.e.Layout(outsideWidth, outsideHeight)
}

func (t *TransformEditor) Update(r *Root) error {
	t.s.Update()
	return t.e.Update(r)
}

func (t *TransformEditor) Draw(screen *ebiten.Image) {
	t.e.Draw(screen)
	t.s.Draw(screen)
	ebitenutil.DebugPrintAt(screen, "Transform Editor", 10, t.e.c.sh-20)
}
