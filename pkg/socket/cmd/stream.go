package cmd

import (
	"fmt"
	"log"
	"strconv"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/util"
	sockutil "github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type StreamCmd struct {
	Command
}

const (
	STREAM_NAME        = "stream"
	STREAM_DESCRIPTION = "controls stream playback (info|pause|play|stop|set|queue|seek|skip)'"
	STREAM_USAGE       = "Usage: /" + STREAM_NAME + " (info|pause|play|stop|skip|seek &lt;seconds&gt;|set &lt;url&gt;|queue &lt;url&gt;)"
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
		log.Printf("ERR SOCKET CLIENT client with id %q (%s) attempted to control stream playback with no room assigned", user.GetId(), username)
		return "", fmt.Errorf("error: you must be in a stream to control stream playback.")
	}

	sPlayback, sPlaybackExists := playbackHandler.GetStreamPlayback(userRoom)
	if !sPlaybackExists {
		log.Printf("ERR SOCKET CLIENT unable to associate client %q (%s) in room %q with any stream playback objects", user.GetId(), username, userRoom)
		return "", fmt.Errorf("error: no stream playback is currently loaded for your room")
	}

	// used as flag to allow "play" to assume "skip" behavior when no
	// stream is contained within the playback object.
	playStreamOnSkip := false

	switch args[0] {
	case "info":
		status, err := sPlayback.GetStatus().Serialize()
		if err != nil {
			return "", err
		}

		return string(status), nil
	case "play":
		// if a stream has not been set, fallthrough - allow "play"
		// to behave like "skip". If a stream has been set, allow
		// "play" case below to handle command.
		_, streamExists := sPlayback.GetStream()
		if streamExists {
			err := sPlayback.Play()
			if err != nil {
				return "", err
			}

			res := &client.Response{
				Id:   user.GetId(),
				From: username,
			}

			err = sockutil.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
			if err != nil {
				return "", err
			}

			user.BroadcastAll("streamsync", res)
			return "playing stream...", nil
		}

		playStreamOnSkip = true
		fallthrough
	case "skip":
		// skip the currently-playing stream and replace it with the next item in the queue
		queue := sPlayback.GetQueue()
		nextStream, err := queue.Pop()
		if err != nil {
			return "", fmt.Errorf("error: %v", err)
		}

		sPlayback.SetStream(nextStream)
		sPlayback.Reset()
		sPlayback.UpdateStartedBy(user)

		if playStreamOnSkip {
			sPlayback.Play()
		}

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err = sockutil.SerializeIntoResponse(nextStream.Codec(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("streamload", res)
		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has attempted to load the next item in the queue: %q", username, nextStream.GetStreamURL()))
		return fmt.Sprintf("attempting to load the next item in the queue: %q", nextStream.GetStreamURL()), nil
	case "load":
		fallthrough
	case "set":
		// skip adding a stream to the queue, and set as currently playing playback stream
		url, err := getStreamUrlFromArgs(args)
		if err != nil {
			return "", err
		}

		s, err := sPlayback.GetOrCreateStreamFromUrl(url, streamHandler)
		if err != nil {
			return "", err
		}

		sPlayback.SetStream(s)
		sPlayback.Reset()
		sPlayback.UpdateStartedBy(user)

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err = sockutil.SerializeIntoResponse(s.Codec(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("streamload", res)
		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has attempted to load a %s stream: %q", username, s.GetKind(), url))

		return fmt.Sprintf("attempting to load %q", args[1]), nil
	case "queue":
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

	}

	// require stream data to have been loaded before proceeding with cases below
	_, streamExists := sPlayback.GetStream()
	if !streamExists {
		return "", fmt.Errorf("error: no stream is currently loaded for your room - use /stream set &lt;url&gt;")
	}

	switch args[0] {
	case "pause":
		sPlayback.Pause()

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err := sockutil.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("streamsync", res)
		return "pausing stream...", nil
	case "stop":
		sPlayback.Stop()

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err := sockutil.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("streamsync", res)
		return "stopping stream...", nil
	case "seek":
		if len(args) < 2 || len(args[1]) == 0 {
			return "", fmt.Errorf("a time (in seconds) must be provided. See usage info.")
		}

		rawTime := args[1]
		modifier := string(rawTime[0])
		if modifier == "+" || modifier == "-" {
			rawTime = rawTime[1:]
		} else {
			modifier = ""
		}

		newTime, err := strconv.Atoi(rawTime)
		if err != nil {
			// if an int was not received, try to parse human-readable time format (0h0m0s)
			newTime, err = util.HumanTimeToSeconds(rawTime)
			if err != nil {
				return "", fmt.Errorf("error: cannot interpret %q as a valid time. Must be of the form 12345 or 0h0m0s", args[1])
			}
		}

		if len(modifier) > 0 {
			if modifier == "+" {
				sPlayback.SetTime(sPlayback.GetTime() + newTime)
			} else {
				sPlayback.SetTime(sPlayback.GetTime() - newTime)
			}
		} else {
			sPlayback.SetTime(newTime)
		}

		res := &client.Response{
			Id:   user.GetId(),
			From: username,
		}

		err = sockutil.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
		if err != nil {
			return "", err
		}

		user.BroadcastAll("streamsync", res)

		return fmt.Sprintf("setting the stream playback to %vs for all clients.", newTime), nil
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

// receives a list of cmd args and returns the slice of the command corresponding to a stream url.
// Returns an error if insufficient args are provided.
func getStreamUrlFromArgs(args []string) (string, error) {
	if len(args) < 2 || len(args[1]) == 0 {
		return "", fmt.Errorf("error: a stream url must be provided")
	}

	return args[1], nil
}
