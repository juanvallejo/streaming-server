package playback

import (
	"encoding/json"
	"fmt"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// PlaybackQueue performs "queue" operations on a list of Stream items.
type PlaybackQueue interface {
	// Pop round-robins accross all registered QueueItems, calling PopStream for
	// each one, per call, in the order in which they were added. If calling
	// PopStream on a QueueItem causes that QueueItem's internal stack to become
	// empty, the QueueItem is removed from the Queue. When a QueueItem is removed
	// from the Queue, the round-robin count is then adjusted by -1, due to all
	// indices greater than the deleted QueueItem being decreased by 1.
	Pop() (stream.Stream, error)
	// Push receives a QueueItem id and a stream and appends the stream
	// to the stack of the QueueItem. If a QueueItem is not found,
	// a new one is created with the given id.
	Push(string, stream.Stream)
	// Size returns the total count of streams in all QueueItems in the Queue
	Size() int
	// Length returns the total count of QueueItems in the Queue
	Length() int
	// Status returns a top-level serializable view of the queue
	// in order, starting from QueueItem[round-robin-index]
	Status() api.ApiCodec
	// StackStatus returns a breath-level and depth-level serializable view
	// of the queue. Requires a stack id to be passed
	StackStatus(string) (api.ApiCodec, error)
}

// A queue item maps a unique id to a stack of stream.Streams
type QueueItem struct {
	id      string
	streams []stream.Stream
}

// PushStream receives a pointer to a stream.Stream and appends it to
// an internal stack of stream.Streams.
func (qi *QueueItem) PushStream(s stream.Stream) {
	qi.streams = append(qi.streams, s)
}

// PopStream returns the first item in the stack of stream.Streams, or
// an error if the stack is empty.
func (qi *QueueItem) PopStream() (stream.Stream, error) {
	if len(qi.streams) == 0 {
		return nil, fmt.Errorf("no streams found for queue-item with id %q (%v)", qi.id, len(qi.streams))
	}

	item := qi.streams[0]
	qi.streams = qi.streams[1:len(qi.streams)]
	return item, nil
}

func NewQueueItem(id string) *QueueItem {
	return &QueueItem{
		id:      id,
		streams: []stream.Stream{},
	}
}

// Queue implements PlaybackQueue.
type Queue struct {
	items     []*QueueItem
	itemsById map[string]*QueueItem

	// count used to round-robin the queue for each QueueItem
	rrCount int
}

func (q *Queue) Pop() (stream.Stream, error) {
	if len(q.items) == 0 {
		return nil, fmt.Errorf("there are no items in the queue.")
	}

	qi := q.items[q.rrCount]
	s, err := qi.PopStream()
	if err != nil {
		return nil, err
	}

	// remove QueueItem if empty
	if len(qi.streams) == 0 {
		q.items = append(q.items[0:q.rrCount], q.items[q.rrCount+1:len(q.items)]...)
		delete(q.itemsById, qi.id)
		q.rrCount--
	}

	q.rrCount++
	if q.rrCount >= len(q.items) {
		q.rrCount = 0
	}
	return s, nil
}

func (q *Queue) Push(id string, s stream.Stream) {
	qi, exists := q.itemsById[id]
	if !exists {
		qi = NewQueueItem(id)
		q.items = append(q.items, qi)
		q.itemsById[id] = qi
	}

	qi.PushStream(s)
}

func (q *Queue) Size() int {
	size := 0
	for _, i := range q.items {
		size += len(i.streams)
	}
	return size
}

func (q *Queue) Length() int {
	return len(q.items)
}

// QueueStatus is a serializable schema representing the top-level state of the queue.
type QueueStatus struct {
	// Items is a slice containing the first item in each queue-item stack
	Items []stream.Stream `json:"items"`
}

func (s *QueueStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (q *Queue) Status() api.ApiCodec {
	items := []stream.Stream{}
	for _, i := range q.items {
		items = append(items, i.streams[0])
	}

	// order items by current round-robin count
	items = append(items[q.rrCount:], items[0:q.rrCount]...)

	return &QueueStatus{
		Items: items,
	}
}

// StackStatus is a serializable schema representing a breath and depth state of the queue.
type StackStatus struct {
	// QueueItems is a slice containing the first item in each queue-item stack
	QueueItems []stream.Stream `json:"queueItems"`
	// StackItems is a slice containing all of the items in the specified stackId
	StackItems []stream.Stream `json:"stackItems"`
}

func (s *StackStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (q *Queue) StackStatus(stackId string) (api.ApiCodec, error) {
	stack, exists := q.itemsById[stackId]
	if !exists {
		return nil, fmt.Errorf("stack with id %q does not exist", stackId)
	}

	items := []stream.Stream{}
	for _, i := range q.items {
		items = append(items, i.streams[0])
	}

	// order items by current round-robin count
	items = append(items[q.rrCount:], items[0:q.rrCount]...)

	return &StackStatus{
		QueueItems: items,
		StackItems: stack.streams,
	}, nil
}

func NewQueue() PlaybackQueue {
	return &Queue{
		items:     []*QueueItem{},
		itemsById: make(map[string]*QueueItem),
	}
}
