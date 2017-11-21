package playback

import (
	"log"
	"time"
)

const (
	MaxStaleSPlaybackObjectDuration time.Duration = 5 * time.Minute // amount of time to wait before reaping a stale playback object
)

// PlaybackReaper is a PlaybackHandler's Garbage Collector.
// Iterates through all playback objects stored in a handler every minute
// reaping objects that are candidates for reaping and have exceeded their
// maxStalePlaybackObjectLifetime. A playback object becomes a "candidate
// for reaping"
type PlaybackReaper struct {
	// max age of a stale playbackobject.
	// a "stale" playbackobject is defined as a playback object that has not had its data
	// updated in more than 1 second.
	maxStalePlaybackObjectLifetime time.Duration
	stopChan                       chan bool
}

func (r *PlaybackReaper) Stop() {
	r.stopChan <- true
}

func (r *PlaybackReaper) Init(handler StreamPlaybackHandler) {
	go reap(r, handler, r.stopChan)
}

func reap(reaper *PlaybackReaper, handler StreamPlaybackHandler, stop chan bool) {
	for {
		for _, s := range handler.GetStreamPlaybacks() {
			if handler.IsReapable(s) && time.Now().Sub(s.GetLastUpdated()) > reaper.maxStalePlaybackObjectLifetime {
				if handler.ReapStreamPlayback(s) {
					log.Printf("INF REAPER room with name %q has become a candidate for reaping after %v. Reaping...\n", s.name, time.Now().Sub(s.GetLastUpdated()))
				}
			}
		}

		select {
		case <-stop:
			log.Printf("INF REAPER PlaybackReaper terminated.\n")
			return
		default:
		}
		time.Sleep(1 * time.Minute)
	}
}

func NewPlaybackReaper() *PlaybackReaper {
	return &PlaybackReaper{
		maxStalePlaybackObjectLifetime: MaxStaleSPlaybackObjectDuration,
		stopChan:                       make(chan bool, 1),
	}
}
