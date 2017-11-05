package util

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/playback/queue"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
)

// GetUserQueue receives a playback.RoundRobinQueue and a
// client.Client and attempts to find an aggregated queue
// matching the client's id.
// Returns an error if a queue is found but is not aggregatable.
func GetUserQueue(user *client.Client, rQueue queue.RoundRobinQueue) (queue.AggregatableQueue, bool, error) {
	return GetQueueForId(user.UUID(), rQueue)
}

// GetQueueForId receives a playback.RoundRobinQueue and a
// unique id and attempts to find an aggregated queue
// matching the given id.
// Returns an error if a queue is found but is not aggregatable.
func GetQueueForId(id string, rQueue queue.RoundRobinQueue) (queue.AggregatableQueue, bool, error) {
	for _, q := range rQueue.List() {
		if q.UUID() == id {
			userQueue, ok := q.(queue.AggregatableQueue)
			if !ok {
				return nil, false, fmt.Errorf("expected user queue to implement playback.AggregatableQueue")
			}

			return userQueue, true, nil
		}
	}

	// queue not found, return empty one
	return nil, false, nil
}
