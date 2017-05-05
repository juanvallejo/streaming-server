package cmd

import "fmt"

type HelpCmd struct {
	Command
}

const (
	NAME        = "help"
	DESCRIPTION = "displays this output"
	USAGE       = "Usage: /help"
)

func (h *HelpCmd) Execute(handler SocketCommandHandler) (string, error) {
	output := "Commands help:<br />"
	for _, command := range handler.GetCommands() {
		output += fmt.Sprintf("<br /><span class='text-hl-name'>%s</span>: %s", command.GetName(), command.GetDescription())
	}

	return output, nil
}

func NewCmdHelp() SocketCommand {
	return &HelpCmd{
		Command{
			name:        NAME,
			description: DESCRIPTION,
			usage:       USAGE,
		},
	}
}
