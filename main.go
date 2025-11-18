package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/gorilla/websocket"
)

type AppState int

const (
	screenWidth  = 1500
	screenHeight = 900

	AppStateStart      AppState = iota
	AppStateRoomConfig          // config menu to either join or create a room
	AppStateDrawStart           // showing 'Draw Here...' text before anything is drawn
	AppStateDrawing             // when the user is actively drawing
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var clientsMu sync.Mutex

type FontSet struct {
	Regular    rl.Font
	Bold       rl.Font
	Italic     rl.Font
	BoldItalic rl.Font
}

type App struct {
	currentAppState AppState // app state to handle events
	font            FontSet  // store fonts

	mouseX float32 // store x coordinates of mouse position
	mouseY float32 // store y coordinates of mouse position

	// store button data to use in the Update() function
	makeRoomButton      rl.Rectangle
	makeRoomButtonColor rl.Color
	joinRoomButton      rl.Rectangle
	joinRoomButtonColor rl.Color

	server         *http.Server
	isServerActive bool
	ws             *websocket.Conn
	isRoomHost     bool
	wsAddr         string

	drawnPixels []rl.Vector2 // store all drawn 'circles' on the screen (not necessarily pixels)
	drawRadius  float32      // radius of the cirlces drawn

	mu sync.RWMutex

	lastDrawnPixel rl.Vector2 // storing last drawn pixel will help interpolation to smooth drawing lines
}

func (a *App) Init() {
	a.currentAppState = AppStateStart

	// set default circle radius to 10
	a.drawRadius = 10

	cps := codePoints()

	// set sizes (helps with rendering the text so raylib-go doesnt need to scale text up or down)
	sizeR := int32(50)
	sizeB := int32(40)
	sizeI := int32(35)
	sizeBI := int32(40)

	// load fonts
	a.font.Regular = rl.LoadFontEx("Fonts/SpaceMono-Regular.ttf", sizeR, cps, int32(len(cps)))
	a.font.Bold = rl.LoadFontEx("Fonts/SpaceMono-Bold.ttf", sizeB, cps, int32(len(cps)))
	a.font.Italic = rl.LoadFontEx("Fonts/SpaceMono-Italic.ttf", sizeI, cps, int32(len(cps)))
	a.font.BoldItalic = rl.LoadFontEx("Fonts/SpaceMono-BoldItalic.ttf", sizeBI, cps, int32(len(cps)))
}

func (a *App) Draw() {
	switch a.currentAppState {

	// draw start screen
	case AppStateStart:
		t1 := "Welcome to Picto-Chat"
		t2 := "Press [Space] to continue..."

		drawTextCentered(a.font.Regular, t1, (screenHeight/2)-40, 50, rl.White)
		drawTextCentered(a.font.Italic, t2, (screenHeight/2)+5, 35, rl.White)

	// draw config screen to enter or join room
	case AppStateRoomConfig:
		t1 := "Select your room option..."
		drawTextCentered(a.font.Regular, t1, (screenHeight/2)-150, 50, rl.White)

		// placing two buttons inside eachother to create a rounded outline for the button
		insertRec1 := rl.NewRectangle((screenWidth/2)+10, (screenHeight / 2), float32(210), float32(100))
		a.joinRoomButton = rl.NewRectangle((insertRec1.X + 5), (insertRec1.Y + 5), float32(200), float32(90))

		// draw insertRec first (white background), then smaller join button (black)
		rl.DrawRectangleRounded(insertRec1, float32(0.5), int32(0), a.joinRoomButtonColor)
		rl.DrawRectangleRounded(a.joinRoomButton, float32(0.5), int32(0), rl.Black)

		// draw the 'Join Room' button text
		joinRoomText := "Join Room"
		rl.DrawTextEx(a.font.BoldItalic, joinRoomText, rl.NewVector2(a.joinRoomButton.X+float32(11), a.joinRoomButton.Y+float32(25)), 40, 3, rl.White)

		// draw the 'Make Room' button text
		insertRec2 := rl.NewRectangle((screenWidth/2)-250, (screenHeight / 2), float32(210), float32(100))
		a.makeRoomButton = rl.NewRectangle((insertRec2.X + 5), (insertRec2.Y + 5), float32(200), float32(90))

		// draw insertRec first (white background), then smaller make room button (black)
		rl.DrawRectangleRounded(insertRec2, float32(0.5), int32(0), a.makeRoomButtonColor)
		rl.DrawRectangleRounded(a.makeRoomButton, float32(0.5), int32(0), rl.Black)

		// draw the 'Make Room' button text
		makeRoomText := "Make Room"
		rl.DrawTextEx(a.font.BoldItalic, makeRoomText, rl.NewVector2(a.makeRoomButton.X+float32(12), a.makeRoomButton.Y+float32(25)), 40, 3, rl.White)

	// essentially the same as drawing but shows 'Draw Here...' prompt
	case AppStateDrawStart:
		t1 := "Draw Here..."
		drawTextCentered(a.font.Italic, t1, (screenHeight/2)-40, 35, rl.White)

		// check if the user is the host of the room
		var hostLabel string
		if a.isRoomHost {
			hostLabel = "Host: You"
		} else {
			hostLabel = fmt.Sprintf("Host: %s", a.wsAddr)
		}

		// draw the host label to identify who is the host
		rl.DrawTextEx(a.font.Italic, hostLabel, rl.NewVector2((screenWidth-500), 10), 35, 3, rl.Red)

		// draw mouse pos and label
		mousePos := fmt.Sprintf("(%.0f, %.0f)", a.mouseX, a.mouseY)
		rl.DrawTextEx(a.font.Italic, "Mouse Pos.", rl.NewVector2(50, 10), 35, 3, rl.White)
		rl.DrawTextEx(a.font.Italic, mousePos, rl.NewVector2(40, 50), 35, 2, rl.White)

		// draw menu shortcut
		rl.DrawTextEx(a.font.Italic, "Menu", rl.NewVector2(300, 10), 35, 2, rl.White)
		rl.DrawTextEx(a.font.Italic, "[M]", rl.NewVector2(305, 50), 35, 2, rl.White)

		// draw space shortcut
		rl.DrawTextEx(a.font.Italic, "Clear", rl.NewVector2(460, 10), 35, 2, rl.White)
		rl.DrawTextEx(a.font.Italic, "[Space]", rl.NewVector2(440, 50), 35, 2, rl.White)

		// draw 'Drawing Tools' section
		insertRec := rl.NewRectangle(float32(40), float32(screenHeight)-150, float32(350), float32(100))
		radiusContainer := rl.NewRectangle(insertRec.X+5, insertRec.Y+5, insertRec.Width-10, insertRec.Height-10)

		rl.DrawTextEx(a.font.Italic, "Drawing Tools", rl.NewVector2(insertRec.X+70, insertRec.Y-40), 35, 2, rl.White)
		rl.DrawRectangleRounded(insertRec, float32(0.5), int32(0), rl.White)
		rl.DrawRectangleRounded(radiusContainer, float32(0.5), int32(0), rl.Black)

	// actively drawing state, drop prompt and and draw the circles
	case AppStateDrawing:
		a.mu.RLock()
		for _, p := range a.drawnPixels {
			rl.DrawCircle(int32(p.X), int32(p.Y), a.drawRadius, rl.White)
		}
		a.mu.RUnlock()

		// check if the user is the host of the room
		var hostLabel string
		if a.isRoomHost {
			hostLabel = "Host: You"
		} else {
			hostLabel = fmt.Sprintf("Host: %s", a.wsAddr)
		}

		// draw the host label to identify who is the host
		rl.DrawTextEx(a.font.Italic, hostLabel, rl.NewVector2((screenWidth-500), 10), 35, 3, rl.Red)

		// draw mouse pos and label
		mousePos := fmt.Sprintf("(%.0f, %.0f)", a.mouseX, a.mouseY)
		rl.DrawTextEx(a.font.Italic, "Mouse Pos.", rl.NewVector2(50, 10), 35, 3, rl.White)
		rl.DrawTextEx(a.font.Italic, mousePos, rl.NewVector2(40, 50), 35, 2, rl.White)

		// draw menu shortcut
		rl.DrawTextEx(a.font.Italic, "Menu", rl.NewVector2(300, 10), 35, 2, rl.White)
		rl.DrawTextEx(a.font.Italic, "[M]", rl.NewVector2(305, 50), 35, 2, rl.White)

		// draw space shortcut
		rl.DrawTextEx(a.font.Italic, "Clear", rl.NewVector2(460, 10), 35, 2, rl.White)
		rl.DrawTextEx(a.font.Italic, "[Space]", rl.NewVector2(440, 50), 35, 2, rl.White)

		// draw 'Drawing Tools' section
		insertRec := rl.NewRectangle(float32(40), float32(screenHeight)-150, float32(350), float32(100))
		radiusContainer := rl.NewRectangle(insertRec.X+5, insertRec.Y+5, insertRec.Width-10, insertRec.Height-10)

		rl.DrawTextEx(a.font.Italic, "Drawing Tools", rl.NewVector2(insertRec.X+70, insertRec.Y-40), 35, 2, rl.White)
		rl.DrawRectangleRounded(insertRec, float32(0.5), int32(0), rl.White)
		rl.DrawRectangleRounded(radiusContainer, float32(0.5), int32(0), rl.Black)
	}
}

func (a *App) Update() {
	switch a.currentAppState {
	// start screen, on space press will enter application, make sure drawn pixels are empty or are reset once visiting menu
	case AppStateStart:
		a.OnSpacePressed()

		a.mu.Lock()
		a.drawnPixels = []rl.Vector2{}
		a.mu.Unlock()

	case AppStateRoomConfig:
		a.GetMousePos()

		if a.wsAddr != "" && a.isServerActive {
			a.currentAppState = AppStateDrawStart
		}

		// change button color is mouse position is inside button and handle click events for both buttons using IsMouseButtonReleased
		if rl.CheckCollisionPointRec(rl.NewVector2(a.mouseX, a.mouseY), a.joinRoomButton) {
			a.joinRoomButtonColor = rl.Blue // change button color to blue on hover

			// handle click events on 'Join Room' button
			if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
				a.isRoomHost = false
				go func() {
					a.JoinWsServer()
				}()
			}
		} else {
			a.joinRoomButtonColor = rl.White // change button color back to white when no collision
		}

		// check collisions for 'Make Room Button'
		if rl.CheckCollisionPointRec(rl.NewVector2(a.mouseX, a.mouseY), a.makeRoomButton) {
			a.makeRoomButtonColor = rl.Blue // change button color to blue on hover

			// handle click events on 'Make Room' button
			if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
				a.isRoomHost = true
				go func() {
					a.StartWsServer()
				}()
				go func() {
					a.JoinWsServer()
				}()
			}
		} else {
			a.makeRoomButtonColor = rl.White // change button color back to white when no collision
		}

	// draw start just to show the draw prompt but there is no handler for clearing drawing
	case AppStateDrawStart:
		a.OnMPressed()
		a.GetMousePos()
		a.OnMousePress()

	// user is actively drawing and has access to shortcut controls
	case AppStateDrawing:
		a.OnSpacePressed()
		a.OnMPressed()
		a.GetMousePos()
		a.OnMousePress()
		a.SendDrawingsToWs()
	}
}

// depending on state handle space press
func (a *App) OnSpacePressed() {
	switch a.currentAppState {
	case AppStateStart:
		if rl.IsKeyPressed(rl.KeySpace) {
			a.currentAppState = AppStateRoomConfig
		}

	case AppStateDrawing:
		if rl.IsKeyPressed(rl.KeySpace) {
			a.mu.Lock()
			a.drawnPixels = nil
			a.mu.Unlock()
		}
	}
}

// shortcut to navigate back to menu on 'M' press
func (a *App) OnMPressed() {
	switch a.currentAppState {
	case AppStateDrawStart:
		if rl.IsKeyPressed(rl.KeyM) {
			if a.isServerActive && a.isRoomHost {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				cancel()

				a.server.Shutdown(ctx)
				fmt.Println("Server Shutdown...")
				a.currentAppState = AppStateStart
			}
		}

	case AppStateDrawing:
		if rl.IsKeyReleased(rl.KeyM) {
			if a.isServerActive && a.isRoomHost {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				cancel()

				a.server.Shutdown(ctx)
				fmt.Println("Server Shutdown...")
				a.currentAppState = AppStateStart
			}
		}
	}
}

// handle mouse button presses (left button)
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
			// interpolate drawings to make them more smooth (instead of drawing 1 cirlce per 1 frame)
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

				a.mu.Lock()
				a.drawnPixels = append(a.drawnPixels, rl.NewVector2(x, y))
				a.mu.Unlock()
			}

			a.lastDrawnPixel = cur
		} else {
			a.lastDrawnPixel = rl.NewVector2(a.mouseX, a.mouseY)
		}
	}
}

// helper to update mouse position
func (a *App) GetMousePos() {
	mousePos := rl.GetMousePosition()

	a.mouseX = mousePos.X
	a.mouseY = mousePos.Y
}

func (a *App) HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clientsMu.Lock()
	clients[ws] = true
	clientsMu.Unlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			fmt.Printf("error reading message from ws: %v\n", err)
			clientsMu.Lock()
			delete(clients, ws)
			clientsMu.Unlock()
			break
		}

		elemSize := binary.Size(rl.Vector2{})
		if elemSize <= 0 || len(msg)%elemSize != 0 {
			fmt.Printf("invalid vector payload size: msg=%d elem=%d\n", len(msg), elemSize)
			continue
		}

		count := len(msg) / elemSize
		vectors := make([]rl.Vector2, count)

		if err := binary.Read(bytes.NewReader(msg), binary.LittleEndian, vectors); err != nil {
			fmt.Printf("failed to read vector data in ws message: %v\n", err)
			continue
		}

		a.mu.Lock()
		a.drawnPixels = vectors
		a.mu.Unlock()

		clientsMu.Lock()
		for client := range clients {
			if err := client.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				fmt.Printf("error writing message [%s]: %v\n", msg, err)
				client.Close()
				delete(clients, client)
			}
		}
		clientsMu.Unlock()
	}
}

func (a *App) StartWsServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", a.HandleConnections)

	a.server = &http.Server{
		Addr:    "0.0.0.0:8000",
		Handler: mux,
	}

	fmt.Println("Started WebSocket Server on :8000")
	a.isServerActive = true
	if err := a.server.ListenAndServe(); err != nil {
		log.Printf("server shutdown error: %v\n", err)
		a.isServerActive = false
	}
}

func (a *App) JoinWsServer() {
	var c *websocket.Conn
	var err error

	// retry connection 3 times with a 200 ms pause in between (helps with host connection)
	for i := 0; i < 3; i++ {
		c, _, err = websocket.DefaultDialer.Dial("ws://192.168.1.113:8000/ws", nil)
		if err != nil {
			log.Printf("failed to connect to web socket server: %v", err)
			//break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// store the connection as App field
	a.mu.Lock()
	if c != nil {
		a.ws = c
		a.wsAddr = c.UnderlyingConn().RemoteAddr().String()
	} else {
		return
	}
	a.mu.Unlock()

	fmt.Println("Connected to WebSocket Server")

	// continuosly read messages received from the server
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			fmt.Printf("failed to read messages from ws: %v\n", err)
		}

		elemSize := binary.Size(rl.Vector2{})
		if elemSize <= 0 || len(msg)%elemSize != 0 {
			fmt.Printf("invalid vector payload size: msg=%d elem=%d\n", len(msg), elemSize)
			continue
		}

		count := len(msg) / elemSize
		vectors := make([]rl.Vector2, count)

		if err := binary.Read(bytes.NewReader(msg), binary.LittleEndian, vectors); err != nil {
			fmt.Printf("failed to read vector data in ws message: %v\n", err)
			continue
		}

		a.mu.Lock()
		a.drawnPixels = vectors
		a.mu.Unlock()

	}
}

func (a *App) SendDrawingsToWs() {
	a.mu.RLock()

	// make sure connection is valid
	if a.ws == nil {
		return
	}

	pixels := make([]rl.Vector2, len(a.drawnPixels))
	copy(pixels, a.drawnPixels)
	a.mu.RUnlock()

	// convert the slice of vectors into bytes
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, pixels); err != nil {
		fmt.Printf("failed to write a.drawnPixels to bytes: %v\n", err)
		return
	}

	// send the bytes to the server
	if err := a.ws.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
		fmt.Printf("failed to write bytes to ws: %v\n", err)
	}
}

func main() {
	rl.InitWindow(screenWidth, screenHeight, "Picto-Chat")
	defer rl.CloseWindow()

	rl.SetTargetFPS(60)

	var app App
	app.Init()

	// unload the loaded fonts
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

// helper function to center drawn text
func drawTextCentered(font rl.Font, text string, y int, fontSize float32, color rl.Color) {
	size := rl.MeasureTextEx(font, text, fontSize, 1)
	x := float32(screenWidth)/2 - size.X/2

	rl.DrawTextEx(font, text, rl.NewVector2(x, float32(y)), fontSize, 1, color)
}

// ensure all characters are rendered correctly
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
