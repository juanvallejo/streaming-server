package connection

import (
	"encoding/json"
	"log"
	"net/http"

	"strings"

	"github.com/gorilla/websocket"
)

// Handler provides methods for managing multiple socket connections
type Handler interface {
	// NewConnection instantiates a new Connection
	// Returns the newly created Connection
	NewConnection(*websocket.Conn, *http.Request) Connection
	// GetConnection receives a Connection uuid, and returns the
	// associated connection. Returns a boolean false if no Connection
	// exists by the given uuid.
	GetConnection(string) (Connection, bool)
	// DeleteConnection receives a Connection and removes it from the internal list
	DeleteConnection(Connection)
	// Handle receives a Connection and creates a goroutine
	// to parse and handle callbacks for incoming messages
	Handle(Connection)
}

// ConnHandler implements Handler
type ConnHandler struct {
	nsHandler Namespace
	connsById map[string]Connection
}

func (h *ConnHandler) NewConnection(ws *websocket.Conn, r *http.Request) Connection {
	c := NewConnection(h.nsHandler, ws, r)
	return c
}

func (h *ConnHandler) GetConnection(uuid string) (Connection, bool) {
	c, exists := h.connsById[uuid]
	if !exists {
		return nil, exists
	}

	return c, exists
}

func (h *ConnHandler) DeleteConnection(conn Connection) {
	if _, exists := h.connsById[conn.Id()]; exists {
		delete(h.connsById, conn.Id())
	}
}

func (h *ConnHandler) Handle(conn Connection) {
	go HandleConnection(h, conn)
}

func NewHandler() Handler {
	return &ConnHandler{
		connsById: make(map[string]Connection),
		nsHandler: NewNamespaceHandler(),
	}
}

func HandleConnection(handler Handler, conn Connection) {
	for {
		var connClosed bool

		mType, data, err := conn.ReadMessage()
		if err != nil {
			connClosed = true
			if strings.HasPrefix(err.Error(), "websocket: close") || websocket.IsCloseError(err) {
				mType = websocket.CloseGoingAway
			} else {
				log.Printf("ERR WS HANDLE %v", err)
			}
		}

		if mType == websocket.CloseMessage || mType == websocket.CloseGoingAway || connClosed {
			conn.Emit("disconnection", &Message{
				Event: "disconnection",
				Data:  make(map[string]interface{}),
			})
			handler.DeleteConnection(conn)
			break
		}

		if mType == websocket.TextMessage {
			var message Message
			err := json.Unmarshal(data, &message)
			if err != nil {
				log.Printf("ERR WS HANDLE received non-json message: %v: %v", string(data), err)
				continue
			}

			conn.Emit(message.Event, &message)
			continue
		}

		log.Printf("WARN WS HANDLE received non-text message from the client: %v", data)
	}
}
