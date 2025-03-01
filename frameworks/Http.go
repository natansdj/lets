package frameworks

import (
	"fmt"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
	"github.com/gin-contrib/gzip"

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

// Run service
func (http *HttpServer) serve() {
	go func(http *HttpServer) {
		err := http.engine.Run(http.server)
		if err != nil {
			lets.LogE(err.Error())
		}

		time.Sleep(time.Second)
		http.serve()
	}(http)
}

func (r *HttpServer) Disconnect() {
	lets.LogI("HTTP Server Stopping ...")

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
