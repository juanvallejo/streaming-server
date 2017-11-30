package cmd

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type WhoamiCmd struct {
	Command
}

const (
	WHOAMI_NAME        = "whoami"
	WHOAMI_DESCRIPTION = "displays the current username for a connection"
	WHOAMI_USAGE       = "Usage: /" + WHOAMI_NAME
)

func (h *WhoamiCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if name, hasName := user.GetUsername(); hasName {
		return name, nil
	}
	log.Printf("SOCKET COMMAND user with id %q requested command %q but has not registered a username yet.", user.UUID(), h.name)
	return "", fmt.Errorf("user with id %q has not registered with a name yet", user.UUID())
}

func NewCmdWhoami() SocketCommand {
	return &WhoamiCmd{
		Command{
			name:        WHOAMI_NAME,
			description: WHOAMI_DESCRIPTION,
			usage:       WHOAMI_USAGE,
		},
	}
}
