package playback

import (
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

type PlaybackHandler interface {
	// NewPlayback receives a playback id and instantiates a new Playback
	// object used to keep track of individual user-created stream sessions.
	// A playback id should be a fully-qualified room name.
	NewPlayback(connection.Namespace, rbac.Authorizer, client.SocketClientHandler) *Playback
	// PlaybackByNamespace receives a connection.Namespace and retrieves a Playback object
	// corresponding to that room. Returns a boolean (false) if a Playback object
	// does not exist by the given roomName.
	PlaybackByNamespace(connection.Namespace) (*Playback, bool)
	// Playbacks returns a list of all composed *Playback objects
	Playbacks() []*Playback
	// ReapPlayback receives a *Playback and removes it from the list of composed *StreamPlaybacks
	ReapPlayback(*Playback) bool
	// IsReapable receives a Playback and determines if it is reapable
	// based on whether or not its corresponding Namespace has any items left
	IsReapable(*Playback) bool
}

// Handler implements StreamPlaybackHandler
type Handler struct {
	isGarbageCollected bool
	garbageCollector   *PlaybackReaper
	// map of stream ids to Playback objects
	streamplaybacks  map[string]*Playback
	namespaceHandler connection.NamespaceHandler
}

func (h *Handler) NewPlayback(ns connection.Namespace, authorizer rbac.Authorizer, clientHandler client.SocketClientHandler) *Playback {
	s := NewPlaybackWithAdminPicker(ns, authorizer, clientHandler, h)
	h.streamplaybacks[ns.Name()] = s
	return s
}

func (h *Handler) ReapPlayback(p *Playback) bool {
	if sp, exists := h.streamplaybacks[p.name]; exists {
		sp.Cleanup()
		delete(h.streamplaybacks, sp.name)

		// clean up composed namespace with name
		// corresponding to the playback object's id
		h.namespaceHandler.DeleteNamespaceByName(sp.UUID())
		return exists
	}
	return false
}

func (h *Handler) IsReapable(p *Playback) bool {
	ns, exists := h.namespaceHandler.NamespaceByName(p.UUID())
	if !exists {
		// if a corresponding namespace for the Playback
		// does not exist, mark as reapable
		return true
	}

	return len(ns.Connections()) == 0
}

func (h *Handler) PlaybackByNamespace(ns connection.Namespace) (*Playback, bool) {
	if sPlayback, exists := h.streamplaybacks[ns.Name()]; exists {
		return sPlayback, true
	}

	return nil, false
}

func (h *Handler) Playbacks() []*Playback {
	playbacks := []*Playback{}
	for _, p := range h.streamplaybacks {
		playbacks = append(playbacks, p)
	}
	return playbacks
}

func (h *Handler) initGarbageCollector() {
	// if handler is already being garbage collected, perform a no-op
	if h.isGarbageCollected {
		return
	}

	// if a garbage collector has not been initialized as part of the handler object, panic
	if h.garbageCollector == nil {
		log.Fatal("attempt to initialize nil garbage collector for plabackHandler")
	}

	h.garbageCollector.Init(h)
	h.isGarbageCollected = true
	log.Printf("INF PlaybackHandler GarbageCollection started.\n")
}

func NewHandler(nsHandler connection.NamespaceHandler) PlaybackHandler {
	return &Handler{
		namespaceHandler: nsHandler,
		streamplaybacks:  make(map[string]*Playback),
	}
}

func NewGarbageCollectedHandler(nsHandler connection.NamespaceHandler) PlaybackHandler {
	h := &Handler{
		namespaceHandler: nsHandler,
		garbageCollector: NewPlaybackReaper(),
		streamplaybacks:  make(map[string]*Playback),
	}
	h.initGarbageCollector()
	return h
}
