package frameworks

import (
	"fmt"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"github.com/gin-gonic/gin"
)

// WebSocket Configurations
var WebSocketConfig types.IWebSocketServer

// WebSocket service struct
type webSocketServer struct {
	server string
	engine *gin.Engine
	routes []types.IWebSocketRoute // Handle endpoint event
	debug  bool
}

// Initialize service
func (ws *webSocketServer) init() {
	gin.SetMode(WebSocketConfig.GetMode())

	ws.server = fmt.Sprintf(":%s", WebSocketConfig.GetPort())
	ws.engine = gin.New()
	ws.routes = WebSocketConfig.GetRoutes()
	ws.debug = WebSocketConfig.GetMode() == "debug"

	var defaultLogFormatter = func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		return fmt.Sprintf("%s[WebSocket]%s %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
			"\x1b[32m", resetColor,
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			param.Path,
			param.ErrorMessage,
		)
	}

	ws.engine.Use(gin.LoggerWithFormatter(defaultLogFormatter))
}

func (ws *webSocketServer) setupRoutes() {
	for _, route := range ws.routes {
		route.Initialize(ws.engine, ws.debug)
	}
}

func (ws *webSocketServer) Disconnect() {
	lets.LogI("WebSocket Server Stopping ...")

	lets.LogI("WebSocket Server Stopped ...")
}

// Run service
func (ws *webSocketServer) serve() {
	go ws.engine.Run(ws.server)
}

// Start WebSocket service
func WebSocket() (disconnectors []func()) {
	if WebSocketConfig == nil {
		return
	}

	lets.LogI("WebSocket Server Starting ...")

	var ws webSocketServer
	ws.init()
	ws.setupRoutes()
	ws.serve()

	disconnectors = append(disconnectors, ws.Disconnect)

	return
}
