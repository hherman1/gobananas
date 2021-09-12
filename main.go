package main

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hherman1/gobananas/resources"
	"image/color"
	"log"
	"math"
)

var mainShader *ebiten.Shader
var outlineShader *ebiten.Shader


// Serializable wrapper around ebiten's matrix transform type.
type Mx struct {
	ebiten.GeoM
}

func (m *Mx) GobDecode(bytes []byte) error {
	m.SetElement(0, 0, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*0:])))
	m.SetElement(0, 1, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*1:])))
	m.SetElement(0, 2, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*2:])))
	m.SetElement(1, 0, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*3:])))
	m.SetElement(1, 1, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*4:])))
	m.SetElement(1, 2, math.Float64frombits(binary.BigEndian.Uint64(bytes[8*5:])))
	return nil
}

func (m Mx) GobEncode() ([]byte, error) {
	var out [8*6]byte
	binary.BigEndian.PutUint64(out[8*0:], math.Float64bits(m.Element(0, 0)))
	binary.BigEndian.PutUint64(out[8*1:], math.Float64bits(m.Element(0, 1)))
	binary.BigEndian.PutUint64(out[8*2:], math.Float64bits(m.Element(0, 2)))
	binary.BigEndian.PutUint64(out[8*3:], math.Float64bits(m.Element(1, 0)))
	binary.BigEndian.PutUint64(out[8*4:], math.Float64bits(m.Element(1, 1)))
	binary.BigEndian.PutUint64(out[8*5:], math.Float64bits(m.Element(1, 2)))
	return out[:], nil
}

// World units: Increasing Y is moving up in the world.
// Screen units: opposite

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var err error
	mainShader, err = resources.Shader("shaders/main_shader.go")
	if err != nil {
		return fmt.Errorf("loading main shader: %w", err)
	}
	outlineShader, err = resources.Shader("shaders/outline_shader.go")
	if err != nil {
		return fmt.Errorf("loading outline shader: %w", err)
	}

	ebiten.SetWindowSize(720, 480)
	ebiten.SetWindowResizable(true)
	r := Root{NewEditor()}
	return fmt.Errorf("run game: %w", ebiten.RunGame(&r))
}

// The root is a wrapper that implements the game interface and allows games to rewrap themselves to promote a new
// leader game.
type Root struct {
	a App
}

func (r *Root) Update() error {
	// Universal updates
	InputsUpdate()
	// Circumvent keyboard disabling
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return fmt.Errorf("escape pressed")
	}
	return r.a.Update(r)
}

func (r *Root) Draw(screen *ebiten.Image) {
	r.a.Draw(screen)
}

func (r *Root) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return r.a.Layout(outsideWidth, outsideHeight)
}

// An app is a composable runtime for ebiten that can manipulate the world state. It is identical to ebiten.Game
// except Update receives a reference to the root game interface so that the top level app can be exchanged
type App interface {
	Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int)
	Update(r *Root) error
	Draw(screen *ebiten.Image)
}

type Camera struct {
	// Screen width and height in pixels
	sw, sh int
	// half width/height of visible world in world units
	hw, hh float64
	// center of the camera in world units
	x, y float64
}

// Returns a transformation that converts points in world coordinates to screen coordinates for the camera
func (c *Camera) ToScreen() Mx {
	geo := Mx{}
	geo.Translate(-c.x+c.hw, -c.y+c.hh)
	geo.Scale(1/(2*c.hw), 1/(2*c.hh))
	//geo.Scale(1, -1)
	geo.Scale(1, -1)
	geo.Translate(0, 1)
	geo.Scale(float64(c.sw), float64(c.sh))
	return geo
}

func (c *Camera) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	if c.hh == 0 {
		c.hh = 8
	}
	if c.hw == 0 {
		c.hw = 12
	}
	c.sw = outsideWidth
	c.sh = outsideHeight
	c.hw = c.hh * float64(c.sw)/float64(c.sh)
	return outsideWidth, outsideHeight
}

// Gets the cursor's position in world coordinates for the given camera
func (c *Camera) Cursor() (wx, wy float64) {
	toWorld := c.ToScreen()
	toWorld.Invert()
	sx, sy := ebiten.CursorPosition()
	wx, wy = toWorld.Apply(float64(sx), float64(sy))
	return
}

// An admin app that wraps the game and exposes shortcuts for swapping into other tools
type Admin struct {
	// Current game instance
	g *Game
}

func (a *Admin) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return a.g.Layout(outsideWidth, outsideHeight)
}

func (a *Admin) Update(r *Root) error {
	if Clicked(ebiten.KeyE) {
		r.a = NewEditor()
		return r.a.Update(r)
	}
	err := a.g.Update()
	if err != nil {
		return fmt.Errorf("playing: %w", err)
	}
	return nil
}

func (a *Admin) Draw(screen *ebiten.Image) {
	a.g.Draw(screen)
	ebitenutil.DebugPrintAt(screen, "(E) Edit Mode", 10, 10)
}


// Utility function for creating a rectange of vertices and indices
func rect(x, y, w, h float32, clr color.RGBA) ([]ebiten.Vertex, []uint16) {
	r := float32(clr.R) / 0xff
	g := float32(clr.G) / 0xff
	b := float32(clr.B) / 0xff
	a := float32(clr.A) / 0xff
	x0 := x
	y0 := y
	x1 := x + w
	y1 := y + h

	return []ebiten.Vertex{
		{
			DstX:   x0,
			DstY:   y0,
			SrcX:   1,
			SrcY:   1,
			ColorR: r,
			ColorG: g,
			ColorB: b,
			ColorA: a,
		},
		{
			DstX:   x1,
			DstY:   y0,
			SrcX:   1,
			SrcY:   1,
			ColorR: r,
			ColorG: g,
			ColorB: b,
			ColorA: a,
		},
		{
			DstX:   x0,
			DstY:   y1,
			SrcX:   1,
			SrcY:   1,
			ColorR: r,
			ColorG: g,
			ColorB: b,
			ColorA: a,
		},
		{
			DstX:   x1,
			DstY:   y1,
			SrcX:   1,
			SrcY:   1,
			ColorR: r,
			ColorG: g,
			ColorB: b,
			ColorA: a,
		},
	}, []uint16{0, 1, 2, 1, 2, 3}
}

