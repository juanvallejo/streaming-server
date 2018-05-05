package stream

import (
	"fmt"
	"log"
	"net/url"
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
func (h *Handler) NewStream(streamUrl string) (Stream, error) {
	if _, exists := h.streams[streamUrl]; exists {
		return nil, fmt.Errorf("error: a stream with resource location %q has already been registered", streamUrl)
	}

	u, err := url.Parse(streamUrl)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "http" || u.Scheme == "https" {
		host := u.Host
		segs := strings.Split(u.Host, "www.")
		if len(segs) > 1 {
			host = segs[1]
		}

		switch host {
		case "youtube.com", "youtu.be", "m.youtube.com":
			s := NewYouTubeStream(streamUrl)
			h.streams[streamUrl] = s
			return s, nil
		case "api.soundcloud.com", "soundcloud.com":
			s := NewSoundCloudStream(streamUrl)
			h.streams[streamUrl] = s
			return s, nil
		case "twitch.tv":
			s := NewTwitchStream(streamUrl)
			h.streams[streamUrl] = s
			return s, nil
		case "clips-media-assets.twitch.tv":
			params := u.Query()
			if len(params.Get("clip")) == 0 {
				return nil, fmt.Errorf("invalid Twitch clip url. Missing ?clip= parameter")
			}

			s := NewTwitchClipStream(streamUrl)
			h.streams[streamUrl] = s
			return s, nil
		default:
			// handle remote urls
			supportedFormats := map[string]bool{
				".mp4":  true,
				".webm": true,
				".mkv":  true,
			}

			format := paths.FileExtensionFromFilePath(u.Path)
			if supported, ok := supportedFormats[strings.ToLower(format)]; ok && supported {
				s := NewRemoteVideoStream(streamUrl)
				h.streams[streamUrl] = s
				return s, nil
			}
		}

		return nil, fmt.Errorf("stream resource location interpreted as url, but stream source is not supported for: %q", streamUrl)
	}

	fpath := paths.StreamDataFilePathFromFilename(streamUrl)

	// determine if a mimetype can be determined from the requested filepath,
	// and that the mimetype (if any) is supported.
	mimeType, err := paths.FileMimeFromFilePath(streamUrl)
	if err != nil || !strings.HasPrefix(mimeType, "video") {
		log.Printf("ERR SOCKET CLIENT error parsing file mimetype (%q): %v", mimeType, err)
		return nil, fmt.Errorf("unable to load %q. Unsupported streaming file.", streamUrl)
	}

	_, err = os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to load %q: video file does not exist.", streamUrl)
		}
		return nil, fmt.Errorf("unable to load %q: %v", streamUrl, err)
	}

	s := NewLocalVideoStream(streamUrl)
	h.streams[streamUrl] = s
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
