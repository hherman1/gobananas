package main

import (
	_ "embed"
	"fmt"
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"image/color"
	"log"
	"strings"
)

//go:embed main_shader.go
var mainShaderRaw []byte
var mainShader *ebiten.Shader


// World units: Increasing Y is moving up in the world.
// Screen units: opposite

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var err error
	mainShader, err = ebiten.NewShader(mainShaderRaw)
	if err != nil {
		return fmt.Errorf("loading main shader: %w", err)
	}

	ebiten.SetWindowSize(720, 480)
	ebiten.SetWindowResizable(true)
	var a App
	a.g = NewGame()
	a.e = &Editor{
		l: Level{Platforms: []*Block{
			{
				W: 100,
				H: 0.5,
				Pos: box2d.B2Vec2{
					X: 0,
					Y: 0,
				},
			},
		}},
	}
	a.e.l.apply(a.g)
	return fmt.Errorf("run game: %w", ebiten.RunGame(&a))
}

type gmode int
const (
	gplay gmode = iota
	gedit
)

func (g gmode) String() string {
	switch g {
	case gedit:
		return "Edit"
	case gplay:
		return "Play"
	}
	return "Unknown"
}

type App struct {
	// Level editor
	e *Editor
	// Current game instance
	g *Game
	// Current engine mode
	mode gmode
}

func (a *App) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	switch a.mode {
	case gplay:
		return a.g.Layout(outsideWidth, outsideHeight)
	case gedit:
		return a.e.Layout(outsideWidth, outsideHeight)
	}
	return outsideWidth, outsideHeight
}

// switch to edit mode
func (a *App) edit() {
	a.mode = gedit
}

// switch to play mode
func (a *App) play() {
	if a.mode == gedit {
		a.g = NewGame()
		a.e.l.apply(a.g)
	}
	a.mode = gplay
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
func (c *Camera) ToScreen() ebiten.GeoM {
	geo := ebiten.GeoM{}
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

func (a *App) Update() error {
	// Universal updates
	InputsUpdate()
	{
		// mode switches
		if Clicked(ebiten.KeyE) {
			a.edit()
			return nil
		}
		if Clicked(ebiten.KeyP) {
			a.play()
			return nil
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return fmt.Errorf("escape pressed")
	}

	// Play mode updates
	if a.mode == gplay {
		err := a.g.Update()
		if err != nil {
			return fmt.Errorf("playing: %w", err)
		}
	} else if a.mode == gedit {
		// edit mode updates
		err := a.e.Update()
		if err != nil {
			return fmt.Errorf("editing: %w", err)
		}
	}

	return nil
}

func (a *App) Draw(screen *ebiten.Image) {

	if a.mode == gplay {
		a.g.Draw(screen)
	} else if a.mode == gedit {
		a.e.Draw(screen)
	}

	var str strings.Builder
	str.WriteString("Mode: ")
	str.WriteString(a.mode.String())
	str.WriteString(" - (e) Edit; (p) Play")
	ebitenutil.DebugPrintAt(screen, str.String(), 10, 10)
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


