package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/juanvallejo/streaming-server/pkg/playback"
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
	QUEUE_USAGE       = "Usage: /" + QUEUE_NAME + " (add &lt;url&gt;|clear &lt;room|mine [url]&gt;|list &lt;mine|room&gt;|order &lt;next &lt;url&gt;|mine &lt;url newposition|0,1,2...&gt;|room &lt; url newposition|0,1,2...&gt;&gt;)"
)

var mux sync.Mutex

func (h *QueueCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	userRoom, hasRoom := user.GetRoom()
	if !hasRoom {
		log.Printf("ERR SOCKET CLIENT client with id %q (%s) attempted to control stream playback with no room assigned", user.GetId(), username)
		return "", fmt.Errorf("error: you must be in a stream to control stream playback.")
	}

	sPlayback, sPlaybackExists := playbackHandler.GetStreamPlayback(userRoom)
	if !sPlaybackExists {
		log.Printf("ERR SOCKET CLIENT unable to associate client %q (%s) in room %q with any stream playback objects", user.GetId(), username, userRoom)
		return "", fmt.Errorf("error: no stream playback is currently loaded for your room")
	}

	switch args[0] {
	case "add":
		// add a stream to the end of the queue
		url, err := getStreamUrlFromArgs(args)
		if err != nil {
			return "", err
		}

		userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
		if err != nil {
			return "", err
		}
		if !exists {
			userQueue = playback.NewAggregatableQueue(user.GetId())
			sPlayback.GetQueue().Push(userQueue)
		}

		s, err := sPlayback.GetOrCreateStreamFromUrl(url, streamHandler, func(user *client.Client, pback *playback.StreamPlayback) func([]byte, bool, error) {
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
			return "", err
		}

		userQueue.Push(s)

		err = sendQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}
		err = sendUserQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}

		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has added %q to the queue", username, url))
		return fmt.Sprintf("successfully queued %q", url), nil
	case "list":
		if len(args) < 2 {
			return "", fmt.Errorf("%v", h.usage)
		}

		if args[1] == "mine" || args[1] == "me" {
			userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
			if err != nil {
				return "", err
			}
			if !exists {
				userQueue = playback.NewAggregatableQueue(user.GetId())
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

		if len(args) < 3 {
			// clear entire queue
			if args[1] == "room" || args[1] == "all" {
				sPlayback.GetQueue().Clear()
				err := sendQueueSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}

				err = sendUserQueueSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}
				return "clearing queue....", nil
			}

			// clear a single client's queue
			if args[1] == "mine" || args[1] == "me" {
				userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
				if err != nil {
					return "", fmt.Errorf("error: %v", err)
				}
				if !exists {
					return "", fmt.Errorf("error: you cannot clear an empty queue.")
				}

				userQueue.Clear()
				err = sendQueueSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}

				err = sendUserQueueSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}
				return "clearing your queue items....", nil
			}

			return h.usage, nil
		}

		// user requested to clear a specific stream
		if args[1] == "mine" || args[1] == "me" {
			s, exists := streamHandler.GetStream(args[2])
			if !exists {
				return "", fmt.Errorf("The provided stream with id %q does not exist in your queue", args[2])
			}

			userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
			if err != nil {
				return "", fmt.Errorf("error: %v", err)
			}
			if !exists {
				return "", fmt.Errorf("error: you cannot delete an item from an empty queue")
			}

			err = userQueue.DeleteItem(s)
			if err != nil {
				return "", err
			}

			err = sendUserQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("deleting stream with url %q", s.GetStreamURL()), nil
		}
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

			userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
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

func sendQueueSyncEvent(user *client.Client, sPlayback *playback.StreamPlayback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	res := &client.Response{
		Id:   user.GetId(),
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
func sendUserQueueSyncEvent(user *client.Client, sPlayback *playback.StreamPlayback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	res := &client.Response{
		Id:   user.GetId(),
		From: username,
	}

	// find queue item with id
	userQueue, exists, err := sockutil.GetUserQueue(user, sPlayback.GetQueue())
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	if !exists {
		userQueue = playback.NewAggregatableQueue(user.GetId())
	}

	err = sockutil.SerializeIntoResponse(userQueue, &res.Extra)
	if err != nil {
		return err
	}

	user.BroadcastTo("stacksync", res)
	return nil
}

//
// queueBreadthItemIndex receives a list of QueueItems and an id.
// Returns index of QueueItem matching the given id, or a bool false.
//
// Breadth-first search variant of this implementation:
// https://play.golang.org/p/48WvSd0UaB
func queueItemIndex(id string, items []playback.QueueItem) (int, bool, error) {
	for idx, qItem := range items {
		if qItem.UUID() == id {
			return idx, true, nil
		}
	}

	return -1, false, nil
}
