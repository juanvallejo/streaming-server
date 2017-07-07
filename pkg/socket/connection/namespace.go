package connection

import (
	"log"
)

// Namespace provides convenience methods for
// handling segments of registered Connections
type Namespace interface {
	// AddToNamespace receives a namespace id and a Connection and adds the
	// Connection to the specified namespace. If the specified namespace does
	// not exist, a new one is created.
	AddToNamespace(string, Connection)
	// RemoveFromNamespace receives a namespace id and a Connection and removes
	// the Connection from the specified namespace. If the namespace does not exist,
	// a no-op occurs. If removing the Connection from the namespace results in an empty
	// namespace, the namespace is deleted.
	RemoveFromNamespace(string, Connection)
	// Broadcast iterates through all connections in a namespace and sends received data.
	// If a given namespace does not exist, a no-op occurs
	Broadcast(int, string, string, []byte)
	// BroadcastFrom behaves like Broadcast, except the connection with provided id is skipped.
	BroadcastFrom(int, string, string, string, []byte)
}

// ConnNamespace implements Namespace
type ConnNamespace struct {
	connsByNamespace map[string][]Connection
}

func (h *ConnNamespace) AddToNamespace(ns string, conn Connection) {
	if len(ns) == 0 {
		return
	}

	_, exists := h.connsByNamespace[ns]
	if !exists {
		h.connsByNamespace[ns] = []Connection{}
	}

	h.connsByNamespace[ns] = append(h.connsByNamespace[ns], conn)
}

func (h *ConnNamespace) RemoveFromNamespace(ns string, conn Connection) {
	conns, exists := h.connsByNamespace[ns]
	if !exists {
		return
	}

	for idx, c := range conns {
		if c.Id() == conn.Id() {
			h.connsByNamespace[ns] = append(conns[0:idx], conns[idx+1:]...)
			if len(h.connsByNamespace[ns]) == 0 {
				delete(h.connsByNamespace, ns)
			}
			return
		}
	}

	log.Printf("WRN SOCKET CONN NAMESPACE attempt to remove connection (%q) from namespace (%q) but connection was not found.", conn.Id(), ns)
}

func (h *ConnNamespace) Broadcast(messageType int, ns, eventName string, data []byte) {
	conns, exists := h.connsByNamespace[ns]
	if !exists {
		return
	}

	for _, c := range conns {
		c.WriteMessage(messageType, data)
	}
}

func (h *ConnNamespace) BroadcastFrom(messageType int, connId, ns, eventName string, data []byte) {
	conns, exists := h.connsByNamespace[ns]
	if !exists {
		return
	}

	for _, c := range conns {
		if c.Id() == connId {
			continue
		}
		c.WriteMessage(messageType, data)
	}
}

func NewNamespaceHandler() Namespace {
	return &ConnNamespace{
		connsByNamespace: make(map[string][]Connection),
	}
}
