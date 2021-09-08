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


// Creates a new editor
func NewEditor() *Editor {
	var e Editor
	err := e.l.load(autosave)
	if err != nil {
		// autosave is broken, reset level
		e.l = Level{Platforms: []*Block{
			{
				W: 100,
				H: 0.5,
				Pos: box2d.B2Vec2{
					X: 0,
					Y: 0,
				},
			},
		}}
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
		name: "Spawn",
		key:  ebiten.KeyW,
		activate: func(r *Root, e *Editor) {
			r.a = &SpawnEditor{e}
		},
	},
	{
		name: "Art",
		key:  ebiten.KeyA,
		activate: func(r *Root, e *Editor) {
			r.a = &ArtEditor{e: e}
		},
	},
}

// Format of objects in saved levels
type Block struct {
	W float64
	H float64
	// width, height, center in world coordinates
	Pos box2d.B2Vec2
}

// Art to display on top of the level for covering up platforms and beautifying the world.
type Art struct {
	// The center of the art
	Pos box2d.B2Vec2
	// The path to load the art from from resources. e.g "resources/grass.png"
	Path string
	// Allows resizing the art.
	Scale box2d.B2Vec2
	// The loaded image. Always set once the level is loaded.
	img *ebiten.Image
}

// Computes the dimensions in world units of this art. Requires the art to be fully loaded
func (a *Art) Dim() (width, height float64) {
	iw, ih := a.img.Size()
	width, height = float64(iw)*a.Scale.X, float64(ih)*a.Scale.Y
	return
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
		body.Position = box2d.B2Vec2{X: p.Pos.X, Y: p.Pos.Y}

		shape := box2d.MakeB2PolygonShape()
		hw := p.W / 2
		hh := p.H / 2
		shape.SetAsBox(hw, hh)
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
			geo := ebiten.GeoM{}
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
			r.a = &Admin{g}
			return r.a.Update(r)
		}
		for _, sub := range subeditors {
			if Clicked(sub.key) {
				sub.activate(r, e)
				break
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
	geo := ebiten.GeoM{}
	geo.Translate(-block.W/2, -block.H/2)
	geo.Translate(block.Pos.X, block.Pos.Y)
	geo.Concat(screenTransform)
	vertices, is := rect(0, 0, float32(block.W), float32(block.H), color.RGBA{})
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
	drawline(screen, ssx-10, ssy-10, ssx+10, ssy+10, 3, ebiten.GeoM{}, color.White)
	drawline(screen, ssx-10, ssy+10, ssx+10, ssy-10, 3, ebiten.GeoM{}, color.White)

	for _, a := range e.l.Art {
		// unflip the images
		var geo ebiten.GeoM
		w, h := a.img.Size()
		geo.Translate(-float64(w)/2, -float64(h)/2)
		geo.Scale(a.Scale.X, a.Scale.Y)
		geo.Scale(1, -1)
		geo.Translate(a.Pos.X, a.Pos.Y)
		geo.Concat(screenTransform)
		screen.DrawImage(a.img, &ebiten.DrawImageOptions{GeoM: geo})
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
			p.creating = &Block{
				W: 0,
				H: 0,
				Pos: box2d.B2Vec2{
					X: wx,
					Y: wy,
				},
			}
			p.cpinx = wx
			p.cpiny = wy
		}
		minx := math.Min(wx, p.cpinx)
		maxx := math.Max(wx, p.cpinx)
		miny := math.Min(wy, p.cpiny)
		maxy := math.Max(wy, p.cpiny)
		p.creating.W = maxx - minx
		p.creating.H = maxy - miny
		p.creating.Pos.X = (maxx + minx) / 2
		p.creating.Pos.Y = (maxy + miny) / 2
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

type SpawnEditor struct{e *Editor}

func (s *SpawnEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return s.e.Layout(outsideWidth, outsideHeight)
}

func (s *SpawnEditor) Draw(screen *ebiten.Image) {
	s.e.Draw(screen)
	ebitenutil.DebugPrintAt(screen, "Spawn Editor", 10, s.e.c.sh-20)
}

func (s *SpawnEditor) Update(r *Root) error {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		wx, wy := s.e.c.Cursor()
		s.e.l.Spawn = box2d.B2Vec2{X: wx, Y: wy}
	}
	return s.e.Update(r)
}

// Editor for manipulating art in the level
type ArtEditor struct {
	// For moving art around
	sel *Art

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
		Scale: box2d.B2Vec2{1, 1},
		img:  img,
	})
	return nil
}

func (a *ArtEditor) String() string {
	return "Art"
}

func (a *ArtEditor) Update(r *Root) error {
	if MouseClicked(ebiten.MouseButtonLeft) {
		a.sel = nil
		wx, wy := a.e.c.Cursor()
		for _, art := range a.e.l.Art {
			w, h := art.Dim()
			if (wx > art.Pos.X-w/2) && (wx < art.Pos.X+w/2) && (wy > art.Pos.Y-h/2) && (wy < art.Pos.Y+h/2) {
				a.sel = art
				break
			}
		}
	}
	drag := MouseDrag(ebiten.MouseButtonLeft)
	if a.sel != nil && drag != (box2d.B2Vec2{}) {
		geo := ebiten.GeoM{}
		geo.Scale(1/(2*a.e.c.hw), 1/(2*a.e.c.hh))
		geo.Scale(1, -1)
		geo.Scale(float64(a.e.c.sw), float64(a.e.c.sh))
		geo.Invert()
		cx, cy := geo.Apply(drag.X, drag.Y)
		a.sel.Pos.OperatorPlusInplace(box2d.B2Vec2{cx, cy})
	}
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
	geom := a.e.c.ToScreen()
	if a.sel != nil {
		w, h := a.sel.Dim()
		drawline(screen, a.sel.Pos.X-w/2, a.sel.Pos.Y-h/2, a.sel.Pos.X+w/2, a.sel.Pos.Y-h/2, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, a.sel.Pos.X+w/2, a.sel.Pos.Y-h/2, a.sel.Pos.X+w/2, a.sel.Pos.Y+h/2, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, a.sel.Pos.X+w/2, a.sel.Pos.Y+h/2, a.sel.Pos.X-w/2, a.sel.Pos.Y+h/2, 3, geom, color.RGBA{R: 255, A: 255})
		drawline(screen, a.sel.Pos.X-w/2, a.sel.Pos.Y+h/2, a.sel.Pos.X-w/2, a.sel.Pos.Y-h/2, 3, geom, color.RGBA{R: 255, A: 255})
	}
	ebitenutil.DebugPrintAt(screen, a.result, 10, a.e.c.sh-20)
	ebitenutil.DebugPrintAt(screen, string(a.cmd), 10, a.e.c.sh-20)
	a.e.Draw(screen)
}
