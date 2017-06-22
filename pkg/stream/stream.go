package stream

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/util"
)

const (
	STREAM_TYPE_YOUTUBE = "youtube"
	STREAM_TYPE_LOCAL   = "movie"
	STREAM_TYPE_TWITCH  = "twitch"

	YT_API_KEY = "AIzaSyCF-AsZFqN_ic0QpqB18Et1cFjAMhpxz8M"
)

type StreamFetchInfoCallback func(Stream, []byte, error)

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
	SetInfo([]byte) error
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
	callback(s, nil, fmt.Errorf("unimplemented procedure"))
}

func (s *StreamSchema) SetInfo(data []byte) error {
	return json.Unmarshal(data, s)
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

type YouTubeVideoListResponse struct {
	Items []YouTubeVideoItem `json:"items"`
}

type YouTubeVideoItem struct {
	ContentDetails map[string]interface{} `json:"contentDetails"`
}

// ParseDuration retrieves a YouTubeVideoItem "duration" field value and
// replaces it with a seconds-parsed int64 value.
func (yt *YouTubeVideoItem) ParseDuration() error {
	duration, exists := yt.ContentDetails["duration"]
	if !exists {
		return fmt.Errorf("missing video data key %q", "duration")
	}

	durationStr, ok := duration.(string)
	if !ok {
		return fmt.Errorf("duration value is not a string")
	}

	segs := strings.Split(string(durationStr), "PT")
	if len(segs) < 2 {
		return fmt.Errorf("invalid time format")
	}

	timeSecs, err := util.HumanTimeToSeconds(segs[1])
	if err != nil {
		return err
	}

	yt.ContentDetails["duration"] = int64(timeSecs)
	return nil
}

func (s *YouTubeStream) FetchInfo(callback StreamFetchInfoCallback) {
	videoId, err := ytVideoIdFromUrl(s.url)
	if err != nil {
		callback(s, []byte{}, fmt.Errorf(""))
		return
	}

	go func(videoId, apiKey string, callback StreamFetchInfoCallback) {
		res, err := http.Get("https://www.googleapis.com/youtube/v3/videos?id=" + videoId + "&key=" + apiKey + "&part=contentDetails")
		if err != nil {
			callback(s, nil, err)
			return
		}

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			callback(s, nil, err)
			return
		}

		dataItems := YouTubeVideoListResponse{
			Items: []YouTubeVideoItem{},
		}
		err = json.Unmarshal(data, &dataItems)
		if err != nil {
			callback(s, nil, err)
			return
		}

		if len(dataItems.Items) == 0 {
			callback(s, nil, fmt.Errorf("no contentData found for video id %q", videoId))
			return
		}

		// parse duration from youtube api format to int64
		videoData := dataItems.Items[0]
		err = videoData.ParseDuration()
		if err != nil {
			callback(s, nil, err)
			return
		}

		jsonData, err := json.Marshal(videoData.ContentDetails)
		if err != nil {
			callback(s, nil, err)
			return
		}

		callback(s, jsonData, nil)
	}(videoId, s.apiKey, callback)
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
	// normalize url
	segs := strings.Split(url, "&")
	if len(segs) > 1 {
		url = segs[0]
	}

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

func ytVideoIdFromUrl(url string) (string, error) {
	segs := strings.Split(url, "/")
	if len(segs) < 2 {
		return "", fmt.Errorf("invalid url")
	}

	lastSeg := segs[len(segs)-1]

	if matched, _ := regexp.MatchString("watch\\?v=", lastSeg); matched {
		idSegs := strings.Split(lastSeg, "watch?v=")
		ampSegs := strings.Split(idSegs[1], "&")
		if len(ampSegs) > 1 {
			return ampSegs[0], nil
		}

		return idSegs[1], nil
	}

	return lastSeg, nil
}
