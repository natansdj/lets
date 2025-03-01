package types

import (
	"reflect"
	"runtime"

	"github.com/natansdj/lets"
)

// Engine for controller
type IWebSocketEventHandler interface {
	Event(string, func(IWebSocketEvent))
	Call(string, IWebSocketEvent)
}

// Engine for controller
type WebSocketEventHandler struct {
	handlers map[string]func(IWebSocketEvent)
	Debug    bool
}

func (me *WebSocketEventHandler) Event(name string, handler func(IWebSocketEvent)) {
	if me.Debug {
		lets.LogI("WebSocket Event: %-20s --> %v", name, runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name())
	}

	if me.handlers == nil {
		me.handlers = map[string]func(IWebSocketEvent){}
	}

	me.handlers[name] = handler
}

func (me *WebSocketEventHandler) Call(name string, event IWebSocketEvent) {
	if adapter := me.handlers[name]; adapter != nil {
		adapter(event)

		if me.Debug {
			lets.LogI("WebSocket Event: Call: %-20s --> %v \n", name, runtime.FuncForPC(reflect.ValueOf(adapter).Pointer()).Name())
		}
		return
	}

	lets.LogE("WebSocket Event: not found: %s", name)
}
