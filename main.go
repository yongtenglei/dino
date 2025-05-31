package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/basicfont"
)

//go:embed assets/sprite.png
var spriteSheet []byte

//go:embed assets/jump.wav
var jumpWav []byte

//go:embed assets/die.wav
var dieWav []byte

//go:embed assets/point.wav
var pointWav []byte

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

	cactuses            []Obstacle
	birds               []Obstacle
	cactusSpawnTick     int
	birdSpawnTick       int
	birdOscillationTime float64

	dinoStandFrames   []*ebiten.Image
	dinoRunningFrames []*ebiten.Image
	dinoDeadFrames    []*ebiten.Image
	dinoDuckFrames    []*ebiten.Image
	cactusFrames      []*ebiten.Image
	birdFrames        []*ebiten.Image
	animFrame         int
	animTick          int

	groundFrame *ebiten.Image
	groundX     float64

	cloudFrame *ebiten.Image
	clouds     []Obstacle

	score       int
	highScore   int
	startScreen bool
	gameOver    bool

	lastSpacePressed bool

	audioContext *audio.Context
	jumpPlayer   *audio.Player
	diePlayer    *audio.Player
	pointPlayer  *audio.Player
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

	minBirdOffset = 100
	maxBirdOffset = 180

	maxCloudsNum     = 4
	minCloudDistance = 160.0

	groundHeight = 100

	gameSpeed = float64(5)

	sampleRate = 44100
)

func (g *Game) Update() error {
	g.animTick++

	// clouds
	if len(g.clouds) < maxCloudsNum && rand.Intn(100) < 1 {
		newCloud := Obstacle{
			x:   float64(screenWidth + rand.Intn(100)),
			y:   float64(20 + rand.Intn(100)),
			img: g.cloudFrame,
		}

		cloudW := float64(newCloud.img.Bounds().Dx())
		cloudH := float64(newCloud.img.Bounds().Dy())

		tooClose := false
		for _, c := range g.clouds {
			// center distance
			dx := (newCloud.x + cloudW/2) - (c.x + cloudW/2)
			dy := (newCloud.y + cloudH/2) - (c.y + cloudH/2)
			distance := math.Hypot(dx, dy)

			if distance < minCloudDistance {
				tooClose = true
				break
			}
		}

		if !tooClose {
			g.clouds = append(g.clouds, newCloud)
		}
	}

	newClouds := g.clouds[:0]
	for _, cloud := range g.clouds {
		cloud.x -= gameSpeed * 0.3
		if cloud.x+float64(g.cloudFrame.Bounds().Dx()) > 0 {
			newClouds = append(newClouds, cloud)
		}
	}
	g.clouds = newClouds

	if g.startScreen {
		if g.animTick >= 10 {
			g.animTick = 0
			g.animFrame = (g.animFrame + 1) % len(g.dinoStandFrames)
		}

		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			g.startScreen = false
			g.animFrame = 0
			g.animTick = 0
		}
		return nil
	}

	if g.gameOver {
		if g.animTick >= 10 {
			g.animTick = 0
			g.animFrame = (g.animFrame + 1) % len(g.dinoDeadFrames)
		}
		if ebiten.IsKeyPressed(ebiten.KeyR) {
			g.playerX = 100
			g.playerY = float64(screenHeight - groundHeight - playerHeight)
			g.vy = 0
			g.onGround = true
			g.animFrame = 0
			g.jumpCount = 0
			g.cactuses = nil
			g.cactusSpawnTick = 0
			g.birdSpawnTick = 0
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
		g.jumpPlayer.Rewind()
		g.jumpPlayer.Play()
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
	// cactus
	g.cactusSpawnTick++
	if g.cactusSpawnTick >= rand.Intn(100)+150 {
		g.cactusSpawnTick = 0

		img := g.cactusFrames[rand.Intn(len(g.cactusFrames))]
		h := img.Bounds().Dy()

		obcactus := Obstacle{
			x:   float64(screenWidth),
			y:   float64(screenHeight - groundHeight - float64(h)),
			img: img,
		}
		g.cactuses = append(g.cactuses, obcactus)
	}

	// birds
	g.birdSpawnTick++
	if g.birdSpawnTick >= rand.Intn(100)+rand.Intn(50)+200 {
		g.birdSpawnTick = 0

		img := g.birdFrames[rand.Intn(len(g.birdFrames))]
		randOffset := float64(rand.Intn(maxBirdOffset-minBirdOffset)) + minBirdOffset
		y := float64(screenHeight - groundHeight - playerHeight - randOffset)

		bird := Obstacle{
			x:   float64(screenWidth),
			y:   y,
			img: img,
		}
		g.birds = append(g.birds, bird)
	}

	g.score++
	if g.score%1000 == 0 {
		g.pointPlayer.Rewind()
		g.pointPlayer.Play()
	}

	// colliding
	// cactus
	if !g.gameOver {
		for _, c := range g.cactuses {
			w, h := c.img.Bounds().Dx(), c.img.Bounds().Dy()

			if isColliding(
				g.playerX, g.playerY, playerWidth, playerHeight, dinoMargin,
				c.x, c.y, float64(w), float64(h), cactusMargin,
			) {
				if g.score > g.highScore {
					g.highScore = g.score
				}
				g.gameOver = true
				break
			}
		}
	}
	// birds
	if !g.gameOver {
		for _, b := range g.birds {
			w, h := b.img.Bounds().Dx(), b.img.Bounds().Dy()
			if isColliding(
				g.playerX, g.playerY, playerWidth, playerHeight, dinoMargin,
				b.x, b.y, float64(w), float64(h), cactusMargin) {
				if g.score > g.highScore {
					g.highScore = g.score
				}
				g.gameOver = true
				break
			}
		}
	}

	if g.gameOver {
		g.diePlayer.Rewind()
		g.diePlayer.Play()
		return nil
	}

	// move ground
	g.groundX += gameSpeed
	groundW := g.groundFrame.Bounds().Dx()
	g.groundX = math.Mod(g.groundX, float64(groundW))

	// move obstacles
	newCactuses := g.cactuses[:0]
	for _, c := range g.cactuses {
		c.x -= gameSpeed
		w := c.img.Bounds().Dx()
		if c.x+float64(w) > 0 {
			newCactuses = append(newCactuses, c)
		}
	}
	g.cactuses = newCactuses

	// move birds
	g.birdOscillationTime += 0.05
	newBirds := g.birds[:0]
	for i, b := range g.birds {
		osc := math.Sin(g.birdOscillationTime + float64(i))
		speed := gameSpeed + osc*1.5
		b.x -= speed
		b.y += osc * 0.5
		w := b.img.Bounds().Dx()
		if b.x+float64(w) > 0 {
			newBirds = append(newBirds, b)
		}
	}
	g.birds = newBirds

	if g.animTick >= 10 {
		g.animTick = 0
		g.animFrame = (g.animFrame + 1) % len(g.dinoRunningFrames)

		for i := range g.birds {
			b := &g.birds[i]
			b.img = g.birdFrames[rand.Intn(len(g.birdFrames))]
		}
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

	// clouds
	for _, cloud := range g.clouds {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(cloud.x, cloud.y)
		screen.DrawImage(g.cloudFrame, op)
	}

	if g.startScreen {
		drawDinoOpts := &ebiten.DrawImageOptions{}
		drawDinoOpts.GeoM.Translate(float64(g.playerX), float64(g.playerY))
		screen.DrawImage(g.dinoStandFrames[g.animFrame%len(g.dinoStandFrames)], drawDinoOpts)

		face := text.NewGoXFace(basicfont.Face7x13)
		drawStartOpts := &text.DrawOptions{}
		drawStartOpts.GeoM.Translate(screenWidth/2-80, screenHeight/2)
		drawStartOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, "Press SPACE to Start", face, drawStartOpts)

		return
	}

	// dino
	drawDinoOpts := &ebiten.DrawImageOptions{}
	drawDinoOpts.GeoM.Translate(float64(g.playerX), float64(g.playerY))
	if g.gameOver {
		screen.DrawImage(g.dinoDeadFrames[g.animFrame%len(g.dinoDeadFrames)], drawDinoOpts)
	} else {
		screen.DrawImage(g.dinoRunningFrames[g.animFrame%len(g.dinoRunningFrames)], drawDinoOpts)
	}

	// obstacles
	// cactuses
	for _, c := range g.cactuses {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(c.x), float64(c.y))
		screen.DrawImage(c.img, op)
	}
	// birds
	for _, b := range g.birds {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(b.x, b.y)
		screen.DrawImage(b.img, op)
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

func loadSprite() *ebiten.Image {
	img, _, err := image.Decode(bytes.NewReader(spriteSheet))
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(img)
}

func loadSoundTrack(audioCtx *audio.Context, sampleRate int, blob *bytes.Reader) *audio.Player {
	stream, err := wav.DecodeWithSampleRate(sampleRate, blob)
	if err != nil {
		panic(err)
	}
	player, err := audioCtx.NewPlayer(stream)
	if err != nil {
		panic(err)
	}
	return player
}

func main() {
	sprite := loadSprite()

	// sound track
	audioCtx := audio.NewContext(sampleRate)
	jumpSoundPlayer := loadSoundTrack(audioCtx, sampleRate, bytes.NewReader(jumpWav))
	dieSoundPlayer := loadSoundTrack(audioCtx, sampleRate, bytes.NewReader(dieWav))
	pointSoundPlayer := loadSoundTrack(audioCtx, sampleRate, bytes.NewReader(pointWav))

	// ground
	groundFrame := sprite.SubImage(image.Rect(0, 104, 2404, 104+18)).(*ebiten.Image)

	// cloud
	cloudFrame := sprite.SubImage(image.Rect(170, 0, 170+90, 0+30)).(*ebiten.Image)

	// dino
	dinoStandFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1336, 0, 1336+88, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1425, 0, 1425+88, 0+94)).(*ebiten.Image),
	}
	dinoRunningFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1514, 0, 1514+88, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1603, 0, 1603+88, 0+94)).(*ebiten.Image),
	}
	dinoDeadFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1692, 0, 1692+88, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1781, 0, 1781+88, 0+94)).(*ebiten.Image),
	}
	dinoDuckFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1866, 0, 1866+118, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1984, 0, 1984+118, 0+94)).(*ebiten.Image),
	}

	// obstacles
	cactusFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(446, 2, 446+34, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(548, 2, 548+68, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(652, 2, 652+49, 2+100)).(*ebiten.Image),
		sprite.SubImage(image.Rect(802, 2, 802+99, 2+100)).(*ebiten.Image),
	}
	birdFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(260, 0, 260+93, 0+69)).(*ebiten.Image),
		sprite.SubImage(image.Rect(355, 0, 355+93, 0+69)).(*ebiten.Image),
	}

	game := &Game{
		playerX:           100,
		playerY:           float64(screenHeight - groundHeight - 94),
		onGround:          true,
		dinoStandFrames:   dinoStandFrames,
		dinoRunningFrames: dinoRunningFrames,
		dinoDeadFrames:    dinoDeadFrames,
		dinoDuckFrames:    dinoDuckFrames,
		cactusFrames:      cactusFrames,
		birdFrames:        birdFrames,
		groundFrame:       groundFrame,
		cloudFrame:        cloudFrame,
		startScreen:       true,

		audioContext: audioCtx,
		jumpPlayer:   jumpSoundPlayer,
		diePlayer:    dieSoundPlayer,
		pointPlayer:  pointSoundPlayer,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Dino makes me feel great again!")
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
