package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type SubtitlesCmd struct {
	Command
}

const (
	SUBTITLES_NAME        = "subtitles"
	SUBTITLES_DESCRIPTION = "controls stream subtitles"
	SUBTITLES_USAGE       = "Usage: /" + SUBTITLES_NAME + " &lt;(on|off)&gt;"

	SUBTITLES_FILE_ROOT = "/src/static/subtitles/"
)

var (
	subtitles_aliases = []string{"subs"}
)

func (h *SubtitlesCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.UUID()
	}

	userRoom, hasRoom := user.Namespace()
	if !hasRoom {
		log.Printf("SOCKET CLIENT ERR client with id %q attempted to control stream playback with no room assigned", user.UUID())
		return "", fmt.Errorf("error: you must be in a stream to control stream playback")
	}

	currentDir := util.GetCurrentDirectory()
	subFile := roomToSubsFile(userRoom.Name())

	if args[0] == "on" {
		_, err := os.Stat(currentDir + "/../../" + SUBTITLES_FILE_ROOT + subFile)
		if err != nil {
			log.Printf("SOCKET CLIENT ERR unable to load subtitle file for stream %q: %v", userRoom, err)
			return "", fmt.Errorf("error: missing subtitles file for current stream")
		}

		user.BroadcastTo("info_subtitles", &client.Response{
			Id:   user.UUID(),
			From: username,
			Extra: map[string]interface{}{
				"path": SUBTITLES_FILE_ROOT + subFile,
				"on":   true,
			},
		})
		user.BroadcastSystemMessageFrom(fmt.Sprintf("%q has requested subtitles", username))
		return "attempting to add subtitles to your stream...", nil
	}

	if args[0] == "off" {
		user.BroadcastTo("info_subtitles", &client.Response{
			Id:   user.UUID(),
			From: username,
			Extra: map[string]interface{}{
				"path": SUBTITLES_FILE_ROOT + subFile,
				"on":   false,
			},
		})
		return "attempting to remove subtitles from your stream...", nil
	}

	return h.usage, nil
}

func roomToSubsFile(roomName string) string {
	segs := strings.Split(roomName, ".")
	return segs[0] + ".vtt"
}

func NewCmdSubtitles() SocketCommand {
	return &SubtitlesCmd{
		Command{
			name:        SUBTITLES_NAME,
			description: SUBTITLES_DESCRIPTION,
			usage:       SUBTITLES_USAGE,

			aliases: subtitles_aliases,
		},
	}
}
