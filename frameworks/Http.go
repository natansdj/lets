package frameworks

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"github.com/gin-gonic/gin"
)

// HTTP Configurations
var HttpConfig types.IHttpServer

// HTTP service struct
type HttpServer struct {
	server     string
	engine     *gin.Engine
	middleware func(*gin.Engine)
	router     func(*gin.Engine)
	gzip       bool
	httpServer *http.Server
}

// Initialize service
func (http *HttpServer) init() {
	gin.SetMode(HttpConfig.GetMode())

	http.server = fmt.Sprintf(":%s", HttpConfig.GetPort())

	http.engine = gin.New()
	http.engine.SetTrustedProxies(nil)

	http.middleware = HttpConfig.GetMiddleware()
	http.router = HttpConfig.GetRouter()
	http.gzip = HttpConfig.GetGzip()

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

		return fmt.Sprintf("%s[HTTP]%s %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
			"\x1b[90;32m", resetColor,
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			param.Path,
			param.ErrorMessage,
		)
	}

	http.engine.Use(gin.LoggerWithFormatter(defaultLogFormatter))

	// Implement GZip Features
	if http.gzip {
		http.engine.Use(gzip.Gzip(gzip.DefaultCompression))
	}
}

// Run service with graceful shutdown support
func (h *HttpServer) serve() {
	h.httpServer = &http.Server{
		Addr:    h.server,
		Handler: h.engine,
	}

	go func() {
		if err := h.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lets.LogERL("http-server-error", "HTTP Server error: %v", err)
			// Wait before potential retry
			time.Sleep(time.Second * 3)
		}
	}()

	lets.LogI("HTTP Server listening on %s", h.server)
}

func (h *HttpServer) Disconnect() {
	lets.LogI("HTTP Server Stopping ...")

	if h.httpServer == nil {
		lets.LogI("HTTP Server already stopped or not started")
		return
	}

	// Create shutdown context with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := h.httpServer.Shutdown(ctx); err != nil {
		lets.LogE("HTTP Server forced shutdown: %v", err)
		// Force close if graceful shutdown fails
		if closeErr := h.httpServer.Close(); closeErr != nil {
			lets.LogE("HTTP Server close error: %v", closeErr)
		}
	}

	lets.LogI("HTTP Server Stopped ...")
}

// Start http service
func Http() (disconnectors []func()) {
	if HttpConfig == nil {
		return
	}

	lets.LogI("HTTP Server Starting ...")

	var http HttpServer

	http.init()
	http.middleware(http.engine)
	http.router(http.engine)
	http.serve()

	disconnectors = append(disconnectors, http.Disconnect)

	return
}
