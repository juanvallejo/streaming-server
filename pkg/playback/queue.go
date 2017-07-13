package playback

import (
	"encoding/json"
	"fmt"
	"log"

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
	// Clear deletes all of the items in the queue
	Clear()
	// ClearStack receives a stackId and deletes the QueueItem.
	// If no QueueItem exists by the given stackId, a no-op takes place.
	ClearStack(string)
	// ClearStackItem receives a stackId and a stream and deletes the given
	// stream.Stream from the stack. If the given stream.Stream does not
	// exist in the stack, a no-op takes place.
	ClearStackItem(string, stream.Stream)
	// Status returns a top-level serializable view of the queue
	// in order, starting from QueueItem[round-robin-index]
	Status() api.ApiCodec
	// StackStatus returns a breath-level and depth-level serializable view
	// of the queue. Requires a stack id to be passed
	StackStatus(string) api.ApiCodec
}

// A queue item maps a unique id to a stack of stream.Streams
type QueueItem struct {
	id      string
	streams []stream.Stream
}

// PushStream receives a stream.Stream and appends it to
// an internal stack of stream.Stream.
func (qi *QueueItem) PushStream(s stream.Stream) {
	qi.streams = append(qi.streams, s)
}

// ClearStream receives a stream.Stream and deletes it from
// an internal stack of stream.Stream.
// If a given stream is not found, a no-op takes place.
func (qi *QueueItem) ClearStream(s stream.Stream) {
	idx := -1
	for i, v := range qi.streams {
		if v.GetUniqueIdentifier() == s.GetUniqueIdentifier() {
			idx = i
			break
		}
	}

	if idx >= 0 {
		qi.streams = append(qi.streams[0:idx], qi.streams[idx+1:len(qi.streams)]...)
		return
	}

	log.Printf("WRN PLAYBACK QUEUE no queue-stack stream found with id %s", s.GetUniqueIdentifier())
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

func (q *Queue) Clear() {
	q.items = []*QueueItem{}
	q.itemsById = make(map[string]*QueueItem)
}

func (q *Queue) ClearStack(stackId string) {
	if stack, exists := q.itemsById[stackId]; exists {
		delete(q.itemsById, stack.id)

		idx := -1
		for i, v := range q.items {
			if v.id == stack.id {
				idx = i
				break
			}
		}

		// if item index was found, delete and adjust
		// round-robin count. If rrCount is >= total
		// length of queue after stack deletion, set
		// rrCount to 0 (next item wraps back to first).
		// If rrCount was
		if idx >= 0 {
			q.items = append(q.items[0:idx], q.items[idx+1:len(q.items)]...)
			if q.rrCount >= len(q.items) {
				q.rrCount = 0
			} else if q.rrCount < idx {
				q.rrCount--
			}

			if q.rrCount < 0 {
				q.rrCount = 0
			}
		}

		return
	}

	log.Printf("WRN PLAYBACK QUEUE no stack found with id %s", stackId)
}

func (q *Queue) ClearStackItem(stackId string, s stream.Stream) {
	if stack, exists := q.itemsById[stackId]; exists {
		stack.ClearStream(s)

		// if removing a stream results in an empty stack, delete the stack
		if len(stack.streams) == 0 {
			q.ClearStack(stack.id)
		}

		return
	}

	log.Printf("WRN PLAYBACK QUEUE no stack found with id %s... Not removing stack item %q", stackId, s.GetUniqueIdentifier())
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
	// Items is a slice containing all items in a QueueItem
	Items []stream.Stream `json:"items"`
}

func (s *StackStatus) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (q *Queue) StackStatus(stackId string) api.ApiCodec {
	stack, exists := q.itemsById[stackId]
	if !exists {
		log.Printf("WRN PLAYBACK QUEUE no stack found with id %s", stackId)
		stack = &QueueItem{
			id:      stackId,
			streams: []stream.Stream{},
		}
	}

	return &StackStatus{
		Items: stack.streams,
	}
}

func NewQueue() PlaybackQueue {
	return &Queue{
		items:     []*QueueItem{},
		itemsById: make(map[string]*QueueItem),
	}
}
