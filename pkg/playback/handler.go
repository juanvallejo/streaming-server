package playback

import (
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

type StreamPlaybackHandler interface {
	// NewStreamPlayback receives a playback id and instantiates a new StreamPlayback
	// object used to keep track of individual user-created stream sessions.
	// A playback id should be a fully-qualified room name.
	NewStreamPlayback(string) *StreamPlayback
	// GetStreamPlayback receives a StreamPlayback id (usually a fully qualified room name)
	// and retrieves a StreamPlayback object corresponding to that room.
	// Returns a boolean (false) if a StreamPlayback object does not exist by the given roomName.
	GetStreamPlayback(string) (*StreamPlayback, bool)
	// GetStreamPlaybacks returns a list of all composed *StreamPlayback objects
	GetStreamPlaybacks() []*StreamPlayback
	// ReapStreamPlayback receives a *StreamPlayback and removes it from the list of composed *StreamPlaybacks
	ReapStreamPlayback(*StreamPlayback) bool
	// IsReapable receives a StreamPlayback and determines if it is reapable
	// based on whether or not its corresponding Namespace has any items left
	IsReapable(*StreamPlayback) bool
}

// Handler implements StreamPlaybackHandler
type Handler struct {
	isGarbageCollected bool
	garbageCollector   *PlaybackReaper
	// map of stream ids to StreamPlayback objects
	streamplaybacks  map[string]*StreamPlayback
	namespaceHandler connection.NamespaceHandler
}

func (h *Handler) NewStreamPlayback(roomName string) *StreamPlayback {
	s := NewStreamPlayback(roomName)
	h.streamplaybacks[roomName] = s
	return s
}

func (h *Handler) ReapStreamPlayback(p *StreamPlayback) bool {
	if sp, exists := h.streamplaybacks[p.id]; exists {
		sp.Cleanup()
		delete(h.streamplaybacks, sp.id)

		// clean up composed namespace with name
		// corresponding to the playback object's id
		h.namespaceHandler.DeleteNamespaceByName(sp.UUID())
		return exists
	}
	return false
}

func (h *Handler) IsReapable(p *StreamPlayback) bool {
	ns, exists := h.namespaceHandler.NamespaceByName(p.UUID())
	if !exists {
		// if a corresponding namespace for the StreamPlayback
		// does not exist, mark as reapable
		return true
	}

	return len(ns.Connections()) == 0
}

func (h *Handler) GetStreamPlayback(roomName string) (*StreamPlayback, bool) {
	if sPlayback, exists := h.streamplaybacks[roomName]; exists {
		return sPlayback, true
	}

	return nil, false
}

func (h *Handler) GetStreamPlaybacks() []*StreamPlayback {
	playbacks := []*StreamPlayback{}
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

func NewHandler(nsHandler connection.NamespaceHandler) StreamPlaybackHandler {
	return &Handler{
		namespaceHandler: nsHandler,
		streamplaybacks:  make(map[string]*StreamPlayback),
	}
}

func NewGarbageCollectedHandler(nsHandler connection.NamespaceHandler) StreamPlaybackHandler {
	h := &Handler{
		namespaceHandler: nsHandler,
		garbageCollector: NewPlaybackReaper(),
		streamplaybacks:  make(map[string]*StreamPlayback),
	}
	h.initGarbageCollector()
	return h
}
