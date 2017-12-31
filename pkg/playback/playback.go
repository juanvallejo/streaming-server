package playback

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/playback/queue"
	"github.com/juanvallejo/streaming-server/pkg/playback/util"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

const (
	// A playback state of "NOT_STARTED" indicates that no
	// streams have been played (or have started to play)
	// since the room's creation, regardless of any items
	// that may currently exist in the queue.
	PLAYBACK_STATE_NOT_STARTED PlaybackState = iota

	// A playback state of "STARTED" indicates that at least
	// one stream has been queued, and it is either playing,
	// or has finished playing.
	PLAYBACK_STATE_STARTED

	// A playback state of "ENDED" indicates that one or more
	// streams have been queued and played all the way through,
	// and have now ended, with no additional items left to
	// play from the queue.
	PLAYBACK_STATE_ENDED
)

// PlaybackStreamMetadataCallback is a callback function called once metadata for a stream has been fetched
type PlaybackStreamMetadataCallback func(data []byte, created bool, err error)

// PlaybackState represents the current state of the room's playback
type PlaybackState int

// Playback represents playback status for a given
// stream - there are one or more StreamPlayback instances
// for every one stream
type Playback struct {
	name               string
	queueHandler       queue.QueueHandler
	adminPicker        AdminPicker
	stream             stream.Stream
	startedBy          string
	timer              *Timer
	lastUpdated        time.Time
	lastAdminDeparture time.Time

	// State indicates the current state of the
	// room's Playback
	state PlaybackState
}

// Cleanup handles resource cleanup for room resources
func (p *Playback) Cleanup() {
	// remove room ref from the current stream
	if p.stream != nil {
		p.stream.Metadata().RemoveParentRef(p)
		p.stream.Metadata().RemoveLabelledRef(p.UUID())
	}

	if p.adminPicker != nil {
		p.adminPicker.Stop()
	}

	p.timer.Stop()
	p.timer.callbacks = []TimerCallback{}
	p.timer = nil
	p.ClearQueue()
	p.stream = nil
}

func (p *Playback) UUID() string {
	return p.name
}

func (p *Playback) SetState(s PlaybackState) {
	p.state = s
}

// State returns the current stream-playback state
func (p *Playback) State() PlaybackState {
	return p.state
}

// HandleAdminDeparture receives a departing connection and determines if at least
// one other connection in its namespace is bound to the admin role. If no other
// admins are found, the adminHandler is notified.
func (p *Playback) HandleDisconnection(conn connection.Connection, authorizer rbac.Authorizer, handler client.SocketClientHandler) {
	if authorizer == nil || conn == nil {
		return
	}

	var adminBinding rbac.RoleBinding
	for _, b := range authorizer.Bindings() {
		if b.Role().Name() == rbac.ADMIN_ROLE {
			adminBinding = b
			break
		}
	}
	if adminBinding == nil {
		return
	}

	for _, admin := range adminBinding.Subjects() {
		if admin.UUID() == conn.UUID() {
			p.lastAdminDeparture = time.Now()
			break
		}
	}
}

// UpdateStartedBy receives a client and updates the
// startedBy field with the client's current username
func (p *Playback) UpdateStartedBy(name string) {
	p.startedBy = name
}

// RefreshInfoFromClient receives a client and updates altered
// client details used as part of playback info.
// Returns a bool (true) if the client received contains
// old info matching the one stored by the playback handler,
// and such info has been since updated in the client.
func (p *Playback) RefreshInfoFromClient(c *client.Client) bool {
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

func (p *Playback) Pause() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Pause()
}

func (p *Playback) Play() error {
	p.SetState(PLAYBACK_STATE_STARTED)
	p.SetLastUpdated(time.Now())
	return p.timer.Play()
}

func (p *Playback) Stop() error {
	p.SetState(PLAYBACK_STATE_ENDED)
	p.SetLastUpdated(time.Now())
	return p.timer.Stop()
}

func (p *Playback) Reset() error {
	p.SetLastUpdated(time.Now())
	return p.timer.Set(0)
}

func (p *Playback) SetTime(newTime int) error {
	p.SetLastUpdated(time.Now())
	p.timer.Set(newTime)
	return nil
}

func (p *Playback) GetTime() int {
	return p.timer.GetTime()
}

func (p *Playback) LastAdminDepartureTime() time.Time {
	return p.lastAdminDeparture
}

func (p *Playback) GetLastUpdated() time.Time {
	return p.lastUpdated
}

func (p *Playback) SetLastUpdated(t time.Time) {
	p.lastUpdated = t
}

// OnTick calls the playback object's timer object and sets its
// "tick" callback function; called every tick increment interval.
func (p *Playback) OnTick(callback TimerCallback) {
	p.timer.OnTick(callback)
}

func (p *Playback) ClearQueue() error {
	var errs []error

	p.queueHandler.Queue().Visit(func(item queue.QueueItem) {
		userQueue, ok := item.(queue.AggregatableQueue)
		if !ok {
			return
		}

		for _, userQueueItem := range userQueue.List() {
			if err := p.ClearQueueItem(userQueue, userQueueItem); err != nil {
				errs = append(errs, err)
			}
		}
	})

	p.queueHandler.Clear()

	var errMsg string
	if len(errs) > 0 {
		errMsg = "INF SOCKET CLIENT the following errors occurred while attempting to clear the queue:"
		for _, e := range errs {
			errMsg += "\n    " + e.Error()
		}
	}
	return fmt.Errorf("%v", errMsg)
}

func (p *Playback) ClearUserQueue(userQueue queue.AggregatableQueue) {
	for _, userQueueItem := range userQueue.List() {
		p.ClearQueueItem(userQueue, userQueueItem)
	}

	userQueue.Clear()
}

func (p *Playback) GetQueue() queue.RoundRobinQueue {
	return p.queueHandler.Queue().(queue.RoundRobinQueue)
}

// PushUserQueue pushes a stream to the queue belonging to the given user
// and adds the Playback object as the parentRef to the pushed stream.
func (p *Playback) PushToQueue(userQueue queue.AggregatableQueue, s stream.Stream) error {
	if err := p.queueHandler.PushToQueue(userQueue, s); err == nil {
		// mark stream as unreapable while it is aggregated in the queue
		if !s.Metadata().AddParentRef(p) {
			log.Printf("INF SOCKET CLIENT duplicate attempt to set parent ref %q to stream %q\n", p.UUID(), s.UUID())
		}
	}
	return nil
}

// PopUserQueue pops a stream from the queue belonging to the given user
// and removes the Playback object from the popped stream's parentRef.
func (p *Playback) ClearQueueItem(userQueue queue.AggregatableQueue, qi queue.QueueItem) error {
	err := p.queueHandler.PopFromQueue(userQueue, qi)
	if err != nil {
		return err
	}

	s, ok := qi.(stream.Stream)
	if !ok {
		log.Printf("INF SOCKET CLIENT unable to remove parent ref %q from QueueItem %q: does not implement stream.Stream", p.UUID(), s.UUID())
		return nil
	}

	if !s.Metadata().RemoveParentRef(p) {
		log.Printf("INF SOCKET CLIENT unable to remove parent ref %q from stream %q\n", p.UUID(), s.UUID())
	}

	return nil
}

// GetStream returns a stream.Stream object containing current stream data
// tied to the current Playback object, or a bool (false) if there
// is no stream information currently loaded for the current Playback
func (p *Playback) GetStream() (stream.Stream, bool) {
	return p.stream, p.stream != nil
}

// SetStream receives a stream.Stream and sets it as the currently-playing stream
func (p *Playback) SetStream(s stream.Stream) {
	if p.stream != nil {
		// remove Playback object from list of current stream's refs
		p.stream.Metadata().RemoveParentRef(p)
		p.stream.Metadata().RemoveLabelledRef(p.UUID())
	}

	startedByUser, exists := s.Metadata().GetLabelledRef(p.UUID())
	if exists {
		u, ok := startedByUser.(*client.Client)
		if ok {
			p.UpdateStartedBy(u.GetUsernameOrId())
		}
	} else {
		log.Printf("INF PLAYBACK unable to find labelled client reference for room with id %v\n", p.UUID())
		p.UpdateStartedBy("<unknown>")
	}

	p.stream = s
	p.stream.Metadata().SetLastUpdated(time.Now())
	p.SetLastUpdated(time.Now())
}

// GetOrCreateStreamFromUrl receives a stream location (path, url, or unique identifier)
// and retrieves a corresponding stream.Stream, or creates a new one.
// Calls callback once a cached stream is fetched, or metadata has been fetched for a
// newly-created stream.
func (p *Playback) GetOrCreateStreamFromUrl(url string, user *client.Client, streamHandler stream.StreamHandler, callback PlaybackStreamMetadataCallback) (stream.Stream, error) {
	if s, exists := streamHandler.GetStream(url); exists {
		log.Printf("INF PLAYBACK found existing stream object with url %q, retrieving...", url)
		callback([]byte{}, false, nil)

		// determine if a labelled reference has already
		// been set for the room - only return an error
		// if the labelled ref still has the stream
		// in their queue.
		ref, exists := s.Metadata().GetLabelledRef(p.UUID())
		if exists {
			if u, ok := ref.(*client.Client); ok {
				if userQueue, userQueueExists, _ := util.GetUserQueue(u, p.GetQueue()); userQueueExists {
					exists := false
					userQueue.Visit(func(item queue.QueueItem) {
						// determine if item we are trying to reference under a new
						// user still exists under the previous user's queue.
						if item.UUID() == s.UUID() {
							exists = true
							return
						}
					})

					if exists {
						if ref.UUID() == user.UUID() {
							return nil, fmt.Errorf("error: that stream already exists in your queue")
						}
						return nil, fmt.Errorf("error: that stream has already added to the queue of another user in your room")
					}
				}
			}
		}

		// replace labelled reference for the queueing client
		// with the current playback id as the key.
		s.Metadata().SetLabelledRef(p.UUID(), user)
		return s, nil
	}

	s, err := streamHandler.NewStream(url)
	if err != nil {
		return nil, err
	}

	s.Metadata().SetCreationSource(user)

	// store queueing-user info as a labelled stream reference
	// using the Playback's id as a namespaced key
	s.Metadata().SetLabelledRef(p.UUID(), user)

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

// PlaybackStatus is a serializable schema representing a summary of information
// about the current state of the Playback.
// Implements api.ApiCodec.
type PlaybackStatus struct {
	QueueLength int          `json:"queueLength"`
	StartedBy   string       `json:"startedBy"`
	CreatedBy   string       `json:"createdBy"`
	Stream      api.ApiCodec `json:"stream"`
	TimerStatus api.ApiCodec `json:"playback"`
}

func (s *PlaybackStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

// Returns a map compatible with json types
// detailing the current playback status
func (p *Playback) GetStatus() api.ApiCodec {
	var streamCodec api.ApiCodec
	var createdBy string

	s, exists := p.GetStream()
	if exists {
		streamCodec = s.Codec()
		createdBy = s.Metadata().GetCreationSource().GetSourceName()
	}

	return &PlaybackStatus{
		QueueLength: p.GetQueue().Size(),
		StartedBy:   p.startedBy,
		CreatedBy:   createdBy,
		TimerStatus: p.timer.Status(),
		Stream:      streamCodec,
	}
}

func NewPlaybackWithAdminPicker(ns connection.Namespace, authorizer rbac.Authorizer, clientHandler client.SocketClientHandler, playbackHandler PlaybackHandler) *Playback {
	picker := NewLeastRecentAdminPicker()

	p := NewPlayback(ns)
	p.adminPicker = picker

	if err := picker.Init(ns, authorizer, clientHandler, playbackHandler); err != nil {
		log.Printf("WRN PLAYBACK ADMIN-PICKER unable to initialize admin picker for room %q: %v\n", ns.Name(), err)
	} else {
		log.Printf("INF PLAYBACK ADMIN-PICKER for room %q started\n", ns.Name())
	}

	return p
}

func NewPlayback(ns connection.Namespace) *Playback {
	if len(ns.Name()) == 0 {
		panic("A namespace with a name is required to instantiate a new playback")
	}

	return &Playback{
		name:               ns.Name(),
		timer:              NewTimer(),
		queueHandler:       queue.NewQueueHandler(queue.NewRoundRobinQueue()),
		lastUpdated:        time.Now(),
		lastAdminDeparture: time.Time{},
		state:              PLAYBACK_STATE_NOT_STARTED,
	}
}
