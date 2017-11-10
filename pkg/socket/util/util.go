package util

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/validation"
)

const ROOM_URL_SEGMENT = "/v/"

// TODO: make this function concurrency-safe
func UpdateClientUsername(c *client.Client, username string, clientHandler client.SocketClientHandler) error {
	err := validation.ValidateClientUsername(username)
	if err != nil {
		return err
	}

	prevName, hasPrevName := c.GetUsername()

	log.Printf("INF SOCKET CLIENT client with id %q requested a username update (%q -> %q)", c.UUID(), prevName, username)

	if hasPrevName && prevName == username {
		return fmt.Errorf("error: you already have that username")
	}

	for _, otherUser := range clientHandler.Clients() {
		otherUserName, hasName := otherUser.GetUsername()
		if !hasName {
			continue
		}
		if username == otherUserName {
			return fmt.Errorf("error: the username %q is taken", username)
		}
	}

	if err := c.UpdateUsername(username); err != nil {
		oldName := "[none]"
		if hasPrevName {
			oldName = prevName
		}

		log.Printf("ERR SOCKET CLIENT failed to update username (%q -> %q) for client with id %q", oldName, username, c.UUID())
		return err
	}

	log.Printf("INF SOCKET CLIENT sending \"updateusername\" event to client with id %q (%s)\n", c.UUID(), username)
	c.BroadcastTo("updateusername", &client.Response{
		From: username,
	})

	isNewUser := ""
	if !hasPrevName {
		isNewUser = "true"
	}

	c.BroadcastFrom("info_updateusername", &client.Response{
		Id:   c.UUID(),
		From: username,
		Extra: map[string]interface{}{
			"oldUser":   prevName,
			"isNewUser": isNewUser,
		},
		IsSystem: true,
	})

	return nil
}

// GetRoomNameFromRequest receives a socket connection request and returns
// a fully-qualified room name from the request's referer information
func GetRoomNameFromRequest(req *http.Request) (string, error) {
	segs := strings.Split(req.URL.String(), ROOM_URL_SEGMENT)
	if len(segs) > 1 {
		return segs[1], nil
	}

	return "", fmt.Errorf("http request referer field (%s) had an unsupported ROOM_URL_SEGMENT(%q) format", req.Referer(), ROOM_URL_SEGMENT)
}

func GetCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(fmt.Sprintf("unable to get filepath: %v", err))
	}

	return dir
}

func bindDefaultRolesFromCookie(authorizer rbac.Authorizer, c *client.Client) bool {
	cookie, err := c.Connection().Request().Cookie(rbac.AuthCookieName)
	if err != nil {
		return false
	}

	fmt.Printf("GOT COOKIE BITCH %v -> %v\n", cookie.Name, cookie.Value)

	return false
}

// BindDefaultuserRoles receives a handler and a client
func BindDefaultUserRoles(authorizer rbac.Authorizer, clientHandler client.SocketClientHandler, c *client.Client) error {
	roomName, hasRoom := c.Namespace()
	if !hasRoom {
		return fmt.Errorf("attempt to bind default roles to user (%v) with no room", c.GetUsernameOrId())
	}

	if authorizer == nil {
		return fmt.Errorf("attempt to assign default roles to user (%v) with no authorizer enabled", c.GetUsernameOrId())
	}

	if bindDefaultRolesFromCookie(authorizer, c) {
		log.Printf("INF SOCKET CLIENT AUTHZ found cookie for client")
		return nil
	}

	log.Printf("INF SOCKET CLIENT AUTHZ no cookies containing authz data found for client (%v). Initializing default roles...\n", c.GetUsernameOrId())

	// assign admin role to user if room already exists,
	// but they are the only client assigned to it
	roleName := rbac.ADMIN_ROLE

	for _, conn := range c.Connections() {
		cs, err := clientHandler.GetClient(conn.Id())
		if err != nil {
			continue
		}

		if cs.UUID() == c.UUID() {
			continue
		}

		roleName = rbac.USER_ROLE
	}

	userRole, found := authorizer.Role(roleName)
	if !found {
		return fmt.Errorf("unable to bind role %q to client %q with id (%s): unable to find role", roleName, c.GetUsernameOrId(), c.UUID())
	}

	authorizer.Bind(userRole, c)
	log.Printf("INF SOCKET CLIENT AUTHZ bound role %q to client %q with id (%s)", roleName, c.GetUsernameOrId(), c.UUID())

	serializedRole, err := rbac.NewSerializableRole(roomName, roleName).Serialize()
	if err != nil {
		return err
	}

	// set auth cookie with computed role
	w := c.Connection().ResponseWriter()
	http.SetCookie(w, &http.Cookie{
		Name:     rbac.AuthCookieName,
		Value:    string(serializedRole),
		HttpOnly: false,
	})

	return nil
}

// serializeIntoResponse receives an api.ApiCodec and
// serializes it into a given structure pointer.
func SerializeIntoResponse(codec api.ApiCodec, dest interface{}) error {
	b, err := codec.Serialize()
	if err != nil {
		return err
	}

	return json.Unmarshal(b, dest)
}
