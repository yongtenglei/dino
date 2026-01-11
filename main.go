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

//go:embed assets/run.wav
var runWav []byte

//go:embed assets/shield.wav
var shieldWav []byte

var gray = color.RGBA{0x88, 0x88, 0x88, 0xff}

type Obstacle struct {
	x   float64
	y   float64
	img *ebiten.Image
}

type Game struct {
	playerX      float64
	playerY      float64
	vy           float64
	jumpCount    int
	onGround     bool
	isDucking    bool
	duckDuration float64

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
	hasShield   bool
	speedLevel   int

	shieldReadyFramesLeft int
	shieldReadyBlinkTick  int
	shieldReadyVisible    bool
	speedUpFramesLeft     int
	speedUpBlinkTick      int
	speedUpVisible        bool

	lastJumpKeyPressed    bool
	lastDuckKeyPressed    bool
	lastRestartKeyPressed bool

	audioContext *audio.Context
	jumpPlayer   *audio.Player
	diePlayer    *audio.Player
	pointPlayer  *audio.Player
	runPlayer    *audio.Player
	shieldPlayer *audio.Player
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

func isJumpKeyPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyK)
}

func isDuckKeyPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyJ)
}

func gameSpeedForScore(score int) float64 {
	speed := baseGameSpeed + float64(score/gameSpeedScoreStep)*gameSpeedStep
	if speed > maxGameSpeed {
		return maxGameSpeed
	}
	return speed
}

const (
	maxjumpCount    = 2
	maxDuckDuration = 3.0 // 3s for 60 FPS

	shieldReadyDurationFrames = 90
	shieldReadyBlinkFrames    = 12

	screenWidth  = 800
	screenHeight = 600

	dinoRunningWidth  = 88
	dinoRunningHeight = 94
	dinoDuckingWidth  = 118
	dinoDuckingHeight = 60

	dinoMargin     = float64(20)
	obstacleMargin = float64(5)

	minBirdOffset = 100
	maxBirdOffset = 180

	duckYOffset = 34

	maxCloudsNum     = 4
	minCloudDistance = 160.0

	groundHeight = 100

	baseGameSpeed      = 5.0
	maxGameSpeed       = 10.0
	gameSpeedStep      = 0.5
	gameSpeedScoreStep = 500

	sampleRate = 44100
)

func (g *Game) Update() error {
	g.animTick++
	currentSpeed := gameSpeedForScore(g.score)
	speedStep := g.score / gameSpeedScoreStep
	maxSpeedStep := int((maxGameSpeed - baseGameSpeed) / gameSpeedStep)
	if speedStep > maxSpeedStep {
		speedStep = maxSpeedStep
	}
	if speedStep > g.speedLevel {
		g.speedLevel = speedStep
		if speedStep > 0 {
			g.speedUpFramesLeft = shieldReadyDurationFrames
			g.speedUpBlinkTick = 0
			g.speedUpVisible = true
		}
	}

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
		cloud.x -= currentSpeed * 0.3
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

		restartNow := ebiten.IsKeyPressed(ebiten.KeyR) || ebiten.IsKeyPressed(ebiten.KeySpace)
		if restartNow && !g.lastRestartKeyPressed {
			g.playerX = 100
			g.playerY = float64(screenHeight - groundHeight - dinoRunningHeight)
			g.vy = 0
			g.onGround = true
			g.animFrame = 0
			g.jumpCount = 0
			g.cactuses = nil
			g.cactusSpawnTick = 0
			g.birdSpawnTick = 0
			g.score = 0
			g.hasShield = false
			g.speedLevel = 0

			g.shieldReadyFramesLeft = 0
			g.shieldReadyBlinkTick = 0
			g.shieldReadyVisible = false
			g.speedUpFramesLeft = 0
			g.speedUpBlinkTick = 0
			g.speedUpVisible = false
			g.gameOver = false
			g.lastRestartKeyPressed = false
			return nil
		}
		g.lastRestartKeyPressed = restartNow
		return nil
	}

	if g.shieldReadyFramesLeft > 0 {
		g.shieldReadyFramesLeft--
		g.shieldReadyBlinkTick++
		if g.shieldReadyBlinkTick >= shieldReadyBlinkFrames {
			g.shieldReadyBlinkTick = 0
			g.shieldReadyVisible = !g.shieldReadyVisible
		}
	} else if g.shieldReadyVisible {
		g.shieldReadyVisible = false
		g.shieldReadyBlinkTick = 0
	}

	if g.speedUpFramesLeft > 0 {
		g.speedUpFramesLeft--
		g.speedUpBlinkTick++
		if g.speedUpBlinkTick >= shieldReadyBlinkFrames {
			g.speedUpBlinkTick = 0
			g.speedUpVisible = !g.speedUpVisible
		}
	} else if g.speedUpVisible {
		g.speedUpVisible = false
		g.speedUpBlinkTick = 0
	}

	// jump
	jumpNow := isJumpKeyPressed()
	if jumpNow && !g.lastJumpKeyPressed && g.jumpCount < maxjumpCount {
		g.onGround = false

		if g.runPlayer.IsPlaying() {
			g.runPlayer.Pause()
		}

		if g.jumpCount == 0 {
			g.vy = -10
		} else {
			g.vy = -9
		}
		g.jumpCount++
		_ = g.jumpPlayer.Rewind()
		g.jumpPlayer.Play()
	}
	g.lastJumpKeyPressed = jumpNow

	g.vy += 0.5
	g.playerY += g.vy
	groundY := float64(screenHeight - groundHeight - dinoRunningHeight)
	if g.playerY >= groundY {
		g.playerY = groundY
		g.vy = 0
		g.onGround = true
		g.jumpCount = 0
	}

	duckNow := isDuckKeyPressed()
	if duckNow && g.lastDuckKeyPressed {
		g.duckDuration += 1.0 / 60.0
		if g.duckDuration <= maxDuckDuration {
			g.isDucking = true
		} else {
			g.isDucking = false
		}
	} else {
		g.isDucking = false
		g.duckDuration = 0
	}
	g.lastDuckKeyPressed = duckNow

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

		minOffset := minBirdOffset
		if len(g.cactuses) > 0 {
			lastCactus := g.cactuses[len(g.cactuses)-1]
			cactusHeight := lastCactus.img.Bounds().Dy()
			if cactusHeight >= 100 {
				minOffset = 160
			}
		}

		randOffset := float64(rand.Intn(maxBirdOffset-minOffset)) + float64(minOffset)
		y := float64(screenHeight - groundHeight - dinoRunningHeight - randOffset)

		bird := Obstacle{
			x:   float64(screenWidth),
			y:   y,
			img: img,
		}
		g.birds = append(g.birds, bird)
	}

	g.score++
	if g.score > g.highScore {
		g.highScore = g.score
	}
	if g.score%1000 == 0 {
		_ = g.pointPlayer.Rewind()
		g.pointPlayer.Play()
	}

	if g.score >= 1100 && (g.score-1100)%1000 == 0 && !g.hasShield {
		g.hasShield = true
		g.shieldReadyFramesLeft = shieldReadyDurationFrames
		g.shieldReadyBlinkTick = 0
		g.shieldReadyVisible = true
		_ = g.shieldPlayer.Rewind()
		g.shieldPlayer.Play()
	}

	// colliding
	// cactus
	if !g.gameOver {
		for i := 0; i < len(g.cactuses); i++ {
			c := g.cactuses[i]
			w, h := c.img.Bounds().Dx(), c.img.Bounds().Dy()

			dinoX := g.playerX
			dinoY := g.playerY
			dinoW := dinoRunningWidth
			dinoH := dinoRunningHeight
			if g.isDucking {
				dinoY = dinoY + duckYOffset
				dinoW = dinoDuckingWidth
				dinoH = dinoDuckingHeight
			}

			margin := obstacleMargin
			if w > 100 {
				margin = 40
			}

			if isColliding(
				dinoX, dinoY, float64(dinoW), float64(dinoH), dinoMargin,
				c.x, c.y, float64(w), float64(h), margin,
			) {
				if g.score > g.highScore {
					g.highScore = g.score
				}
				if g.hasShield {
					g.hasShield = false
					g.cactuses = append(g.cactuses[:i], g.cactuses[i+1:]...)
					i--
					continue
				}
				g.cactuses = append(g.cactuses[:i], g.cactuses[i+1:]...)
				g.gameOver = true
				g.lastRestartKeyPressed = ebiten.IsKeyPressed(ebiten.KeyR) ||
					ebiten.IsKeyPressed(ebiten.KeySpace)
				break
			}
		}
	}
	// birds
	if !g.gameOver {
		for i := 0; i < len(g.birds); i++ {
			b := g.birds[i]
			w, h := b.img.Bounds().Dx(), b.img.Bounds().Dy()

			dinoX := g.playerX
			dinoY := g.playerY
			dinoW := dinoRunningWidth
			dinoH := dinoRunningHeight
			if g.isDucking {
				dinoY = dinoY + duckYOffset
				dinoW = dinoDuckingWidth
				dinoH = dinoDuckingHeight
			}
			if isColliding(
				dinoX, dinoY, float64(dinoW), float64(dinoH), dinoMargin,
				b.x, b.y, float64(w), float64(h), obstacleMargin) {
				if g.score > g.highScore {
					g.highScore = g.score
				}
				if g.hasShield {
					g.hasShield = false
					g.birds = append(g.birds[:i], g.birds[i+1:]...)
					i--
					continue
				}
				g.birds = append(g.birds[:i], g.birds[i+1:]...)
				g.gameOver = true
				g.lastRestartKeyPressed = ebiten.IsKeyPressed(ebiten.KeyR) ||
					ebiten.IsKeyPressed(ebiten.KeySpace)
				break
			}
		}
	}

	if g.gameOver {
		if g.runPlayer.IsPlaying() {
			g.runPlayer.Pause()
		}

		_ = g.diePlayer.Rewind()
		g.diePlayer.Play()
		return nil
	}

	// move ground
	g.groundX += currentSpeed
	groundW := g.groundFrame.Bounds().Dx()
	g.groundX = math.Mod(g.groundX, float64(groundW))

	// move obstacles
	newCactuses := g.cactuses[:0]
	for _, c := range g.cactuses {
		c.x -= currentSpeed
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
		speed := currentSpeed + osc*1.5
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

		if g.onGround {
			_ = g.runPlayer.Rewind()
			g.runPlayer.Play()
		}

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
		screen.Fill(color.RGBA{0x30, 0x30, 0x40, 0xff})

		drawDinoOpts := &ebiten.DrawImageOptions{}
		drawDinoOpts.GeoM.Translate(float64(g.playerX), float64(g.playerY))
		screen.DrawImage(g.dinoStandFrames[g.animFrame%len(g.dinoStandFrames)], drawDinoOpts)

		face := text.NewGoXFace(basicfont.Face7x13)

		startX := float64(screenWidth / 2)
		asciiY := float64(screenHeight/2 - 140)

		dinoASCII := []string{
			"    ____     ____   _   __   ____ ",
			"   / __ \\   /  _/  / | / /  / __ \\",
			"  / / / /   / /   /  |/ /  / / / /",
			" / /_/ /  _/ /   / /|  /  / /_/ / ",
			"/_____/  /___/  /_/ |_/   \\____/  ",
			"                                 ",
		}

		for i, line := range dinoASCII {
			lineX := startX - float64(len(line)*7/2)
			drawLine := &text.DrawOptions{}
			drawLine.GeoM.Translate(lineX, asciiY+float64(i*13))
			drawLine.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, line, face, drawLine)
		}

		startY := float64(screenHeight/2 - 30)

		titleText := "Press SPACE to Start"
		titleX := startX - float64(len(titleText)*7/2)

		drawTitle := &text.DrawOptions{}
		drawTitle.GeoM.Translate(titleX, startY)
		drawTitle.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, titleText, face, drawTitle)

		instructionText := "SPACE/K: Jump | DOWN/J: Duck"
		instructionX := startX - float64(len(instructionText)*7/2)

		drawInstruction := &text.DrawOptions{}
		drawInstruction.GeoM.Translate(instructionX, startY+30)
		drawInstruction.ColorScale.ScaleWithColor(color.White)
		text.Draw(screen, instructionText, face, drawInstruction)

		return
	}

	face := text.NewGoXFace(basicfont.Face7x13)

	// dino
	drawDinoOpts := &ebiten.DrawImageOptions{}
	drawDinoOpts.GeoM.Translate(float64(g.playerX), float64(g.playerY))
	if g.gameOver {
		screen.DrawImage(g.dinoDeadFrames[g.animFrame%len(g.dinoDeadFrames)], drawDinoOpts)
	} else if g.isDucking {
		drawDinoOpts.GeoM.Translate(0, duckYOffset)
		screen.DrawImage(g.dinoDuckFrames[g.animFrame%len(g.dinoDuckFrames)], drawDinoOpts)
	} else if !g.onGround {
		if g.vy < 0 {
			screen.DrawImage(g.dinoStandFrames[g.animFrame%len(g.dinoStandFrames)], drawDinoOpts)
		} else {
			screen.DrawImage(g.dinoRunningFrames[g.animFrame%len(g.dinoRunningFrames)], drawDinoOpts)
		}
	} else {
		screen.DrawImage(g.dinoRunningFrames[g.animFrame%len(g.dinoRunningFrames)], drawDinoOpts)
	}

	if g.hasShield {
		exclaimText := "!"
		dinoX := g.playerX
		dinoY := g.playerY
		dinoW := dinoRunningWidth
		dinoH := dinoRunningHeight
		if g.isDucking {
			dinoY = dinoY + duckYOffset
			dinoW = dinoDuckingWidth
			dinoH = dinoDuckingHeight
		}
		exclaimX := dinoX + float64(dinoW) + 6
		exclaimY := dinoY + float64(dinoH)/2 - 6
		exclaimOpts := &text.DrawOptions{}
		exclaimOpts.GeoM.Translate(exclaimX, exclaimY)
		exclaimOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, exclaimText, face, exclaimOpts)

		exclaimBoldOpts := &text.DrawOptions{}
		exclaimBoldOpts.GeoM.Translate(exclaimX+1, exclaimY)
		exclaimBoldOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, exclaimText, face, exclaimBoldOpts)
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

	drawScoreOpts := &text.DrawOptions{}
	drawScoreOpts.GeoM.Translate(10, 20)
	drawScoreOpts.ColorScale.ScaleWithColor(gray)
	text.Draw(screen, scoreText, face, drawScoreOpts)

	drawHighScoreOpts := &text.DrawOptions{}
	drawHighScoreOpts.GeoM.Translate(10, 40)
	drawHighScoreOpts.ColorScale.ScaleWithColor(gray)
	text.Draw(screen, highScoreText, face, drawHighScoreOpts)

	// duck hint
	if g.isDucking {
		hint := max(3.0-g.duckDuration, 0)
		duckHintText := fmt.Sprintf("Duck timeout: %.1fs", hint)
		drawDuckHintOpts := &text.DrawOptions{}
		drawDuckHintOpts.GeoM.Translate(10, 60)
		drawDuckHintOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, duckHintText, face, drawDuckHintOpts)
	}

	if g.hasShield {
		shieldText := "Shield: READY"
		drawShieldOpts := &text.DrawOptions{}
		drawShieldOpts.GeoM.Translate(10, 80)
		drawShieldOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, shieldText, face, drawShieldOpts)
	}

	if g.speedUpFramesLeft > 0 && g.speedUpVisible && !g.gameOver {
		speedUpText := "SPEED UP!"
		levelText := fmt.Sprintf("LEVEL %d", g.speedLevel)
		speedUpX := float64(screenWidth)/2 - float64(len(speedUpText)*7/2)
		levelX := float64(screenWidth)/2 - float64(len(levelText)*7/2)
		speedUpY := float64(screenHeight)/2 - 50
		levelY := float64(screenHeight)/2 - 30
		drawSpeedUpOpts := &text.DrawOptions{}
		drawSpeedUpOpts.GeoM.Translate(speedUpX, speedUpY)
		drawSpeedUpOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, speedUpText, face, drawSpeedUpOpts)
		drawLevelOpts := &text.DrawOptions{}
		drawLevelOpts.GeoM.Translate(levelX, levelY)
		drawLevelOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, levelText, face, drawLevelOpts)
	}

	if g.shieldReadyFramesLeft > 0 && g.shieldReadyVisible && !g.gameOver {
		shieldReadyText := "SHIELD IS READY"
		shieldReadyX := float64(screenWidth)/2 - float64(len(shieldReadyText)*7/2)
		shieldReadyY := float64(screenHeight)/2 - 10
		drawShieldReadyOpts := &text.DrawOptions{}
		drawShieldReadyOpts.GeoM.Translate(shieldReadyX, shieldReadyY)
		drawShieldReadyOpts.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, shieldReadyText, face, drawShieldReadyOpts)
	}

	if g.gameOver {
		red := color.RGBA{0xff, 0x00, 0x00, 0xff}

		gameOverText := "GAME OVER"
		gameOverX := float64(screenWidth)/2 - float64(len(gameOverText)*7/2)

		drawGameOverOpts := &text.DrawOptions{}
		drawGameOverOpts.GeoM.Translate(gameOverX, 60)
		drawGameOverOpts.ColorScale.ScaleWithColor(red)
		text.Draw(screen, gameOverText, face, drawGameOverOpts)

		restartY := float64(90)
		restartText := "Press SPACE or R to Restart"
		restartX := float64(screenWidth)/2 - float64(len(restartText)*7/2)

		drawRestart := &text.DrawOptions{}
		drawRestart.GeoM.Translate(restartX, restartY)
		drawRestart.ColorScale.ScaleWithColor(gray)
		text.Draw(screen, restartText, face, drawRestart)
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
	runSoundPlayer := loadSoundTrack(audioCtx, sampleRate, bytes.NewReader(runWav))
	shieldSoundPlayer := loadSoundTrack(audioCtx, sampleRate, bytes.NewReader(shieldWav))

	// ground
	groundFrame := sprite.SubImage(image.Rect(0, 104, 2404, 104+18)).(*ebiten.Image)

	// cloud
	cloudFrame := sprite.SubImage(image.Rect(170, 0, 170+90, 0+30)).(*ebiten.Image)

	// dino
	dinoStandFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(1336, 0, 1336+88, 0+94)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1426, 0, 1425+88, 0+94)).(*ebiten.Image),
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
		sprite.SubImage(image.Rect(1866, 34, 1866+118, 34+60)).(*ebiten.Image),
		sprite.SubImage(image.Rect(1984, 34, 1984+118, 34+60)).(*ebiten.Image),
	}

	// obstacles
	cactusFrames := []*ebiten.Image{
		sprite.SubImage(image.Rect(446, 2, 446+34, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(548, 2, 548+68, 2+70)).(*ebiten.Image),
		sprite.SubImage(image.Rect(652, 2, 652+49, 2+100)).(*ebiten.Image),
		sprite.SubImage(image.Rect(752, 2, 752+199, 2+100)).(*ebiten.Image),
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

		lastJumpKeyPressed:    false,
		lastDuckKeyPressed:    false,
		lastRestartKeyPressed: false,

		audioContext: audioCtx,
		jumpPlayer:   jumpSoundPlayer,
		diePlayer:    dieSoundPlayer,
		pointPlayer:  pointSoundPlayer,
		runPlayer:    runSoundPlayer,
		shieldPlayer: shieldSoundPlayer,
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Dino makes me feel great again!")
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
