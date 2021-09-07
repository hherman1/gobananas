package main

import (
	"encoding/gob"
	"fmt"
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"image/color"
	"math"
	"os"
)

// Different editor modes
type emode int

const (
	eplatforms emode = iota
)

var emodes = []emode{eplatforms}

func (e emode) String() string {
	switch e {
	case eplatforms:
		return "Platforms"
	}
	return "Unknown"
}

// Level editing mode with routines for saving and loading levels
type Editor struct {
	// Where in the level are we looking
	c Camera
	// When you mouse dwon you start creating an entity and when you release you save it (making this nil).
	creating *Block
	// The point that was initially clicked when creating the current block
	cpinx float64
	cpiny float64

	// The actual level
	level Level

	// Current editing mode.
	emode emode
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return e.c.Layout(outsideWidth, outsideHeight)
}

// Format of objects in saved levels
type Block struct {W float64; H float64; X float64; Y float64} // width, heigh, center in world coordinates

// Struct used for editing, saving, and loading levels
type Level struct {
	Platforms []*Block
}
// Saves the level design to the given path
func (l Level) save(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		return fmt.Errorf("open file to save level: %w", err)
	}
	defer f.Close()
	encoder := gob.NewEncoder(f)
	err = encoder.Encode(l)
	if err != nil {
		return fmt.Errorf("save level: %w", err)
	}
	return nil
}

// Replaces a level with the one stored at the given path
func (l *Level) load(path string) error {
	*l = Level{}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file to load level: %w", err)
	}
	defer f.Close()
	decoder := gob.NewDecoder(f)
	err = decoder.Decode(l)
	if err != nil {
		return fmt.Errorf("decoding level from file: %w", err)
	}
	return nil
}

// Adds the contents of this level to a given game world
func (l Level) apply(g *Game) {
	for _, p := range l.Platforms {
		// make a body
		body := box2d.NewB2BodyDef()
		body.Position = box2d.B2Vec2{ X: p.X, Y: p.Y,}

		shape := box2d.MakeB2PolygonShape()
		hw := p.W / 2
		hh := p.H / 2
		shape.SetAsBox(hw, hh)
		def := box2d.MakeB2FixtureDef()
		def.Shape = &shape
		def.Density = 1
		def.Friction = 0.3
		entity := Entity{
			w: hw*2,
			h: hh*2,
			b: g.world.CreateBody(body),
			restoresJump: true,
		}
		entity.b.SetUserData(&entity)
		g.entities = append(g.entities, &entity)
		entity.b.CreateFixtureFromDef(&def)
	}
}

// Run a single tick of editing updates
func (e *Editor) Update() error {
	{
		// camera zoom
		_, yoff := ebiten.Wheel()
		if yoff != 0 {
			e.c.hh *= math.Pow(0.98, yoff)
			e.c.hw *= math.Pow(0.98, yoff)
		}
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				e.c.hw *= 0.99
				e.c.hh *= 0.99
			} else {
				e.c.hw *= 1.01
				e.c.hh *= 1.01
			}
		}
	}
	// save/load level
	if Clicked(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyMeta){
		err := e.level.save("created.lvl")
		if err != nil {
			return fmt.Errorf("save created.lvl: %w", err)
		}
		fmt.Println("saved")
	}
	if Clicked(ebiten.KeyL) && ebiten.IsKeyPressed(ebiten.KeyMeta){
		err := e.level.load("created.lvl")
		if err != nil {
			return fmt.Errorf("load created.lvl: %w")
		}
	}
	if Clicked(ebiten.KeyR) {
		e.level = Level{}
		return nil
	}
	if !ebiten.IsKeyPressed(ebiten.KeyMeta) {
		if ebiten.IsKeyPressed(ebiten.KeyA) {
			e.c.x -= 3
		}
		if ebiten.IsKeyPressed(ebiten.KeyD) {
			e.c.x += 3
		}
		if ebiten.IsKeyPressed(ebiten.KeyW) {
			e.c.y += 3
		}
		if ebiten.IsKeyPressed(ebiten.KeyS) {
			e.c.y -= 3
		}
	}

	// platform editing
	if e.emode == eplatforms {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			wx, wy := e.c.Cursor()
			if e.creating == nil {
				e.creating = &Block{
					W: 0,
					H: 0,
					X: wx,
					Y: wy,
				}
				e.cpinx = wx
				e.cpiny = wy
			}
			minx := math.Min(wx, e.cpinx)
			maxx := math.Max(wx, e.cpinx)
			miny := math.Min(wy, e.cpiny)
			maxy := math.Max(wy, e.cpiny)
			e.creating.W = maxx - minx
			e.creating.H = maxy - miny
			e.creating.X = (maxx + minx)/2
			e.creating.Y = (maxy + miny)/2
		}
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && e.creating != nil {
			e.level.Platforms = append(e.level.Platforms, e.creating)
			e.creating = nil
		}
	}
	return nil
}

func (e *Editor) drawBlock(screen *ebiten.Image, block *Block) {
	screenTransform := e.c.ToScreen()
	geo := ebiten.GeoM{}
	geo.Translate(-block.W/2, -block.H/2)
	geo.Translate(block.X, block.Y)
	geo.Concat(screenTransform)
	vertices, is := rect(0, 0, float32(block.W), float32(block.H), color.RGBA{})
	for i, v := range vertices {
		sx, sy := geo.Apply(float64(v.DstX), float64(v.DstY))
		v.DstX = float32(sx)
		v.DstY = float32(sy)
		vertices[i] = v
	}
	screen.DrawTrianglesShader(vertices, is, mainShader, &ebiten.DrawTrianglesShaderOptions{
		CompositeMode: 0,
		Uniforms: map[string]interface{}{
			"ScreenPixels": []float32{float32(e.c.sw), float32(e.c.sh)},
		},
		Images:        [4]*ebiten.Image{},
	})

}

func (e *Editor) Draw(screen *ebiten.Image) {
	for _, entity := range e.level.Platforms {
		e.drawBlock(screen, entity)
	}
	if e.creating != nil {
		e.drawBlock(screen, e.creating)
	}
	for i, m := range emodes {
		str := m.String()
		if e.emode == m {
			str += " *"
		}
		ebitenutil.DebugPrintAt(screen, str, 10, 30 + 10*i)
	}
}
