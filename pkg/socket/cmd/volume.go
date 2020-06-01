package cmd

import (
	"strconv"

	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type VolumeCmd struct {
	*Command
}

const (
	VOLUME_NAME        = "volume"
	VOLUME_DESCRIPTION = "increase, decrease, or set a volume value"
	VOLUME_USAGE       = "Usage: /" + VOLUME_NAME
)

var (
	volume_aliases = []string{"vol"}
)

func (h *VolumeCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	rawVol := args[0]
	modifier := string(rawVol[0])
	if modifier == "+" || modifier == "-" {
		rawVol = rawVol[1:]
	} else {
		modifier = ""
	}

	newVol, err := strconv.Atoi(rawVol)
	if err != nil {
		return "", fmt.Errorf("error: volume must be an integer, optionally followed by a %q or a %q sign", "+", "-")
	}

	if len(modifier) > 0 {
		evtName := "decreaseVolume"
		if modifier == "+" {
			evtName = "increaseVolume"
		}
		user.BroadcastChatActionTo(evtName, []interface{}{
			newVol,
		})

		return "Modifying volume...", nil
	}

	user.BroadcastChatActionTo("setVolume", []interface{}{
		newVol,
	})
	return fmt.Sprintf("Setting volume to %v...", newVol), nil
}

func NewCmdVolume() SocketCommand {
	return &VolumeCmd{
		&Command{
			name:        VOLUME_NAME,
			description: VOLUME_DESCRIPTION,
			usage:       VOLUME_USAGE,

			aliases: volume_aliases,
		},
	}
}
