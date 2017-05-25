package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type UserCmd struct {
	Command
}

const (
	USER_NAME        = "user"
	USER_DESCRIPTION = "controls user settings"
	USER_USAGE       = "Usage: /" + USER_NAME + " (name &lt;username&gt;|list)"
)

func (h *UserCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	if args[0] == "name" {
		if len(args) < 2 {
			return h.usage, nil
		}

		err := util.UpdateClientUsername(user, args[1], clientHandler, playbackHandler)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("attempting to update username to %q", args[1]), nil

	}

	if args[0] == "list" {
		userName, userHasName := user.GetUsername()

		output := "All users in the stream:<br />"
		for _, c := range clientHandler.GetClients() {
			prefix := "<br />    "
			name, hasName := c.GetUsername()
			if !hasName {
				output += prefix + "[Not in chat] " + c.GetId()
				continue
			}
			if userHasName && name == userName {
				name = "<span class='text-hl-name'>" + name + "</span>"
			}

			output += prefix + name
		}

		return output, nil
	}

	return h.usage, nil
}

func NewCmdUser() SocketCommand {
	return &UserCmd{
		Command{
			name:        USER_NAME,
			description: USER_DESCRIPTION,
			usage:       USER_USAGE,
		},
	}
}
