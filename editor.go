package main

import (
	"encoding/json"
	"fmt"
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hherman1/gobananas/resources"
	"image/color"
	"math"
	"os"
	"strings"
	"time"
)

// We automatically write updates to the current level to the autosave file periodically
const autosave = "autosave.lvl"

// Level editing mode with routines for saving and loading levels
type Editor struct {
	// Where in the level are we looking
	c Camera

	// The actual level
	l Level

	autotimer *time.Ticker
}
var unitVertices, unitIs = rect(0, 0, 1, 1, color.RGBA{})


// Creates a new editor
func NewEditor() *Editor {
	var e Editor
	e.autotimer = time.NewTicker(10 * time.Second)
	err := e.l.load(autosave)
	if err != nil {
		// autosave is broken, reset level
		geo := Mx{}
		geo.Scale(100, 0.5)
		e.l = NewLevel()
		e.l.Blocks = []*Block{{T: geo}}
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
		name: "Blocks",
		key:  ebiten.KeyL,
		activate: func(r *Root, e *Editor) {
			r.a = &PlatformEditor{e: e}
		},
	},
	{
		name: "Art",
		key:  ebiten.KeyA,
		activate: func(r *Root, e *Editor) {
			r.a = &ArtEditor{e: e, t: &Typer{
				Placeholder: "Art Editor: Press enter to load art resources into the level",
				C:           &e.c,
			}}
		},
	},
	{
		name:     "Select",
		key:      ebiten.KeyS,
		activate: ActivateSelectEditor,
	},
}

// Blocks are the serializable format for platforms in the game.
type Block struct {
	// A transformation that maps a unit square to a rectangle representing this block in world coordinates.
	T Mx
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

// Load the art from resources
func (a *Art) Load() error {
	img, err := resources.Image(a.Path)
	if err != nil {
		return fmt.Errorf("load image: %w", err)
	}
	a.img = img
	return nil
}

// Serializable audio file reference for use in level files
type Audio struct {
	// The file path for loading the audio
	Path string
	// A number between 0 and 1 indicating the volume to use for this audio. Default is 1
	Volume *float64
	// The decoded file for use as a player
	decoded []byte
	// The player for this audio, once loaded.
	player *audio.Player
}

// Loads the audio player into the audio struct. Must be called before sending the audio to the game
func (a *Audio) Load() error {
	p, err := resources.Audio(a.Path)
	if err != nil {
		return fmt.Errorf("load %v: %w", a.Path, err)
	}
	a.decoded = p
	a.player = audio.NewPlayerFromBytes(Actx, a.decoded)
	if a.Volume != nil {
		a.player.SetVolume(*a.Volume)
	}
	return nil
}

// Struct used for editing, saving, and loading levels
type Level struct {
	// Where does the player spawn in the level
	Spawn box2d.B2Vec2
	// All the platforms in the physics world
	Blocks []*Block
	// Images for display
	Art []*Art `json:",omitempty"`
	// Path to background audio which should play when game is running
	BGAudio *Audio `json:",omitempty"`
	// Art to display behind the camera at all times on this level. Transform is ignored.
	BGArt *Art
	// Art to render over the character
	PlayerArt *Art
	// Functions to call on certain game events
	Triggers map[string]Trigger
}

func NewLevel() Level {
	var l Level
	l.Triggers = make(map[string]Trigger)
	return l
}

// A trigger is some function that is called when a certain game event happens, e.g a player jump.
type Trigger struct {
	// If set, when this trigger is called it will play the given audio once.
	Audio *Audio
}

// Runs the actual trigger. Should only be called when the event its associated with happens.
func (t Trigger) Activate() {
	if t.Audio != nil  {
		_ = t.Audio.player.Seek(0)
		t.Audio.player.Play()
	}
}

func (t *Trigger) Load() error {
	if t.Audio != nil {
		err := t.Audio.Load()
		if err != nil {
			return fmt.Errorf("load audio: %w", err)
		}
	}
	return nil
}

// Saves the level design to the given path
func (l Level) save(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		return fmt.Errorf("open file to save level: %w", err)
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	err = encoder.Encode(l)
	if err != nil {
		return fmt.Errorf("save level: %w", err)
	}
	return nil
}

// Replaces a level with the one stored at the given path
func (l *Level) load(path string) error {
	*l = NewLevel()
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file to load level: %w", err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
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
	if l.PlayerArt != nil {
		err := l.PlayerArt.Load()
		if err != nil {
			return fmt.Errorf("load player art %v: %w", l.PlayerArt.Path, err)
		}
	}
	if l.BGArt != nil {
		err := l.BGArt.Load()
		if err != nil {
			return fmt.Errorf("load BG art %v: %w", l.BGArt.Path, err)
		}
	}
	if l.BGAudio != nil {
		err = l.BGAudio.Load()
		if err != nil {
			return fmt.Errorf("load bg audio: %w", err)
		}
	}
	for n, t := range l.Triggers {
		err := t.Load()
		if err != nil {
			return fmt.Errorf("load trigger '%v': %w", n, err)
		}
		l.Triggers[n] = t
	}
	return nil
}

// Adds the contents of this level to a given game world
func (l Level) apply(g *Game) {
	g.p.b.SetTransform(l.Spawn, 0)
	for _, p := range l.Blocks {
		// make a body
		body := box2d.NewB2BodyDef()
		cx, cy := p.T.Apply(0, 0)
		body.Position = box2d.B2Vec2{X: cx, Y: cy}

		// Compute half width, distance from center to right edge
		wx, wy := p.T.Apply(0.5, 0)
		hw := math.Sqrt((wx-cx)*(wx-cx) + (wy-cy)*(wy-cy))
		// Half height
		hx, hy := p.T.Apply(0, 0.5)
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
	g.bgArt = l.BGArt
	g.bgAudio = l.BGAudio
	g.pArt = l.PlayerArt
	g.Triggers = l.Triggers
}

// Run a single tick of editing updates
func (e *Editor) Update(r *Root) error {
	{
		// autosave
		select {
		case <-e.autotimer.C:
			err := e.l.save(autosave)
			if err != nil {
				fmt.Println("Failed to autosave:", err)
			}
		default:
		}
	}
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
			ActivateSave(r, e)
			return r.Update()
		}
		if Clicked(ebiten.KeyL) && ebiten.IsKeyPressed(ebiten.KeyMeta) {
			ActivateLoad(r, e)
			return r.Update()
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
		e.l = NewLevel()
		return nil
	}
	return nil
}

func (e *Editor) drawBlock(screen *ebiten.Image, block *Block) {
	screenTransform := e.c.ToScreen()
	geo := block.T
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
	// bg art
	if e.l.BGArt != nil {
		var geo Mx
		w, h := e.l.BGArt.img.Size()
		geo.Translate(float64(-w)/2, -float64(h)/2)
		scale := math.Max(float64(e.c.sh) / float64(h), float64(e.c.sw) / float64(w))
		geo.Scale(scale, scale)
		geo.Translate(float64(e.c.sw)/2, float64(e.c.sh)/2)
		screen.DrawImage(e.l.BGArt.img, &ebiten.DrawImageOptions{GeoM: geo.GeoM})
	}

	for _, entity := range e.l.Blocks {
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
	drawpoint(screen, e.l.Spawn.X, e.l.Spawn.Y, 20, screenTransform, color.White)

	for _, a := range e.l.Art {
		// unflip the images
		var geo Mx
		w, h := a.img.Size()
		geo.Scale(1/float64(w), 1/float64(h))
		geo.Translate(-0.5, -0.5)
		geo.Scale(1, -1)
		geo.Concat(a.T.GeoM)
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
	return "Blocks"
}

func (p *PlatformEditor) Update(r *Root) error {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		wx, wy := p.e.c.Cursor()
		if p.creating == nil {
			geo := Mx{}
			geo.Scale(0, 0)
			geo.Translate(wx, wy)
			p.creating = &Block{
				T: geo,
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
		p.creating.T = geo
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && p.creating != nil {
		p.e.l.Blocks = append(p.e.l.Blocks, p.creating)
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
	t *Typer

	// The editor we came from
	e *Editor
}

func (a *ArtEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return a.e.Layout(outsideWidth, outsideHeight)
}


func (a *ArtEditor) AddImage(e *Editor, path string) error {
	img, err := resources.Image(path)
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
	cmd, typ := a.t.Update()
	if typ {
		return nil
	}
	if cmd != "" {
		err := a.AddImage(a.e, cmd)
		if err != nil {
			a.t.Placeholder = fmt.Sprintf("Failed to load %v: %v", string(cmd), err)
		} else {
			a.t.Placeholder = fmt.Sprintf("Successfully loaded %v", string(cmd))
		}
	}
	return a.e.Update(r)
}

func (a *ArtEditor) Draw(screen *ebiten.Image) {
	a.e.Draw(screen)
	a.t.Draw(screen)
}

// A widget for creating interactive text inputs
type Typer struct {
	// For entering commands
	cmd []rune
	// Are we typing?
	typ bool

	// Text to print when input is empty
	Placeholder string
	// For drawing
	C *Camera
}

// Updates the state of the typer for this frame. Returns a complete message from the user, if available, and a boolean
// indicating whether or not the typer is currently typing. If the typer is typing, keyboard inputs should be ignored
// by the app.
func (t *Typer) Update() (string, bool) {
	// command processing
	{
		if Clicked(ebiten.KeyEnter) && !t.typ {
			t.typ = true
		} else if Clicked(ebiten.KeyEnter) && t.typ {
			cmd := string(t.cmd)
			t.typ = false
			t.cmd = []rune{}
			return cmd, false
		}
		if t.typ {
			if len(t.cmd) > 0 && Clicked(ebiten.KeyBackspace) {
				t.cmd = t.cmd[:len(t.cmd) - 1]
			}
			t.cmd = append(t.cmd, ebiten.InputChars()...)
			return "", true
		}
	}
	return "", false
}

func (t *Typer) Draw(screen *ebiten.Image) {
	if !t.typ {
		ebitenutil.DebugPrintAt(screen, t.Placeholder, 10, t.C.sh-20)
	} else {
		ebitenutil.DebugPrintAt(screen, string(t.cmd), 10, t.C.sh-20)
	}
}

// Save/load controller
type SaveAndLoadEditor struct {
	e *Editor
	t *Typer

	// If false, this is a save, if true this is a load
	load bool
}

// Activates a save editor
func ActivateSave(r *Root, e *Editor) {
	r.a = &SaveAndLoadEditor{
		e:    e,
		t:    &Typer{
			typ:         true,
			Placeholder: "",
			C:           &e.c,
		},
		load: false,
	}
}

// Activates a load editor
func ActivateLoad(r *Root, e *Editor) {
	r.a = &SaveAndLoadEditor{
		e:    e,
		t:    &Typer{
			typ:         true,
			Placeholder: "",
			C:           &e.c,
		},
		load: true,
	}
}

func (s *SaveAndLoadEditor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return s.e.Layout(outsideWidth, outsideHeight)
}

func (s *SaveAndLoadEditor) Update(r *Root) error {
	path, typ := s.t.Update()
	if typ {
		return nil
	}
	if path != "" {
		if s.load {
			err := s.e.l.load(path)
			if err != nil {
				return fmt.Errorf("failed to load %v: %w", path, err)
			}
		} else {
			err := s.e.l.save(path)
			if err != nil {
				fmt.Println("Failed to save:", err)
			}
		}
		r.a = s.e
		return r.Update()
	}
	return s.e.Update(r)
}

func (s *SaveAndLoadEditor) Draw(screen *ebiten.Image) {
	s.e.Draw(screen)
	s.t.Draw(screen)
}