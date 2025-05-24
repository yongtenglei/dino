package main

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Obstacle struct {
	x   float32
	y   float32
	img *ebiten.Image
}

type Game struct {
	playerX   float32
	playerY   float32
	vy        float32
	jumpCount int
	onGround  bool

	obstacles []Obstacle
	spawnTick int

	dinoFrames   []*ebiten.Image
	cactusFrames []*ebiten.Image
	animFrame    int
	animTick     int

	score     int
	highScore int
	gameOver  bool

	lastSpacePressed bool
}

func isColliding(ax, ay, aw, ah, am float32, bx, by, bw, bh, bm float32) bool {
	ax += am
	ay += am
	aw -= 2 * am
	ah -= 2 * am

	bx += bm
	by += bm
	bw -= 2 * bm
	bh -= 2 * bm

	return ax < bx+bw &&
		ax+aw > bx &&
		ay < by+bh &&
		ay+ah > by
}

const (
	maxjumpCount = 2

	screenWidth  = 800
	screenHeight = 600

	playerWidth  = 88
	playerHeight = 94

	dinoMargin   = float32(20)
	cactusMargin = float32(5)

	groundHeight = 100
)

func (g *Game) Update() error {
	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeyR) {
			g.playerX = 100
			g.playerY = float32(screenHeight - groundHeight - playerHeight)
			g.vy = 0
			g.onGround = true
			g.jumpCount = 0
			g.obstacles = nil
			g.spawnTick = 0
			g.score = 0
			g.gameOver = false
			return nil

		}
		return nil
	}

	// jump
	spaceNow := ebiten.IsKeyPressed(ebiten.KeySpace)
	if spaceNow && !g.lastSpacePressed && g.jumpCount < maxjumpCount {
		if g.jumpCount == 0 {
			g.vy = -10
		} else {
			g.vy = -9
		}
		g.jumpCount++
	}
	g.lastSpacePressed = spaceNow

	g.vy += 0.5
	g.playerY += g.vy
	groundY := float32(screenHeight - groundHeight - playerHeight)
	if g.playerY >= groundY {
		g.playerY = groundY
		g.vy = 0
		g.onGround = true
		g.jumpCount = 0
	}

	// obstacles
	g.spawnTick++
	if g.spawnTick >= rand.Intn(100)+150 {
		g.spawnTick = 0

		img := g.cactusFrames[rand.Intn(len(g.cactusFrames))]
		h := img.Bounds().Dy()

		ob := Obstacle{
			x:   float32(screenWidth),
			y:   float32(screenHeight - groundHeight - float32(h)),
			img: img,
		}
		g.obstacles = append(g.obstacles, ob)
	}

	// colliding
	if !g.gameOver {
		g.score++

		for _, ob := range g.obstacles {
			w, h := ob.img.Bounds().Dx(), ob.img.Bounds().Dy()

			if isColliding(
				g.playerX, g.playerY, playerWidth, playerHeight, dinoMargin,
				ob.x, ob.y, float32(w), float32(h), cactusMargin,
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
		w := ob.img.Bounds().Dx()
		if ob.x+float32(w) > 0 {
			newObstacles = append(newObstacles, ob)
		}
	}
	g.obstacles = newObstacles

	g.animTick++
	if g.animTick >= 10 {
		g.animTick = 0
		g.animFrame = (g.animFrame + 1) % len(g.dinoFrames)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// background
	screen.Fill(color.White)

	// ground
	drawRect(screen, 0, float32(screenHeight-groundHeight), float32(screenWidth), 2, color.Black)

	// dino
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(g.playerX), float64(g.playerY))
	screen.DrawImage(g.dinoFrames[g.animFrame], op)

	// obstacles
	for _, ob := range g.obstacles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(ob.x), float64(ob.y))
		screen.DrawImage(ob.img, op)
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

func loadSprite(path string) *ebiten.Image {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(img)
}

func main() {
	sprite := loadSprite("assets/sprite.png")

	// dino
	dinoFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1514, 0, 1514+88, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1603, 0, 1603+88, 0+94)).(*ebiten.Image),
	}

	// obstacles
	cactusFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(446, 2, 446+34, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(548, 2, 548+68, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(652, 2, 652+49, 2+100)).(*ebiten.Image),
		sprite.SubImage(image.Rect(802, 2, 802+99, 2+100)).(*ebiten.Image),
	}

	game := &Game{
		playerX:      100,
		playerY:      float32(screenHeight - groundHeight - 94),
		onGround:     true,
		dinoFrames:   dinoFrames,
		cactusFrames: cactusFrames,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Dino Jump in Go")
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
