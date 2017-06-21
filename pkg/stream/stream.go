package stream

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	STREAM_TYPE_YOUTUBE = "youtube"
	STREAM_TYPE_LOCAL   = "movie"
	STREAM_TYPE_TWITCH  = "twitch"

	YT_API_KEY = "AIzaSyCF-AsZFqN_ic0QpqB18Et1cFjAMhpxz8M"
)

type StreamFetchInfoCallback func(*http.Response, error)

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
	// GetDuration returns the stream's saved duration
	GetDuration() float64
	// GetInfo returns a map -> interface{} of json friendly data
	// describing the current stream object
	GetInfo() map[string]interface{}
	// FetchInfo calls the necessary apis / libraries needed to load
	// extra stream information
	FetchInfo(StreamFetchInfoCallback)
	// SetInfo receives a map of string->interface{} and unmarshals it into
	SetInfo(map[string]interface{}) error
}

// StreamSchema implements Stream
type StreamSchema struct {
	// kind describes the type of stream resource
	kind string
	// name describes a title assigned to the stream resource
	name string
	// url is a fully qualified resource locator
	url string
	// duration is the total time for the current stream
	Duration float64 `json:"duration"`
}

func (s *StreamSchema) GetStreamURL() string {
	return s.url
}

func (s *StreamSchema) GetName() string {
	return s.name
}

func (s *StreamSchema) GetKind() string {
	return s.kind
}

func (s *StreamSchema) GetDuration() float64 {
	return s.Duration
}

func (s *StreamSchema) FetchInfo(callback StreamFetchInfoCallback) {
	callback(nil, fmt.Errorf("unimplemented procedure"))
}

func (s *StreamSchema) SetInfo(data map[string]interface{}) error {
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonStr, s)
	return err
}

func (s *StreamSchema) GetInfo() map[string]interface{} {
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
	apiKey string
	*StreamSchema
}

func (s *YouTubeStream) FetchInfo(callback StreamFetchInfoCallback) {
	//res, err := http.Get("https://www.googleapis.com/youtube/v3/videos?id=" + ytVideoIdFromUrl(s.url) + "&key=" + s.apiKey + "&part=contentDetails")
	//if err != nil {
	//	callback(nil, err)
	//	return
	//}

	return
}

// LocalVideoStream implements Stream
// and represents a video stream from
// a local filepath.
type LocalVideoStream struct {
	*StreamSchema
}

// TwitchStream implements Stream
// and represents a twitch.tv video stream
// data and state
type TwitchStream struct {
	*StreamSchema
}

func NewYouTubeStream(url string) Stream {
	return &YouTubeStream{
		StreamSchema: &StreamSchema{
			url:  url,
			kind: STREAM_TYPE_YOUTUBE,
		},

		apiKey: YT_API_KEY,
	}
}

func NewTwitchStream(url string) Stream {
	return &TwitchStream{
		&StreamSchema{
			url:  url,
			kind: STREAM_TYPE_TWITCH,
		},
	}
}

func NewLocalVideoStream(filepath string) Stream {
	return &LocalVideoStream{
		&StreamSchema{
			url:  filepath,
			kind: STREAM_TYPE_LOCAL,
		},
	}
}

func ytVideoIdFromUrl(url string) string {
	return ""
}
