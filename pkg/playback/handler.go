package playback

type StreamPlaybackHandler interface {
	// NewStreamPlayback receives a playback id and instantiates a new StreamPlayback
	// object used to keep track of individual user-created stream sessions.
	// A playback id should be a fully-qualified room name.
	NewStreamPlayback(string) *StreamPlayback
	// GetStreamPlayback receives a StreamPlayback id (usually a fully qualified room name)
	// and retrieves a StreamPlayback object corresponding to that room.
	// Returns a boolean (false) if a StreamPlayback object does not exist by the given roomName.
	GetStreamPlayback(string) (*StreamPlayback, bool)
}

// Handler implements StreamPlaybackHandler
type Handler struct {
	// map of stream ids to StreamPlayback objects
	streamplaybacks map[string]*StreamPlayback
}

func (h *Handler) NewStreamPlayback(roomName string) *StreamPlayback {
	s := NewStreamPlayback(roomName)
	h.streamplaybacks[roomName] = s
	return s
}

func (h *Handler) GetStreamPlayback(roomName string) (*StreamPlayback, bool) {
	if sPlayback, exists := h.streamplaybacks[roomName]; exists {
		return sPlayback, true
	}

	return nil, false
}

func NewHandler() StreamPlaybackHandler {
	return &Handler{
		streamplaybacks: make(map[string]*StreamPlayback),
	}
}
