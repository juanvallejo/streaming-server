package stream

import (
	"fmt"
	"log"
	"os"
	"strings"

	paths "github.com/juanvallejo/streaming-server/pkg/server/path"
)

type StreamHandler interface {
	// GetStream returns a registered stream by the given url
	// a url is used as a stream's unique identifier.
	// Returns a Stream object or a bool (false) if a stream
	// does not exist by the given url.
	GetStream(string) (Stream, bool)
	// ReapStream receives a Stream to remove from the list of composed Streams.
	// Returns a boolean false if the stream could not be deleted, or true otherwise.
	ReapStream(Stream) bool
	// GetStreams returns a list of all composed streams by the handler
	GetStreams() []Stream
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
	isGarbageCollected bool
	garbageCollector   *StreamReaper
	streams            map[string]Stream
}

// GetStream retrieves a stream by its assigned url
// or a bool (false) if a stream does not exist by the
// given resource location
func (h *Handler) GetStream(url string) (Stream, bool) {
	s, exists := h.streams[url]
	return s, exists
}

func (h *Handler) ReapStream(s Stream) bool {
	if _, exists := h.streams[s.GetStreamURL()]; exists {
		delete(h.streams, s.GetStreamURL())
		return exists
	}
	return false
}

func (h *Handler) GetStreams() []Stream {
	streams := []Stream{}
	for _, s := range h.streams {
		streams = append(streams, s)
	}
	return streams
}

func (h *Handler) GetSize() int {
	return len(h.streams)
}

func (h *Handler) initGarbageCollector() {
	// if handler is already being garbage collected, perform a no-op
	if h.isGarbageCollected {
		return
	}

	// if a garbage collector has not been initialized as part of the handler object, panic
	if h.garbageCollector == nil {
		log.Fatal("attempt to initialize nil garbage collector for streamHandler")
	}

	h.garbageCollector.Init(h)
	h.isGarbageCollected = true
	log.Printf("INF StreamHandler GarbageCollection started.\n")
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

	fpath := paths.StreamDataFilePathFromFilename(url)

	// determine if a mimetype can be determined from the requested filepath,
	// and that the mimetype (if any) is supported.
	mimeType, err := paths.FileMimeFromFilePath(url)
	if err != nil || !strings.HasPrefix(mimeType, "video") {
		log.Printf("ERR SOCKET CLIENT error parsing file mimetype (%q): %v", mimeType, err)
		return nil, fmt.Errorf("unable to load %q. Unsupported streaming file.", url)
	}

	_, err = os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to load %q: video file does not exist.", url)
		}
		return nil, fmt.Errorf("unable to load %q: %v", url, err)
	}

	s := NewLocalVideoStream(url)
	h.streams[url] = s
	return s, nil
}

func NewHandler() StreamHandler {
	return &Handler{
		streams: make(map[string]Stream),
	}
}

func NewGarbageCollectedHandler() StreamHandler {
	h := &Handler{
		garbageCollector: NewStreamReaper(),
		streams:          make(map[string]Stream),
	}
	h.initGarbageCollector()
	return h
}
