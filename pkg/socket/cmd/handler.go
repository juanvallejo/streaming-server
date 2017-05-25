package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type SocketCommandHandler interface {
	// AddCommand receives a SocketCommand and adds it to
	// an internal map of commands
	AddCommand(SocketCommand)
	// ExecuteCommand receives a command's unique name, obtains
	// the command from the handler's internal map, and calls the
	// SocketCommand's execute method
	ExecuteCommand(string, []string, *client.Client, client.SocketClientHandler, playback.StreamPlaybackHandler) (string, error)
	// GetCommands receives
	GetCommands() map[string]SocketCommand
}

// Handler implements SocketCommandHandler
type Handler struct {
	commands map[string]SocketCommand
}

// AddCommand panics if a given command has already been added
// or adds the new command to a map of [commandName]command
func (h *Handler) AddCommand(cmd SocketCommand) {
	if _, exists := h.commands[cmd.GetName()]; exists {
		panic(fmt.Sprintf("tried to register duplicate command: %q", cmd.GetName()))
	}

	h.commands[cmd.GetName()] = cmd
}

func (h *Handler) GetCommands() map[string]SocketCommand {
	return h.commands
}

// ExecuteCommand receives a command id string
// and executes the matching StreamCommand.
// If no StreamCommand is found by the given id,
// an error is returned; else, a command return
// string is returned.
func (h *Handler) ExecuteCommand(cmdRoot string, args []string, client *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler) (string, error) {
	command, exists := h.commands[cmdRoot]
	if !exists {
		return "", fmt.Errorf("error: that command does not exist")
	}

	return command.Execute(h, args, client, clientHandler, playbackHandler)
}

// NewHandler creates a new SocketCommand handler
// that registers a list of pre-defined commands
// invoked through an assigned command id string
func NewHandler() *Handler {
	h := &Handler{
		commands: make(map[string]SocketCommand),
	}

	addSocketCommands(h)
	return h
}

// instantiate and append known socket commands
// to a SocketCommand handler
func addSocketCommands(handler *Handler) {
	handler.AddCommand(NewCmdHelp())
	handler.AddCommand(NewCmdWhoami())
	handler.AddCommand(NewCmdClear())
	handler.AddCommand(NewCmdUser())
}
