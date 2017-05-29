package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type StreamCmd struct {
	Command
}

const (
	STREAM_NAME        = "stream"
	STREAM_DESCRIPTION = "controls stream playback (pause|play|stop|load)'"
	STREAM_USAGE       = "Usage: /" + STREAM_NAME + " (pause|play|stop|info|set &lt;seconds&gt;|load &lt;url&gt;)"
)

var (
	stream_aliases = []string{}
)

func (h *StreamCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.GetId()
	}

	userRoom, hasRoom := user.GetRoom()
	if !hasRoom {
		log.Printf("SOCKET CLIENT ERR client with id %q (%s) attempted to control stream playback with no room assigned", user.GetId(), username)
		return "", fmt.Errorf("error: you must be in a stream to control stream playback.")
	}

	sPlayback, sPlaybackExists := playbackHandler.GetStreamPlayback(userRoom)
	if !sPlaybackExists {
		log.Printf("SOCKET CLIENT ERR unable to associate client %q (%s) in room %q with any stream playback objects", user.GetId(), username, userRoom)
		return "", fmt.Errorf("error: no stream playback is currently loaded for your room")
	}

	switch args[0] {
	case "info":
		output := "Stream playback info:<br />"
		for k, v := range sPlayback.GetStatus() {
			output += fmt.Sprintf("<br /><span class='text-hl-name'>%s</span>: %v", k, v)
		}

		return output, nil
	case "load":
		if len(args) < 2 || len(args[1]) == 0 {
			return "", fmt.Errorf("error: a stream url must be provided")
		}
		s, err := sPlayback.LoadStream(args[1], streamHandler)
		if err != nil {
			return "", err
		}

		sPlayback.UpdateStartedBy(user)

		user.BroadcastAll("streamload", &client.Response{
			Id:    user.GetId(),
			From:  username,
			Extra: s.GetInfo(),
		})
		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has attempted to load a %s stream: %q", username, s.GetKind(), args[1]))

		return fmt.Sprintf("attempting to load %q", args[1]), nil
	}

	// require stream data to have been loaded before proceeding with cases below
	_, streamExists := sPlayback.GetStream()
	if !streamExists {
		return "", fmt.Errorf("error: no stream is currently loaded for your room - use /stream load &lt;url&gt;")
	}

	switch args[0] {
	case "pause":
		sPlayback.Pause()
		user.BroadcastAll("streamsync", &client.Response{
			Id:    user.GetId(),
			From:  username,
			Extra: sPlayback.GetStatus(),
		})
		return "pausing stream...", nil
	case "stop":
		sPlayback.Stop()
		user.BroadcastAll("streamsync", &client.Response{
			Id:    user.GetId(),
			From:  username,
			Extra: sPlayback.GetStatus(),
		})
		return "stopping stream...", nil
	case "play":
		err := sPlayback.Play()
		if err != nil {
			return "", err
		}
		user.BroadcastAll("streamsync", &client.Response{
			Id:    user.GetId(),
			From:  username,
			Extra: sPlayback.GetStatus(),
		})
		return "playing stream...", nil
	case "set":
		if len(args) < 2 || len(args[1]) == 0 {
			return "", fmt.Errorf("a time (in seconds) must be provided. See usage info.")
		}

		newTime, err := strconv.Atoi(args[1])
		if err != nil {
			return "", fmt.Errorf("error: unable to convert user-provided argument: %v", err)
		}

		sPlayback.SetTime(newTime)
		user.BroadcastAll("streamsync", &client.Response{
			Id:    user.GetId(),
			From:  username,
			Extra: sPlayback.GetStatus(),
		})

		return "setting the stream playback to " + args[1] + "s for all clients.", nil
	}

	return h.usage, nil
}

func NewCmdStream() SocketCommand {
	return &StreamCmd{
		Command{
			name:        STREAM_NAME,
			description: STREAM_DESCRIPTION,
			usage:       STREAM_USAGE,

			aliases: stream_aliases,
		},
	}
}
