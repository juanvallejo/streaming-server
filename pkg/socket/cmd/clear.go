package cmd

import (
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type ClearCmd struct {
	Command
}

const (
	CLEAR_NAME        = "clear"
	CLEAR_DESCRIPTION = "clears all messages from the chat window"
	CLEAR_USAGE       = "Usage: /" + CLEAR_NAME
)

func (h *ClearCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler) (string, error) {
	user.BroadcastChatActionTo("clearView", nil)
	return "Clearing chat window messages...", nil
}

func NewCmdClear() SocketCommand {
	return &ClearCmd{
		Command{
			name:        CLEAR_NAME,
			description: CLEAR_DESCRIPTION,
			usage:       CLEAR_USAGE,
		},
	}
}
