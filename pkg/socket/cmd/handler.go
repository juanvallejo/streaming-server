package cmd

import (
	"fmt"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type SocketCommandHandler interface {
	// AddCommand receives a SocketCommand and adds it to
	// an internal map of commands
	AddCommand(SocketCommand)
	// ExecuteCommand receives a command's unique name, obtains
	// the command from the handler's internal map, and calls the
	// SocketCommand's execute method
	ExecuteCommand(string, []string, *client.Client, client.SocketClientHandler, playback.StreamPlaybackHandler, stream.StreamHandler) (string, error)
	// GetCommands receives
	GetCommands() map[string]SocketCommand
}

// Handler implements SocketCommandHandler
type Handler struct {
	commands map[string]SocketCommand
	aliases  map[string]SocketCommand
}

// AddCommand panics if a given command has already been added
// or adds the new command to a map of [commandName]command
func (h *Handler) AddCommand(cmd SocketCommand) {
	if _, exists := h.commands[cmd.GetName()]; exists {
		panic(fmt.Sprintf("tried to register duplicate command: %q", cmd.GetName()))
	}
	if _, exists := h.aliases[cmd.GetName()]; exists {
		panic(fmt.Sprintf("tried to register command name, but alias with same name exists: %q", cmd.GetName()))
	}

	h.commands[cmd.GetName()] = cmd

	// add command aliases
	for _, alias := range cmd.GetAliases() {
		if _, exists := h.aliases[alias]; exists {
			panic(fmt.Sprintf("tried to register duplicate command alias %q -> %q", cmd.GetName(), alias))
		}
		if _, exists := h.commands[alias]; exists {
			panic(fmt.Sprintf("tried to register command alias with existing registered command name: %q", alias))
		}

		h.aliases[alias] = cmd
	}
}

func (h *Handler) GetCommands() map[string]SocketCommand {
	return h.commands
}

func (h *Handler) ExecuteCommand(cmdRoot string, args []string, client *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	var command SocketCommand

	command, exists := h.commands[cmdRoot]
	if !exists {
		command, exists = h.aliases[cmdRoot]
	}

	if exists {
		return command.Execute(h, args, client, clientHandler, playbackHandler, streamHandler)
	}

	return "", fmt.Errorf("error: that command does not exist")
}

// NewHandler creates a new SocketCommand handler
// that registers a list of pre-defined commands
// invoked through an assigned command id string
func NewHandler() SocketCommandHandler {
	h := &Handler{
		commands: make(map[string]SocketCommand),
		aliases:  make(map[string]SocketCommand),
	}

	addSocketCommands(h)
	return h
}

// instantiate and append known socket commands
// to a SocketCommand handler
func addSocketCommands(handler *Handler) {
	handler.AddCommand(NewCmdClear())
	handler.AddCommand(NewCmdDebug())
	handler.AddCommand(NewCmdHelp())
	handler.AddCommand(NewCmdStream())
	handler.AddCommand(NewCmdSubtitles())
	handler.AddCommand(NewCmdUser())
	handler.AddCommand(NewCmdWhoami())
}
