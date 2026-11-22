package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	appVersion = "v1.0.0"
	gridSize   = 15
	tileSize   = 44
	hudHeight  = 76
	screenW    = gridSize * tileSize
	screenH    = gridSize*tileSize + hudHeight
)

//go:embed assets/icon.png
var appIconPNG []byte

type Screen int

const (
	ScreenMenu Screen = iota
	ScreenLevelSelect
	ScreenPlaying
	ScreenPaused
	ScreenSettings
	ScreenAchievements
	ScreenHelp
	ScreenAbout
	ScreenGameOver
	ScreenWon
	ScreenComplete
)

type Pulse struct {
	X, Y, Radius, MaxRadius, Alpha float64
	IsDecoy                        bool
	IsNoise                        bool
}

type EnemyState int

const (
	StateIdle EnemyState = iota
	StateInvestigating
)

type Enemy struct {
	X, Y, TargetX, TargetY float64
	State                  EnemyState
	Alpha, Speed, HearDist float64
	StuckTicks             int
	AlertTimer             int
}

type Game struct {
	screen                             Screen
	menuIndex                          int
	levelSelectIndex                   int
	muted                              bool
	showHints                          bool
	levelIdx                           int
	grid                               [gridSize][gridSize]int
	wallAlpha                          [gridSize][gridSize]float64
	playerX, playerY, facingX, facingY float64
	keyX, keyY, keyAlpha               float64
	hasKey                             bool
	exitX, exitY, exitAlpha            float64
	decoysLeft, pulsesUsed             int
	decoysUsedInLevel                  int
	sprintedInLevel                    bool
	pulses                             []Pulse
	enemies                            []Enemy
	sounds                             *SoundBank
	achievements                       *AchievementManager
	moveNoiseClock                     int
	stepSoundClock                     int
	resetConfirmTimer                  int
}

func NewGame() *Game {
	return &Game{
		sounds:       NewSoundBank(),
		achievements: NewAchievementManager(),
		screen:       ScreenMenu,
		showHints:    true,
	}
}

func (g *Game) loadLevel(idx int) {
	if idx >= len(Levels) {
		g.screen = ScreenComplete
		if g.achievements.Unlock("campaign_clear") {
			g.play(g.sounds.achieveWave)
		}
		return
	}
	lvl := Levels[idx]
	g.levelIdx = idx
	g.grid = lvl.Grid
	g.wallAlpha = [gridSize][gridSize]float64{}
	g.playerX = float64(lvl.PlayerX)*tileSize + tileSize/2
	g.playerY = float64(lvl.PlayerY)*tileSize + tileSize/2
	g.facingX, g.facingY = 1, 0
	g.keyX = float64(lvl.KeyX)*tileSize + tileSize/2
	g.keyY = float64(lvl.KeyY)*tileSize + tileSize/2
	g.keyAlpha, g.hasKey = 0, false
	g.exitX = float64(lvl.ExitX)*tileSize + tileSize/2
	g.exitY = float64(lvl.ExitY)*tileSize + tileSize/2
	g.exitAlpha = 0
	g.decoysLeft = lvl.MaxDecoys
	g.pulsesUsed = 0
	g.decoysUsedInLevel = 0
	g.sprintedInLevel = false
	g.pulses = nil
	g.enemies = make([]Enemy, len(lvl.Enemies))
	for i, es := range lvl.Enemies {
		g.enemies[i] = Enemy{
			X:        float64(es.X)*tileSize + tileSize/2,
			Y:        float64(es.Y)*tileSize + tileSize/2,
			State:    StateIdle,
			Speed:    es.Speed,
			HearDist: es.HearDist * tileSize,
		}
	}
	g.moveNoiseClock = 0
	g.stepSoundClock = 0
	g.screen = ScreenPlaying
}

func (g *Game) emitPulse(x, y, maxRadius float64, isDecoy bool) {
	g.pulses = append(g.pulses, Pulse{
		X: x, Y: y, Radius: 4, MaxRadius: maxRadius, Alpha: 1, IsDecoy: isDecoy,
	})
	if isDecoy {
		g.decoysUsedInLevel++
		g.achievements.TotalDecoys++
		g.achievements.save()
		if g.achievements.TotalDecoys >= 5 {
			if g.achievements.Unlock("master_deceiver") {
				g.play(g.sounds.achieveWave)
			}
		}
		g.play(g.sounds.decoyWave)
	} else {
		g.pulsesUsed++
		g.play(g.sounds.pulseWave)
	}
	g.alertEnemiesAt(x, y, 1.0)
}

func (g *Game) alertEnemiesAt(x, y, hearMultiplier float64) {
	playedAlert := false
	for i := range g.enemies {
		e := &g.enemies[i]
		if math.Hypot(e.X-x, e.Y-y) <= e.HearDist*hearMultiplier {
			if e.State == StateIdle && !playedAlert {
				g.play(g.sounds.alertWave)
				playedAlert = true
			}
			e.State = StateInvestigating
			e.TargetX = x
			e.TargetY = y
			e.StuckTicks = 0
			e.AlertTimer = 50
		}
	}
}

func (g *Game) play(data []byte) {
	if !g.muted {
		g.sounds.Play(data)
	}
}

func (g *Game) collidesWithWall(nx, ny, radius float64, reveal bool) bool {
	minC, maxC := int((nx-radius)/tileSize), int((nx+radius)/tileSize)
	minR, maxR := int((ny-radius)/tileSize), int((ny+radius)/tileSize)
	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			if r < 0 || r >= gridSize || c < 0 || c >= gridSize {
				return true
			}
			cell := g.grid[r][c]
			if cell == 1 || (cell == 2 && !g.hasKey) {
				if reveal && g.wallAlpha[r][c] < 0.35 {
					g.wallAlpha[r][c] = 0.35
				}
				return true
			}
		}
	}
	return false
}

func (g *Game) moveEnemyToward(e *Enemy, goalX, goalY float64) bool {
	startC := int(e.X / tileSize)
	startR := int(e.Y / tileSize)
	goalC := int(goalX / tileSize)
	goalR := int(goalY / tileSize)

	wpX, wpY := goalX, goalY
	if startR != goalR || startC != goalC {
		if nextR, nextC, ok := findNextGridStep(&g.grid, startR, startC, goalR, goalC, g.hasKey); ok {
			wpX = float64(nextC)*tileSize + float64(tileSize)/2
			wpY = float64(nextR)*tileSize + float64(tileSize)/2
		}
	}

	dx, dy := wpX-e.X, wpY-e.Y
	dist := math.Hypot(dx, dy)
	if dist < 1e-6 {
		return false
	}
	nx, ny := dx/dist, dy/dist
	moved := false

	stepX := nx * e.Speed
	if math.Abs(stepX) > dist {
		stepX = nx * dist
	}
	if !g.collidesWithWall(e.X+stepX, e.Y, 9, false) {
		e.X += stepX
		moved = true
	}

	stepY := ny * e.Speed
	if math.Abs(stepY) > dist {
		stepY = ny * dist
	}
	if !g.collidesWithWall(e.X, e.Y+stepY, 9, false) {
		e.Y += stepY
		moved = true
	}

	if !moved {
		slide := e.Speed * 0.7
		if math.Abs(nx) >= math.Abs(ny) {
			if !g.collidesWithWall(e.X, e.Y+slide, 9, false) {
				e.Y += slide
				moved = true
			} else if !g.collidesWithWall(e.X, e.Y-slide, 9, false) {
				e.Y -= slide
				moved = true
			}
		} else {
			if !g.collidesWithWall(e.X+slide, e.Y, 9, false) {
				e.X += slide
				moved = true
			} else if !g.collidesWithWall(e.X-slide, e.Y, 9, false) {
				e.X -= slide
				moved = true
			}
		}
	}
	return moved
}

func (g *Game) Update() error {
	g.achievements.UpdateToast()
	if g.resetConfirmTimer > 0 {
		g.resetConfirmTimer--
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		g.muted = !g.muted
	}

	if g.screen == ScreenMenu {
		const menuCount = 6
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.menuIndex = (g.menuIndex + 1) % menuCount
			g.play(g.sounds.uiWave)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.menuIndex = (g.menuIndex + menuCount - 1) % menuCount
			g.play(g.sounds.uiWave)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.play(g.sounds.uiWave)
			switch g.menuIndex {
			case 0:
				g.levelSelectIndex = 0
				g.screen = ScreenLevelSelect
			case 1:
				g.screen = ScreenAchievements
			case 2:
				g.screen = ScreenHelp
			case 3:
				g.resetConfirmTimer = 0
				g.screen = ScreenSettings
			case 4:
				g.screen = ScreenAbout
			case 5:
				return ebiten.Termination
			}
		}
		return nil
	}

	if g.screen == ScreenLevelSelect {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.screen = ScreenMenu
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.levelSelectIndex = (g.levelSelectIndex + 1) % len(Levels)
			g.play(g.sounds.uiWave)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
			g.levelSelectIndex = (g.levelSelectIndex + len(Levels) - 1) % len(Levels)
			g.play(g.sounds.uiWave)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			if g.levelSelectIndex <= g.achievements.MaxUnlockedLevel {
				g.loadLevel(g.levelSelectIndex)
			} else {
				g.play(g.sounds.bumpWave)
			}
		}
		return nil
	}

	if g.screen == ScreenAchievements || g.screen == ScreenHelp || g.screen == ScreenAbout {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.screen = ScreenMenu
		}
		return nil
	}

	if g.screen == ScreenSettings {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.screen = ScreenMenu
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyM) {
			g.muted = !g.muted
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			g.showHints = !g.showHints
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if g.resetConfirmTimer > 0 {
				g.achievements.ResetAll()
				g.resetConfirmTimer = 0
				g.play(g.sounds.bumpWave)
			} else {
				g.resetConfirmTimer = 180
			}
		}
		return nil
	}

	if g.screen == ScreenPaused {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.screen = ScreenPlaying
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.loadLevel(g.levelIdx)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyM) {
			g.screen = ScreenMenu
		}
		return nil
	}

	if g.screen == ScreenGameOver || g.screen == ScreenWon || g.screen == ScreenComplete {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			switch g.screen {
			case ScreenGameOver:
				g.loadLevel(g.levelIdx)
			case ScreenWon:
				g.loadLevel(g.levelIdx + 1)
			case ScreenComplete:
				g.loadLevel(0)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.screen = ScreenMenu
		}
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.screen = ScreenPaused
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.loadLevel(g.levelIdx)
		return nil
	}

	var dx, dy float64
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		dy--
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		dy++
	}

	sprinting := ebiten.IsKeyPressed(ebiten.KeyShift)
	if dx != 0 || dy != 0 {
		l := math.Hypot(dx, dy)
		dx /= l
		dy /= l
		g.facingX, g.facingY = dx, dy
		speed := 2.3
		if sprinting {
			speed = 3.7
			g.sprintedInLevel = true
		}
		bumped := false
		if !g.collidesWithWall(g.playerX+dx*speed, g.playerY, 9, true) {
			g.playerX += dx * speed
		} else {
			bumped = true
		}
		if !g.collidesWithWall(g.playerX, g.playerY+dy*speed, 9, true) {
			g.playerY += dy * speed
		} else {
			bumped = true
		}

		if sprinting {
			g.stepSoundClock++
			if g.stepSoundClock >= 14 {
				g.stepSoundClock = 0
				g.play(g.sounds.stepWave)
				g.pulses = append(g.pulses, Pulse{
					X: g.playerX, Y: g.playerY, Radius: 2, MaxRadius: 1.4 * tileSize, Alpha: 0.65, IsNoise: true,
				})
				g.alertEnemiesAt(g.playerX, g.playerY, 0.72)
			}
		} else {
			g.stepSoundClock = 0
		}

		g.moveNoiseClock++
		if bumped && g.moveNoiseClock%10 == 0 {
			g.play(g.sounds.bumpWave)
		}
	} else {
		g.moveNoiseClock = 0
		g.stepSoundClock = 0
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.emitPulse(g.playerX, g.playerY, 4.2*tileSize, false)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) && g.decoysLeft > 0 {
		g.decoysLeft--
		tx := g.playerX + g.facingX*3*tileSize
		ty := g.playerY + g.facingY*3*tileSize
		tx = math.Max(tileSize, math.Min(float64(screenW-tileSize), tx))
		ty = math.Max(tileSize, math.Min(float64(gridSize*tileSize-tileSize), ty))
		g.emitPulse(tx, ty, 2.6*tileSize, true)
	}

	g.updateWorld()
	return nil
}

func (g *Game) updateWorld() {
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			g.wallAlpha[r][c] = math.Max(0, g.wallAlpha[r][c]-0.009)
		}
	}
	g.keyAlpha = math.Max(0, g.keyAlpha-0.009)
	g.exitAlpha = math.Max(0, g.exitAlpha-0.009)

	active := g.pulses[:0]
	for _, p := range g.pulses {
		step := 4.5
		if p.IsNoise {
			step = 2.6
		}
		p.Radius += step
		p.Alpha = 1 - p.Radius/p.MaxRadius
		if p.Alpha > 0 {
			active = append(active, p)
			if !p.IsNoise {
				for r := 0; r < gridSize; r++ {
					for c := 0; c < gridSize; c++ {
						if g.grid[r][c] == 1 || g.grid[r][c] == 2 {
							cx := float64(c*tileSize + tileSize/2)
							cy := float64(r*tileSize + tileSize/2)
							if math.Hypot(cx-p.X, cy-p.Y) <= p.Radius {
								g.wallAlpha[r][c] = math.Max(g.wallAlpha[r][c], p.Alpha)
							}
						}
					}
				}
				if !g.hasKey && math.Hypot(g.keyX-p.X, g.keyY-p.Y) <= p.Radius {
					g.keyAlpha = math.Max(g.keyAlpha, p.Alpha)
				}
				if math.Hypot(g.exitX-p.X, g.exitY-p.Y) <= p.Radius {
					g.exitAlpha = math.Max(g.exitAlpha, p.Alpha)
				}
				for i := range g.enemies {
					if math.Hypot(g.enemies[i].X-p.X, g.enemies[i].Y-p.Y) <= p.Radius {
						g.enemies[i].Alpha = math.Max(g.enemies[i].Alpha, p.Alpha)
					}
				}
			}
		}
	}
	g.pulses = active

	for i := range g.enemies {
		e := &g.enemies[i]
		e.Alpha = math.Max(0, e.Alpha-0.012)
		if e.AlertTimer > 0 {
			e.AlertTimer--
		}
		if e.State == StateInvestigating {
			dist := math.Hypot(e.TargetX-e.X, e.TargetY-e.Y)
			if dist < 6 {
				e.State = StateIdle
				e.StuckTicks = 0
			} else {
				moved := g.moveEnemyToward(e, e.TargetX, e.TargetY)
				if !moved {
					e.StuckTicks++
					if e.StuckTicks > 40 {
						e.State = StateIdle
						e.StuckTicks = 0
					}
				} else {
					e.StuckTicks = 0
				}
			}
		}
		if math.Hypot(g.playerX-e.X, g.playerY-e.Y) < 18 {
			g.screen = ScreenGameOver
			g.play(g.sounds.loseWave)
			return
		}
	}

	if !g.hasKey && math.Hypot(g.playerX-g.keyX, g.playerY-g.keyY) < 20 {
		g.hasKey = true
		g.play(g.sounds.keyWave)
		for r := range g.grid {
			for c := range g.grid[r] {
				if g.grid[r][c] == 2 {
					g.grid[r][c] = 0
				}
			}
		}
	}

	if g.hasKey && math.Hypot(g.playerX-g.exitX, g.playerY-g.exitY) < 22 {
		g.handleLevelWin()
	}
}

func (g *Game) handleLevelWin() {
	g.screen = ScreenWon
	g.play(g.sounds.winWave)

	if g.levelIdx+1 > g.achievements.MaxUnlockedLevel && g.levelIdx+1 < len(Levels) {
		g.achievements.MaxUnlockedLevel = g.levelIdx + 1
	}
	g.achievements.TotalEscapes++
	g.achievements.save()

	anyNew := false
	if g.achievements.Unlock("first_escape") {
		anyNew = true
	}
	if g.pulsesUsed <= 2 {
		if g.achievements.Unlock("ghost_runner") {
			anyNew = true
		}
	}
	if !g.sprintedInLevel {
		if g.achievements.Unlock("silent_step") {
			anyNew = true
		}
	}
	if g.decoysUsedInLevel == 0 {
		if g.achievements.Unlock("pure_instinct") {
			anyNew = true
		}
	}
	for _, e := range g.enemies {
		if e.State == StateInvestigating {
			if g.achievements.Unlock("close_call") {
				anyNew = true
			}
			break
		}
	}
	if anyNew {
		g.play(g.sounds.achieveWave)
	}
}

func drawPanel(screen *ebiten.Image, x, y, w, h float32, fill color.RGBA) {
	vector.DrawFilledRect(screen, x, y, w, h, fill, false)
	vector.StrokeRect(screen, x, y, w, h, 2, color.RGBA{35, 170, 230, 220}, false)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{5, 8, 16, 255})

	switch g.screen {
	case ScreenMenu, ScreenSettings:
		g.drawMenu(screen)
		g.drawToast(screen)
		return
	case ScreenLevelSelect:
		g.drawLevelSelect(screen)
		g.drawToast(screen)
		return
	case ScreenAchievements:
		g.drawAchievements(screen)
		g.drawToast(screen)
		return
	case ScreenHelp:
		g.drawHelp(screen)
		g.drawToast(screen)
		return
	case ScreenAbout:
		g.drawAbout(screen)
		g.drawToast(screen)
		return
	}

	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			a := g.wallAlpha[r][c]
			if a <= 0 {
				continue
			}
			alpha := uint8(math.Max(0, math.Min(255, a*255)))
			x, y := float32(c*tileSize), float32(r*tileSize)
			if g.grid[r][c] == 1 {
				vector.DrawFilledRect(screen, x+1, y+1, tileSize-2, tileSize-2, color.RGBA{35, 125, 190, alpha}, false)
			} else if g.grid[r][c] == 2 {
				vector.DrawFilledRect(screen, x+1, y+1, tileSize-2, tileSize-2, color.RGBA{225, 155, 30, alpha}, false)
			}
		}
	}

	if !g.hasKey && g.keyAlpha > 0 {
		vector.DrawFilledCircle(screen, float32(g.keyX), float32(g.keyY), 8, color.RGBA{255, 220, 45, uint8(g.keyAlpha * 255)}, true)
	}

	if g.exitAlpha > 0 || g.hasKey {
		a := math.Max(g.exitAlpha, 0.35*boolFloat(g.hasKey))
		vector.DrawFilledRect(screen, float32(g.exitX-12), float32(g.exitY-12), 24, 24, color.RGBA{40, 225, 125, uint8(a * 255)}, false)
	}

	for _, e := range g.enemies {
		if e.Alpha > 0 {
			vector.DrawFilledCircle(screen, float32(e.X), float32(e.Y), 10, color.RGBA{245, 45, 65, uint8(e.Alpha * 255)}, true)
		}
		if e.AlertTimer > 0 {
			a := uint8(float64(e.AlertTimer) / 50.0 * 180)
			vector.StrokeCircle(screen, float32(e.X), float32(e.Y), 16, 2, color.RGBA{255, 70, 70, a}, true)
		}
	}

	for _, p := range g.pulses {
		col := color.RGBA{70, 220, 255, uint8(p.Alpha * 200)}
		if p.IsDecoy {
			col = color.RGBA{255, 165, 50, uint8(p.Alpha * 200)}
		} else if p.IsNoise {
			col = color.RGBA{160, 175, 195, uint8(p.Alpha * 130)}
		}
		vector.StrokeCircle(screen, float32(p.X), float32(p.Y), float32(p.Radius), 2, col, true)
	}

	vector.DrawFilledCircle(screen, float32(g.playerX), float32(g.playerY), 8, color.RGBA{245, 250, 255, 255}, true)

	lvl := Levels[g.levelIdx]
	if g.showHints && g.pulsesUsed == 0 && lvl.Hint != "" {
		ebitenutil.DebugPrintAt(screen, "TIP: "+lvl.Hint, 14, 10)
	}

	hudY := float32(gridSize * tileSize)
	vector.DrawFilledRect(screen, 0, hudY, screenW, hudHeight, color.RGBA{12, 18, 31, 255}, false)
	status := "KEY: NO"
	if g.hasKey {
		status = "KEY: YES"
	}
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("%s  |  %s  |  DECOYS: %d  |  ECHOES: %d (PAR %d)  |  ACH: %d/%d",
			lvl.Name, status, g.decoysLeft, g.pulsesUsed, lvl.ParEchoes,
			g.achievements.UnlockedCount(), len(g.achievements.Items)),
		12, int(hudY)+10)
	ebitenutil.DebugPrintAt(screen, "MOVE: WASD/ARROWS   ECHO: SPACE   DECOY: Q   RUN: SHIFT   PAUSE: ESC   RESTART: R", 12, int(hudY)+35)

	if g.screen != ScreenPlaying {
		g.drawOverlay(screen)
	}
	g.drawToast(screen)
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	for i := 0; i < 7; i++ {
		vector.StrokeCircle(screen, screenW/2, 155, float32(45+i*18), 2, color.RGBA{15, 105, 190, uint8(95 - i*10)}, false)
	}
	ebitenutil.DebugPrintAt(screen, "S I G N A L", screenW/2-34, 136)
	ebitenutil.DebugPrintAt(screen, "E C H O", screenW/2-22, 156)
	ebitenutil.DebugPrintAt(screen, "HEAR  /  SURVIVE  /  ESCAPE", screenW/2-82, 190)

	if g.screen == ScreenSettings {
		drawPanel(screen, 95, 240, 470, 270, color.RGBA{8, 16, 30, 245})
		ebitenutil.DebugPrintAt(screen, "SETTINGS", 295, 262)
		sndState := "ON"
		if g.muted {
			sndState = "OFF"
		}
		hintState := "ON"
		if !g.showHints {
			hintState = "OFF"
		}
		ebitenutil.DebugPrintAt(screen, "Sound Effects: "+sndState+"   (Press M to toggle)", 165, 305)
		ebitenutil.DebugPrintAt(screen, "Level Tips:    "+hintState+"   (Press H to toggle)", 165, 340)
		ebitenutil.DebugPrintAt(screen, "F10: Global Mute / Unmute anytime", 165, 375)
		if g.resetConfirmTimer > 0 {
			ebitenutil.DebugPrintAt(screen, "PRESS BACKSPACE AGAIN TO CONFIRM RESET!", 165, 415)
		} else {
			ebitenutil.DebugPrintAt(screen, "BACKSPACE: Reset Progress & Achievements", 165, 415)
		}
		ebitenutil.DebugPrintAt(screen, "ENTER or ESC: Back to Menu", 215, 468)
		return
	}

	items := []string{
		"PLAY / SELECT SECTOR",
		fmt.Sprintf("ACHIEVEMENTS (%d/%d)", g.achievements.UnlockedCount(), len(g.achievements.Items)),
		"HELP & CONTROLS",
		"SETTINGS",
		"ABOUT",
		"QUIT",
	}
	for i, item := range items {
		y := 245 + i*48
		if i == g.menuIndex {
			drawPanel(screen, 175, float32(y-8), 310, 36, color.RGBA{10, 48, 78, 245})
		}
		ebitenutil.DebugPrintAt(screen, item, 245, y+4)
	}
	ebitenutil.DebugPrintAt(screen, "UP / DOWN: SELECT     ENTER: CONFIRM", 195, 555)
}

func (g *Game) drawLevelSelect(screen *ebiten.Image) {
	drawPanel(screen, 55, 45, screenW-110, screenH-90, color.RGBA{8, 16, 30, 248})
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("SELECT SECTOR   |   UNLOCKED: %d / %d",
			g.achievements.MaxUnlockedLevel+1, len(Levels)),
		195, 70)

	const visibleRows = 7
	const rowH = 66
	const gap = 8
	startIdx := 0
	if g.levelSelectIndex >= visibleRows {
		startIdx = g.levelSelectIndex - visibleRows + 1
	}
	if startIdx+visibleRows > len(Levels) {
		startIdx = maxInt(0, len(Levels)-visibleRows)
	}

	for row := 0; row < visibleRows && startIdx+row < len(Levels); row++ {
		i := startIdx + row
		lvl := Levels[i]
		y := 105 + row*(rowH+gap)
		unlocked := i <= g.achievements.MaxUnlockedLevel
		fill := color.RGBA{12, 20, 34, 220}
		if i == g.levelSelectIndex {
			fill = color.RGBA{12, 54, 86, 245}
		}
		drawPanel(screen, 80, float32(y), screenW-160, rowH, fill)
		if unlocked {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s   (Enemies: %d | Decoys: %d | Par: %d)",
				lvl.Name, len(lvl.Enemies), lvl.MaxDecoys, lvl.ParEchoes), 98, y+13)
			ebitenutil.DebugPrintAt(screen, lvl.Hint, 98, y+37)
		} else {
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%02d // [LOCKED SECTOR]", i+1), 98, y+13)
			ebitenutil.DebugPrintAt(screen, "Complete the previous sector to unlock.", 98, y+37)
		}
	}
	ebitenutil.DebugPrintAt(screen, "ENTER: START SECTOR     ESC: BACK", 205, screenH-72)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	drawPanel(screen, 55, 42, screenW-110, screenH-84, color.RGBA{8, 16, 30, 248})
	ebitenutil.DebugPrintAt(screen, "HELP & FIELD MANUAL", 245, 60)

	lines := []string{
		"CAMPAIGN: 10 levels on a 15x15 grid.",
		"GOAL: Find the yellow Key, open the amber gate, reach green Exit.",
		"",
		"MECHANICS:",
		"  - Pulse (SPACE): Reveals walls, key, exit, and enemies",
		"    briefly, and alerts enemies in hearing range.",
		"  - Decoy (Q): Emits a sound pulse to attract enemies.",
		"  - Sprint (SHIFT): Faster movement, emits footstep ripples,",
		"    and flags the level as sprinted. Walking is silent.",
		"  - Wall Bump: Hitting a wall softly reveals that tile.",
		"",
		"ACHIEVEMENTS (0/7): Independent counter from the 10 levels.",
		"  - First Escape: Clear any level.",
		"  - Ghost Runner: Clear a level using 2 pulses or fewer.",
		"  - Silent Step: Clear a level without sprinting.",
		"  - Pure Instinct: Clear a level using 0 decoys.",
		"  - Close Call: Clear a level while an enemy is investigating.",
		"  - Master Deceiver: Use 5 decoys total across play.",
		"  - Campaign Clear: Reach beyond the final level.",
		"",
		"CONTROLS: WASD/Arrows Move | SHIFT Sprint | SPACE Pulse | Q Decoy | R Retry | ESC/F10",
	}

	startY := 86
	stepY := 20
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, 72, startY+i*stepY)
	}
	ebitenutil.DebugPrintAt(screen, "PRESS ENTER OR ESC TO RETURN", 230, screenH-64)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	drawPanel(screen, 55, 65, screenW-110, screenH-130, color.RGBA{8, 16, 30, 248})
	ebitenutil.DebugPrintAt(screen, "ABOUT SIGNAL ECHO", 250, 95)

	lines := []string{
		"Title:      Signal Echo (" + appVersion + ")",
		"Genre:      2D Acoustic Stealth Puzzle",
		"Engine:     Go + Ebitengine v2 (100% Procedural Audio & Art)",
		"",
		"Concept:",
		"  Every pulse of light is also a sound. Master the balance",
		"  between seeing the maze and remaining undetected in the dark.",
		"",
		"Save Data Location:",
		"  " + g.achievements.savePath,
	}
	for i, line := range lines {
		ebitenutil.DebugPrintAt(screen, line, 85, 150+i*32)
	}
	ebitenutil.DebugPrintAt(screen, "PRESS ENTER OR ESC TO RETURN", 230, screenH-92)
}

func (g *Game) drawAchievements(screen *ebiten.Image) {
	drawPanel(screen, 55, 45, screenW-110, screenH-90, color.RGBA{8, 16, 30, 248})
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("ACHIEVEMENTS  (%d / %d UNLOCKED)   |   TOTAL ESCAPES: %d",
			g.achievements.UnlockedCount(), len(g.achievements.Items), g.achievements.TotalEscapes),
		95, 70)

	for i, a := range g.achievements.Items {
		y := 115 + i*74
		fill := color.RGBA{12, 20, 34, 220}
		status := "[LOCKED]"
		if a.Unlocked {
			fill = color.RGBA{12, 52, 65, 240}
			status = "[UNLOCKED]"
		}
		drawPanel(screen, 80, float32(y), screenW-160, 58, fill)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s  %s", status, a.Title), 98, y+12)
		ebitenutil.DebugPrintAt(screen, a.Description, 98, y+32)
	}
	ebitenutil.DebugPrintAt(screen, "PRESS ENTER OR ESC TO RETURN", 230, screenH-72)
}

func (g *Game) drawOverlay(screen *ebiten.Image) {
	drawPanel(screen, 145, 270, 370, 120, color.RGBA{5, 12, 24, 245})
	title, sub := "PAUSED", "ENTER / ESC: RESUME     M: MAIN MENU"
	switch g.screen {
	case ScreenGameOver:
		title, sub = "SIGNAL LOST", "ENTER: RETRY     ESC: MAIN MENU"
	case ScreenWon:
		title, sub = "LEVEL CLEAR", "ENTER: NEXT LEVEL     ESC: MAIN MENU"
	case ScreenComplete:
		title, sub = "ALL SIGNALS FOUND", "ENTER: PLAY AGAIN     ESC: MAIN MENU"
	}
	ebitenutil.DebugPrintAt(screen, title, 270, 300)
	ebitenutil.DebugPrintAt(screen, sub, 190, 345)
}

func (g *Game) drawToast(screen *ebiten.Image) {
	t := g.achievements.ActiveToast
	if t == nil {
		return
	}
	drawPanel(screen, 150, 14, 360, 52, color.RGBA{10, 55, 65, 248})
	ebitenutil.DebugPrintAt(screen, "ACHIEVEMENT UNLOCKED: "+t.Title, 168, 24)
	ebitenutil.DebugPrintAt(screen, t.Description, 168, 42)
}

func (g *Game) Layout(_, _ int) (int, int) { return screenW, screenH }

func main() {
	if img, _, err := image.Decode(bytes.NewReader(appIconPNG)); err == nil {
		ebiten.SetWindowIcon([]image.Image{img})
	}
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Signal Echo")
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}