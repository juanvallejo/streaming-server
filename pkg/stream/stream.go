package stream

const (
	STREAM_TYPE_YOUTUBE = "youtube"
	STREAM_TYPE_LOCAL   = "movie"
	STREAM_TYPE_TWITCH  = "twitch"
)

// StreamData keeps track of a stream's information
// such as a given name, filepath, etc.
type Stream interface {
	// GetStreamURL returns a stream's resource locator
	// (web url, filepath, etc.)
	GetStreamURL() string
	// GetName returns the name / title assigned to the stream
	GetName() string
	// GetKind returns the type of stream
	GetKind() string
	// GetInfo returns a map -> interface{} of json friendly data
	// describing the current stream object
	GetInfo() map[string]interface{}
}

// StreamSchema implements Stream
type StreamSchema struct {
	// kind describes the type of stream resource
	kind string
	// name describes a title assigned to the stream resource
	name string
	// url is a fully qualified resource locator
	url string
}

func (s StreamSchema) GetStreamURL() string {
	return s.url
}

func (s StreamSchema) GetName() string {
	return s.name
}

func (s StreamSchema) GetKind() string {
	return s.kind
}

func (s StreamSchema) GetInfo() map[string]interface{} {
	return map[string]interface{}{
		"kind": s.kind,
		"name": s.name,
		"url":  s.url,
	}
}

// YouTubeStream implements Stream
// and represents a youtube video stream
// data and state
type YouTubeStream struct {
	StreamSchema
}

// LocalVideoStream implements Stream
// and represents a video stream from
// a local filepath.
type LocalVideoStream struct {
	StreamSchema
}

// TwitchStream implements Stream
// and represents a twitch.tv video stream
// data and state
type TwitchStream struct {
	StreamSchema
}

func NewYouTubeStream(url string) Stream {
	return YouTubeStream{
		StreamSchema{
			url:  url,
			kind: STREAM_TYPE_YOUTUBE,
		},
	}
}

func NewTwitchStream(url string) Stream {
	return TwitchStream{
		StreamSchema{
			url:  url,
			kind: STREAM_TYPE_TWITCH,
		},
	}
}

func NewLocalVideoStream(filepath string) Stream {
	return LocalVideoStream{
		StreamSchema{
			url:  filepath,
			kind: STREAM_TYPE_LOCAL,
		},
	}
}
