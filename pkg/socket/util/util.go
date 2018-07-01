package util

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"time"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
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
func NamespaceFromRequest(req *http.Request) (string, error) {
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

func rolesFromCookie(r *http.Request, authorizer rbac.Authorizer, namespace connection.Namespace) ([]rbac.Role, error) {
	cookie, err := r.Cookie(rbac.AuthCookieName)
	if err != nil {
		return []rbac.Role{}, fmt.Errorf("unable to retrieve cookie by name %q: %v", rbac.AuthCookieName, err)
	}

	roleData := &rbac.AuthCookieData{}
	if err := roleData.Decode([]byte(cookie.Value)); err != nil {
		return []rbac.Role{}, fmt.Errorf("unable to decode cookie data %v: %v", cookie.Value, err)
	}

	roles := []rbac.Role{}
	for _, ns := range roleData.Namespaces {
		if ns.Name != namespace.Name() {
			continue
		}

		// if namespace name matched, but not id, the
		// auth data in cookie is no longer valid -
		// the namespace for which the stored auth
		// data applies to no longer exists.
		// this gate is optional, in order to enforce a
		// more temporary usage of rooms.
		//if ns.Id != namespace.UUID() {
		//	return []rbac.Role{}, fmt.Errorf("valid namespace found, but auth-cookie uuid did not match namespace uuid (%q != %q)", ns.Id, namespace.UUID())
		//}

		for _, r := range ns.Roles {
			if role, exists := authorizer.Role(r); exists {
				roles = append(roles, role)
			}
		}
		break
	}

	if len(roles) == 0 {
		return roles, fmt.Errorf("no roles found for namespace %q", namespace)
	}

	return roles, nil
}

// BindDefaultuserRoles computes the default roles to assign to a given connection request
// based on previously stored auth data as well as a connection's namespace state.
// The following rules will be followed (in the given order) when determining which roles to return:
//  - If no other connections are bound to the given namespace, an "admin" role will be assigned
//    and forced onto the connection - regardless of previously stored data on an existing auth cookie.
//  - If auth information was previously stored in an auth cookie, and the computed role based on
//    the namespace state is not "admin", the stored roles will be forced onto the connection.
//  - If there is no previously stored information for the given namespace in an auth cookie, or
//    the auth cookie does not exist, and there is at least one other connection assigned to the
//    given namespace, a "user" role will be forced onto the connection.
func DefaultRoles(r *http.Request, authorizer rbac.Authorizer, connUUID string, namespace connection.Namespace) ([]rbac.Role, error) {
	if authorizer == nil {
		return []rbac.Role{}, fmt.Errorf("attempt to assign default roles to user (%s) with no authorizer enabled", connUUID)
	}

	// assign admin role to user if room already exists,
	// but they are the only client assigned to it
	roleName := rbac.ADMIN_ROLE
	for _, c := range namespace.Connections() {
		if c.UUID() == connUUID {
			continue
		}

		roleName = rbac.USER_ROLE
	}

	role, found := authorizer.Role(roleName)
	if !found {
		return []rbac.Role{}, fmt.Errorf("unable to bind role %q to connection with id (%s): unable to find role", roleName, connUUID)
	}

	// if an "admin" role is computed for the current connection,
	// override any previous roles stored in the auth cookie. An
	// "admin" role takes precedence over any previously stored roles.
	if role.Name() == rbac.ADMIN_ROLE {
		log.Printf("INF SOCKET SERVER AUTHZ %q role was computed for current connection with id (%s). Ignoring previously stored roles in auth cookie...", "admin", connUUID)
		return []rbac.Role{role}, nil
	}

	// return role data saved in cookie - if not,
	// compute default roles based on given data
	roles, err := rolesFromCookie(r, authorizer, namespace)
	if err == nil && len(roles) > 0 {
		log.Printf("INF SOCKET SERVER AUTHZ found auth cookie with valid role data. Retrieving...\n")
		return roles, nil
	}

	log.Printf("ERR SOCKET SERVER AUTHZ unable to retrieve auth cookie data. Defaulting connection to non-admin role (%s): %v\n", role.Name(), err)
	return []rbac.Role{role}, nil
}

func GenerateAuthCookie(cookieData *rbac.AuthCookieData) (*http.Cookie, error) {
	data, err := cookieData.Serialize()
	if err != nil {
		return nil, err
	}

	month := 24 * time.Hour * 7 * 4

	return &http.Cookie{
		Name:     rbac.AuthCookieName,
		Value:    string(data),
		HttpOnly: false,
		Expires:  time.Now().Add(month), // set cookie lifetime to 1 month
	}, nil
}

func SetAuthCookie(w http.ResponseWriter, r *http.Request, namespace connection.Namespace, roles []rbac.Role) (bool, error) {
	cookie, created, err := UpdatedAuthCookie(r, namespace, roles)
	if err != nil {
		return false, err
	}

	// set both "Cookie" and "Set-Cookie" headers since
	// client might be making request via XMLHttpRequest
	// and cookie data might not update immediately.
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	w.Header().Set("Cookie", fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	return created, nil
}

// UpdatedAuthCookie receives a request, namespace, and set of roles
// and returns an existing auth cookie with its role data updated, or
// a new auth cookie with the given role data
func UpdatedAuthCookie(r *http.Request, namespace connection.Namespace, roles []rbac.Role) (*http.Cookie, bool, error) {
	cookie, err := r.Cookie(rbac.AuthCookieName)
	if err != nil {
		roleGroup := []string{}
		for _, r := range roles {
			roleGroup = append(roleGroup, r.Name())
		}

		var genErr error
		cookie, genErr = GenerateAuthCookie(&rbac.AuthCookieData{
			Namespaces: []*rbac.AuthCookieDataNs{
				{
					Id:    namespace.UUID(),
					Name:  namespace.Name(),
					Roles: roleGroup,
				},
			},
		})
		if genErr != nil {
			return nil, false, fmt.Errorf("unable to create cookie by name %q: %v", rbac.AuthCookieName, err)
		}

		return cookie, true, nil
	}

	// cookie exists - determine if namespace entry exists and replace or set its roles
	cookieData := &rbac.AuthCookieData{}
	if err := cookieData.Decode([]byte(cookie.Value)); err != nil {
		return nil, false, fmt.Errorf("unable to decode auth-cookie data %v: %v", cookie.Value, err)
	}

	// remove current namespace data from cookie (if any)
	newCookieData := &rbac.AuthCookieData{}
	for _, ns := range cookieData.Namespaces {
		if ns.Name != namespace.Name() {
			newCookieData.Namespaces = append(newCookieData.Namespaces, ns)
		}
	}

	roleGroup := []string{}
	for _, r := range roles {
		roleGroup = append(roleGroup, r.Name())
	}

	// create new namespace data entry with updated roles
	newCookieData.Namespaces = append(newCookieData.Namespaces, &rbac.AuthCookieDataNs{
		Id:    namespace.UUID(),
		Name:  namespace.Name(),
		Roles: roleGroup,
	})

	newCookie, err := GenerateAuthCookie(newCookieData)
	if err != nil {
		return nil, false, fmt.Errorf("unable to update cookie by name %q: %v", rbac.AuthCookieName, err)
	}

	return newCookie, false, nil
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
