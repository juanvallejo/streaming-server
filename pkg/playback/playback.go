package playback

import (
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// StreamPlayback represents playback status for a given
// stream - there are one or more StreamPlayback instances
// for every one stream
type StreamPlayback struct {
	id        string
	stream    stream.Stream
	startedBy string
	timer     *Timer
}

// UpdateStartedBy receives a client and updates the
// startedBy field with the client's current username
func (p *StreamPlayback) UpdateStartedBy(c *client.Client) {
	if name, hasName := c.GetUsername(); hasName {
		p.startedBy = name
		return
	}

	log.Printf("SOCKET CLIENT PLAYBACK attempted to update `startedBy` information, but the current client with id %q has no registered username.", c.GetId())
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

func (p *StreamPlayback) Pause() error {
	return p.timer.Pause()
}

func (p *StreamPlayback) Play() error {
	return p.timer.Play()
}

func (p *StreamPlayback) Stop() error {
	return p.timer.Stop()
}

func (p *StreamPlayback) GetTime() int {
	if p.timer == nil {
		log.Panic("attempt was made to retrieve StreamPlayback Timer.time, but Timer.time was nil. Was StreamPlayback initialized properly?")
	}
	return p.timer.GetTime()
}

// GetStream returns a stream.Stream object containing current stream data
// tied to the current StreamPlayback object, or a bool (false) if there
// is no stream information currently loaded for the current StreamPlayback
func (p *StreamPlayback) GetStream() (stream.Stream, bool) {
	return p.stream, p.stream != nil
}

// LoadStream receives a stream location (path, url, or unique identifier)
// and instantia
func (p *StreamPlayback) LoadStream(url string, streamHandler stream.StreamHandler) (stream.Stream, error) {
	if s, exists := streamHandler.GetStream(url); exists {
		log.Printf("PLAYBACK found existing stream object with url %q, loading...", url)
		p.stream = s
		return s, nil
	}

	s, err := streamHandler.NewStream(url)
	if err != nil {
		return nil, err
	}

	log.Printf("PLAYBACK no stream found with url %q; creating... There are now %v registered streams", url, streamHandler.GetSize())

	p.stream = s
	return s, nil
}

// Returns a map compatible with json types
// detailing the current playback status
func (p *StreamPlayback) GetStatus() map[string]interface{} {
	streamInfo := "&lt;no stream loaded&gt;"
	s, exists := p.GetStream()
	if exists {
		streamInfo = s.GetStreamURL()
	}

	return map[string]interface{}{
		"stream":    streamInfo,
		"timer":     p.GetTime(),
		"isPlaying": p.timer.GetStatus() == TIMER_PLAY,
		"isStopped": p.timer.GetStatus() == TIMER_STOP,
		"isPaused":  p.timer.GetStatus() == TIMER_PAUSE,
		"startedBy": p.startedBy,
	}
}

func (p *StreamPlayback) SetTime(newTime int) error {
	p.timer.Set(newTime)
	return nil
}

func NewStreamPlayback(id string) *StreamPlayback {
	if len(id) == 0 {
		panic("A playback id is required to instantiate a new playback")
	}

	return &StreamPlayback{
		id:    id,
		timer: NewTimer(),
	}
}
