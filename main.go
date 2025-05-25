package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/basicfont"
)

var gray = color.RGBA{0x88, 0x88, 0x88, 0xff}

type Obstacle struct {
	x   float64
	y   float64
	img *ebiten.Image
}

type Game struct {
	playerX   float64
	playerY   float64
	vy        float64
	jumpCount int
	onGround  bool

	obstacles []Obstacle
	spawnTick int

	dinoFrames   []*ebiten.Image
	cactusFrames []*ebiten.Image
	animFrame    int
	animTick     int

	groundFrame *ebiten.Image
	groundX     float64

	score     int
	highScore int
	gameOver  bool

	lastSpacePressed bool
}

func isColliding(ax, ay, aw, ah, am float64, bx, by, bw, bh, bm float64) bool {
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

	dinoMargin   = float64(20)
	cactusMargin = float64(5)

	groundHeight = 100

	gameSpeed = float64(5)
)

func (g *Game) Update() error {
	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeyR) {
			g.playerX = 100
			g.playerY = float64(screenHeight - groundHeight - playerHeight)
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
	groundY := float64(screenHeight - groundHeight - playerHeight)
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
			x:   float64(screenWidth),
			y:   float64(screenHeight - groundHeight - float64(h)),
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
				ob.x, ob.y, float64(w), float64(h), cactusMargin,
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

	// move ground
	g.groundX += gameSpeed
	groundW := g.groundFrame.Bounds().Dx()
	g.groundX = math.Mod(g.groundX, float64(groundW))

	// move obstacles
	newObstacles := g.obstacles[:0]
	for _, ob := range g.obstacles {
		ob.x -= gameSpeed
		w := ob.img.Bounds().Dx()
		if ob.x+float64(w) > 0 {
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
	groundY := float64(screenHeight - groundHeight - 18)
	groundW := g.groundFrame.Bounds().Dx()
	for i := 0; i < 2; i++ {
		op := &ebiten.DrawImageOptions{}
		offsetX := -g.groundX + float64(groundW*i)
		if i == 1 {
			offsetX -= 5 // fix the little gap
		}
		op.GeoM.Translate(offsetX, groundY)
		screen.DrawImage(g.groundFrame, op)
	}

	// dino
	drawDinoOpts := &ebiten.DrawImageOptions{}
	drawDinoOpts.GeoM.Translate(float64(g.playerX), float64(g.playerY))
	screen.DrawImage(g.dinoFrames[g.animFrame], drawDinoOpts)

	// obstacles
	for _, ob := range g.obstacles {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(ob.x), float64(ob.y))
		screen.DrawImage(ob.img, op)
	}

	// score
	scoreText := fmt.Sprintf("Score: %d", g.score)
	highScoreText := fmt.Sprintf("High Score: %d", g.highScore)

	face := text.NewGoXFace(basicfont.Face7x13)

	drawScoreOpts := &text.DrawOptions{}
	drawScoreOpts.GeoM.Translate(10, 20)
	drawScoreOpts.ColorScale.ScaleWithColor(gray)
	text.Draw(screen, scoreText, face, drawScoreOpts)

	drawHighScoreOpts := &text.DrawOptions{}
	drawHighScoreOpts.GeoM.Translate(10, 40)
	drawHighScoreOpts.ColorScale.ScaleWithColor(gray)
	text.Draw(screen, highScoreText, face, drawHighScoreOpts)

	if g.gameOver {
		drawGameOverOpts := &text.DrawOptions{}
		drawGameOverOpts.GeoM.Translate(screenWidth/2-100, 60)
		drawGameOverOpts.ColorScale.ScaleWithColor(gray)
		msg := "GAME OVER - Press R to Restart"
		text.Draw(screen, msg, face, drawGameOverOpts)
	}
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

	// ground
	groundFrame := sprite.SubImage(image.Rect(0, 104, 2404, 104+18)).(*ebiten.Image)

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
		playerY:      float64(screenHeight - groundHeight - 94),
		onGround:     true,
		dinoFrames:   dinoFrames,
		cactusFrames: cactusFrames,
		groundFrame:  groundFrame,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Dino makes me feel great again!")
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
