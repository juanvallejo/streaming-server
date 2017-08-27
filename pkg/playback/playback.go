package playback

import (
	"encoding/json"
	"log"
	"time"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// PlaybackStreamMetadataCallback is a callback function called once metadata for a stream has been fetched
type PlaybackStreamMetadataCallback func(data []byte, created bool, err error)

// StreamPlayback represents playback status for a given
// stream - there are one or more StreamPlayback instances
// for every one stream
type StreamPlayback struct {
	id          string
	queue       RoundRobinQueue
	stream      stream.Stream
	startedBy   string
	timer       *Timer
	lastUpdated time.Time

	// reapable indicates whether the object
	// is a candidate for being reaped from
	// a StreamPlayback composer
	Reapable bool
}

// UpdateStartedBy receives a client and updates the
// startedBy field with the client's current username
func (p *StreamPlayback) UpdateStartedBy(name string) {
	p.startedBy = name
}

// RefreshInfoFromClient receives a client and updates altered
// client details used as part of playback info.
// Returns a bool (true) if the client received contains
// old info matching the one stored by the playback handler,
// and such info has been since updated in the client.
func (p *StreamPlayback) RefreshInfoFromClient(c *client.Client) bool {
	cOldUser, hasOldUser := c.GetPreviousUsername()
	if !hasOldUser {
		return false
	}

	if len(p.startedBy) > 0 && p.startedBy == cOldUser {
		cUser, hasUser := c.GetUsername()
		if !hasUser {
			// should never happen, if a client has an "old username"
			// they MUST have a currently active username
			panic("client had previous username without an active / current username")
		}
		p.startedBy = cUser
		return true
	}

	return false
}

// Cleanup handles resource cleanup for room resources
func (p *StreamPlayback) Cleanup() {
	p.timer.Stop()
	p.timer.callback = nil
	p.timer = nil
	p.queue.Clear()
	p.queue = nil
	p.stream = nil
}

func (p *StreamPlayback) Pause() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Pause()
}

func (p *StreamPlayback) Play() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Play()
}

func (p *StreamPlayback) Stop() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Stop()
}

func (p *StreamPlayback) Reset() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Set(0)
}

func (p *StreamPlayback) SetTime(newTime int) error {
	p.SetLastUpdated(time.Now())
	p.timer.Set(newTime)
	return nil
}

func (p *StreamPlayback) GetTime() int {
	return p.timer.GetTime()
}

func (p *StreamPlayback) GetLastUpdated() time.Time {
	return p.lastUpdated
}

func (p *StreamPlayback) SetLastUpdated(t time.Time) {
	p.lastUpdated = t
}

// OnTick calls the playback object's timer object and sets its
// "tick" callback function; called every tick increment interval.
func (p *StreamPlayback) OnTick(callback TimerCallback) {
	p.timer.OnTick(callback)
}

func (p *StreamPlayback) GetQueue() RoundRobinQueue {
	return p.queue
}

// GetStream returns a stream.Stream object containing current stream data
// tied to the current StreamPlayback object, or a bool (false) if there
// is no stream information currently loaded for the current StreamPlayback
func (p *StreamPlayback) GetStream() (stream.Stream, bool) {
	return p.stream, p.stream != nil
}

// SetStream receives a stream.Stream and sets it as the currently-playing stream
func (p *StreamPlayback) SetStream(s stream.Stream) {
	p.UpdateStartedBy(s.Metadata().GetCreationSource().GetSourceName())
	p.stream = s
	p.stream.Metadata().SetLastUpdated(time.Now())
	p.SetLastUpdated(time.Now())
}

// GetOrCreateStreamFromUrl receives a stream location (path, url, or unique identifier)
// and retrieves a corresponding stream.Stream, or creates a new one.
// Calls callback once a cached stream is fetched, or metadata has been fetched for a
// newly-created stream.
func (p *StreamPlayback) GetOrCreateStreamFromUrl(url string, user *client.Client, streamHandler stream.StreamHandler, callback PlaybackStreamMetadataCallback) (stream.Stream, error) {
	if s, exists := streamHandler.GetStream(url); exists {
		log.Printf("INF PLAYBACK found existing stream object with url %q, retrieving...", url)
		callback([]byte{}, false, nil)
		s.Metadata().SetCreationSource(user)
		return s, nil
	}

	s, err := streamHandler.NewStream(url)
	if err != nil {
		return nil, err
	}

	s.Metadata().SetCreationSource(user)

	// if created new stream, fetch its duration info
	s.FetchMetadata(func(s stream.Stream, data []byte, err error) {
		if err != nil {
			log.Printf("ERR PLAYBACK FETCH-INFO-CALLBACK unable to calculate video metadata. Some information, such as media duration, will not be available: %v", err)
			callback(data, true, err)
			return
		}

		err = s.SetInfo(data)
		if err != nil {
			log.Printf("ERR PLAYBACK FETCH-INFO-CALLBACK unable to set parsed stream info: %v", err)
			callback(data, true, err)
			return
		}
		callback(data, true, nil)
	})

	log.Printf("INF PLAYBACK no stream found with url %q; creating... There are now %v registered streams", url, streamHandler.GetSize())
	return s, nil
}

// StreamPlaybackStatus is a serializable schema representing a summary of information
// about the current state of the StreamPlayback.
// Implements api.ApiCodec.
type StreamPlaybackStatus struct {
	QueueLength int          `json:"queueLength"`
	StartedBy   string       `json:"startedBy"`
	Stream      api.ApiCodec `json:"stream"`
	TimerStatus api.ApiCodec `json:"playback"`
}

func (s *StreamPlaybackStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

// Returns a map compatible with json types
// detailing the current playback status
func (p *StreamPlayback) GetStatus() api.ApiCodec {
	var streamCodec api.ApiCodec

	s, exists := p.GetStream()
	if exists {
		streamCodec = s.Codec()
	}

	return &StreamPlaybackStatus{
		QueueLength: p.queue.Size(),
		StartedBy:   p.startedBy,
		TimerStatus: p.timer.Status(),
		Stream:      streamCodec,
	}
}

func NewStreamPlayback(id string) *StreamPlayback {
	if len(id) == 0 {
		panic("A playback id is required to instantiate a new playback")
	}

	return &StreamPlayback{
		id:          id,
		timer:       NewTimer(),
		queue:       NewRoundRobinQueue(),
		lastUpdated: time.Now(),
	}
}
