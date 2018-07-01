package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
)

const (
	MaxAggregatableQueueItems = 20
)

var (
	ErrNoItemsInQueue       = errors.New("there are no items in the queue")
	ErrNoSuchQueueStr       = "no queue found with id %v"
	ErrMaxQueueSizeExceeded = fmt.Errorf("you cannot store more than %v items in your queue.", MaxAggregatableQueueItems)
)

// TODO: break this file out into its own "queue" package
// have a separate file for the controller.
type QueueHandler interface {
	// Clear calls the handled queue's Clear method
	Clear()
	// PopFromQueue receives an AggregatableQueue
	// and a QueueItem belonging to the AggregatableQueue and
	// attempts to pop that item from the AggregatableQueue.
	// If at least one parent ref is received, it is removed
	// from the list of the passed QueueItem's parentRef list.
	PopFromQueue(AggregatableQueue, QueueItem) error
	// PushToQueue receives an AggregatableQueue and a QueueItem
	// and pushes the given QueueItem to the AggregatableQueue.
	PushToQueue(AggregatableQueue, QueueItem) error
	// GetQueue returns the handled queue
	Queue() Queue
}

// QueueHandlerSpec implements QueueHandler
type QueueHandlerSpec struct {
	queue Queue
}

func (h *QueueHandlerSpec) Clear() {
	h.queue.Clear()
}

func (h *QueueHandlerSpec) PopFromQueue(aggQueue AggregatableQueue, item QueueItem) error {
	rrQueue, ok := h.queue.(RoundRobinQueue)
	if !ok {
		return fmt.Errorf("attempt to PopFromQueue on a handled queue that does not implement a RoundRobinQueue")
	}

	return rrQueue.DeleteFromQueue(aggQueue, item)
}

func (h *QueueHandlerSpec) PushToQueue(aggQueue AggregatableQueue, item QueueItem) error {
	return aggQueue.Push(item)
}

func (h *QueueHandlerSpec) Queue() Queue {
	return h.queue
}

func NewQueueHandler(queue Queue) QueueHandler {
	return &QueueHandlerSpec{
		queue: queue,
	}
}

type QueueVisitor func(QueueItem) bool

// Queue performs collection operations on a fifo structure
type Queue interface {
	// Clear empties all QueueItems in the queue
	Clear()
	// DeleteItem deletes the given QueueItem from the queue.
	// Returns an error if QueueItem is not found
	DeleteItem(QueueItem) error
	// List returns a slice of aggregated QueueItems
	List() []QueueItem
	// Pop pops the first QueueItem in the queue.
	// Returns the popped QueueItem or an error.
	Pop() (QueueItem, error)
	// Push appends a QueueItem to the queue
	// a new one is created with the given id.
	// Returns error if item could not be pushed.
	Push(QueueItem) error
	// Set replaces its internal list of QueueItems with a received slice of QueueItems
	Set([]QueueItem)
	// Size returns the total amount of QueueItems in the queue
	Size() int
	// Visit visits each QueueItem aggregated by the RoundRobinQueue and
	// passes each item to the RoundRobinQueueVisitor.
	Visit(QueueVisitor)
}

type ReorderableQueue interface {
	Queue

	// Lock locks the ReorderableQueue mutex
	Lock()
	// Unlock unlocks the ReorderableQueue mutex
	Unlock()
	// Reorder is a concurrency-safe method that receives an array of integers
	// representing the new index order of QueueItems. If the length N of new
	// indices is greater than total amount of QueueItems, the remaining new
	// indices are ignored. If the length N of new indices is less than total
	// QueueItems, then a total of N QueueItems will be re-ordered. The remaining
	// QueueItems are not affected and are pushed (in their original order) after
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
	// provided index is greater than the size of the Queue.
	Reorder([]int) error
}

// SerializableQueue represents a queue that can be handled by a rest client
type SerializableQueue interface {
	Queue
	api.ApiCodec
}

// RoundRobinQueue aggregates a collection of Queues and steps through
// them in fifo order.
type RoundRobinQueue interface {
	api.ApiCodec
	ReorderableQueue

	// CurrentIndex returns the current round-robin index
	CurrentIndex() int
	// DeleteFromQueue receives an aggregated queue within the round-robin
	// queue and attempts to delete a QueueItem from it.
	DeleteFromQueue(Queue, QueueItem) error
	// Next fetches the Queue at the current round-robin
	// index and pops its first QueueItem.
	// If popping a QueueItem results in an empty Queue,
	// that Queue is removed from the aggregated Queues.
	// Returns the popped QueueItem or an error.
	Next() (QueueItem, error)
	// PeekItems returns a slice containing the first item
	// from each aggregated QueueItem in the queue.
	PeekItems() []QueueItem
}

// AggregatableQueue is a queue that can be aggregated as a QueueItem
// and implements ReorderableQueue
type AggregatableQueue interface {
	api.ApiCodec
	QueueItem
	ReorderableQueue
}

// QueueItem represents internal queue storage with a unique identifier
type QueueItem interface {
	UUID() string
}

// QueueItemSchema implements Queue and QueueItem
type QueueItemSchema struct {
	id string
}

func (qi *QueueItemSchema) UUID() string {
	return qi.id
}

func NewQueueItem(id string) QueueItem {
	if len(id) == 0 {
		log.Panic("attempt to create QueueItem with empty id")
	}

	return &QueueItemSchema{
		id: id,
	}
}

// QueueSchema implements Queue
type QueueSchema struct {
	Items []QueueItem `json:"items"`

	mux sync.Mutex
}

func (q *QueueSchema) Clear() {
	q.Items = []QueueItem{}
}

func (q *QueueSchema) DeleteItem(item QueueItem) error {
	idx := -1
	for i, v := range q.Items {
		if v.UUID() == item.UUID() {
			idx = i
			break
		}
	}

	if idx >= 0 {
		q.Items = append(q.Items[0:idx], q.Items[idx+1:len(q.Items)]...)
		return nil
	}

	return fmt.Errorf("the item with id %q was not found in the queue", item.UUID())
}

func (q *QueueSchema) List() []QueueItem {
	return q.Items
}

func (q *QueueSchema) Pop() (QueueItem, error) {
	if len(q.Items) == 0 {
		return nil, ErrNoItemsInQueue
	}

	q.mux.Lock()
	defer q.mux.Unlock()

	item := q.Items[0]
	q.Items = q.Items[1:]
	return item, nil
}

func (q *QueueSchema) Push(item QueueItem) error {
	q.Items = append(q.Items, item)
	return nil
}

func (q *QueueSchema) Set(items []QueueItem) {
	q.Items = items
}

func (q *QueueSchema) Size() int {
	return len(q.Items)
}

func (q *QueueSchema) Visit(visitor QueueVisitor) {
	for _, queueItem := range q.Items {
		if !visitor(queueItem) {
			break
		}
	}
}

func NewQueue() Queue {
	return &QueueSchema{
		Items: []QueueItem{},
	}
}

// ReorderableQueueSchema implements ReorderableQueue
type ReorderableQueueSchema struct {
	Queue
	mux sync.Mutex
}

func (q *ReorderableQueueSchema) Lock() {
	q.mux.Lock()
}

func (q *ReorderableQueueSchema) Unlock() {
	q.mux.Unlock()
}

func (q *ReorderableQueueSchema) Reorder(newOrder []int) error {
	if len(newOrder) == 0 {
		return nil
	}

	q.Lock()
	defer q.Unlock()

	items := q.List()
	seen := make(map[int]bool)
	newQueueItemList := make([]QueueItem, 0, q.Size())

	for idx, newPosition := range newOrder {
		// stop iterating if we exceed length of existing QueueStacks
		if idx >= q.Size() {
			break
		}

		// if newPosition exceeds length of existing QueueStacks, error
		if newPosition >= q.Size() {
			return fmt.Errorf("error: queue re-order index out of range: %v", newPosition)
		}

		if _, exists := seen[newPosition]; exists {
			return fmt.Errorf("error: duplicate queue re-order index: %v", newPosition)
		}

		newQueueItemList = append(newQueueItemList, items[newPosition])
		seen[newPosition] = true
	}

	// there are still items left to copy from original queue
	if q.Size() > len(newOrder) {
		for idx, origItem := range q.List() {
			if _, copied := seen[idx]; copied {
				continue
			}

			newQueueItemList = append(newQueueItemList, origItem)
		}
	}

	q.Set(newQueueItemList)
	return nil
}

func NewReorderableQueue() ReorderableQueue {
	return &ReorderableQueueSchema{
		Queue: NewQueue(),
	}
}

// AggregatableQueueSchema implements AggregatableQueue,
type AggregatableQueueSchema struct {
	ReorderableQueue
	QueueItem
}

func (q *AggregatableQueueSchema) Serialize() ([]byte, error) {
	b, err := json.Marshal(&QueueSchema{
		Items: q.List(),
	})
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func (q *AggregatableQueueSchema) Push(item QueueItem) error {
	if q.Size() >= MaxAggregatableQueueItems {
		return ErrMaxQueueSizeExceeded
	}

	return q.ReorderableQueue.Push(item)
}

func NewAggregatableQueue(id string) AggregatableQueue {
	if len(id) == 0 {
		log.Panic("attempt to create QueueItem with empty id")
	}

	return &AggregatableQueueSchema{
		ReorderableQueue: NewReorderableQueue(),
		QueueItem:        NewQueueItem(id),
	}
}

// RoundRobinQueueSchema implements RoundRobinQueue
type RoundRobinQueueSchema struct {
	ReorderableQueue

	itemsById map[string]AggregatableQueue
	mux       sync.Mutex

	// count used to round-robin the queue for each QueueItem
	rrCount int
}

func (q *RoundRobinQueueSchema) Clear() {
	for _, i := range q.itemsById {
		agg, ok := i.(AggregatableQueue)
		if !ok {
			continue
		}
		agg.Clear()
	}

	q.ReorderableQueue.Clear()
	q.itemsById = make(map[string]AggregatableQueue)
	q.rrCount = 0
}

func (q *RoundRobinQueueSchema) Visit(visitor QueueVisitor) {
	for _, i := range q.itemsById {
		agg, ok := i.(AggregatableQueue)
		if !ok {
			continue
		}
		if !visitor(agg) {
			break
		}
	}
}

func (q *RoundRobinQueueSchema) Push(item QueueItem) error {
	newQueue, ok := item.(AggregatableQueue)
	if !ok {
		return fmt.Errorf("expected RoundRobinQueue push item to implement AggregatableQueue")
	}

	q.Lock()
	defer q.Unlock()

	id := newQueue.UUID()
	existingQueue, exists := q.itemsById[id]
	if exists {
		// if agg queue exists, append items from new queue to existing one
		for _, newItem := range newQueue.List() {
			existingQueue.Push(newItem)
		}
		return nil
	}

	q.itemsById[newQueue.UUID()] = newQueue

	// if not exists, simply push entire queue
	// to the "end" relative to round-robin index
	if q.rrCount > 0 {
		newItems := make([]QueueItem, 0, q.Size()+1)
		for idx, i := range q.List() {
			if idx == q.rrCount {
				newItems = append(newItems, item)
			}

			newItems = append(newItems, i)
		}

		// increase round-robin count to account
		// for newly added item behind item[rrCount]
		q.rrCount++
		q.Set(newItems)

		return nil
	}

	q.ReorderableQueue.Push(item)
	return nil
}

func (q *RoundRobinQueueSchema) CurrentIndex() int {
	return q.rrCount
}

func (q *RoundRobinQueueSchema) DeleteItem(queue QueueItem) error {
	q.Lock()
	defer q.Unlock()

	if qItem, exists := q.itemsById[queue.UUID()]; exists {
		idx := -1
		for i, v := range q.List() {
			if v.UUID() == qItem.UUID() {
				idx = i
				break
			}
		}

		// if queue index was found, delete and adjust
		// round-robin count. If rrCount is >= total
		// length of queue after stack deletion, set
		// rrCount to 0 (next item wraps back to first).
		// If deleted QueueItem index is less than rrCount,
		// decrease rrCount by one to "pull" items back.
		if idx >= 0 {
			q.ReorderableQueue.DeleteItem(queue)
			if idx < q.rrCount {
				q.rrCount--
			}
			if q.rrCount < 0 {
				q.rrCount = 0
			}
			if q.rrCount >= q.Size() {
				q.rrCount = 0
			}
		}

		delete(q.itemsById, qItem.UUID())
		return nil
	}

	return fmt.Errorf(ErrNoSuchQueueStr, queue.UUID())
}

func (q *RoundRobinQueueSchema) DeleteFromQueue(queue Queue, qItem QueueItem) error {
	// if not aggregatable queue, skip
	aggQueue, ok := queue.(AggregatableQueue)
	if !ok {
		return nil
	}

	err := aggQueue.DeleteItem(qItem)
	if err != nil {
		return err
	}

	if aggQueue.Size() == 0 {
		err = q.DeleteItem(aggQueue)
	}

	return err
}

func (q *RoundRobinQueueSchema) Next() (QueueItem, error) {
	if q.Size() == 0 {
		return nil, ErrNoItemsInQueue
	}

	qItems := q.List()
	qItem := qItems[q.rrCount]
	aggQueue, ok := qItem.(AggregatableQueue)
	if !ok {
		return nil, fmt.Errorf("expected QueueItem at round-robin count %q to implement AggregatableQueue", q.rrCount)
	}

	// get next queue - if empty,
	// skip and try again
	if aggQueue.Size() == 0 {
		err := q.DeleteItem(aggQueue)
		if err != nil {
			return nil, err
		}
		return q.Next()
	}

	poppedItem, err := aggQueue.Pop()
	if err != nil {
		return nil, err
	}

	// remove Queue if empty
	if aggQueue.Size() == 0 {
		q.ReorderableQueue.DeleteItem(aggQueue)
		delete(q.itemsById, aggQueue.UUID())
		q.rrCount--
	}

	q.rrCount++
	if q.rrCount >= q.Size() {
		q.rrCount = 0
	}
	return poppedItem, nil
}

func (q *RoundRobinQueueSchema) PeekItems() []QueueItem {
	items := []QueueItem{}
	for _, queue := range q.List() {
		aggQueue, ok := queue.(AggregatableQueue)
		if !ok {
			continue
		}

		aggQueueItems := aggQueue.List()
		if len(aggQueueItems) > 0 {
			items = append(items, aggQueueItems[0])
		}
	}

	return items
}

func (q *RoundRobinQueueSchema) Serialize() ([]byte, error) {
	items := q.PeekItems()

	// sort items by round-robin index
	b, err := json.Marshal(&QueueSchema{
		Items: append(items[q.rrCount:], items[0:q.rrCount]...),
	})
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func NewRoundRobinQueue() RoundRobinQueue {
	return &RoundRobinQueueSchema{
		ReorderableQueue: NewReorderableQueue(),

		itemsById: make(map[string]AggregatableQueue),
	}
}
