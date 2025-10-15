/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/gorilla/websocket"
)

// -------------------------------------------------------------------------------------
// Window Mode (existing)
// -------------------------------------------------------------------------------------

type windowMode = int

const (
	windowModeNormal windowMode = iota
	windowModeFullscreen
	windowModeBorderless
)

func setWindowMode(mode windowMode) {
	switch mode {
	case windowModeNormal:
		platformMgr.SetWindowFullscreen(false)
	case windowModeFullscreen:
		platformMgr.SetWindowFullscreen(true)
	case windowModeBorderless:
		// TODO tanjp fix it
		platformMgr.SetWindowFullscreen(false)
	}
}

func setWindowPosition(x, y float64) {
	platformMgr.SetWindowPosition(mathf.Vec2{X: x, Y: y})
}

func setWindowSize(width, height int64) {
	platformMgr.SetWindowSize(width, height)
}

// -------------------------------------------------------------------------------------
// Window Sync (WebSocket-based remote control)
// -------------------------------------------------------------------------------------

// elementRect element position information
type elementRect struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// wailsWindow wails window position
type wailsWindow struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// borders window border information
type borders struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// calculations position calculations
type calculations struct {
	RawX      float64 `json:"raw_x"`
	AdjustedX float64 `json:"adjusted_x"`
	RawY      float64 `json:"raw_y"`
	AdjustedY float64 `json:"adjusted_y"`
}

// systemDecorations system decoration information
type systemDecorations struct {
	TotalHeight             int    `json:"totalHeight"`
	TotalWidth              int    `json:"totalWidth"`
	EstimatedTitlebarHeight int    `json:"estimatedTitlebarHeight"`
	OperatingSystem         string `json:"operatingSystem"`
	TitlebarHeight          int    `json:"titlebarHeight"`
}

// debugInfo detailed debug information for position calculation
type debugInfo struct {
	ElementRect       elementRect       `json:"elementRect"`
	WailsWindow       wailsWindow       `json:"wailsWindow"`
	Borders           borders           `json:"borders"`
	SystemDecorations systemDecorations `json:"systemDecorations"`
	Calculations      calculations      `json:"calculations"`
}

// positionMessage WebSocket message for window position/size updates
type positionMessage struct {
	Type      string     `json:"type"`
	X         int        `json:"x"`
	Y         int        `json:"y"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	Visible   bool       `json:"visible"`
	Timestamp int64      `json:"timestamp"`
	DebugInfo *debugInfo `json:"debugInfo,omitempty"`
}

// -------------------------------------------------------------------------------------
// WebSocket Client
// -------------------------------------------------------------------------------------

type websocketClient struct {
	conn        *websocket.Conn
	url         string
	connected   bool
	mutex       sync.RWMutex
	msgCallback func(positionMessage)
	stopChan    chan struct{}
}

func newWebSocketClient(wsURL string) *websocketClient {
	return &websocketClient{
		url:      wsURL,
		stopChan: make(chan struct{}),
	}
}

func (ws *websocketClient) connect() error {
	u, err := url.Parse(ws.url)
	if err != nil {
		log.Printf("❌ WebSocket: URL parse failed - %v", err)
		return fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	log.Printf("🔗 WebSocket: Connecting to %s", ws.url)
	log.Printf("📊 WebSocket: Connection details - Host: %s, Path: %s", u.Host, u.Path)

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("❌ WebSocket: Connection handshake failed - %v", err)
		if resp != nil {
			log.Printf("❌ WebSocket: HTTP response status - %s", resp.Status)
		}
		log.Println("⚠️ WebSocket: Cannot connect to server. Please ensure the server is running.")
		return err
	}

	ws.mutex.Lock()
	ws.conn = conn
	ws.connected = true
	ws.mutex.Unlock()

	log.Printf("✅ WebSocket: Connection established!")
	log.Printf("🔗 WebSocket: Connection status - LocalAddr: %s, RemoteAddr: %s",
		conn.LocalAddr(), conn.RemoteAddr())

	// Start message reading goroutine
	go ws.readMessages()

	return nil
}

func (ws *websocketClient) disconnect() {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	if ws.conn != nil {
		ws.conn.Close()
		ws.connected = false
		close(ws.stopChan)
		log.Printf("⚠️ WebSocket: Disconnected")
	}
}

func (ws *websocketClient) isConnected() bool {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return ws.connected
}

func (ws *websocketClient) setMessageCallback(callback func(positionMessage)) {
	ws.msgCallback = callback
}

func (ws *websocketClient) readMessages() {
	defer func() {
		ws.mutex.Lock()
		ws.connected = false
		ws.mutex.Unlock()
	}()

	for {
		select {
		case <-ws.stopChan:
			return
		default:
			var msg positionMessage
			err := ws.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("❌ WebSocket: Read message error - %v", err)
				} else {
					log.Printf("🔌 WebSocket: Connection closed normally")
				}
				return
			}

			log.Printf("📨 WebSocket: Received message - Type: %s, Pos: (%d,%d), Size: %dx%d, Visible: %v",
				msg.Type, msg.X, msg.Y, msg.Width, msg.Height, msg.Visible)

			if ws.msgCallback != nil {
				log.Printf("🔄 WebSocket: Calling message callback handler")
				ws.msgCallback(msg)
			} else {
				log.Printf("⚠️ WebSocket: No message callback handler")
			}
		}
	}
}

// -------------------------------------------------------------------------------------
// Window Controller
// -------------------------------------------------------------------------------------

// windowController manages window synchronization via WebSocket
type windowController struct {
	config *windowSyncConfig
	game   *Game

	wsClient *websocketClient

	// Window state
	isHidden      bool
	savedPosition mathf.Vec2
	savedWidth    int64
	savedHeight   int64

	// Control
	mutex       sync.RWMutex
	stopChan    chan struct{}
	initialized bool
}

// newWindowController creates a new window controller
func newWindowController(game *Game, config *windowSyncConfig) *windowController {
	// Set defaults
	if config.ServerHost == "" {
		config.ServerHost = "localhost"
	}
	if config.ServerPort == 0 {
		config.ServerPort = 8080
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 20
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 50
	}

	wsURL := fmt.Sprintf("ws://%s:%d/ws", config.ServerHost, config.ServerPort)

	wc := &windowController{
		config:   config,
		game:     game,
		wsClient: newWebSocketClient(wsURL),
		stopChan: make(chan struct{}),
	}

	wc.wsClient.setMessageCallback(wc.onPositionMessage)

	return wc
}

// start starts the window synchronization service
func (wc *windowController) start() error {
	if !wc.config.Enabled {
		log.Println("⏸️ WindowController: Disabled, not starting")
		return nil
	}

	log.Println("🚀 WindowController: Starting window synchronization service")
	log.Printf("🔧 WindowController: Config - Host: %s, Port: %d, AutoReconnect: %v, MaxRetries: %d",
		wc.config.ServerHost, wc.config.ServerPort, wc.config.AutoReconnect, wc.config.MaxRetries)

	// Try to connect asynchronously
	go wc.connectWithRetry()

	wc.initialized = true
	return nil
}

// stop stops the window synchronization service
func (wc *windowController) stop() {
	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	if !wc.initialized {
		return
	}

	log.Println("🛑 WindowController: Stopping window synchronization service")
	close(wc.stopChan)

	if wc.wsClient != nil {
		wc.wsClient.disconnect()
	}

	wc.initialized = false
}

// connectWithRetry attempts to connect with retry logic
func (wc *windowController) connectWithRetry() {
	log.Println("⏳ WindowController: Waiting for WebSocket server to start...")

	retries := 0
	for {
		select {
		case <-wc.stopChan:
			log.Println("🛑 WindowController: Connection retry stopped")
			return
		default:
			retries++
			log.Printf("🔄 WindowController: Attempting to connect (attempt %d/%d)", retries, wc.config.MaxRetries)

			err := wc.wsClient.connect()
			if err == nil {
				log.Println("✅ WindowController: WebSocket connection successful!")
				return
			}

			log.Printf("❌ WindowController: Connection failed: %v", err)

			if !wc.config.AutoReconnect && retries >= wc.config.MaxRetries {
				log.Printf("⚠️ WindowController: Max retries reached (%d), giving up", wc.config.MaxRetries)
				return
			}

			log.Printf("⏳ WindowController: Waiting %d ms before retry...", wc.config.RetryInterval)
			time.Sleep(time.Duration(wc.config.RetryInterval) * time.Millisecond)
		}
	}
}

// onPositionMessage handles incoming position messages
func (wc *windowController) onPositionMessage(msg positionMessage) {
	if msg.Type != "position_update" {
		log.Printf("⚠️ WindowController: Unknown message type: %s", msg.Type)
		return
	}

	wc.updateWindow(msg)
}

// updateWindow updates the window based on the message
func (wc *windowController) updateWindow(msg positionMessage) {
	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	// Handle visibility changes
	if !msg.Visible && !wc.isHidden {
		wc.hideWindow()
		return
	} else if msg.Visible && wc.isHidden {
		wc.showWindow()
	}

	// Only update position and size when visible
	if !msg.Visible || wc.isHidden {
		return
	}

	// Calculate optimal position
	targetX := msg.X
	targetY := msg.Y

	// Use debug info for more accurate positioning if available
	if msg.DebugInfo != nil {
		borderTop := msg.DebugInfo.Borders.Top
		rectOffset := msg.DebugInfo.ElementRect.Top
		windowY := msg.DebugInfo.WailsWindow.Y
		targetY = windowY + borderTop + int(rectOffset)
	}

	// Ensure minimum size
	width := int64(msg.Width)
	height := int64(msg.Height)
	if width < 300 {
		width = 300
	}
	if height < 200 {
		height = 200
	}

	log.Printf("🎮 WindowController: Updating window - pos=(%d,%d), size=%dx%d",
		targetX, targetY, width, height)

	// Update window position and size via platformMgr
	setWindowPosition(float64(targetX), float64(targetY))
	setWindowSize(width, height)
}

// hideWindow hides the window by moving it off-screen
func (wc *windowController) hideWindow() {
	log.Println("🙈 WindowController: Hiding window")

	// Save current position and size
	wc.savedPosition = platformMgr.GetWindowPosition()
	size := platformMgr.GetWindowSize()
	wc.savedWidth = int64(size.X)
	wc.savedHeight = int64(size.Y)

	// Move window off-screen (far right and down)
	setWindowPosition(10000, 10000)

	wc.isHidden = true
	log.Printf("💾 WindowController: Saved position: (%.0f, %.0f) %dx%d",
		wc.savedPosition.X, wc.savedPosition.Y, wc.savedWidth, wc.savedHeight)
}

// showWindow restores the window to its saved position
func (wc *windowController) showWindow() {
	log.Println("👁️ WindowController: Showing window")

	if wc.isHidden {
		// Restore window position and size
		setWindowPosition(wc.savedPosition.X, wc.savedPosition.Y)
		setWindowSize(wc.savedWidth, wc.savedHeight)

		wc.isHidden = false
		log.Printf("✅ WindowController: Window restored to position: (%.0f, %.0f) %dx%d",
			wc.savedPosition.X, wc.savedPosition.Y, wc.savedWidth, wc.savedHeight)
	}
}
