package types

import (
	"encoding/json"
	"reflect"

	"github.com/natansdj/lets"
	"github.com/gorilla/websocket"
)

type IWebSocketEvent interface {
	GetConnection() *websocket.Conn
	GetName() string
	GetData() interface{}
	GetBody() []byte
	GetDebug() bool
}

type WebSocketEvent struct {
	Connection *websocket.Conn
	Name       string
	Data       interface{}
	Debug      bool
	Body       IWebSocketBody
}

func (m *WebSocketEvent) GetName() string {
	return m.Name
}

func (m *WebSocketEvent) GetData() interface{} {
	return m.Data
}

func (m *WebSocketEvent) GetDebug() bool {
	return m.Debug
}

func (m *WebSocketEvent) GetConnection() *websocket.Conn {
	return m.Connection
}

func (m *WebSocketEvent) NilBody() bool {
	if m.Body == nil {
		return true
	}

	switch reflect.TypeOf(m.Body).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(m.Body).IsNil()
	}

	return false
}

func (m *WebSocketEvent) GetBody() []byte {
	// Check if body is not set
	if m.NilBody() {
		m.Body = &WebSocketBody{}
	}

	m.Body.SetAction(m.Name)
	m.Body.SetData(m.Data)
	m.Body.Setup()

	body, err := json.Marshal(m.Body)
	if err != nil {
		lets.LogE("WebSocketEvent: %s", err.Error())
		return nil
	}

	return body
}
