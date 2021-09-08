package main

import (
	"github.com/ByteArena/box2d"
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"math"
)

// A renderable object in the physics sim
type Entity struct {
	// width and height for rendering
	w, h float64
	b *box2d.B2Body

	// If true, the players jump will be restored on contact with this entity
	restoresJump bool
}


// A game actually simulates a level and allows player control.
type Game struct {
	world box2d.B2World

	c Camera
	p Player

	// Entities to draw on each frame
	entities []*Entity

	// Fully loaded art for rendering
	art []*Art

	// Number of evaluated ticks for timekeeping.
	time int
}

// Creates a new game with a default player and empty world
func NewGame() *Game {
	var g Game
	g.c = Camera{
		hw: 12,
		hh: 8,
		x:  0,
		y:  0,
	}
	g.world = box2d.MakeB2World(box2d.MakeB2Vec2(0.0, -10.0))

	// set up the player
	player := box2d.NewB2BodyDef()
	player.Type = box2d.B2BodyType.B2_dynamicBody
	player.Awake = true
	player.Position = box2d.B2Vec2{ X: 0, Y: 3,}

	shape := box2d.MakeB2PolygonShape()
	shape.SetAsBox(0.5, 0.5)
	def := box2d.MakeB2FixtureDef()
	def.Shape = &shape
	def.Density = 1
	def.Friction = 3
	g.p = Player{
		w: 1,
		h: 1,
		s: &shape,
		b: g.world.CreateBody(player),
	}
	g.p.b.SetLinearDamping(0)
	g.p.b.CreateFixtureFromDef(&def)
	g.p.b.SetUserData(&g.p)

	return &g
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


func (g *Game) Update() error {
	g.time++
	for next := g.p.b.GetContactList(); next != nil; next = next.Next {
		g.BeginContact(next.Contact)
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
	{
		// movement
		velocity := g.p.b.GetLinearVelocity()
		if ebiten.IsKeyPressed(ebiten.KeyD) && velocity.X < 5 {
			g.p.b.ApplyForceToCenter(box2d.B2Vec2{60, 0}, true)
		}
		if ebiten.IsKeyPressed(ebiten.KeyA) && velocity.X > -5 {
			g.p.b.ApplyForceToCenter(box2d.B2Vec2{-60, 0}, true)
		}
		if ebiten.IsKeyPressed(ebiten.KeyW) {
			if g.p.hasJump && g.time - g.p.lastJump > 30 {
				g.p.b.ApplyForceToCenter(box2d.B2Vec2{0, 60*5}, true)
				g.p.lastJump = g.time
			}
			g.p.hasJump = false
		}
	}
	// have camera approach player
	{
		position := g.p.b.GetPosition()
		g.c.x += 0.1 * (position.X - g.c.x)
		g.c.y += 0.1 * (position.Y - g.c.y)
	}
	{
		// shooting
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) && g.time - g.p.lastShot > 30 {
			// fire away
			g.p.lastShot = g.time
			wx, wy := g.c.Cursor()
			pos := g.p.b.GetPosition()

			force := box2d.B2Vec2{wx - pos.X, wy - pos.Y}
			force.Normalize()
			force.OperatorScalarMulInplace(0.5)

			// Spawn bullet
			body := box2d.NewB2BodyDef()
			body.Position = box2d.B2Vec2{pos.X + force.X, pos.Y + force.Y}
			body.Type = box2d.B2BodyType.B2_dynamicBody
			e := &Entity{
				w:            0.25,
				h:            0.25,
				b:            g.world.CreateBody(body),
				restoresJump: false,
			}
			e.b.SetUserData(e)
			g.entities = append(g.entities, e)
			shape := box2d.MakeB2PolygonShape()
			shape.SetAsBox(0.125, 0.125)
			def := box2d.MakeB2FixtureDef()
			def.Shape = &shape
			def.Density = 1
			def.Friction = 0.3
			def.Restitution = 0.7
			e.b.CreateFixtureFromDef(&def)

			force.OperatorScalarMulInplace(100)
			e.b.ApplyForceToCenter(force, true)
			force.OperatorScalarMulInplace(-10)
			g.p.b.ApplyForceToCenter(force, true)
		}
	}
	g.world.Step(1.0/60., 16, 3)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	//geo.Scale(1, -1)
	geo := ebiten.GeoM{}
	position := g.p.b.GetPosition()
	geo.Translate(-g.p.w/2, -g.p.h/2)
	geo.Rotate(g.p.b.GetAngle())
	geo.Translate(position.X, position.Y)
	screenTransform := g.c.ToScreen()
	geo.Concat(screenTransform)

	velocity := g.p.b.GetLinearVelocity()

	screen.DrawRectShader(g.c.sw, g.c.sh, mainShader, &ebiten.DrawRectShaderOptions{
		Uniforms: map[string]interface{}{
			"Vx": float32(velocity.X),
			"Vy": float32(velocity.Y),
			"ScreenPixels": []float32{float32(g.c.sw)*2, float32(g.c.sh)*2},
		},
	})

	screen.DrawRectShader(int(g.p.w), int(g.p.h), mainShader, &ebiten.DrawRectShaderOptions{GeoM: geo,
		Uniforms: map[string]interface{}{
			"Vx": float32(velocity.X),
			"Vy": float32(velocity.Y),
			"ScreenPixels": []float32{float32(g.c.sw), float32(g.c.sh)},
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
				"ScreenPixels": []float32{float32(g.c.sw), float32(g.c.sh)},
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

	for _, a := range g.art {
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

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return g.c.Layout(outsideWidth, outsideHeight)
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
