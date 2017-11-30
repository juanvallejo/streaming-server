package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/playback/queue"
	playbackutil "github.com/juanvallejo/streaming-server/pkg/playback/util"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	sockutil "github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type QueueCmd struct {
	Command
}

const (
	QUEUE_NAME        = "queue"
	QUEUE_DESCRIPTION = "control the room queue"
	QUEUE_USAGE       = "Usage: /" + QUEUE_NAME + " (migrate &lt;newQueueKey&gt;|add &lt;url&gt;|clear &lt;room|mine [url]&gt;|list &lt;mine|room&gt;|order &lt;next &lt;url&gt;|mine &lt;url newposition|0,1,2...&gt;|room &lt; url newposition|0,1,2...&gt;&gt;)"
)

var mux sync.Mutex

func (h *QueueCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.UUID()
	}

	userRoom, hasRoom := user.Namespace()
	if !hasRoom {
		log.Printf("ERR SOCKET CLIENT client with id %q (%s) attempted to control stream playback with no room assigned", user.UUID(), username)
		return "", fmt.Errorf("error: you must be in a stream to control stream playback.")
	}

	sPlayback, sPlaybackExists := playbackHandler.PlaybackByNamespace(userRoom)
	if !sPlaybackExists {
		log.Printf("ERR SOCKET CLIENT unable to associate client %q (%s) in room %q with any stream playback objects", user.UUID(), username, userRoom)
		return "", fmt.Errorf("error: no stream playback is currently loaded for your room")
	}

	switch args[0] {
	case "add":
		// add a stream to the end of the queue
		url, err := getStreamUrlFromArgs(args)
		if err != nil {
			return "", err
		}

		userQueue, exists, err := playbackutil.GetUserQueue(user, sPlayback.GetQueue())
		if err != nil {
			return "", err
		}
		if !exists {
			userQueue = queue.NewAggregatableQueue(user.UUID())
			err := sPlayback.GetQueue().Push(userQueue)
			if err != nil {
				return "", err
			}
		}

		// do not create and push stream if user queue is at its storage limit
		if userQueue.Size() >= queue.MaxAggregatableQueueItems {
			return "", queue.ErrMaxQueueSizeExceeded
		}

		s, err := sPlayback.GetOrCreateStreamFromUrl(url, user, streamHandler, func(user *client.Client, pback *playback.Playback) func([]byte, bool, error) {
			return func(data []byte, created bool, err error) {
				// if a new stream was created, sync fetched metadata with client
				if !created {
					return
				}

				err = sendQueueSyncEvent(user, sPlayback)
				if err != nil {
					log.Printf("ERR SOCKET CLIENT PLAYBACK-FETCHMETADATA-CALLBACK unable to send queue-sync event to client")
					return
				}
				err = sendUserQueueSyncEvent(user, sPlayback)
				if err != nil {
					log.Printf("ERR SOCKET CLIENT PLAYBACK-FETCHMETADATA-CALLBACK unable to send user-queue-sync event to client")
					return
				}
			}
		}(user, sPlayback))
		if err != nil {
			user.BroadcastErrorTo(err)
			return "", err
		}

		err = sPlayback.PushToQueue(userQueue, s)
		if err != nil {
			return "", err
		}

		err = sendQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}
		err = sendUserQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}

		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has added %q to the queue", username, url))

		// TODO: turn this code-block into a helper (currently used here, socket/handler.go, and cmd/stream.go)
		// if room playback state is PLAYBACK_STATE_ENDED, auto-play the next queued item (if found)
		if sPlayback.State() == playback.PLAYBACK_STATE_ENDED || sPlayback.State() == playback.PLAYBACK_STATE_NOT_STARTED {
			roomQueue := sPlayback.GetQueue()
			nextQueueItem, err := roomQueue.Next()
			if err == nil {
				nextStream, ok := nextQueueItem.(stream.Stream)
				if !ok {
					return fmt.Sprintf("successfully queued %q - unable to auto-play: queued item does not implement stream.Stream.", url), nil
				}

				sPlayback.SetStream(nextStream)
				sPlayback.Reset()

				res := &client.Response{
					Id:   user.UUID(),
					From: username,
				}

				err = sockutil.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
				if err != nil {
					return fmt.Sprintf("successfully queued %q - unable to auto-play: an error ocurred serializing status response: %v", url, err), nil
				}

				user.BroadcastAll("streamload", res)

				// play the newly loaded stream
				err := sPlayback.Play()
				if err != nil {
					return fmt.Sprintf("auto-loaded the requested queue item (%q) - unable to auto-play: %v", url, err), nil
				}

				user.BroadcastAll("streamsync", res)
				return fmt.Sprintf("successfully queued %q. Auto-playing...", url), nil
			}
		}

		return fmt.Sprintf("successfully queued %q", url), nil
	case "list":
		if len(args) < 2 {
			return "", fmt.Errorf("%v", h.usage)
		}

		if args[1] == "mine" || args[1] == "me" {
			userQueue, exists, err := playbackutil.GetUserQueue(user, sPlayback.GetQueue())
			if err != nil {
				return "", err
			}
			if !exists {
				userQueue = queue.NewAggregatableQueue(user.UUID())
			}

			status, err := userQueue.Serialize()
			if err != nil {
				return "", err
			}

			m := make(map[string]interface{})
			err = json.Unmarshal(status, &m)
			if err != nil {
				return "", err
			}

			mUser, exists := m["items"]
			if !exists {
				return "", fmt.Errorf("malformed serialized queue-stack response")
			}

			output := "Your queue:<br />" + unpackList([]interface{}{mUser}, "<br />")
			return output, nil
		}

		if args[1] == "room" || args[1] == "all" {
			status, err := sPlayback.GetQueue().Serialize()
			if err != nil {
				return "", err
			}

			m := make(map[string]interface{})
			err = json.Unmarshal(status, &m)
			if err != nil {
				return "", err
			}

			output := "Queue status:<br />" + unpackMap(m, "<br />")
			return output, nil
		}
	case "clear":
		if len(args) < 2 {
			return "", fmt.Errorf("%v", h.usage)
		}

		// clear entire queue
		if args[1] == "room" || args[1] == "all" {
			msg := "clearing queue..."

			// if 3 agrs, treat last arg as url of stream to delete
			// from the current round-robin lineup
			if len(args) > 2 {
				var itemToDelete queue.QueueItem
				found := false
				userQueueIdx := -1

				for idx, qItem := range sPlayback.GetQueue().PeekItems() {
					if qItem.UUID() == args[2] {
						itemToDelete = qItem
						found = true
						userQueueIdx = idx
						break
					}
				}
				if !found {
					return "", fmt.Errorf("unable to find item with id %v in list of upcoming streams", args[2])
				}

				queues := sPlayback.GetQueue().List()
				userQueueItem := queues[userQueueIdx]
				userQueue, ok := userQueueItem.(queue.AggregatableQueue)
				if !ok {
					return "", fmt.Errorf("error: expected user queue for user with id %q to implement playback.AggregatableQueue", userQueueItem.UUID())
				}

				err := sPlayback.ClearQueueItem(userQueue, itemToDelete)
				if err != nil {
					return "", err
				}

				msg = fmt.Sprintf("deleting stream with url %q from the queue...", args[2])
			} else {
				sPlayback.ClearQueue()
			}

			err := sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			err = sendUserQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			return msg, nil
		}

		// clear a single client's queue
		if args[1] == "mine" || args[1] == "me" {
			userQueue, exists, err := playbackutil.GetUserQueue(user, sPlayback.GetQueue())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}
			if !exists {
				return "", fmt.Errorf("error: you cannot perform this action on an empty queue.")
			}

			msg := "clearing your queue items...."

			// if 3 args, treat last arg as url of stream to delete
			if len(args) > 2 {
				s, exists := streamHandler.GetStream(args[2])
				if !exists {
					return "", fmt.Errorf("The provided stream with id %q does not exist in your queue", args[2])
				}

				err := sPlayback.ClearQueueItem(userQueue, s)
				if err != nil {
					return "", err
				}

				msg = fmt.Sprintf("deleting stream with url %q", s.GetStreamURL())
			} else {
				sPlayback.ClearUserQueue(userQueue)
			}

			err = sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			err = sendUserQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			return msg, nil
		}

		return h.usage, nil
	case "order":
		if len(args) < 3 {
			return "", fmt.Errorf("%v", h.usage)
		}

		// allow only a single client to perform an "order" operation on the queue
		mux.Lock()
		defer mux.Unlock()

		// bump item to next position in queue - relative to round-robin index
		// only applies to overall queue -- not individual stacks since idx 0
		// always means first on a stack.
		if args[1] == "next" {
			streamId := args[2]
			sourceIdx, found, err := queueItemIndex(streamId, sPlayback.GetQueue().PeekItems())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}
			if !found {
				return "", fmt.Errorf("error: source item id (%v) was not found in the queue", streamId)
			}

			// set destination index to the next index to be popped off from the queue
			destIdx := sPlayback.GetQueue().CurrentIndex()

			newOrder, err := calculateQueueOrder(sourceIdx, destIdx, sPlayback.GetQueue().Size())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}

			err = sPlayback.GetQueue().Reorder(newOrder)
			if err != nil {
				return "", fmt.Errorf("error: unable to re-order queue: %v", err)
			}

			err = sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("re-ordering queue: setting %v as the next stream in the queue...", streamId), nil
		}

		if args[1] == "room" || args[1] == "mine" {
			// if less than 4 args, interpret remaining arg as a comma delimited input representing
			// the new overall queue order: "0,2,1,3"
			if len(args) < 4 {
				return "", fmt.Errorf("%v", "unimplemented")
			}
		}

		if args[1] == "room" {
			// if we receive two extra arguments (5 total), interpret the last two args as:
			// a string containing the queue id of the item to order, and an int defining the new
			// destination for the given item id.

			streamId := args[2]
			sourceIdx, found, err := queueItemIndex(streamId, sPlayback.GetQueue().PeekItems())
			if !found {
				return "", fmt.Errorf("error: source item id (%v) was not found in the queue", streamId)
			}

			destIdx, err := strconv.Atoi(args[3])
			if err != nil {
				return "", fmt.Errorf("error: unable to convert destination item index: %v", err)
			}

			newOrder, err := calculateQueueOrder(sourceIdx, destIdx, sPlayback.GetQueue().Size())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}

			err = sPlayback.GetQueue().Reorder(newOrder)
			if err != nil {
				return "", fmt.Errorf("error: unable to re-order queue: %v", err)
			}

			err = sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("re-ordering queue: moving %v to position %v...", streamId, destIdx), nil
		}

		if args[1] == "mine" {
			streamId := args[2]

			userQueue, exists, err := playbackutil.GetUserQueue(user, sPlayback.GetQueue())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}
			if !exists {
				return "", fmt.Errorf("error: unable to re-order an empty queue")
			}

			sourceIdx, found, err := queueItemIndex(streamId, userQueue.List())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}
			if !found {
				return "", fmt.Errorf("error: source item id (%v) was not found in your queue", streamId)
			}

			destIdx, err := strconv.Atoi(args[3])
			if err != nil {
				return "", fmt.Errorf("error: unable to convert destination item index: %v", err)
			}

			newOrder, err := calculateQueueOrder(sourceIdx, destIdx, userQueue.Size())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}

			err = userQueue.Reorder(newOrder)
			if err != nil {
				return "", fmt.Errorf("error: unable to re-order your queue: %v", err)
			}

			err = sendUserQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}

			err = sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("re-ordering your queue: moving %v to position %v...", streamId, destIdx), nil
		}

		return h.usage, nil
	case "migrate":
		if len(args) < 2 {
			return h.usage, nil
		}

		fromKey := args[1]
		oldUserQueue, exists, err := playbackutil.GetQueueForId(fromKey, sPlayback.GetQueue())
		if err != nil {
			return "", fmt.Errorf("error: %v", err)
		}
		if !exists {
			return "", fmt.Errorf("error: queue with id %q does not exist. Unable to migrate queue", fromKey)
		}

		// requested queue exists, migrate items to new queue
		newQueue := queue.NewAggregatableQueue(user.UUID())
		for _, oldItem := range oldUserQueue.List() {
			newQueue.Push(oldItem)
		}
		err = sPlayback.GetQueue().Push(newQueue)
		if err != nil {
			return "", fmt.Errorf("error: unable to migrate queue: %v", err)
		}

		// delete old queue - no need to delete parentRef
		sPlayback.GetQueue().DeleteItem(oldUserQueue)

		err = sendUserQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}
		err = sendQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}

		// send user-queue-sync event to user with "fromKey" id, if still exists
		oldUser, err := clientHandler.GetClient(fromKey)
		if err == nil {
			err = sendUserQueueSyncEvent(oldUser, sPlayback)
			if err != nil {
				log.Printf("ERR SOCKET CLIENT old client (target of migrating queue) exists, but an error ocurred emitting user-queue-sync event: %v", err)
			} else {
				oldUser.BroadcastSystemMessageTo(fmt.Sprintf("user %q has migrated your queue. It is... theirs now.", username))
			}
		}
		return "migrating queue...", nil
	}

	return h.usage, nil
}

func NewCmdQueue() SocketCommand {
	return &QueueCmd{
		Command{
			name:        QUEUE_NAME,
			description: QUEUE_DESCRIPTION,
			usage:       QUEUE_USAGE,
		},
	}
}

// calculateQueueOrder receives a sourceIdx and
// a destIdx and returns a slice describing the
// new order of the queue with slice[destIdx]
// containing the value of sourceIdx.
// Returns an int slice, or an error.
func calculateQueueOrder(sourceIdx, destIdx, length int) ([]int, error) {
	// if source and destination indices are the same, no-op
	if sourceIdx == destIdx {
		return []int{}, fmt.Errorf("current source item index (%v) is equal to destination index (%v)", sourceIdx, destIdx)
	}

	newOrder := make([]int, 0, length)
	if destIdx > sourceIdx {
		for i := 0; i < length; i++ {
			if i == sourceIdx {
				continue
			}
			newOrder = append(newOrder, i)
			if i == destIdx {
				break
			}
		}
	} else {
		for i := 0; i < length; i++ {
			if i == destIdx {
				break
			}
			if i == sourceIdx {
				continue
			}
			newOrder = append(newOrder, i)
		}
	}

	return append(newOrder, sourceIdx), nil
}

func sendQueueSyncEvent(user *client.Client, sPlayback *playback.Playback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.UUID()
	}

	res := &client.Response{
		Id:   user.UUID(),
		From: username,
	}

	err := sockutil.SerializeIntoResponse(sPlayback.GetQueue(), &res.Extra)
	if err != nil {
		return err
	}

	user.BroadcastAll("queuesync", res)
	return nil
}

// sendUserQueueSyncEvent sends a queue stacksync event only to the user requesting data
func sendUserQueueSyncEvent(user *client.Client, sPlayback *playback.Playback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.UUID()
	}

	res := &client.Response{
		Id:   user.UUID(),
		From: username,
	}

	// find queue item with id
	userQueue, exists, err := playbackutil.GetUserQueue(user, sPlayback.GetQueue())
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	if !exists {
		userQueue = queue.NewAggregatableQueue(user.UUID())
	}

	err = sockutil.SerializeIntoResponse(userQueue, &res.Extra)
	if err != nil {
		return err
	}

	user.BroadcastTo("stacksync", res)
	return nil
}

// queueItemIndex receives a list of QueueItems and an id.
// Returns index of QueueItem matching the given id, or a bool false.
//
// Breadth-first search variant of this implementation:
// https://play.golang.org/p/48WvSd0UaB
func queueItemIndex(id string, items []queue.QueueItem) (int, bool, error) {
	for idx, qItem := range items {
		if qItem.UUID() == id {
			return idx, true, nil
		}
	}

	return -1, false, nil
}
