package stream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	apiconfig "github.com/juanvallejo/streaming-server/pkg/api/config"
	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	pathutil "github.com/juanvallejo/streaming-server/pkg/server/path"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/util"
)

const (
	STREAM_TYPE_YOUTUBE = "youtube"
	STREAM_TYPE_LOCAL   = "movie"
	STREAM_TYPE_TWITCH  = "twitch"

	DEFAULT_LIB_AV_BIN = "ffprobe" // used to extract local media file metadata
)

type StreamMetadataCallback func(Stream, []byte, error)

// StreamCreationSource describes a source of creation for a stream
type StreamCreationSource interface {
	GetSourceName() string
}

// StreamRef is an object that references a Stream object
type StreamRef interface {
	// UUID returns a unique uuid for a streamRef
	UUID() string
}

type UnknownStreamCreationSourceSchema struct{}

func (u *UnknownStreamCreationSourceSchema) GetSourceName() string {
	return "no source info"
}

type StreamCreationSourceSchema struct {
	SourceName string `json:"name"`
}

func (c *StreamCreationSourceSchema) GetSourceName() string {
	return c.SourceName
}
func NewStreamCreationSource(name string) StreamCreationSource {
	return &StreamCreationSourceSchema{
		SourceName: name,
	}
}

// StreamMeta represents a Stream's metadata information
type StreamMeta interface {
	// SetCreationSource sets a source of creation for the stream
	SetCreationSource(StreamCreationSource)
	// GetCreationSource retrieves a stored source of creation for the stream
	GetCreationSource() StreamCreationSource
	// SetLastUpdated receives a timestamp indicating the last time
	// a Stream's data was updated
	SetLastUpdated(time.Time)
	// GetLastUpdated returns a timestamp indicating the last time
	// a Stream's data was updated
	GetLastUpdated() time.Time
	// AddParentRef receives a StreamRef and appends it to an internal list of parentRefs
	// if a duplicate parentRef is given, a boolean false is returned
	AddParentRef(StreamRef) bool
	// RemoveParentRef receives a StreamRef and removes it from an internal list of parentRefs
	// if a given parentRef is not found, a boolean false is returned
	RemoveParentRef(StreamRef) bool
	// GetParentRefs returns a list of all currently stored parentRefs
	GetParentRefs() []StreamRef
	// SetLabelledRef receives a key, value pair, adding the new pair
	// if the key does not yet exist in the list of labelledRefs, or
	// replacing the value of the key if the key already exists.
	// Returns a boolean (true) if a label was successfully added,
	// or false if the label already exists and its value was replaced.
	SetLabelledRef(string, StreamRef) bool
	// RemoveLabelledRef receives a key and removes the corresponding key-value pair
	// if it exists.
	// Returns a boolean (false) if a key-value pair did not exist by the given key,
	// or if the key-value pair exists but it failed to be removed.
	RemoveLabelledRef(string) bool
	// GetLabelledRef returns the ref stored under the given key and a boolean true,
	// or a boolean false if the given key does not exist.
	GetLabelledRef(string) (StreamRef, bool)
}

// StreamMetaSchema implements StreamMeta
type StreamMetaSchema struct {
	// CreationSource is extra info about the stream source
	CreationSource StreamCreationSource
	// LastUpdated is extra info signifying the stream's last data update
	LastUpdated time.Time `json:"lastUpdated"`
	// ParentRefs stores a map of objects aggregating this object.
	ParentRefs map[string]StreamRef
	// LabelledRefs store an object reference to the
	// Stream object under a given string label key.
	LabelledRefs map[string]StreamRef
}

func (s *StreamMetaSchema) GetCreationSource() StreamCreationSource {
	return s.CreationSource
}

func (s *StreamMetaSchema) SetCreationSource(source StreamCreationSource) {
	s.CreationSource = source
}

func (s *StreamMetaSchema) SetLastUpdated(t time.Time) {
	s.LastUpdated = t
}

func (s *StreamMetaSchema) GetLastUpdated() time.Time {
	return s.LastUpdated
}

func (s *StreamMetaSchema) GetParentRefs() []StreamRef {
	refs := []StreamRef{}
	for _, r := range s.ParentRefs {
		refs = append(refs, r)
	}
	return refs
}

func (s *StreamMetaSchema) AddParentRef(ref StreamRef) bool {
	if _, exists := s.ParentRefs[ref.UUID()]; exists {
		return false
	}
	s.ParentRefs[ref.UUID()] = ref
	return true
}

func (s *StreamMetaSchema) RemoveParentRef(ref StreamRef) bool {
	if _, exists := s.ParentRefs[ref.UUID()]; exists {
		delete(s.ParentRefs, ref.UUID())
		return true
	}
	return false
}

func (s *StreamMetaSchema) SetLabelledRef(key string, value StreamRef) bool {
	if _, exists := s.LabelledRefs[key]; exists {
		s.LabelledRefs[key] = value
		return false
	}

	s.LabelledRefs[key] = value
	return true
}

func (s *StreamMetaSchema) GetLabelledRef(key string) (StreamRef, bool) {
	if ref, exists := s.LabelledRefs[key]; exists {
		return ref, true
	}

	return nil, false
}

func (s *StreamMetaSchema) RemoveLabelledRef(key string) bool {
	if _, exists := s.LabelledRefs[key]; exists {
		delete(s.LabelledRefs, key)
		return true
	}

	return false
}

func NewStreamMeta() StreamMeta {
	return &StreamMetaSchema{
		CreationSource: &UnknownStreamCreationSourceSchema{},
		LastUpdated:    time.Now(),
		ParentRefs:     make(map[string]StreamRef),
		LabelledRefs:   make(map[string]StreamRef),
	}
}

// StreamData keeps track of a stream's information
// such as a given name, filepath, etc.
// Implements playback.QueueItem
type Stream interface {
	// UUID returns a unique id
	// assigned during stream creation, used to
	// distinguish the stream from other streams.
	UUID() string
	// GetStreamURL returns a stream's resource locator
	// (web url, filepath, etc.)
	GetStreamURL() string
	// GetName returns the name / title assigned to the stream
	GetName() string
	// GetKind returns the type of stream
	GetKind() string
	// GetDuration returns the stream's saved duration
	GetDuration() float64
	// Codec returns a serializable representation of the
	// current stream
	Codec() api.ApiCodec
	// Metadata returns the Stream's stored Meta information
	Metadata() StreamMeta
	// FetchMetadata calls the necessary apis / libraries needed to load
	// extra stream information in a separate goroutine. This asynchronous
	// method calls a passed callback function with retrieved metadata info.
	FetchMetadata(StreamMetadataCallback)
	// SetInfo receives a map of string->interface{} and unmarshals it into
	SetInfo([]byte) error
}

// StreamSchema implements Stream
// also implements an pkg/api/types.ApiCodec
type StreamSchema struct {
	// Kind describes the type of stream resource
	Kind string `json:"kind"`
	// Name describes a title assigned to the stream resource
	Name string `json:"name"`
	// Url is a fully qualified resource locator
	Url string `json:"url"`
	// Duration is the total time for the current stream
	Duration float64 `json:"duration"`
	// Thumbnail is a url pointing to a still of the stream
	Thumbnail string `json:"thumb"`
	// Metadata stores Stream abject meta information
	Meta StreamMeta `json:"metadata"`
}

func (s *StreamSchema) GetStreamURL() string {
	return s.Url
}

func (s *StreamSchema) UUID() string {
	return s.Url
}

func (s *StreamSchema) GetName() string {
	return s.Name
}

func (s *StreamSchema) GetKind() string {
	return s.Kind
}

func (s *StreamSchema) GetDuration() float64 {
	return s.Duration
}

func (s *StreamSchema) Metadata() StreamMeta {
	return s.Meta
}

func (s *StreamSchema) FetchMetadata(callback StreamMetadataCallback) {
	callback(s, nil, fmt.Errorf("Stream schema of kind %q has no FetchMetadata method implemented.", s.Kind))
}

func (s *StreamSchema) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (s *StreamSchema) SetInfo(data []byte) error {
	s.Meta.SetLastUpdated(time.Now())
	return json.Unmarshal(data, s)
}

func (s *StreamSchema) Codec() api.ApiCodec {
	return s
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
	Snippet        struct {
		Title string `json:"title"`
	} `json:"snippet"`
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

func (s *YouTubeStream) FetchMetadata(callback StreamMetadataCallback) {
	videoId, err := ytVideoIdFromUrl(s.Url)
	if err != nil {
		callback(s, []byte{}, err)
		return
	}

	go func(videoId, apiKey string, callback StreamMetadataCallback) {
		res, err := http.Get("https://www.googleapis.com/youtube/v3/videos?id=" + videoId + "&key=" + apiKey + "&part=contentDetails,snippet")
		if err != nil {
			callback(s, nil, err)
			return
		}

		defer res.Body.Close()

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

		// append title
		videoData.ContentDetails["name"] = videoData.Snippet.Title
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

	libAvRootPath string
	libAvFile     string
}

func (s *LocalVideoStream) FetchMetadata(callback StreamMetadataCallback) {
	if len(s.libAvRootPath) == 0 {
		callback(s, []byte{}, fmt.Errorf("unsupported os. Skipping local file duration calculation."))
		return
	}

	go func(s *LocalVideoStream, callback StreamMetadataCallback) {
		data, err := FetchLocalVideoMetadata(s)
		if err != nil {
			callback(s, []byte{}, err)
			return
		}

		callback(s, data, nil)
	}(s, callback)
}

// FetchLocalVideoMetadata is a blocking function that retrieves metadata for a local video stream
func FetchLocalVideoMetadata(s *LocalVideoStream) ([]byte, error) {
	fpath := pathutil.StreamDataFilePathFromUrl(s.Url)

	args := []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=duration", "-of", "default=noprint_wrappers=1:nokey=1", fpath}
	command := exec.Command(s.libAvRootPath+s.libAvFile, args...)

	var buff bytes.Buffer
	command.Stdout = &buff

	err := command.Run()
	if err != nil {
		return []byte{}, err
	}

	duration, err := strconv.ParseFloat(strings.Trim(buff.String(), "\n"), 32)
	if err != nil {
		return []byte{}, err
	}

	kv := map[string]interface{}{
		"duration": duration,
	}

	m, err := json.Marshal(kv)
	if err != nil {
		return []byte{}, err
	}

	return m, nil
}

// TwitchStream implements Stream
// and represents a twitch.tv video stream
// data and state
type TwitchStream struct {
	*StreamSchema

	apiKey string
}

// TwitchResponseItem contains twitch api response data
// for a unique twitch video
type TwitchResponseItem struct {
	Title      string                        `json:"title"`
	Length     int                           `json:"length"`
	Thumbnails []TwitchResponseItemThumbnail `json:"thumbnails"`
}

type TwitchResponseItemThumbnail struct {
	Url string `json:"url"`
}

type TwitchVideoItem map[string]interface{}

func (s *TwitchStream) FetchMetadata(callback StreamMetadataCallback) {
	videoId, err := twitchVideoIdFromUrl(s.Url)
	if err != nil {
		callback(s, []byte{}, err)
		return
	}

	go func(videoId, apiKey string, callback StreamMetadataCallback) {
		client := &http.Client{}

		req, err := http.NewRequest("GET", "https://api.twitch.tv/kraken/videos/"+videoId, nil)
		if err != nil {
			callback(s, nil, err)
			return
		}

		req.Header.Set("Client-ID", apiKey)

		res, err := client.Do(req)
		if err != nil {
			callback(s, nil, err)
			return
		}

		defer res.Body.Close()

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			callback(s, nil, err)
			return
		}

		twitchResponseItem := &TwitchResponseItem{}
		err = json.Unmarshal(data, twitchResponseItem)
		if err != nil {
			callback(s, nil, err)
			return
		}

		// craft callback metadata response with default fields
		twitchVideoItem := TwitchVideoItem{}
		twitchVideoItem["name"] = twitchResponseItem.Title
		twitchVideoItem["duration"] = float64(twitchResponseItem.Length)

		if len(twitchResponseItem.Thumbnails) > 0 {
			twitchVideoItem["thumbnail"] = twitchResponseItem.Thumbnails[0].Url
		}

		jsonData, err := json.Marshal(twitchVideoItem)
		if err != nil {
			callback(s, nil, err)
			return
		}

		callback(s, jsonData, nil)
	}(videoId, s.apiKey, callback)
}

func NewYouTubeStream(url string) Stream {
	// normalize url
	segs := strings.Split(url, "&")
	if len(segs) > 1 {
		url = segs[0]
	}

	thumb := ""
	id, err := ytVideoIdFromUrl(url)
	if err == nil {
		thumb = "https://img.youtube.com/vi/" + id + "/default.jpg"
	}

	return &YouTubeStream{
		StreamSchema: &StreamSchema{
			Url:       url,
			Thumbnail: thumb,
			Kind:      STREAM_TYPE_YOUTUBE,
			Meta:      NewStreamMeta(),
		},

		apiKey: apiconfig.YT_API_KEY,
	}
}

func NewTwitchStream(url string) Stream {
	return &TwitchStream{
		StreamSchema: &StreamSchema{
			Url:  url,
			Kind: STREAM_TYPE_TWITCH,
			Meta: NewStreamMeta(),
		},

		apiKey: apiconfig.TWITCH_API_KEY,
	}
}

func NewLocalVideoStream(filepath string) Stream {
	libAvBin := DEFAULT_LIB_AV_BIN

	ops := runtime.GOOS
	avRootPath := "lib/linux/x86_64/"
	if ops == "windows" {
		avRootPath = "lib/windows/x86_64/"
		libAvBin = libAvBin + ".exe"
	} else if ops == "darwin" {
		avRootPath = "lib/darwin/x86_64/"
	} else {
		if ops != "linux" {
			avRootPath = ""
		}
	}

	return &LocalVideoStream{
		StreamSchema: &StreamSchema{
			Url:  filepath,
			Kind: STREAM_TYPE_LOCAL,
			Meta: NewStreamMeta(),
		},

		libAvRootPath: avRootPath,
		libAvFile:     libAvBin,
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

func twitchVideoIdFromUrl(url string) (string, error) {
	segs := strings.Split(url, "/videos/")
	if len(segs) != 2 {
		return "", fmt.Errorf("invalid url")
	}

	return segs[1], nil
}
