package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type AppState int

const (
	screenWidth  = 1500
	screenHeight = 900

	AppStateStart AppState = iota
	AppStateDrawStart
	AppStateDrawing
)

type FontSet struct {
	Regular    rl.Font
	Bold       rl.Font
	Italic     rl.Font
	BoldItalic rl.Font
}

type App struct {
	currentAppState AppState
	font            FontSet

	mouseX float32
	mouseY float32

	drawnPixels []rl.Vector2
	drawRadius  float32

	lastDrawnPixel rl.Vector2
}

func (a *App) Init() {
	a.currentAppState = AppStateStart
	a.drawRadius = 10

	cps := codePoints()
	sizeR := int32(50)
	sizeB := int32(40)
	sizeI := int32(35)
	sizeBI := int32(40)

	a.font.Regular = rl.LoadFontEx("Fonts/SpaceMono-Regular.ttf", sizeR, cps, int32(len(cps)))
	a.font.Bold = rl.LoadFontEx("Fonts/SpaceMono-Bold.ttf", sizeB, cps, int32(len(cps)))
	a.font.Italic = rl.LoadFontEx("Fonts/SpaceMono-Italic.ttf", sizeI, cps, int32(len(cps)))
	a.font.BoldItalic = rl.LoadFontEx("Fonts/SpaceMono-BoldItalic.ttf", sizeBI, cps, int32(len(cps)))
}

func (a *App) Draw() {
	switch a.currentAppState {
	case AppStateStart:
		t1 := "Welcome to Picto-Chat"
		t2 := "Press [Space] to continue..."

		drawTextCentered(a.font.Regular, t1, (screenHeight/2)-40, 50, rl.White)
		drawTextCentered(a.font.Italic, t2, (screenHeight/2)+5, 35, rl.White)

	case AppStateDrawStart:
		t1 := "Draw Here..."

		mousePos := fmt.Sprintf("(%.0f, %.0f)", a.mouseX, a.mouseY)

		drawTextCentered(a.font.Italic, t1, (screenHeight/2)-40, 35, rl.White)
		rl.DrawTextEx(a.font.Italic, mousePos, rl.NewVector2(50, 50), 35, 2, rl.White)

	case AppStateDrawing:
		mousePos := fmt.Sprintf("(%.0f, %.0f)", a.mouseX, a.mouseY)

		for _, p := range a.drawnPixels {
			rl.DrawCircle(int32(p.X), int32(p.Y), a.drawRadius, rl.White)
		}
		rl.DrawTextEx(a.font.Italic, mousePos, rl.NewVector2(50, 50), 35, 2, rl.White)

	}
}

func (a *App) Update() {
	switch a.currentAppState {
	case AppStateStart:
		a.OnSpacePressed()
		a.drawnPixels = []rl.Vector2{}

	case AppStateDrawStart:
		a.OnSpacePressed()
		a.GetMousePos()
		a.OnMousePress()

	case AppStateDrawing:
		a.OnSpacePressed()
		a.GetMousePos()
		a.OnMousePress()
	}
}

func (a *App) OnSpacePressed() {
	switch a.currentAppState {
	case AppStateStart:
		if rl.IsKeyPressed(rl.KeySpace) {
			a.currentAppState = AppStateDrawStart
		}

	case AppStateDrawStart:
		if rl.IsKeyPressed(rl.KeySpace) {
			a.currentAppState = AppStateStart
		}

	case AppStateDrawing:
		if rl.IsKeyPressed(rl.KeySpace) {
			a.currentAppState = AppStateStart
		}
	}
}

func (a *App) OnMousePress() {
	switch a.currentAppState {
	case AppStateDrawStart:
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			a.currentAppState = AppStateDrawing
		} else {
			a.lastDrawnPixel = rl.NewVector2(a.mouseX, a.mouseY)
		}
	case AppStateDrawing:
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			cur := rl.NewVector2(a.mouseX, a.mouseY)

			dx := cur.X - a.lastDrawnPixel.X
			dy := cur.Y - a.lastDrawnPixel.Y
			dist := rl.Vector2Length(rl.NewVector2(dx, dy))

			step := a.drawRadius * 0.5
			if step < 1 {
				step = 1
			}

			steps := int(dist / step)
			if steps < 1 {
				steps = 1
			}

			for i := 1; i <= steps; i++ {
				t := float32(i) / float32(steps)
				x := a.lastDrawnPixel.X + dx*t
				y := a.lastDrawnPixel.Y + dy*t
				a.drawnPixels = append(a.drawnPixels, rl.NewVector2(x, y))
			}

			a.lastDrawnPixel = cur
		} else {
			a.lastDrawnPixel = rl.NewVector2(a.mouseX, a.mouseY)
		}
	}
}

func (a *App) GetMousePos() {
	mousePos := rl.GetMousePosition()

	a.mouseX = mousePos.X
	a.mouseY = mousePos.Y
}

func main() {
	rl.InitWindow(screenWidth, screenHeight, "Picto-Chat")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	var app App
	app.Init()

	defer rl.UnloadFont(app.font.Regular)
	defer rl.UnloadFont(app.font.Bold)
	defer rl.UnloadFont(app.font.Italic)
	defer rl.UnloadFont(app.font.BoldItalic)

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		app.Update()
		app.Draw()

		rl.EndDrawing()
	}
}

func drawTextCentered(font rl.Font, text string, y int, fontSize float32, color rl.Color) {
	size := rl.MeasureTextEx(font, text, fontSize, 1)
	x := float32(screenWidth)/2 - size.X/2

	rl.DrawTextEx(font, text, rl.NewVector2(x, float32(y)), fontSize, 1, color)
}

func codePoints() []int32 {
	cps := make([]int32, 0, 96+96+128)

	for r := int32(32); r <= 126; r++ {
		cps = append(cps, r)
	}

	for r := int32(0x00A0); r <= 0x00FF; r++ {
		cps = append(cps, r)
	}

	for r := int32(0x0100); r <= 0x017F; r++ {
		cps = append(cps, r)
	}
	return cps
}
