package connection

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// MessageDataCodec is a serializable schema representing
// the contents of a socket connection message
type MessageDataCodec interface {
	Serialize() ([]byte, error)
}

// MessageData implements MessageDataCodec
type MessageData map[string]interface{}

func (d *MessageData) Serialize() ([]byte, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (d *MessageData) Get(key string) (interface{}, bool) {
	val := (*d)[key]
	if val == nil {
		return nil, false
	}

	return val, true
}

type Message struct {
	Event string           `json:"event"`
	Data  MessageDataCodec `json:"data"`
}

type SocketEventCallback func(MessageDataCodec)

type Connection interface {
	// Broadcast calls the namespace handler Broadcast method
	// to scope the function's effects to the current connection's namespace
	Broadcast(string, string, []byte)
	// BroadcastFrom behaves like Broadcast, except the connection id provided
	// is skipped from any effects or mutations taken by the handler's method.
	BroadcastFrom(string, string, []byte)
	// Emit iterates through all stored SocketEventCallback functions and calls
	// them with the given Message argument.
	Emit(string, MessageDataCodec)
	// Id retrieves the connection's uuid
	Id() string
	// Join assigns the connection to a namespace
	Join(string)
	// TODO: add support for multiple namespaces per connection
	// Leave leaves any namespace the connection has been assigned to
	Leave(string)
	// On receives a key and a SocketEventCallback and pushes the SocketEventCallback
	// to a list of SocketEventCallback functions mapped to the given key
	On(string, SocketEventCallback)
	// ReadMessage reads a text message from the connection
	ReadMessage() (int, []byte, error)
	// Request returns the saved http.Request for this connection
	Request() *http.Request
	// Send receives an array of bytes to send to the connection
	Send([]byte)
	// WriteMessage writes sends a text message as an array of bytes to the connection
	WriteMessage(int, []byte) error
}

// Socket composes a websocket.Conn and implements Connection
type SocketConn struct {
	*websocket.Conn

	callbacks map[string][]SocketEventCallback
	connId    string
	httpReq   *http.Request
	nsHandler Namespace
}

func (c *SocketConn) On(eventName string, callback SocketEventCallback) {
	_, exists := c.callbacks[eventName]
	if !exists {
		c.callbacks[eventName] = []SocketEventCallback{}
	}

	c.callbacks[eventName] = append(c.callbacks[eventName], callback)
}

func (c *SocketConn) Emit(eventName string, data MessageDataCodec) {
	ec, exists := c.callbacks[eventName]
	if !exists {
		return
	}

	for _, callback := range ec {
		callback(data)
	}
}

func (c *SocketConn) Id() string {
	return c.connId
}

func (c *SocketConn) Send(data []byte) {
	c.WriteMessage(websocket.TextMessage, data)
}

func (c *SocketConn) BroadcastFrom(roomName, eventName string, data []byte) {
	c.nsHandler.BroadcastFrom(websocket.TextMessage, c.Id(), roomName, eventName, data)
}

func (c *SocketConn) Broadcast(roomName, eventName string, data []byte) {
	c.nsHandler.Broadcast(websocket.TextMessage, roomName, eventName, data)
}

func (c *SocketConn) Join(roomName string) {
	c.nsHandler.AddToNamespace(roomName, c)
}

func (c *SocketConn) Leave(roomName string) {
	c.nsHandler.RemoveFromNamespace(roomName, c)
}

func (c *SocketConn) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

func (c *SocketConn) WriteMessage(messageType int, data []byte) error {
	return c.Conn.WriteMessage(messageType, data)
}

func (c *SocketConn) Request() *http.Request {
	return c.httpReq
}

func NewConnection(nsHandler Namespace, ws *websocket.Conn, r *http.Request) Connection {
	// generate a uuid for this connection, or panic
	uuid, err := GenerateUUID()
	if err != nil {
		log.Panic(fmt.Sprintf("unable to generate uuid for new socket connection: %v", err))
	}

	return &SocketConn{
		Conn: ws,

		httpReq:   r,
		connId:    uuid,
		callbacks: make(map[string][]SocketEventCallback),
		nsHandler: nsHandler,
	}
}
