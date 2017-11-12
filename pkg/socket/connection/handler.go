package connection

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
)

// ConnectionHandler provides methods for managing multiple socket connections
type ConnectionHandler interface {
	// Authorizer returns an RBAC authorizer or nil
	Authorizer() rbac.Authorizer
	// NewConnection instantiates a new Connection
	// if a non-empty uuid string is given, a new
	// connection is spawned with the given uuid.
	// Returns the newly created Connection
	NewConnection(string, *websocket.Conn, http.ResponseWriter, *http.Request) Connection
	// GetConnection receives a Connection uuid, and returns the
	// associated connection. Returns a boolean false if no Connection
	// exists by the given uuid.
	GetConnection(string) (Connection, bool)
	// DeleteConnection receives a Connection and removes it from the internal list
	DeleteConnection(Connection)
	// NamespaceByName returns a connection Namespace for the given Namespace name
	// Returns a boolean (false) if a namespace by the given name does not exist
	NamespaceByName(string) (Namespace, bool)
	// Handle receives a Connection and creates a goroutine
	// to parse and handle callbacks for incoming messages
	Handle(Connection)
}

// ConnHandler implements Handler
type ConnHandler struct {
	nsHandler NamespaceHandler
	connsById map[string]Connection
}

func (h *ConnHandler) Authorizer() rbac.Authorizer {
	return nil
}

func (h *ConnHandler) NewConnection(uuid string, ws *websocket.Conn, w http.ResponseWriter, r *http.Request) Connection {
	if len(uuid) > 0 {
		return NewConnectionWithUUID(uuid, h.nsHandler, ws, w, r)
	}

	return NewConnection(h.nsHandler, ws, w, r)
}

func (h *ConnHandler) GetConnection(uuid string) (Connection, bool) {
	c, exists := h.connsById[uuid]
	if !exists {
		return nil, exists
	}

	return c, exists
}

func (h *ConnHandler) DeleteConnection(conn Connection) {
	if _, exists := h.connsById[conn.UUID()]; exists {
		delete(h.connsById, conn.UUID())
	}
}

func (h *ConnHandler) NamespaceByName(ns string) (Namespace, bool) {
	return h.nsHandler.NamespaceByName(ns)
}

func (h *ConnHandler) Handle(conn Connection) {
	go HandleConnection(h, conn)
}

func NewHandler(nsHandler NamespaceHandler) ConnectionHandler {
	return &ConnHandler{
		connsById: make(map[string]Connection),
		nsHandler: nsHandler,
	}
}

type ConnHandlerWithRBAC struct {
	ConnectionHandler

	authorizer rbac.Authorizer
}

func (r *ConnHandlerWithRBAC) Authorizer() rbac.Authorizer {
	return r.authorizer
}

func NewHandlerWithRBAC(authorizer rbac.Authorizer, nsHandler NamespaceHandler) ConnectionHandler {
	return &ConnHandlerWithRBAC{
		ConnectionHandler: NewHandler(nsHandler),
		authorizer:        authorizer,
	}
}

func HandleConnection(handler ConnectionHandler, conn Connection) {
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
			conn.Emit("disconnection", NewMessageData())
			handler.DeleteConnection(conn)
			break
		}

		if mType == websocket.TextMessage {
			var message Message
			message.Data = NewMessageData()
			err := json.Unmarshal(data, &message)
			if err != nil {
				log.Printf("ERR WS HANDLE received non-json message: %v: %v", string(data), err)
				continue
			}

			conn.Emit(message.Event, message.Data)
			continue
		}

		log.Printf("WRN WS HANDLE received non-text message from the client: %v", data)
	}
}
