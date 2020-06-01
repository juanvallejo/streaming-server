package cmd

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type RoleCmd struct {
	*Command
}

const (
	ROLE_NAME        = "role"
	ROLE_DESCRIPTION = "add, replace, or remove roles for a subject (requires rbac to be enabled)"
	ROLE_USAGE       = "Usage: /" + ROLE_NAME + " &lt;add | set | remove&gt; &lt;role&gt; &lt;subject&gt;"
)

func (h *RoleCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	// TODO: move to api - send client event requesting http request against api endpoint
	// once authorizer is moved outside of pkg/socket/cmd/rbac

	if len(args) == 0 {
		return h.usage, nil
	}

	if len(args) < 3 {
		return h.usage, nil
	}

	roleName := args[1]
	subjectName := args[2]

	namespace, exists := user.Namespace()
	if !exists {
		return "", fmt.Errorf("unable to obtain namespace information")
	}

	authorizer := cmdHandler.Authorizer()
	if authorizer == nil {
		return "", fmt.Errorf("authorizer not enabled")
	}

	subjects := []*client.Client{}
	for _, c := range namespace.Connections() {
		cl, err := clientHandler.GetClient(c.UUID())
		if err != nil {
			continue
		}

		if subjectName == "*" {
			subjects = append(subjects, cl)
			continue
		}

		uName, hasName := cl.GetUsername()
		if !hasName {
			continue
		}

		if uName == subjectName {
			subjects = append(subjects, cl)
			break
		}
	}

	if len(subjects) == 0 {
		return "", fmt.Errorf("error: unable to find subject %q in your namespace", subjectName)
	}

	role, exists := authorizer.Role(roleName)
	if !exists {
		return "", fmt.Errorf("error: role %q not found", roleName)
	}

	switch args[0] {
	case "set":
		errs := []string{}
		bound := []string{}
		for _, subject := range subjects {
			if err := addRole(authorizer, role, subject); err != nil {
				errs = append(errs, err.Error())
				continue
			}

			bound = append(bound, subject.GetUsernameOrId())

			// if no errors adding role, remove all other roles from subject
			for _, b := range authorizer.Bindings() {
				if b.Role().Name() == roleName {
					continue
				}

				b.RemoveSubject(subject)
			}

			subject.BroadcastAuthRequestTo("cookie")
		}

		msg := ""
		for _, e := range errs {
			msg += fmt.Sprintf("%s\n<br />", e)
		}

		if subjectName == "*" {
			if len(bound) > 0 {
				msg += fmt.Sprintf("bound subjects to role %q: %s", roleName, bound)
			}
			return msg, nil
		}

		msg += fmt.Sprintf("subject %q was successfully bound to role %q", subjectName, roleName)
		return msg, nil
	case "add":
		errs := []string{}
		bound := []string{}

		for _, subject := range subjects {
			if err := addRole(authorizer, role, subject); err != nil {
				errs = append(errs, err.Error())
				continue
			}

			bound = append(bound, subject.GetUsernameOrId())
			subject.BroadcastAuthRequestTo("cookie")
		}

		msg := ""
		for _, e := range errs {
			msg += fmt.Sprintf("%s<br />", e)
		}

		if subjectName == "*" {
			if len(bound) > 0 {
				msg += fmt.Sprintf("bound (additive) subjects to role %q: %s", roleName, bound)
			}
			return msg, nil
		}

		return fmt.Sprintf("subject %q was successfully bound (additive) to role %q", subjectName, roleName), nil
	case "remove":
		messages := []string{}

		for _, subject := range subjects {
			for _, b := range authorizer.Bindings() {
				if b.Role().Name() != roleName {
					continue
				}

				removed := b.RemoveSubject(subject)
				if removed {
					subject.BroadcastSystemMessageTo(fmt.Sprintf("You have been removed from the %q role", role.Name()))
					subject.BroadcastAll("info_userlistupdated", &client.Response{
						Id: subject.UUID(),
					})

					subject.BroadcastAuthRequestTo("cookie")
					messages = append(messages, fmt.Sprintf("user %q unbound from role %q", subjectName, roleName))
					break
				}

				messages = append(messages, fmt.Sprintf("WARN: user %q was not bound to role %q", subjectName, roleName))
				break
			}
		}

		message := ""
		for i, m := range messages {
			br := ""
			if i+i != len(messages) {
				br = "<br />"
			}

			message += m + br
		}

		return message, nil
	}

	return h.usage, nil
}

func NewCmdRole() SocketCommand {
	return &RoleCmd{
		&Command{
			name:        ROLE_NAME,
			description: ROLE_DESCRIPTION,
			usage:       ROLE_USAGE,
		},
	}
}

func addRole(authorizer rbac.Authorizer, role rbac.Role, subject *client.Client) error {
	for _, b := range authorizer.Bindings() {
		if b.Role().Name() != role.Name() {
			continue
		}

		for _, s := range b.Subjects() {
			if s.UUID() == subject.UUID() {
				return fmt.Errorf("error: subject %q is already bound to role %q", subject.GetUsernameOrId(), role.Name())
			}
		}

		// found binding for role, but subject not bound; add
		b.AddSubject(subject)
		subject.BroadcastSystemMessageTo(fmt.Sprintf("You have been assigned to the %q role", role.Name()))
		subject.BroadcastAll("info_userlistupdated", &client.Response{
			Id: subject.UUID(),
		})
		return nil
	}

	log.Printf("INF SOCKET CMD ROLE client requested a binding for role %q but one was not found. Creating...\n", role.Name())

	// no binding exists for given role, create...
	authorizer.Bind(role, subject)

	subject.BroadcastSystemMessageTo(fmt.Sprintf("You have been assigned to the %q role", role.Name()))
	subject.BroadcastAll("info_userlistupdated", &client.Response{
		Id: subject.UUID(),
	})
	return nil
}
