package playback

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// PlaybackQueue performs "queue" operations on a list of Stream items.
type PlaybackQueue interface {
	// Pop removes the first item from the queue and returns that item.
	// If the queue had no items, an error is returned.
	Pop() (*stream.Stream, error)
	// Push adds a stream to the end of the queue
	Push(stream.Stream)
}

// Queue implements PlaybackQueue.
type Queue struct {
	items []stream.Stream
}

func (q *Queue) Pop() (*stream.Stream, error) {
	if len(q.items) == 0 {
		return nil, fmt.Errorf("there are no items in the queue.")
	}

	item := &(q.items[0])
	q.items = q.items[1:len(q.items)]
	return item, nil
}

func (q *Queue) Push(s stream.Stream) {
	q.items = append(q.items, s)
}

func NewQueue() *Queue {
	return &Queue{
		items: []stream.Stream{},
	}
}
