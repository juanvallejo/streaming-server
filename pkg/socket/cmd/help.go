package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type HelpCmd struct {
	Command
}

const (
	HELP_NAME        = "help"
	HELP_DESCRIPTION = "displays this output"
	HELP_USAGE       = "Usage: /" + HELP_NAME
)

func (h *HelpCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler) (string, error) {
	output := "Commands help:<br />"
	for _, command := range cmdHandler.GetCommands() {
		output += fmt.Sprintf("<br /><span class='text-hl-name'>%s</span>: %s", command.GetName(), command.GetDescription())
	}

	return output, nil
}

func NewCmdHelp() SocketCommand {
	return &HelpCmd{
		Command{
			name:        HELP_NAME,
			description: HELP_DESCRIPTION,
			usage:       HELP_USAGE,
		},
	}
}
