package main

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Obstacle struct {
	x float32
	y float32
	w float32
	h float32
}

type Game struct {
	playerX  float32
	playerY  float32
	vy       float32
	onGround bool

	obstacles []Obstacle
	spawnTick int

	score     int
	highScore int
	gameOver  bool
}

func isColliding(ax, ay, aw, ah, bx, by, bw, bh float32) bool {
	return ax < bx+bw &&
		ax+aw > bx &&
		ay < by+bh &&
		ay+ah > by
}

const (
	screenWidth  = 800
	screenHeight = 600

	playerWidth  = 40
	playerHeight = 40

	groundHeight = 100
)

func (g *Game) Update() error {
	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeyR) {
			g.playerX = 100
			g.playerY = float32(screenHeight - groundHeight - playerHeight)
			g.vy = 0
			g.onGround = true
			g.obstacles = nil
			g.spawnTick = 0
			g.score = 0
			g.gameOver = false
			return nil

		}
		return nil
	}

	// jump
	if ebiten.IsKeyPressed(ebiten.KeySpace) && g.onGround {
		g.vy = -10
		g.onGround = false
	}
	g.vy += 0.5
	g.playerY += g.vy
	groundY := float32(screenHeight - groundHeight - playerHeight)
	if g.playerY >= groundY {
		g.playerY = groundY
		g.vy = 0
		g.onGround = true
	}

	// obstacles
	g.spawnTick++
	if g.spawnTick >= 120 {
		g.spawnTick = 0
		ob := Obstacle{
			x: float32(screenWidth),
			y: float32(screenHeight - groundHeight - 40),
			w: 20,
			h: 40,
		}
		g.obstacles = append(g.obstacles, ob)

	}

	// colliding
	if !g.gameOver {
		g.score++

		for _, ob := range g.obstacles {
			if isColliding(
				g.playerX,
				g.playerY,
				playerWidth,
				playerHeight,
				ob.x,
				ob.y,
				ob.w,
				ob.h,
			) {
				if g.score > g.highScore {
					g.highScore = g.score
				}
				g.gameOver = true
				break
			}
		}
	}

	if g.gameOver {
		return nil
	}

	// move obstacles
	speed := float32(5)
	newObstacles := g.obstacles[:0]
	for _, ob := range g.obstacles {
		ob.x -= speed
		if ob.x+ob.w > 0 {
			newObstacles = append(newObstacles, ob)
		}
	}
	g.obstacles = newObstacles

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// background
	screen.Fill(color.White)

	// ground
	drawRect(screen, 0, float32(screenHeight-groundHeight), float32(screenWidth), 2, color.Black)

	// dino
	drawRect(screen, g.playerX, g.playerY, playerWidth, playerHeight, color.Black)

	// obstacles
	for _, ob := range g.obstacles {
		drawRect(screen, ob.x, ob.y, ob.w, ob.h, color.RGBA{0x22, 0x88, 0x22, 0xff})
	}

	// score
	scoreText := fmt.Sprintf("Score: %d", g.score)
	highScoreText := fmt.Sprintf("High Score: %d", g.highScore)
	ebitenutil.DebugPrintAt(screen, scoreText, 10, 10)
	ebitenutil.DebugPrintAt(screen, highScoreText, 10, 30)

	if g.gameOver {
		msg := "GAME OVER - Press R to Restart"
		ebitenutil.DebugPrintAt(screen, msg, screenWidth/2-100, 50)
	}
}

func drawRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color) {
	rect := ebiten.NewImage(int(w), int(h))
	rect.Fill(clr)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(rect, op)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	game := &Game{
		playerX:  100,
		playerY:  float32(screenHeight - groundHeight - playerHeight),
		onGround: true,
	}
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Dino Jump in Go")
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
