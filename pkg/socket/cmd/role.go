package cmd

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/api/endpoint"
	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type RoleCmd struct {
	Command
}

const (
	ROLE_NAME        = "role"
	ROLE_DESCRIPTION = "add, replace, or remove roles for a subject (requires rbac to be enabled)"
	ROLE_USAGE       = "Usage: /" + ROLE_NAME + " &lt;add | set | remove&gt; &lt;role&gt; &lt;subject&gt;"
)

func (h *RoleCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
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

	var subject *client.Client
	for _, c := range clientHandler.Clients() {
		ns, hasNs := c.Namespace()
		if !hasNs || ns != namespace {
			continue
		}

		uName, hasUname := c.GetUsername()
		if !hasUname {
			continue
		}

		if uName == subjectName {
			subject = c
			break
		}
	}

	if subject == nil {
		return "", fmt.Errorf("error: unable to find subject %q in your namespace", subjectName)
	}

	// endpoint the client should hit in order to save roles in cookie
	targetEndpoint := fmt.Sprintf("/api/auth/cookie?%s=%s", endpoint.CONN_ID_KEY, subject.UUID())

	switch args[0] {
	case "set":
		if err := addRole(authorizer, subject, roleName, subjectName); err != nil {
			return "", err
		}

		// if no errors adding role, remove all other roles from subject
		for _, b := range authorizer.Bindings() {
			if b.Role().Name() == roleName {
				continue
			}

			b.RemoveSubject(subject)
		}

		util.BroadcastHttpRequest(subject, targetEndpoint)
		return fmt.Sprintf("subject %q was successfully bound to role %q", subjectName, roleName), nil
	case "add":
		if err := addRole(authorizer, subject, roleName, subjectName); err != nil {
			return "", err
		}

		util.BroadcastHttpRequest(subject, targetEndpoint)
		return fmt.Sprintf("subject %q was successfully bound to role %q", subjectName, roleName), nil
	case "remove":
		for _, b := range authorizer.Bindings() {
			if b.Role().Name() != roleName {
				continue
			}

			removed := b.RemoveSubject(subject)
			if removed {
				util.BroadcastHttpRequest(subject, targetEndpoint)
				return fmt.Sprintf("user %q unbound from role %q", subjectName, roleName), nil
			}

			return "", fmt.Errorf("user %q was not bound to role %q", subjectName, roleName)
		}

		return "", fmt.Errorf("user %q was not bound to role %q", subjectName, roleName)
	}

	return h.usage, nil
}

func NewCmdRole() SocketCommand {
	return &RoleCmd{
		Command{
			name:        ROLE_NAME,
			description: ROLE_DESCRIPTION,
			usage:       ROLE_USAGE,
		},
	}
}

func addRole(authorizer rbac.Authorizer, subject *client.Client, roleName, subjectName string) error {
	role, exists := authorizer.Role(roleName)
	if !exists {
		return fmt.Errorf("error: role %q not found", roleName)
	}

	// check if subject already bound to role
	for _, b := range authorizer.Bindings() {
		if b.Role().Name() != role.Name() {
			continue
		}

		for _, s := range b.Subjects() {
			if s.UUID() == subject.UUID() {
				return fmt.Errorf("error: subject %q is already bound to role %q", subjectName, roleName)
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

	log.Printf("INF SOCKET CMD ROLE client requested a binding for role %q but one was not found. Creating...\n", roleName)

	// no binding exists for given role, create...
	authorizer.Bind(role, subject)

	subject.BroadcastSystemMessageTo(fmt.Sprintf("You have been assigned to the %q role", role.Name()))
	subject.BroadcastAll("info_userlistupdated", &client.Response{
		Id: subject.UUID(),
	})
	return nil
}
