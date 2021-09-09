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
	"strings"
)

// We automatically write updates to the current level to the autosave file periodically
const autosave = "autosave.lvl"

// Level editing mode with routines for saving and loading levels
type Editor struct {
	// Where in the level are we looking
	c Camera

	// The actual level
	l Level
}
var unitVertices, unitIs = rect(0, 0, 1, 1, color.RGBA{})


// Creates a new editor
func NewEditor() *Editor {
	var e Editor
	err := e.l.load(autosave)
	if err != nil {
		// autosave is broken, reset level
		geo := Mx{}
		geo.Scale(100, 0.5)
		e.l = Level{Platforms: []*Block{{Mat: geo}}}
	}
	return &e
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return e.c.Layout(outsideWidth, outsideHeight)
}

var subeditors = []struct {
	name     string
	key      ebiten.Key
	activate func(r *Root, e *Editor)
}{
	{
		name: "Platforms",
		key:  ebiten.KeyL,
		activate: func(r *Root, e *Editor) {
			r.a = &PlatformEditor{e: e}
		},
	},
	{
		name: "Art",
		key:  ebiten.KeyA,
		activate: func(r *Root, e *Editor) {
			r.a = &ArtEditor{e: e}
		},
	},
	{
		name: "Transform",
		key:  ebiten.KeyT,
		activate: ActivateTransformEditor,
	},
}

// Blocks are the serializable format for platforms in the game.
type Block struct {
	// A transformation that maps a unit square to a rectangle representing this block in world coordinates.
	Mat Mx
}

func (b *Block) Transform() Mx {
	return b.Mat
}

func (b *Block) SetTransform(m Mx) {
	b.Mat = m
}

// Art to display on top of the level for covering up platforms and beautifying the world.
type Art struct {
	// Transform that positions a unit square centered at 0,0 to the correct rectangle on which to draw this art
	T Mx
	// The path to load the art from from resources. e.g "resources/grass.png"
	Path string
	// The loaded image. Always set once the level is loaded.
	img *ebiten.Image
}

func (a *Art) Transform() Mx {
	return a.T
}

func (a *Art) SetTransform(m Mx) {
	a.T = m
}

// Load the art from resources
func (a *Art) Load() error {
	img, err := Image(a.Path)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	a.img = img
	return nil
}

// Struct used for editing, saving, and loading levels
type Level struct {
	// Where does the player spawn in the level
	Spawn box2d.B2Vec2
	// All the platforms in the physics world
	Platforms []*Block
	// Images for display
	Art []*Art
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
	for _, a := range l.Art {
		err := a.Load()
		if err != nil {
			return fmt.Errorf("load %v: %w", a.Path, err)
		}
	}
	return nil
}

// Adds the contents of this level to a given game world
func (l Level) apply(g *Game) {
	g.p.b.SetTransform(l.Spawn, 0)
	for _, p := range l.Platforms {
		// make a body
		body := box2d.NewB2BodyDef()
		cx, cy := p.Mat.Apply(0, 0)
		body.Position = box2d.B2Vec2{X: cx, Y: cy}

		// Compute half width, distance from center to right edge
		wx, wy := p.Mat.Apply(0.5, 0)
		hw := math.Sqrt((wx-cx)*(wx-cx) + (wy-cy)*(wy-cy))
		// Half height
		hx, hy := p.Mat.Apply(0, 0.5)
		hh := math.Sqrt((hx-cx)*(hx-cx) + (hy-cy)*(hy-cy))
		shape := box2d.MakeB2PolygonShape()
		shape.SetAsBox(hw, hh)

		// Angle, rotation between transformed right edge and original right edge
		ax, ay := wx - cx, wy - cy
		body.Angle = math.Atan2(ay, ax)

		def := box2d.MakeB2FixtureDef()
		def.Shape = &shape
		def.Density = 1
		def.Friction = 0.3
		entity := Entity{
			w:            hw * 2,
			h:            hh * 2,
			b:            g.world.CreateBody(body),
			restoresJump: true,
		}
		entity.b.SetUserData(&entity)
		g.entities = append(g.entities, &entity)
		entity.b.CreateFixtureFromDef(&def)
	}
	for _, a := range l.Art {
		g.art = append(g.art, a)
	}
}

// Run a single tick of editing updates
func (e *Editor) Update(r *Root) error {
	{
		// camera controls
		_, yoff := ebiten.Wheel()
		if yoff != 0 {
			e.c.hh *= math.Pow(0.98, yoff)
			e.c.hw *= math.Pow(0.98, yoff)
		}
		d := MouseDrag(ebiten.MouseButtonRight)
		if d != (box2d.B2Vec2{}) {
			geo := Mx{}
			geo.Scale(1/(2*e.c.hw), 1/(2*e.c.hh))
			geo.Scale(-1, 1)
			geo.Scale(float64(e.c.sw), float64(e.c.sh))
			geo.Invert()
			cx, cy := geo.Apply(d.X, d.Y)
			e.c.x += cx
			e.c.y += cy
		}
	}
	// save/load level
	{
		if Clicked(ebiten.KeyS) && ebiten.IsKeyPressed(ebiten.KeyMeta) {
			err := e.l.save("created.lvl")
			if err != nil {
				return fmt.Errorf("save created.lvl: %w", err)
			}
			fmt.Println("saved")
		}
		if Clicked(ebiten.KeyL) && ebiten.IsKeyPressed(ebiten.KeyMeta) {
			err := e.l.load("created.lvl")
			if err != nil {
				return fmt.Errorf("load created.lvl: %w")
			}
		}
	}
	// switch mode
	{
		if Clicked(ebiten.KeyP) {
			// play mode
			g := NewGame()
			e.l.apply(g)
			err := e.l.save(autosave)
			if err != nil {
				// still usable, just buggy
				fmt.Println("Failed to autosave:", err)
			}
			r.a = &Admin{g}
			return r.a.Update(r)
		}
		for _, sub := range subeditors {
			if Clicked(sub.key) {
				sub.activate(r, e)
				return r.Update()
			}
		}
	}
	// reset
	if Clicked(ebiten.KeyR) {
		e.l = Level{}
		return nil
	}
	return nil
}

func (e *Editor) drawBlock(screen *ebiten.Image, block *Block) {
	screenTransform := e.c.ToScreen()
	geo := block.Mat
	geo.Concat(screenTransform.GeoM)
	vertices, is := rect(-0.5, -0.5, 1, 1, color.RGBA{})
	for i, v := range vertices {
		sx, sy := geo.Apply(float64(v.DstX), float64(v.DstY))
		v.DstX = float32(sx)
		v.DstY = float32(sy)
		vertices[i] = v
	}
	screen.DrawTrianglesShader(vertices, is, mainShader, &ebiten.DrawTrianglesShaderOptions{
		Uniforms: map[string]interface{}{
			"ScreenPixels": []float32{float32(e.c.sw), float32(e.c.sh)},
		},
		Images: [4]*ebiten.Image{},
	})
}

func (e *Editor) Draw(screen *ebiten.Image) {
	for _, entity := range e.l.Platforms {
		e.drawBlock(screen, entity)
	}
	var s strings.Builder
	s.WriteString(`(P) Play

Editors:
`)
	for _, sub := range subeditors {
		_, _ = fmt.Fprintf(&s, "(%v) %v\n", sub.key, sub.name)
	}
	ebitenutil.DebugPrintAt(screen, s.String(), 10, 5)

	// player spawn
	screenTransform := e.c.ToScreen()
	ssx, ssy := screenTransform.Apply(e.l.Spawn.X, e.l.Spawn.Y)
	drawline(screen, ssx-10, ssy-10, ssx+10, ssy+10, 3, Mx{}, color.White)
	drawline(screen, ssx-10, ssy+10, ssx+10, ssy-10, 3, Mx{}, color.White)

	for _, a := range e.l.Art {
		// unflip the images
		var geo Mx
		w, h := a.img.Size()
		geo.Scale(1/float64(w), 1/float64(h))
		geo.Translate(-0.5, -0.5)
		geo.Scale(1, -1)
		geo.Concat(a.Transform().GeoM)
		geo.Concat(screenTransform.GeoM)
		screen.DrawImage(a.img, &ebiten.DrawImageOptions{GeoM: geo.GeoM})
	}
}

type PlatformEditor struct {
	// When you mouse dwon you start creating an entity and when you release you save it (making this nil).
	creating *Block
	// The point that was initially clicked when creating the current block
	cpinx float64
	cpiny float64

	e *Editor
}

func (p *PlatformEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return p.e.Layout(outsideWidth, outsideHeight)
}


func (p *PlatformEditor) String() string {
	return "Platforms"
}

func (p *PlatformEditor) Update(r *Root) error {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		wx, wy := p.e.c.Cursor()
		if p.creating == nil {
			geo := Mx{}
			geo.Scale(0, 0)
			geo.Translate(wx, wy)
			p.creating = &Block{
				Mat: geo,
			}
			p.cpinx = wx
			p.cpiny = wy
		}
		minx := math.Min(wx, p.cpinx)
		maxx := math.Max(wx, p.cpinx)
		miny := math.Min(wy, p.cpiny)
		maxy := math.Max(wy, p.cpiny)
		geo := Mx{}
		geo.Translate(0.5, 0.5)
		geo.Scale(maxx - minx, maxy - miny)
		geo.Translate(minx, miny)
		p.creating.Mat = geo
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && p.creating != nil {
		p.e.l.Platforms = append(p.e.l.Platforms, p.creating)
		p.creating = nil
	}
	return p.e.Update(r)
}

func (p *PlatformEditor) Draw(screen *ebiten.Image) {
	if p.creating != nil {
		p.e.drawBlock(screen, p.creating)
	}
	ebitenutil.DebugPrintAt(screen, "Platform Editor", 10, p.e.c.sh-20)
	p.e.Draw(screen)
}

// Editor for manipulating art in the level
type ArtEditor struct {
	// For entering commands
	cmd []rune
	// Response from the subeditor
	result string
	// Are we typing?
	typ bool

	// The editor we came from
	e *Editor
}

func (a *ArtEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return a.e.Layout(outsideWidth, outsideHeight)
}


func (a *ArtEditor) AddImage(e *Editor, path string) error {
	img, err := Image(path)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	e.l.Art = append(e.l.Art, &Art{
		Path: path,
		img:  img,
	})
	return nil
}

func (a *ArtEditor) String() string {
	return "Art"
}

func (a *ArtEditor) Update(r *Root) error {
	// command processing
	{
		if !a.typ && a.result == "" {
			a.result = "Art Editor: Press enter to load an image by path (e.g resources/grass.png)"
		}
		if Clicked(ebiten.KeyEnter) && !a.typ {
			a.typ = true
			a.result = ""
		} else if Clicked(ebiten.KeyEnter) && a.typ {
			err := a.AddImage(a.e, string(a.cmd))
			if err != nil {
				a.result = fmt.Sprintf("Failed to load %v: %v", string(a.cmd), err)
			} else {
				a.result = fmt.Sprintf("Successfully loaded %v", string(a.cmd))
			}
			a.typ = false
			a.cmd = []rune{}
		}
		if a.typ {
			a.cmd = append(a.cmd, ebiten.InputChars()...)
			return nil
		}
	}
	return a.e.Update(r)
}

func (a *ArtEditor) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, a.result, 10, a.e.c.sh-20)
	ebitenutil.DebugPrintAt(screen, string(a.cmd), 10, a.e.c.sh-20)
	a.e.Draw(screen)
}
