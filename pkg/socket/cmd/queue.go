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
	QUEUE_USAGE       = "Usage: /" + QUEUE_NAME + " (add &lt;url&gt;|clear|list &lt;mine|room&gt;)"
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

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err = sockutil.SerializeIntoResponse(sPlayback.GetQueueStatus(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("queuesync", res)
		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has added %q to the queue", username, url))
		return fmt.Sprintf("successfully queued %q", url), nil
	case "list":
		if len(args) < 2 {
			return "", fmt.Errorf("%v", h.usage)
		}

		if args[1] == "mine" || args[1] == "me" {
			return "", fmt.Errorf("unimplemented")
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
