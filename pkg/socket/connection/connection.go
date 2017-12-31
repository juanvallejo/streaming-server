package connection

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/juanvallejo/streaming-server/pkg/socket/connection/util"
)

// MessageDataCodec is a serializable schema representing
// the contents of a socket connection message
type MessageDataCodec interface {
	Serialize() ([]byte, error)
}

type MessageData interface {
	MessageDataCodec

	Key(string) (value interface{}, keyExists bool)
	Set(string, interface{})
}

// MessageDataSchema implements MessageData
type MessageDataSchema map[string]interface{}

func (d *MessageDataSchema) Serialize() ([]byte, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (d *MessageDataSchema) Key(key string) (interface{}, bool) {
	val := (*d)[key]
	if val == nil {
		return nil, false
	}

	return val, true
}

func (d *MessageDataSchema) Set(key string, val interface{}) {
	(*d)[key] = val
}

func NewMessageData() MessageData {
	return &MessageDataSchema{}
}

type Message struct {
	Event string           `json:"event"`
	Data  MessageDataCodec `json:"data"`
}

type ConnectionMetadata interface {
	CreationTimestamp() time.Time
}

type ConnectionMetadataSpec struct {
	creationTimestamp time.Time
}

func (m *ConnectionMetadataSpec) CreationTimestamp() time.Time {
	return m.creationTimestamp
}

func NewConnectionMetadata() ConnectionMetadata {
	return &ConnectionMetadataSpec{
		creationTimestamp: time.Now(),
	}
}

type SocketEventCallback func(MessageDataCodec)

type Connection interface {
	// Broadcast calls the namespace handler Broadcast method
	// to scope the function's effects to the current connection's namespace
	Broadcast(string, string, []byte)
	// BroadcastFrom behaves like Broadcast, except the connection id provided
	// is skipped from any effects or mutations taken by the handler's method.
	BroadcastFrom(string, string, []byte)
	// Metadata returns ConnectionMetadata for the current connection
	Metadata() ConnectionMetadata
	// Connections returns socket connections that are in the same namespace as the connection
	Connections() []Connection
	// Emit iterates through all stored SocketEventCallback functions and calls
	// them with the given Message argument.
	Emit(string, MessageDataCodec)
	// UUID retrieves the connection's uuid
	UUID() string
	// Join assigns the connection to a namespace
	Join(string)
	// TODO: add support for multiple namespaces per connection
	// Leave leaves any namespace the connection has been assigned to
	Leave(string)
	// Namespace returns the namespace the connection has been bound to
	// or a boolean false if the connection has not yet been bound to one.
	Namespace() (Namespace, bool)
	// On receives a key and a SocketEventCallback and pushes the SocketEventCallback
	// to a list of SocketEventCallback functions mapped to the given key
	On(string, SocketEventCallback)
	// ReadMessage reads a text message from the connection
	ReadMessage() (int, []byte, error)
	// ResponseWriter returns the saved http.ResponseWriter for this connection
	ResponseWriter() http.ResponseWriter
	// Request returns the saved http.Request for this connection
	Request() *http.Request
	// Send receives an array of bytes to send to the connection
	Send([]byte)
	// WriteMessage sends a text message as an array of bytes to the connection
	WriteMessage(int, []byte) error
}

// Socket composes a websocket.Conn and implements Connection
type SocketConn struct {
	*websocket.Conn

	metadata   ConnectionMetadata
	callbacks  map[string][]SocketEventCallback
	connId     string
	respWriter http.ResponseWriter
	httpReq    *http.Request
	nsHandler  NamespaceHandler
	ns         string

	mutex sync.Mutex
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

func (c *SocketConn) UUID() string {
	return c.connId
}

func (c *SocketConn) Send(data []byte) {
	c.WriteMessage(websocket.TextMessage, data)
}

func (c *SocketConn) BroadcastFrom(roomName, eventName string, data []byte) {
	c.nsHandler.BroadcastFrom(websocket.TextMessage, c.UUID(), roomName, eventName, data)
}

func (c *SocketConn) Broadcast(roomName, eventName string, data []byte) {
	c.nsHandler.Broadcast(websocket.TextMessage, roomName, eventName, data)
}

func (c *SocketConn) Connections() []Connection {
	if len(c.ns) == 0 {
		return []Connection{}
	}

	namespace, exists := c.nsHandler.NamespaceByName(c.ns)
	if !exists {
		return []Connection{}
	}

	return namespace.Connections()
}

func (c *SocketConn) Metadata() ConnectionMetadata {
	return c.metadata
}

func (c *SocketConn) Join(roomName string) {
	c.ns = roomName
	c.nsHandler.AddToNamespace(roomName, c)
}

func (c *SocketConn) Leave(roomName string) {
	c.nsHandler.RemoveFromNamespace(roomName, c)
}

func (c *SocketConn) Namespace() (Namespace, bool) {
	return c.nsHandler.NamespaceByName(c.ns)
}

func (c *SocketConn) ReadMessage() (int, []byte, error) {
	return c.Conn.ReadMessage()
}

func (c *SocketConn) WriteMessage(messageType int, data []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.Conn.WriteMessage(messageType, data)
}

func (c *SocketConn) ResponseWriter() http.ResponseWriter {
	return c.respWriter
}

func (c *SocketConn) Request() *http.Request {
	return c.httpReq
}

func NewConnection(nsHandler NamespaceHandler, ws *websocket.Conn, w http.ResponseWriter, r *http.Request) Connection {
	// generate a uuid for this connection, or panic
	uuid, err := util.GenerateUUID()
	if err != nil {
		log.Panic(fmt.Sprintf("unable to generate uuid for new socket connection: %v", err))
	}

	return NewConnectionWithUUID(uuid, nsHandler, ws, w, r)
}

func NewConnectionWithUUID(uuid string, nsHandler NamespaceHandler, ws *websocket.Conn, w http.ResponseWriter, r *http.Request) Connection {
	return &SocketConn{
		Conn: ws,

		metadata:   NewConnectionMetadata(),
		respWriter: w,
		httpReq:    r,
		connId:     uuid,
		callbacks:  make(map[string][]SocketEventCallback),
		nsHandler:  nsHandler,
	}
}
