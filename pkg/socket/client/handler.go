package client

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

type SocketClientHandler interface {
	// CreateClient receives a socket connection, creates a Client,
	// and adds it to an internal map. The new created Client is returned.
	CreateClient(connection.Connection) *Client
	// DestroyClient receives a socket connection, finds a Client by its id,
	// removes it from its room (if joined), and deletes the Client from an
	// internal map. If no client exists by that id, an error is returned.
	DestroyClient(connection.Connection) error
	// GetClient receives a client's string id, and returns a Client instance
	// associated with that id, or an error if no client by that id is found.
	GetClient(string) (*Client, error)
	// GetClientSize returns the amount of Client instances saved in the internal map
	GetClientSize() int
	// Clients returns a slice of Client instances saved in the internal map
	Clients() []*Client
}

// Handler implements ClientHandler
type Handler struct {
	clientsById map[string]*Client
}

func (h *Handler) CreateClient(socket connection.Connection) *Client {
	c := NewClient(socket)
	h.clientsById[socket.UUID()] = c

	return c
}

func (h *Handler) DestroyClient(socket connection.Connection) error {
	id := socket.UUID()
	if c, ok := h.clientsById[id]; ok {
		c.UnsetNamespace()
		delete(h.clientsById, id)
		return nil
	}
	return fmt.Errorf("client with id %q does not exist", id)
}

func (h *Handler) GetClient(id string) (*Client, error) {
	if c, found := h.clientsById[id]; found {
		return c, nil
	}
	return nil, fmt.Errorf("client with id %q does not exist", id)
}

func (h *Handler) Clients() []*Client {
	clients := make([]*Client, 0, len(h.clientsById))
	for _, c := range h.clientsById {
		clients = append(clients, c)
	}
	return clients
}

func (h *Handler) GetClientSize() int {
	return len(h.clientsById)
}

func NewHandler() SocketClientHandler {
	return &Handler{
		clientsById: make(map[string]*Client),
	}
}
