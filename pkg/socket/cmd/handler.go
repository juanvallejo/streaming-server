package cmd

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type SocketCommandHandler interface {
	// returns an Authorizer if one has been set by a
	// command handler supporting access control.
	Authorizer() (rbac.Authorizer, bool)
	// AddCommand receives a SocketCommand and adds it to
	// an internal map of commands
	AddCommand(SocketCommand)
	// Aliases returns a map of [commandAlias]SocketCommand
	Aliases() map[string]SocketCommand
	// Commands returns a map of [commandName]SocketCommand
	Commands() map[string]SocketCommand
	// ExecuteCommand receives a command's unique name, obtains
	// the command from the handler's internal map, and calls the
	// SocketCommand's execute method
	ExecuteCommand(string, []string, *client.Client, client.SocketClientHandler, playback.StreamPlaybackHandler, stream.StreamHandler) (string, error)
}

// Handler implements SocketCommandHandler
type Handler struct {
	commands map[string]SocketCommand
	aliases  map[string]SocketCommand
}

func (h *Handler) Authorizer() (rbac.Authorizer, bool) {
	return nil, false
}

// AddCommand panics if a given command has already been added
// or adds the new command to a map of [commandName]command
func (h *Handler) AddCommand(cmd SocketCommand) {
	if _, exists := h.commands[cmd.Name()]; exists {
		panic(fmt.Sprintf("tried to register duplicate command: %q", cmd.Name()))
	}
	if _, exists := h.aliases[cmd.Name()]; exists {
		panic(fmt.Sprintf("tried to register command name, but alias with same name exists: %q", cmd.Name()))
	}

	h.commands[cmd.Name()] = cmd

	// add command aliases
	for _, alias := range cmd.GetAliases() {
		if _, exists := h.aliases[alias]; exists {
			panic(fmt.Sprintf("tried to register duplicate command alias %q -> %q", cmd.Name(), alias))
		}
		if _, exists := h.commands[alias]; exists {
			panic(fmt.Sprintf("tried to register command alias with existing registered command name: %q", alias))
		}

		h.aliases[alias] = cmd
	}
}

func (h *Handler) Commands() map[string]SocketCommand {
	return h.commands
}

func (h *Handler) Aliases() map[string]SocketCommand {
	return h.aliases
}

func (h *Handler) ExecuteCommand(cmdRoot string, args []string, client *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	command, exists := resolveCommandAlias(cmdRoot, h.commands, h.aliases)
	if !exists {
		return "", fmt.Errorf("error: that command does not exist")
	}

	return command.Execute(h, args, client, clientHandler, playbackHandler, streamHandler)

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

// HandlerWithRBAC is a SocketCommandHandler that
// manages role-based access control to commands
type HandlerWithRBAC struct {
	SocketCommandHandler

	AccessController rbac.Authorizer
}

func (c *HandlerWithRBAC) Authorizer() (rbac.Authorizer, bool) {
	return c.AccessController, c.AccessController != nil
}

func (c *HandlerWithRBAC) ExecuteCommand(cmdRoot string, args []string, client *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	command, exists := resolveCommandAlias(cmdRoot, c.Commands(), c.Aliases())
	if !exists {
		return "", fmt.Errorf("error: that command does not exist")
	}

	action := util.CommandAction(command.Name(), args)

	rule, exists := rbac.RuleByAction(c.AccessController.Bindings(), action)
	if !exists {
		log.Printf("ERR SOCKET CMD AUTHZ unable to find rule for action %q for client %q with id (%s)", action, client.GetUsernameOrId(), client.UUID())
		return "", fmt.Errorf("error: unable to authorize the requested command")
	}

	if c.AccessController.Verify(client, rule) {
		return c.SocketCommandHandler.ExecuteCommand(cmdRoot, args, client, clientHandler, playbackHandler, streamHandler)
	}

	log.Printf("ERR SOCKET CMD AUTHZ client %q with id (%s) has attempted to perform unauthorized action: %q", client.GetUsernameOrId(), client.UUID(), action)
	return "", fmt.Errorf("error: you are not authorized to perform that command")
}

// NewControlledHandler returns a command handler capable
// of restricting command access based on a client's role
func NewHandlerWithRBAC() SocketCommandHandler {
	h := &HandlerWithRBAC{
		SocketCommandHandler: NewHandler(),
		AccessController:     rbac.NewAuthorizer(),
	}

	addDefaultRoles(h.AccessController)

	return h
}

// instantiate and append known socket commands
// to a SocketCommand handler
func addSocketCommands(handler SocketCommandHandler) {
	handler.AddCommand(NewCmdClear())
	handler.AddCommand(NewCmdDebug())
	handler.AddCommand(NewCmdHelp())
	handler.AddCommand(NewCmdStream())
	handler.AddCommand(NewCmdSubtitles())
	handler.AddCommand(NewCmdQueue())
	handler.AddCommand(NewCmdUser())
	handler.AddCommand(NewCmdVolume())
	handler.AddCommand(NewCmdWhoami())
}

func addDefaultRoles(authz rbac.Authorizer) {
	// default rules
	clearChat := rbac.NewRule("clear the chat", []string{"clear"})
	debugReload := rbac.NewRule("reload all clients", []string{
		"debug/reload",
		"debug/refresh",
	})
	help := rbac.NewRule("access command help", []string{"help"})
	streamInfo := rbac.NewRule("access stream info", []string{"stream/info"})
	streamControl := rbac.NewRule("play/pause/skip/reset/load the stream", []string{
		"stream/play",
		"stream/skip",
		"stream/load",
		"stream/set",
		"stream/pause",
		"stream/stop",
		"stream/seek",
	})
	subtitles := rbac.NewRule("control stream subtitles", []string{
		"subtitles/on",
		"subtitles/off",
	})
	queueAdd := rbac.NewRule("add streams to the queue", []string{
		"queue/add/*",
	})
	queueList := rbac.NewRule("list items in the queue", []string{
		"queue/list/*",
	})
	queueClearMine := rbac.NewRule("clear items in your queue", []string{
		"queue/clear/mine",
		"queue/clear/mine/*",
		"queue/clear/me",
		"queue/clear/me/*",
	})
	queueClearRoom := rbac.NewRule("clear items in the room's queue", []string{
		"queue/clear/room",
		"queue/clear/room/*",
		"queue/clear/all",
		"queue/clear/all/*",
	})
	queueOrderMine := rbac.NewRule("re-order items in your queue", []string{
		"queue/order/mine",
		"queue/order/mine/*",
		"queue/order/me",
		"queue/order/me/*",
	})
	queueOrderRoom := rbac.NewRule("re-order items in the room's queue", []string{
		"queue/order/room",
		"queue/order/room/*",
		"queue/order/all",
		"queue/order/all/*",
	})
	userUpdateName := rbac.NewRule("update a client's username", []string{
		"user/name/*",
	})
	userList := rbac.NewRule("list users in a room", []string{
		"user/list",
	})
	volume := rbac.NewRule("update your volume", []string{
		"volume/*",
	})
	whoami := rbac.NewRule("list your current username", []string{
		"whoami",
	})

	// TODO: This is an admin rule - move to admin role
	// once persistent client sessions are implemented.
	queueMigrate := rbac.NewRule("migrate a user's queue to yours", []string{
		"queue/migrate/*",
	})

	// default roles
	viewerRole := rbac.NewRole(rbac.VIEWER_ROLE, []rbac.Rule{
		help,
		streamInfo,
		subtitles,
		queueList,
		userList,
		volume,
		whoami,
	})
	userRole := rbac.NewRole(rbac.USER_ROLE, append([]rbac.Rule{
		clearChat,
		queueAdd,
		queueClearMine,
		queueMigrate,
		queueOrderMine,
		userUpdateName,
	}, viewerRole.Rules()...))
	adminRole := rbac.NewRole(rbac.ADMIN_ROLE, append([]rbac.Rule{
		streamControl,
		debugReload,
		queueClearRoom,
		queueOrderRoom,
	}, userRole.Rules()...))

	roles := []rbac.Role{
		viewerRole,
		userRole,
		adminRole,
	}

	for _, role := range roles {
		authz.AddRole(role)
	}
}

func resolveCommandAlias(cmdRoot string, commands, aliases map[string]SocketCommand) (SocketCommand, bool) {
	command, exists := commands[cmdRoot]
	if !exists {
		command, exists = aliases[cmdRoot]
	}

	return command, exists
}
