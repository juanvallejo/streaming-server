package stream

import (
	"log"
	"time"
)

const (
	MaxStaleStreamDuration time.Duration = 5 * time.Minute // amount of time to wait before reaping a stale stream
)

// StreamReaper is a StreamHandler's Garbage Collector.
// Iterates through all streams stored in a handler every minute
// reaping streams that have not been updated longer than maxStaleStreamLifetime
type StreamReaper struct {
	// max age of a stale stream.
	// a "stale" stream is defined as a stream that has not had its data
	// updated in more than 1 second.
	maxStaleStreamLifetime time.Duration
	stopChan               chan bool
}

func (r *StreamReaper) Stop() {
	r.stopChan <- true
}

func (r *StreamReaper) Init(handler StreamHandler) {
	go reap(r, handler, r.stopChan)
}

func reap(reaper *StreamReaper, handler StreamHandler, stop chan bool) {
	for {
		for _, s := range handler.GetStreams() {
			if time.Now().Sub(s.Metadata().GetLastUpdated()) > reaper.maxStaleStreamLifetime {
				if handler.ReapStream(s) {
					log.Printf("INF stream with url %q has become a candidate for reaping after %v. Reaping...\n", s.GetStreamURL(), time.Now().Sub(s.Metadata().GetLastUpdated()))
				}
			}
		}

		select {
		case <-stop:
			log.Printf("INF StreamReaper terminated.\n")
			return
		default:
		}
		time.Sleep(1 * time.Minute)
	}
}

func NewStreamReaper() *StreamReaper {
	return &StreamReaper{
		maxStaleStreamLifetime: MaxStaleStreamDuration,
		stopChan:               make(chan bool, 1),
	}
}
