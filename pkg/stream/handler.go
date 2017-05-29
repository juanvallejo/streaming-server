package stream

import (
	"fmt"
	"strings"
)

const (
	LOCAL_FILE_ROOT = ""
)

type StreamHandler interface {
	// GetStream returns a registered stream by the given url
	// a url is used as a stream's unique identifier.
	// Returns a Stream object or a bool (false) if a stream
	// does not exist by the given url.
	GetStream(string) (Stream, bool)
	// NewStream creates and registers a new stream object
	// with a unique identifier url.
	// Returns a Stream object or an error if a stream has already
	// been registered with the given url
	NewStream(string) (Stream, error)
	// GetSize returns the number of stream objects currently registered
	GetSize() int
}

// Handler provides a convenience set of methods for
// managing supported stream instances
type Handler struct {
	streams map[string]Stream
}

// GetStream retrieves a stream by its assigned url
// or a bool (false) if a stream does not exist by the
// given resource location
func (h *Handler) GetStream(url string) (Stream, bool) {
	s, exists := h.streams[url]
	return s, exists
}

func (h *Handler) GetSize() int {
	return len(h.streams)
}

// NewStream receives a url and resolves it
// into a specific supported stream type
func (h *Handler) NewStream(url string) (Stream, error) {
	if _, exists := h.streams[url]; exists {
		return nil, fmt.Errorf("error: a stream with resource location %q has already been registered", url)
	}

	if strings.HasPrefix(url, "http") {
		if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
			s := NewYouTubeStream(url)
			h.streams[url] = s
			return s, nil
		}
		if strings.Contains(url, "twitch.tv") {
			s := NewTwitchStream(url)
			h.streams[url] = s
			return s, nil
		}
		return nil, fmt.Errorf("error: stream resource location interpreted as url, but stream source is not supported for: %q", url)
	}

	// TODO: restore this block once serving frontend as well
	//_, err := os.Stat(url)
	//if err != nil {
	//	return nil, fmt.Errorf("error: stream resource location interpreted as local filepath, but local file %q  was not found", url)
	//}

	s := NewLocalVideoStream(url)
	h.streams[url] = s
	return s, nil
}

func NewHandler() *Handler {
	return &Handler{
		streams: make(map[string]Stream),
	}
}
