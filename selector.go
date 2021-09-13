package main

import (
	"github.com/ByteArena/box2d"
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

// If an object can be deleted from the level with the delete key while selected, and implements this interface, the
// select UX will support its deletion.
type Deletable interface {
	Delete()
}

// If a selectable implements copyable, it will allow the user to, via Cmd+C and Cmd+V, to copy and paste the object.
type Copyable interface {
	// Adds a copy of this object to the scene and returns it.
	Paste() Selectable
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

	// If the user presses Cmd+C while selecting something, it will be stored to the clipboard for later pasting.
	clipboard Copyable

	state selstate

	// Which scalar was clicked on.
	scalar int
}

// determines if the given coordinates intersect with a 1x1 square transformed by the inverse of the given matrix.
func (s *Selector) hit(x, y float64, m Mx) bool {
	m.Invert()
	tx, ty := m.Apply(x, y)
	return tx >= -0.5 && tx <= 0.5 && ty >= -0.5 && ty <= 0.5
}

// Computes the selection matrix for the rotator handle.
func (s *Selector) rotator() Mx {
	// draw rotation handle 25px above middle of top edge
	t := s.s.Transform()
	cx, cy := t.Apply(0, 0)
	x, y := t.Apply(0, 0.5)
	up := box2d.B2Vec2{x - cx, y - cy}
	up.Normalize()
	pxToWorld := 2 * s.C.hw / float64(s.C.sw)
	up.OperatorScalarMulInplace(25 * 2 * s.C.hw / float64(s.C.sw))
	up.OperatorPlusInplace(box2d.B2Vec2{x, y})
	var m Mx
	m.Scale(pxToWorld * 10, pxToWorld * 10)
	m.Translate(up.X, up.Y)
	return m
}

// Computes the selection matrices for the scalar handles.
func (s *Selector) scalars() [4]Mx {
	var out [4]Mx
	positions := []struct{X, Y float64}{
		{0.5, 0.5},
		{-0.5, 0.5},
		{0.5, -0.5},
		{-0.5, -0.5},
	}
	for i, p := range positions {
		// draw scale handle at position
		t := s.s.Transform()
		cx, cy := t.Apply(p.X, p.Y)
		pxToWorld := 2 * s.C.hw / float64(s.C.sw)
		var m Mx
		m.Scale(pxToWorld * 10, pxToWorld * 10)
		m.Translate(cx, cy)
		out[i] = m
	}
	return out
}

func (s *Selector) Update() {
	if s.s != nil && Clicked(ebiten.KeyBackspace) {
		if del, ok := s.s.(Deletable); ok {
			s.s = nil
			del.Delete()
		}
	}
	if s.s != nil && Clicked(ebiten.KeyC) && ebiten.IsKeyPressed(ebiten.KeyMeta) {
		// Copy triggered
		if kopy, ok := s.s.(Copyable); ok {
			s.clipboard = kopy
		}
	}
	if s.clipboard != nil && Clicked(ebiten.KeyV) && ebiten.IsKeyPressed(ebiten.KeyMeta) {
		// Paste triggered
		s.s = s.clipboard.Paste()
		s.Selectables = append(s.Selectables, s.s)
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		s.state = selidle
		return
	}
	if MouseClicked(ebiten.MouseButtonLeft) {
		cx, cy := s.C.Cursor()
		if s.s != nil {
			// still hitting?
			if !s.hit(cx, cy, s.s.Transform()) && !s.hit(cx, cy, s.rotator()) {
				s.s = nil
			}
		}
		if s.s == nil {
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
		if s.s == nil {
			// Nothing left to do
			return
		}
		// what type of dragging should we do?
		s.state = selmoving
		if s.hit(cx, cy, s.rotator()) {
			s.state = selrotating
		}
		for i, m := range s.scalars() {
			if s.hit(cx, cy, m) {
				s.state = selscaling
				s.scalar = i
				break
			}
		}
	}
	if s.s == nil {
		return
	}
	switch s.state {
	case selrotating:
		t := s.s.Transform()

		// rotation of selectable
		cx, cy := t.Apply(0, 0)
		tx, ty := t.Apply(0, 0.5)
		rotv := box2d.B2Vec2{tx - cx, ty - cy}
		rotv.Normalize()
		curang := math.Atan2(rotv.Y, rotv.X)

		// rotation of cursor
		wx, wy := s.C.Cursor()
		crotv := box2d.B2Vec2{wx - cx, wy - cy}
		crotv.Normalize()
		newang := math.Atan2(crotv.Y, crotv.X)

		// fix
		t.Translate(-cx, -cy)
		t.Rotate(newang - curang)
		t.Translate(cx, cy)
		s.s.SetTransform(t)
	case selmoving:
		d := MouseDrag(ebiten.MouseButtonLeft)
		wdx, wdy := 2 * s.C.hw * d.X / float64(s.C.sw), -2 * s.C.hh * d.Y / float64(s.C.sh)
		t := s.s.Transform()
		t.Translate(wdx, wdy)
		s.s.SetTransform(t)
	case selscaling:
		t := s.s.Transform()
		cx, cy := t.Apply(0, 0)

		scalar := s.scalars()[s.scalar]
		sx, sy := scalar.Apply(0, 0)

		mx, my := s.C.Cursor()

		scalex := (sx - cx)/(mx - cx)
		scaley := (sy - cy)/(my - cy)
		var m Mx
		m.Scale(scalex, scaley)
		m.Concat(t.GeoM)
		s.s.SetTransform(m)
	}
}

func (s *Selector) Draw(screen *ebiten.Image) {
	geom := s.C.ToScreen()
	// draws an outline in pre-m coordinates
	outline := func(sblx, sbly, strx, stry float64, m Mx) {
		blx, bly := m.Apply(sblx, sbly)
		brx, bry := m.Apply(strx, sbly)
		tlx, tly := m.Apply(sblx, stry)
		trx, try := m.Apply(strx, stry)
		drawline(screen, blx, bly, brx, bry, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, brx, bry, trx, try, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, trx, try, tlx, tly, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, tlx, tly, blx, bly, 3, geom, color.RGBA{R: 255, A: 255})
	}
	if s.s != nil {
		t := s.s.Transform()
		outline(-0.5, -0.5, 0.5, 0.5, t)
		rotator := s.rotator()
		rx, ry := rotator.Apply(0, 0)
		drawpoint(screen, rx, ry, 10, geom, color.RGBA{R:255, A:255})
		for _, m := range s.scalars() {
			sx, sy := m.Apply(0, 0)
			drawpoint(screen, sx, sy, 10, geom, color.RGBA{R:255, A:255})
		}
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

// Adds delete functionality to art
type ArtSelector struct {
	l *Level
	a *Art
}

func (a *ArtSelector) Paste() Selectable {
	kopy := *a.a
	a.l.Art = append(a.l.Art, &kopy)
	return &ArtSelector{l: a.l, a: &kopy}
}

func (a *ArtSelector) Delete() {
	found := -1
	for i, o := range a.l.Art {
		if o == a.a {
			found = i
			break
		}
	}
	a.l.Art = append(a.l.Art[:found], a.l.Art[found+1:]...)
}

func (a *ArtSelector) Transform() Mx {
	return a.a.T
}

func (a *ArtSelector) SetTransform(m Mx) {
	a.a.T = m
}

// Adds delete functionality to blocks
type BlockSelector struct {
	l *Level
	b *Block
}

func (b *BlockSelector) Paste() Selectable {
	kopy := *b.b
	b.l.Blocks = append(b.l.Blocks, &kopy)
	return &BlockSelector{l: b.l, b: &kopy}
}

func (b *BlockSelector) Delete() {
	found := -1
	for i, o := range b.l.Blocks {
		if o == b.b {
			found = i
			break
		}
	}
	b.l.Blocks = append(b.l.Blocks[:found], b.l.Blocks[found+1:]...)
}

func (b *BlockSelector) Transform() Mx {
	return b.b.T
}

func (b *BlockSelector) SetTransform(m Mx) {
	b.b.T = m
}

// An editor that supports transforming arbitrary objects in the scene
type SelectEditor struct {
	s Selector
	e *Editor
}

func ActivateSelectEditor(r *Root, e *Editor) {
	var ss []Selectable
	ss = append(ss, &SpawnSelector{
		C: &e.c,
		L: &e.l,
	})
	for _, a := range e.l.Art {
		ss = append(ss, &ArtSelector{l: &e.l, a:a})
	}
	for _, b := range e.l.Blocks {
		ss = append(ss, &BlockSelector{b: b, l: &e.l})
	}
	r.a = &SelectEditor{
		s: Selector{
			C:           &e.c,
			Selectables: ss,
		},
		e: e,
	}
}

func (t *SelectEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return t.e.Layout(outsideWidth, outsideHeight)
}

func (t *SelectEditor) Update(r *Root) error {
	t.s.Update()
	return t.e.Update(r)
}

func (t *SelectEditor) Draw(screen *ebiten.Image) {
	t.e.Draw(screen)
	t.s.Draw(screen)
	ebitenutil.DebugPrintAt(screen, "Transform Editor", 10, t.e.c.sh-20)
}
