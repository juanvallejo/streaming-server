package playback

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// PlaybackQueue performs "queue" operations on a list of Stream items.
type PlaybackQueue interface {
	// Pop round-robins accross all registered QueueStacks, calling PopStream for
	// each one, per call, in the order in which they were added. If calling
	// PopStream on a QueueStack causes that QueueStack's internal stack to become
	// empty, the QueueStack is removed from the Queue. When a QueueStack is removed
	// from the Queue, the round-robin count is then adjusted by -1, due to all
	// indices greater than the deleted QueueStack being decreased by 1.
	Pop() (stream.Stream, error)
	// Push receives a QueueStack id and a stream and appends the stream
	// to the stack of the QueueStack. If a QueueStack is not found,
	// a new one is created with the given id.
	Push(string, stream.Stream)
	// Size returns the total count of streams in all QueueStacks in the Queue
	Size() int
	// Length returns the total count of QueueStacks in the Queue
	Length() int
	// StackLength receives a QueueStack id and returns its length or an error
	// if a stack by that id is not found.
	StackLength(string) (int, error)
	// Clear deletes all of the items in the queue
	Clear()
	// ClearStack receives a stackId and deletes the QueueStack.
	// If no QueueStack exists by the given stackId, a no-op takes place.
	ClearStack(string)
	// ClearStackItem receives a stackId and a stream and deletes the given
	// stream.Stream from the stack. If the given stream.Stream does not
	// exist in the stack, a no-op takes place. If deleting a StackItem
	// results in an empty QueueStack, the QueueStack is then cleared from
	// the Queue.
	ClearStackItem(string, stream.Stream)
	// Status returns a top-level serializable view of the queue
	// in order, starting from QueueStack[round-robin-index]
	Status() api.ApiCodec
	// StackStatus returns a breadth-level and depth-level serializable view
	// of the queue. Requires a stack id to be passed
	StackStatus(string) api.ApiCodec
	// Reorder is a concurrency-safe method that receives an array of integers
	// representing the new index order of QueueStacks. If the length N of new
	// indices is greater than total amount of QueueStacks, the remaining new
	// indices are ignored. If the length N of new indices is less than total
	// QueueStacks, then a total of N QueueStacks will be re-ordered. The remaining
	// QueueStacks are not affected and are pushed (in their original order) after
	// the newly affected affected items.
	//
	// Example:
	//
	//   Existing queue order: [A, B, C, D]
	//   New queue order:      [3, 1]
	//   Resulting order:      [D, B, A, C]
	//
	//   Existing queue order: [A, B, C, D]
	//   New queue order:      [3, 1, 2, 0]
	//   Resulting order:      [D, B, C, A]
	//
	// Returns an error if a list of new indices contains duplicate indices, or if any
	// provided new index is greater than the length of the original QueueStacks list.
	Reorder([]int) error
	// ReorderStack receives an array of integers representing the new index order of
	// QueueStack items. Works the same as Reorder() but at the individual QueueStack level.
	ReorderStack(string, []int) error
	// ItemIndex receives a stream.Stream id and returns its current index in the queue
	// or a boolean false if a stream.Stream by that id is not found.
	ItemIndex(string) (int, bool)
	// StackItemIndex receives a QueueStack id and a stream.Stream id and returns the
	// stream.Stream's current index in the QueueStack or a boolean false if a
	// stream.Stream is not found by that id in the specified stack. An error is returned
	// if a QueueStack is not found by the given stack id.
	StackItemIndex(string, string) (int, bool, error)
	// NextIndex returns the current round-robin index - index of QueueStack to be
	// popped by the next call to Pop()
	NextIndex() int
}

// QueueStack maps a unique id to a stack of stream.Streams
type QueueStack struct {
	id      string
	streams []stream.Stream
}

// PushStream receives a stream.Stream and appends it to
// an internal stack of stream.Stream.
func (qi *QueueStack) PushStream(s stream.Stream) {
	qi.streams = append(qi.streams, s)
}

// ClearStream receives a stream.Stream and deletes it from
// an internal stack of stream.Stream.
// If a given stream is not found, a no-op takes place.
func (qi *QueueStack) ClearStream(s stream.Stream) {
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
func (qi *QueueStack) PopStream() (stream.Stream, error) {
	if len(qi.streams) == 0 {
		return nil, fmt.Errorf("no streams found for queue-item with id %q (%v)", qi.id, len(qi.streams))
	}

	item := qi.streams[0]
	qi.streams = qi.streams[1:len(qi.streams)]
	return item, nil
}

func NewQueueStack(id string) *QueueStack {
	return &QueueStack{
		id:      id,
		streams: []stream.Stream{},
	}
}

// Queue implements PlaybackQueue.
type Queue struct {
	items     []*QueueStack
	itemsById map[string]*QueueStack

	mux sync.Mutex

	// count used to round-robin the queue for each QueueStack
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

	// remove QueueStack if empty
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
		qi = NewQueueStack(id)
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

func (q *Queue) StackLength(stackId string) (int, error) {
	stack, exists := q.itemsById[stackId]
	if !exists {
		return 0, fmt.Errorf("stack with id %v not found in the queue", stackId)
	}

	return len(stack.streams), nil
}

func (q *Queue) Clear() {
	q.items = []*QueueStack{}
	q.itemsById = make(map[string]*QueueStack)
	q.rrCount = 0
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

func (q *Queue) NextIndex() int {
	return q.rrCount
}

func (q *Queue) StackItemIndex(stackId, streamId string) (int, bool, error) {
	stack, exists := q.itemsById[stackId]
	if !exists {
		return -1, false, fmt.Errorf("stack with id %v not found in the queue", stackId)
	}

	for idx, s := range stack.streams {
		if s.GetUniqueIdentifier() == streamId {
			return idx, true, nil
		}
	}

	return -1, false, nil
}

func (q *Queue) ItemIndex(id string) (int, bool) {
	for idx, item := range q.items {
		headItem := item.streams[0]
		if headItem.GetUniqueIdentifier() == id {
			return idx, true
		}
	}

	return -1, false
}

// tests https://play.golang.org/p/0iL0wdLnvI
func (q *Queue) Reorder(newOrder []int) error {
	if len(newOrder) == 0 {
		return nil
	}

	q.mux.Lock()
	defer q.mux.Unlock()

	seen := make(map[int]bool)
	newQueueStackList := make([]*QueueStack, 0, len(q.items))

	for idx, newPosition := range newOrder {
		// stop iterating if we exceed length of existing QueueStacks
		if idx >= len(q.items) {
			break
		}

		// if newPosition exceeds length of existing QueueStacks, error
		if newPosition >= len(q.items) {
			return fmt.Errorf("error: queue re-order index out of range: %v", newPosition)
		}

		if _, exists := seen[newPosition]; exists {
			return fmt.Errorf("error: duplicate queue re-order index: %v", newPosition)
		}

		newQueueStackList = append(newQueueStackList, q.items[newPosition])
		seen[newPosition] = true
	}

	// there are still items left to copy from original queue
	if len(q.items) > len(newOrder) {
		for idx, origItem := range q.items {
			if _, copied := seen[idx]; copied {
				continue
			}

			newQueueStackList = append(newQueueStackList, origItem)
		}
	}

	q.items = newQueueStackList
	return nil
}

func (q *Queue) ReorderStack(stackId string, newOrder []int) error {
	stack, exists := q.itemsById[stackId]
	if !exists {
		return fmt.Errorf("stack with id %v not found in queue", stackId)
	}

	if len(newOrder) == 0 {
		return nil
	}

	q.mux.Lock()
	defer q.mux.Unlock()

	seen := make(map[int]bool)
	newStackList := make([]stream.Stream, 0, len(stack.streams))

	for idx, newPosition := range newOrder {
		// stop iterating if we exceed length of existing QueueStack items
		if idx >= len(stack.streams) {
			break
		}

		// if newPosition exceeds length of existing QueueStacks, error
		if newPosition >= len(stack.streams) {
			return fmt.Errorf("error: re-order index out of range: %v", newPosition)
		}

		if _, exists := seen[newPosition]; exists {
			return fmt.Errorf("error: duplicate re-order index: %v", newPosition)
		}

		newStackList = append(newStackList, stack.streams[newPosition])
		seen[newPosition] = true
	}

	// there are still items left to copy from original stack
	if len(stack.streams) > len(newOrder) {
		for idx, origItem := range stack.streams {
			if _, copied := seen[idx]; copied {
				continue
			}

			newStackList = append(newStackList, origItem)
		}
	}

	stack.streams = newStackList
	return nil
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

// StackStatus is a serializable schema representing a breadth and depth state of the queue.
type StackStatus struct {
	// Items is a slice containing all items in a QueueStack
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
		stack = &QueueStack{
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
		items:     []*QueueStack{},
		itemsById: make(map[string]*QueueStack),
	}
}
