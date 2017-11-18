package connection

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/connection/util"
)

type Namespace interface {
	// Add receives a Connection to compose
	Add(Connection) error
	// Remove receives a Connection to remove from the list
	// of composed connections. Returns an error if the
	// connection is not aggregated by this namespace
	Remove(Connection) error
	// Connection receives a connection uuid and returns the
	// connection associated with it, or a boolean (false)
	// if a connection does not exist by the specified uuid.
	Connection(string) (Connection, bool)
	// Connections returns a slice of connections aggregated by
	// the current namespace
	Connections() []Connection
	// Name returns the given namespace name
	Name() string
	// UUID returns the unique identifier for the namespace
	UUID() string
}

type NamespaceSpec struct {
	name      string
	id        string
	connsById map[string]Connection
}

func (n *NamespaceSpec) Add(conn Connection) error {
	if _, exists := n.connsById[conn.UUID()]; exists {
		return fmt.Errorf("connection with id (%s) has already been added to namespace %q", conn.UUID(), n.name)
	}

	n.connsById[conn.UUID()] = conn
	return nil
}

func (n *NamespaceSpec) Remove(conn Connection) error {
	if _, exists := n.connsById[conn.UUID()]; exists {
		delete(n.connsById, conn.UUID())
		return nil
	}

	return fmt.Errorf("connection with id (%s) does not exist in namespace %q", conn.UUID(), n.name)
}

func (n *NamespaceSpec) Connection(uuid string) (Connection, bool) {
	c, exists := n.connsById[uuid]
	return c, exists
}

func (n *NamespaceSpec) Connections() []Connection {
	conns := []Connection{}
	for _, c := range n.connsById {
		conns = append(conns, c)
	}

	return conns
}

func (n *NamespaceSpec) Name() string {
	return n.name
}

func (n *NamespaceSpec) UUID() string {
	return n.id
}

func NewNamespace(name string) Namespace {
	id, err := util.GenerateUUID()
	if err != nil {
		log.Panic(fmt.Sprintf("unable to generate namespace uuid: %v", err))
	}

	return &NamespaceSpec{
		id:        id,
		name:      name,
		connsById: make(map[string]Connection),
	}
}

// Namespace provides convenience methods for
// handling segments of registered Connections
type NamespaceHandler interface {
	// AddToNamespace receives a namespace name and a Connection and adds the
	// Connection to the specified namespace. If the specified namespace does
	// not exist, a new one is created.
	AddToNamespace(string, Connection)
	// NamespaceByName receives a namespace name and returns the corresponding connections
	// assigned to it, or a boolean (false) if it does not exist.
	NamespaceByName(string) (Namespace, bool)
	// NewNamespace creates a namespace with the given name
	// or returns an existing namespace if one already exists
	// by the given name
	NewNamespace(string) Namespace
	// RemoveFromNamespace receives a namespace name and a Connection and removes
	// the Connection from the specified namespace. If the namespace does not exist,
	// a no-op occurs. If removing the Connection from the namespace results in an empty
	// namespace, the namespace is deleted.
	RemoveFromNamespace(string, Connection)
	// DeleteNamespaceByName receives a namespace name and removes it
	// from the list of composed namespaces
	DeleteNamespaceByName(string) error
	// Broadcast iterates through all connections in a namespace and sends received data.
	// If a given namespace does not exist, a no-op occurs
	Broadcast(int, string, string, []byte)
	// BroadcastFrom behaves like Broadcast, except the connection with provided id is skipped.
	BroadcastFrom(int, string, string, string, []byte)
}

// NamespaceHandlerSpec implements Namespace
type NamespaceHandlerSpec struct {
	nsByName map[string]Namespace
}

func (h *NamespaceHandlerSpec) AddToNamespace(ns string, conn Connection) {
	if len(ns) == 0 {
		return
	}

	namespace, exists := h.nsByName[ns]
	if !exists {
		namespace = NewNamespace(ns)
		h.nsByName[ns] = namespace
	}

	namespace.Add(conn)
}

func (h *NamespaceHandlerSpec) NewNamespace(ns string) Namespace {
	namespace, exists := h.nsByName[ns]
	if !exists {
		namespace = NewNamespace(ns)
		h.nsByName[ns] = namespace
	}

	return namespace
}

func (h *NamespaceHandlerSpec) NamespaceByName(ns string) (Namespace, bool) {
	conns, exist := h.nsByName[ns]
	return conns, exist
}

func (h *NamespaceHandlerSpec) DeleteNamespaceByName(ns string) error {
	if _, exists := h.nsByName[ns]; exists {
		delete(h.nsByName, ns)
		return nil
	}

	return fmt.Errorf("unable to remove non-existent namespace with name %q", ns)
}

func (h *NamespaceHandlerSpec) RemoveFromNamespace(ns string, conn Connection) {
	namespace, exists := h.nsByName[ns]
	if !exists {
		return
	}

	if err := namespace.Remove(conn); err != nil {
		log.Printf("WRN SOCKET CONN NAMESPACE unable to remove connection (%q) from namespace (%q): %v", conn.UUID(), ns, err)
	}
}

func (h *NamespaceHandlerSpec) Broadcast(messageType int, ns, eventName string, data []byte) {
	namespace, exists := h.nsByName[ns]
	if !exists {
		return
	}

	for _, c := range namespace.Connections() {
		c.WriteMessage(messageType, data)
	}
}

func (h *NamespaceHandlerSpec) BroadcastFrom(messageType int, connId, ns, eventName string, data []byte) {
	namespace, exists := h.nsByName[ns]
	if !exists {
		return
	}

	for _, c := range namespace.Connections() {
		if c.UUID() == connId {
			continue
		}
		c.WriteMessage(messageType, data)
	}
}

func NewNamespaceHandler() NamespaceHandler {
	return &NamespaceHandlerSpec{
		nsByName: make(map[string]Namespace),
	}
}
