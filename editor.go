package main

import (
	"encoding/gob"
	"fmt"
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"math"
	"os"
)

// Level editing mode with routines for saving and loading levels
type Editor struct {
	g *Game
	// When you mouse dwon you start creating an entity and when you release you save it (making this nil).
	creating *Entity
	// The point that was initially clicked when creating the current entity
	cpin box2d.B2Vec2
	// Entities that were created manually and will be saved as a level when the game is saved
	created []*Entity
}

// Format of objects in saved levels
type Block struct {W float64; H float64; X float64; Y float64} // width, heigh, center in world coordinates

// Saves the level design to the given path
func (e *Editor) save(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		return fmt.Errorf("open file to save level: %w", err)
	}
	defer f.Close()
	encoder := gob.NewEncoder(f)
	for _, e := range e.created {
		position := e.b.GetPosition()
		block := Block{
			W: e.w,
			H: e.h,
			X: position.X,
			Y: position.Y,
		}
		err := encoder.Encode(block)
		if err != nil {
			return fmt.Errorf("save level: writing block %v: %w", block, err)
		}
	}
	fmt.Println("saved")
	return nil
}

// Loads the level at the given path into the game
func (e *Editor) load(path string) error {
	g := e.g
	g.init()
	// reset editor state
	*e = Editor{g: g}
	e.g.e = e
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file to load level: %w", err)
	}
	defer f.Close()
	decoder := gob.NewDecoder(f)
	var b Block
	for err = decoder.Decode(&b); err == nil; err = decoder.Decode(&b) {
		e.addRect(b.X, b.Y, b.W/2, b.H/2)
	}
	fmt.Println("loaded")
	return nil
}


// Adds a static rectangle to the physics simulation and render list with the given center and half width/half height.
func (e *Editor) addRect(cx, cy float64, hw, hh float64) *Entity {
	// make a body
	body := box2d.NewB2BodyDef()
	body.Position = box2d.B2Vec2{ X: cx, Y: cy,}

	shape := box2d.MakeB2PolygonShape()
	shape.SetAsBox(hw, hh)
	def := box2d.MakeB2FixtureDef()
	def.Shape = &shape
	def.Density = 1
	def.Friction = 0.3
	entity := Entity{
		w: hw*2,
		h: hh*2,
		b: e.g.world.CreateBody(body),
		restoresJump: true,
	}
	entity.b.SetUserData(&entity)
	e.g.entities = append(e.g.entities, &entity)
	entity.b.CreateFixtureFromDef(&def)
	e.created = append(e.created, &entity)
	return &entity
}

// Run a single tick of editing updates
func (e *Editor) Update() error {
	// save/load level
	if e.g.Clicked(ebiten.KeyS) {
		err := e.save("created.lvl")
		if err != nil {
			return fmt.Errorf("save created.lvl: %w", err)
		}
	}
	if e.g.Clicked(ebiten.KeyL) {
		err := e.load("created.lvl")
		if err != nil {
			return fmt.Errorf("load created.lvl: %w")
		}
	}
	if e.g.Clicked(ebiten.KeyR) {
		e.g.init()
		return nil
	}
	// level editing
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		wx, wy := e.g.Cursor()
		if e.creating == nil {
			// make a floor
			body := box2d.NewB2BodyDef()
			body.Position = box2d.B2Vec2{X: wx, Y: wy}
			e.creating = &Entity{
				w:            0,
				h:            0,
				b:            e.g.world.CreateBody(body),
				restoresJump: true,
			}
			e.creating.b.SetUserData(e.creating)
			e.cpin = body.Position
			e.g.entities = append(e.g.entities, e.creating)
		}
		minx := math.Min(wx, e.cpin.X)
		maxx := math.Max(wx, e.cpin.X)
		miny := math.Min(wy, e.cpin.Y)
		maxy := math.Max(wy, e.cpin.Y)
		e.creating.w = maxx - minx
		e.creating.h = maxy - miny
		e.creating.b.M_xf.P = box2d.B2Vec2{(maxx + minx)/2, (maxy + miny)/2}
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && e.creating != nil {
		shape := box2d.MakeB2PolygonShape()
		shape.SetAsBox(e.creating.w/2, e.creating.h/2)
		def := box2d.MakeB2FixtureDef()
		def.Shape = &shape
		def.Density = 1
		def.Friction = 0.3
		e.creating.b.CreateFixtureFromDef(&def)
		e.created = append(e.created, e.creating)
		e.creating = nil
	}
	return nil
}
