package cmd

import (
	"fmt"
	"log"

	"encoding/json"

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
	QUEUE_USAGE       = "Usage: /" + QUEUE_NAME + " (add &lt;url&gt;|clear &lt;room|mine [url]&gt;|list &lt;mine|room&gt;)"
)

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

		err = sPlayback.QueueStreamFromUrl(url, user, streamHandler)
		if err != nil {
			return "", err
		}

		err = sendQueueSyncEvent(user, sPlayback)
		if err != nil {
			return "", err
		}

		err = sendStackSyncEvent(user, sPlayback)
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
			status, err := sPlayback.GetQueue().StackStatus(user.GetId()).Serialize()
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
			status, err := sPlayback.GetQueueStatus().Serialize()
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

				err = sendStackSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}
				return "clearing queue....", nil
			}

			// clear a single client's queue
			if args[1] == "mine" || args[1] == "me" {
				sPlayback.GetQueue().ClearStack(user.GetId())
				err := sendQueueSyncEvent(user, sPlayback)
				if err != nil {
					return "", err
				}

				err = sendStackSyncEvent(user, sPlayback)
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
			sPlayback.GetQueue().ClearStackItem(user.GetId(), s)
			err := sendQueueSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}

			err = sendStackSyncEvent(user, sPlayback)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("deleting stream with url %q", s.GetStreamURL()), nil
		}
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

func sendQueueSyncEvent(user *client.Client, sPlayback *playback.StreamPlayback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	res := &client.Response{
		Id:   user.GetId(),
		From: username,
	}

	err := sockutil.SerializeIntoResponse(sPlayback.GetQueue().Status(), &res.Extra)
	if err != nil {
		return err
	}

	user.BroadcastAll("queuesync", res)
	return nil
}

// sendStackSyncEvent sends a queue stacksync event only to the user requesting data
func sendStackSyncEvent(user *client.Client, sPlayback *playback.StreamPlayback) error {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	res := &client.Response{
		Id:   user.GetId(),
		From: username,
	}

	err := sockutil.SerializeIntoResponse(sPlayback.GetQueue().StackStatus(user.GetId()), &res.Extra)
	if err != nil {
		return err
	}

	user.BroadcastTo("stacksync", res)
	return nil
}
