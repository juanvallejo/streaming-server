package playback

import (
	"log"
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
}

// Handler implements StreamPlaybackHandler
type Handler struct {
	isGarbageCollected bool
	garbageCollector   *PlaybackReaper
	// map of stream ids to StreamPlayback objects
	streamplaybacks map[string]*StreamPlayback
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
		return exists
	}
	return false
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

func NewHandler() StreamPlaybackHandler {
	return &Handler{
		streamplaybacks: make(map[string]*StreamPlayback),
	}
}

func NewGarbageCollectedHandler() StreamPlaybackHandler {
	h := &Handler{
		garbageCollector: NewPlaybackReaper(),
		streamplaybacks:  make(map[string]*StreamPlayback),
	}
	h.initGarbageCollector()
	return h
}
