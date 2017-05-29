package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type SocketCommand interface {
	// Execute runs a SocketCommand's main logic
	// returns an error if a problem occurs during
	// command execution, or a string ("status")
	// containing the return value for the command
	Execute(SocketCommandHandler, []string, *client.Client, client.SocketClientHandler, playback.StreamPlaybackHandler, stream.StreamHandler) (string, error)
	// GetName returns the command's unique string identifier
	GetName() string
	// GetDescription returns the command's summarized readme
	GetDescription() string
	// GetUsage returns the command's help usage
	GetUsage() string
	// GetAliases returns the command's known aliases
	GetAliases() []string
}

// Command implements SocketCommand
type Command struct {
	// command's readable unique identifier
	name string
	// one-line explanation of the command
	usage string
	// command's long description
	description string
	// slice of alternative command root names
	aliases []string
}

func (c *Command) Execute(cmdHandler SocketCommandHandler, args []string, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	return "", fmt.Errorf("unimplemented command.")
}

func (c *Command) GetName() string {
	return c.name
}

func (c *Command) GetDescription() string {
	return c.description
}

func (c *Command) GetUsage() string {
	return c.usage
}

func (c *Command) GetAliases() []string {
	if c.aliases == nil {
		return []string{}
	}

	return c.aliases
}
