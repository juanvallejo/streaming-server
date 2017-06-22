package cmd

import (
	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type DebugCmd struct {
	Command
}

const (
	DEBUG_NAME        = "debug"
	DEBUG_DESCRIPTION = "suite of basic admin debugging tools"
	DEBUG_USAGE       = "Usage: /" + DEBUG_NAME + " &lt;refresh&gt;"
)

func (h *DebugCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	if args[0] == "refresh" || args[0] == "reload" {
		user.BroadcastChatActionAll("reloadClient", nil)
		return "Reloading all clients", nil
	}

	return h.usage, nil
}

func NewCmdDebug() SocketCommand {
	return &DebugCmd{
		Command{
			name:        DEBUG_NAME,
			description: DEBUG_DESCRIPTION,
			usage:       DEBUG_USAGE,
		},
	}
}
