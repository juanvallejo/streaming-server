package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type UserCmd struct {
	*Command
}

const (
	USER_NAME        = "user"
	USER_DESCRIPTION = "controls user settings"
	USER_USAGE       = "Usage: /" + USER_NAME + " (name &lt;username&gt;|list)"
)

var (
	user_aliases = []string{"u"}
)

func (h *UserCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	if len(args) == 0 {
		return h.usage, nil
	}

	if args[0] == "name" {
		if len(args) < 2 {
			return h.usage, nil
		}

		err := util.UpdateClientUsername(user, args[1], clientHandler)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("attempting to update username to %q", args[1]), nil

	}

	_, exists := user.Namespace()
	if !exists {
		return "", fmt.Errorf("no room associated with user")
	}

	if args[0] == "list" {
		userName, userHasName := user.GetUsername()

		output := "All users in the current room:<br />"
		for _, conn := range user.Connections() {
			c, err := clientHandler.GetClient(conn.UUID())
			if err != nil {
				continue
			}

			prefix := "<br />    "
			name, hasName := c.GetUsername()
			if !hasName {
				output += prefix + "[Not in chat] " + c.UUID()
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
		&Command{
			name:        USER_NAME,
			description: USER_DESCRIPTION,
			usage:       USER_USAGE,

			aliases: user_aliases,
		},
	}
}
