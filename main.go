package main

import (
	_ "embed"
	"fmt"
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"log"
	"math"
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
	var g Game
	g.init()
	return fmt.Errorf("run game: %w", ebiten.RunGame(&g))
}

type Game struct {
	// width / height of screen
	w, h int

	world box2d.B2World

	c Camera
	p Player

	// Level editor
	e *Editor

	// Entities to draw on each frame
	entities []*Entity

	// Number of evaluated ticks for timekeeping.
	time int

	// Keys that have been clicked and are still being held, preventing them from generating additional clicks.
	down map[ebiten.Key]struct{}

}

// Resets the game to a clean initial state.
func (g *Game) init() {
	down := g.down
	*g = Game{}
	g.e = &Editor{g: g}
	g.down = make(map[ebiten.Key]struct{})
	if down != nil {
		g.down = down
	}
	g.c = Camera{
		hw: 180,
		hh: 120,
		x:  -60,
		y:  -40,
	}
	g.world = box2d.MakeB2World(box2d.MakeB2Vec2(0.0, -150.8))


	// set up the player
	player := box2d.NewB2BodyDef()
	player.Type = box2d.B2BodyType.B2_dynamicBody
	player.Awake = true
	player.Position = box2d.B2Vec2{ X: 10, Y: 20,}

	shape := box2d.MakeB2PolygonShape()
	shape.SetAsBox(5, 5)
	def := box2d.MakeB2FixtureDef()
	def.Shape = &shape
	def.Density = 1
	def.Friction = 3
	g.p = Player{
		w: 10,
		h: 10,
		s: &shape,
		b: g.world.CreateBody(player),
	}
	g.p.b.SetLinearDamping(0)
	g.p.b.CreateFixtureFromDef(&def)
	g.p.b.SetUserData(&g.p)

	// make a floor
	g.e.addRect(0, 0, 100, 3)
}


func (g *Game) BeginContact(contact box2d.B2ContactInterface) {
	var p *Player
	var e *Entity
	if np, ok := contact.GetFixtureA().GetBody().GetUserData().(*Player); ok {
		p = np
		e = contact.GetFixtureB().GetBody().GetUserData().(*Entity)
	} else if np, ok := contact.GetFixtureB().GetBody().GetUserData().(*Player); ok {
		p = np
		e = contact.GetFixtureA().GetBody().GetUserData().(*Entity)
	} else {
		return
	}
	if e.restoresJump {
		p.hasJump = true
	}
}

func (g *Game) EndContact(contact box2d.B2ContactInterface) {
}

func (g *Game) PreSolve(contact box2d.B2ContactInterface, oldManifold box2d.B2Manifold) {
}

func (g *Game) PostSolve(contact box2d.B2ContactInterface, impulse *box2d.B2ContactImpulse) {
}

// A renderable object in the physics sim
type Entity struct {
	// width and height for rendering
	w, h float64
	b *box2d.B2Body

	// If true, the players jump will be restored on contact with this entity
	restoresJump bool
}

type Camera struct {
	// half width/height of visible world in world units
	hw, hh float64
	// center of the camera in world units
	x, y float64
}

type Player struct {
	// dimensions in word units to use for rendering
	w, h float64
	// Width in world units of the drawing size of the player
	s *box2d.B2PolygonShape
	b *box2d.B2Body

	// when the player contacts a jump restoring surface it refreshes its ability to jump
	hasJump bool
	// used for jump cooldowns
	lastJump int

	// shooting cooldowns
	lastShot int
}

// Gets the cursor's position in world coordinates
func (g *Game) Cursor() (wx, wy float64) {
	toWorld := g.ToScreen()
	toWorld.Invert()
	sx, sy := ebiten.CursorPosition()
	wx, wy = toWorld.Apply(float64(sx), float64(sy))
	return
}

// Returns true if a given k has just started to be pressed
func (g *Game) Clicked(k ebiten.Key) bool {
	if !ebiten.IsKeyPressed(k) {
		return false
	}
	if _, ok := g.down[k]; ok {
		return false
	}
	g.down[k] = struct{}{}
	return true
}

func (g *Game) Update() error {
	g.time++
	for next := g.p.b.GetContactList(); next != nil; next = next.Next {
		g.BeginContact(next.Contact)
	}
	{
		// update down map
		for k, _ := range g.down {
			if !ebiten.IsKeyPressed(k) {
				delete(g.down, k)
			}
		}
	}
	{
		// movement
		velocity := g.p.b.GetLinearVelocity()
		if ebiten.IsKeyPressed(ebiten.KeyD) && velocity.X < 50 {
			g.p.b.ApplyForceToCenter(box2d.B2Vec2{1000*60, 0}, true)
		}
		if ebiten.IsKeyPressed(ebiten.KeyA) && velocity.X > -50 {
			g.p.b.ApplyForceToCenter(box2d.B2Vec2{-1000*60, 0}, true)
		}
		if ebiten.IsKeyPressed(ebiten.KeyW) {
			if g.p.hasJump && g.time - g.p.lastJump > 30 {
				g.p.b.ApplyForceToCenter(box2d.B2Vec2{0, 10000 * 60}, true)
				g.p.lastJump = g.time
			}
			g.p.hasJump = false
		}
		if ebiten.IsKeyPressed(ebiten.KeyE) {
			g.p.b.SetTransform(box2d.B2Vec2{0, 20}, g.p.b.GetAngle())
		}
	}
	{
		// camera zoom
		_, yoff := ebiten.Wheel()
		if yoff != 0 {
			g.c.hh *= math.Pow(0.98, yoff)
			g.c.hw *= math.Pow(0.98, yoff)
		}
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				g.c.hw *= 0.99
				g.c.hh *= 0.99
			} else {
				g.c.hw *= 1.01
				g.c.hh *= 1.01
			}
		}
	}
	// have camera approach player
	{
		position := g.p.b.GetPosition()
		g.c.x += 0.1 * (position.X - g.c.x)
		g.c.y += 0.1 * (position.Y - g.c.y)
	}
	{
		// camera pan
		if ebiten.IsKeyPressed(ebiten.KeyRight) {
			g.c.x++
		}
		if ebiten.IsKeyPressed(ebiten.KeyLeft) {
			g.c.x--
		}
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			g.c.y++
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			g.c.y--
		}
	}
	{
		// shooting
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) && g.time - g.p.lastShot > 30 {
			// fire away
			g.p.lastShot = g.time
			wx, wy := g.Cursor()
			pos := g.p.b.GetPosition()

			force := box2d.B2Vec2{wx - pos.X, wy - pos.Y}
			force.Normalize()
			force.OperatorScalarMulInplace(5)

			// Spawn bullet
			body := box2d.NewB2BodyDef()
			body.Position = box2d.B2Vec2{pos.X + force.X, pos.Y + force.Y}
			body.Type = box2d.B2BodyType.B2_dynamicBody
			e := &Entity{
				w:            2,
				h:            2,
				b:            g.world.CreateBody(body),
				restoresJump: false,
			}
			e.b.SetUserData(e)
			g.entities = append(g.entities, e)
			shape := box2d.MakeB2PolygonShape()
			shape.SetAsBox(1, 1)
			def := box2d.MakeB2FixtureDef()
			def.Shape = &shape
			def.Density = 1
			def.Friction = 0.3
			def.Restitution = 0.7
			e.b.CreateFixtureFromDef(&def)

			force.OperatorScalarMulInplace(100 * 60)
			e.b.ApplyForceToCenter(force, true)
			force.OperatorScalarMulInplace(-100)
			g.p.b.ApplyForceToCenter(force, true)
		}
	}
	err := g.e.Update()
	if err != nil {
		return fmt.Errorf("editing: %w", err)
	}

	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return fmt.Errorf("escape pressed")
	}

	g.world.Step(1.0/60., 16, 3)

	return nil
}

// Returns a transformation that converts points in world coordinates to screen coordinates
func (g *Game) ToScreen() ebiten.GeoM {
	geo := ebiten.GeoM{}
	geo.Translate(-g.c.x+g.c.hw, -g.c.y+g.c.hh)
	geo.Scale(1/(2*g.c.hw), 1/(2*g.c.hh))
	//geo.Scale(1, -1)
	geo.Scale(1, -1)
	geo.Translate(0, 1)
	geo.Scale(float64(g.w), float64(g.h))
	return geo
}

func (g *Game) Draw(screen *ebiten.Image) {
	//geo.Scale(1, -1)
	geo := ebiten.GeoM{}
	position := g.p.b.GetPosition()
	geo.Translate(-g.p.w/2, -g.p.h/2)
	geo.Rotate(g.p.b.GetAngle())
	geo.Translate(position.X, position.Y)
	screenTransform := g.ToScreen()
	geo.Concat(screenTransform)

	velocity := g.p.b.GetLinearVelocity()

	screen.DrawRectShader(g.w, g.h, mainShader, nil)
	screen.DrawRectShader(g.w, g.h, mainShader, &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Vx": float32(velocity.X),
			"Vy": float32(velocity.Y),
			"ScreenPixels": []float32{float32(g.w)*2, float32(g.h)*2},
		},
	})

	screen.DrawRectShader(int(g.p.w), int(g.p.h), mainShader, &ebiten.DrawRectShaderOptions{GeoM: geo,
		Uniforms: map[string]interface{}{
			"Vx": float32(velocity.X),
			"Vy": float32(velocity.Y),
			"ScreenPixels": []float32{float32(g.w), float32(g.h)},
		},
	})

	for _, e := range g.entities {
		geo := ebiten.GeoM{}
		position := e.b.GetPosition()
		geo.Translate(-e.w/2, -e.h/2)
		geo.Rotate(e.b.GetAngle())
		geo.Translate(position.X, position.Y)
		geo.Concat(screenTransform)
		velocity = e.b.GetLinearVelocity()
		vertices, is := rect(0, 0, float32(e.w), float32(e.h), color.RGBA{})
		for i, v := range vertices {
			sx, sy := geo.Apply(float64(v.DstX), float64(v.DstY))
			v.DstX = float32(sx)
			v.DstY = float32(sy)
			vertices[i] = v
		}
		screen.DrawTrianglesShader(vertices, is, mainShader, &ebiten.DrawTrianglesShaderOptions{
			CompositeMode: 0,
			Uniforms: map[string]interface{}{
				"Vx": float32(velocity.X),
				"Vy": float32(velocity.Y),
				"ScreenPixels": []float32{float32(g.w), float32(g.h)},
			},
			Images:        [4]*ebiten.Image{},
		})
		//screen.DrawRectShader(int(e.w), int(e.h), mainShader, &ebiten.DrawRectShaderOptions{GeoM: geo,
		//	Uniforms: map[string]interface{}{
		//		"Vx": float32(velocity.X),
		//		"Vy": float32(velocity.Y),
		//	},
		//})
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	g.w = outsideWidth
	g.h = outsideHeight
	g.c.hw = g.c.hh * float64(g.w)/float64(g.h)
	return outsideWidth, outsideHeight
}


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


