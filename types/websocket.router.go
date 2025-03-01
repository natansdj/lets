package types

import (
	"encoding/json"
	"net/http"

	"github.com/natansdj/lets"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type IWebSocketRoute interface {
	Initialize(*gin.Engine, bool)
}

type WebSocketRoute struct {
	Path          string
	Handlers      func(IWebSocketEventHandler) // Service to inject
	CustomPayload IWebSocketBody

	server       *gin.Engine            // HTTP Server
	debug        bool                   // Debug
	eventHandler IWebSocketEventHandler // Bucket for service handler
}

func (wsr *WebSocketRoute) Initialize(server *gin.Engine, debug bool) {
	wsr.server = server
	wsr.debug = debug

	// Inject websocket handler to websocket services
	wsr.eventHandler = &WebSocketEventHandler{
		Debug: debug,
	}
	wsr.Handlers(wsr.eventHandler)

	// Instantiate path into controller
	wsr.server.GET(wsr.Path, wsr.controller)
}

// Get payload structure for received message.
func (wsr *WebSocketRoute) getBody() IWebSocketBody {
	if lets.IsNil(wsr.CustomPayload) {
		return &WebSocketBody{}
	}

	return wsr.CustomPayload
}

func (wsr *WebSocketRoute) SetupHandler() {

}

func (wsr *WebSocketRoute) controller(c *gin.Context) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		lets.LogE("WebSocket Server.Upgrade: %v", err)
		return
	}
	defer ws.Close()

	wsr.handleConnection(ws, "connected")

	for {
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			lets.LogE("WebSocket Server.ReadMessage: %v", err)
			break
		}

		if messageType == websocket.TextMessage {
			wsr.handleTextMessage(ws, message)
		} else {
			lets.LogI("MessageType: %s", messageType)
			lets.LogI("Message    : %s", message)
		}
	}

	wsr.handleConnection(ws, "disconnected")
}

func (wsr *WebSocketRoute) handleConnection(conn *websocket.Conn, status string) {
	// Create event data.
	event := WebSocketEvent{
		Connection: conn,
		Name:       status,
	}

	// Call event handler.
	wsr.eventHandler.Call(event.GetName(), &event)
}

func (wsr *WebSocketRoute) handleTextMessage(conn *websocket.Conn, message []byte) {
	// Bind body into Payload.
	body := wsr.getBody()
	err := json.Unmarshal(message, &body)
	if err != nil {
		lets.LogE("WebSocket Server: %s", err.Error())
	}

	// Create event data.
	event := WebSocketEvent{
		Connection: conn,
		Name:       body.GetAction(),
		Data:       body.GetData(),
	}

	// Call event handler.
	wsr.eventHandler.Call(event.GetName(), &event)
}
