package endpoint

import (
	"fmt"
	"log"
	"net/http"

	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
)

const (
	AUTH_ENDPOINT_PREFIX = "/auth"
	CONN_ID_KEY          = "id"
)

// AuthEndpoint implements ApiEndpoint
type AuthEndpoint struct {
	*ApiEndpointSchema
}

// sets a cookie with rbac roles from the given connection id
func (e *AuthEndpoint) Handle(connHandler connection.ConnectionHandler, segments []string, w http.ResponseWriter, r *http.Request) {
	if len(segments) < 2 {
		HandleEndpointError(fmt.Errorf("unimplemented endpoint"), w)
		return
	}

	// no-op if authorizer does not exist
	authorizer := connHandler.Authorizer()
	if authorizer == nil {
		HandleEndpointError(fmt.Errorf("authorizer not enabled; endpoint unavailable"), w)
		return
	}

	connId := r.URL.Query().Get(CONN_ID_KEY)
	if len(connId) == 0 {
		HandleEndpointError(fmt.Errorf("missing required parameter: id"), w)
		return
	}

	referrer := r.Referer()
	if len(referrer) == 0 {
		HandleEndpointError(fmt.Errorf("requests to this endpoint must be made from a different endpoint"), w)
		return
	}

	conn, exists := connHandler.Connection(connId)
	if !exists {
		HandleEndpointError(fmt.Errorf("unable to find connection by id %v", connId), w)
		return
	}

	switch {
	case segments[1] == "save":
		fallthrough
	case segments[1] == "cookie":
		handleCookieReq(conn, connHandler, w, r)
		return
	case segments[1] == "set":
		fallthrough
	case segments[1] == "init":
		handleInitReq(conn, connHandler, w, r)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented endpoint"), w)
}

// handleInitReq initializes a connection's rbac roles from an auth cookie
// or creates an auth cookie with default rbac roles if one does not exist.
// roles are then bound to the given connection id.
func handleInitReq(conn connection.Connection, handler connection.ConnectionHandler, w http.ResponseWriter, r *http.Request) {
	ns, exists := conn.Namespace()
	if !exists {
		HandleEndpointError(fmt.Errorf("the connection specified has not been bound to a namespace"), w)
		return
	}

	roles, err := util.DefaultRoles(r, handler.Authorizer(), conn.UUID(), ns)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	_, err = util.SetAuthCookie(w, r, ns, roles)
	if err != nil {
		HandleEndpointError(fmt.Errorf("unable to set auth cookie: %v", err), w)
		return
	}

	// bind roles to connection
	for _, r := range roles {
		if handler.Authorizer().Bind(r, conn) {
			log.Printf("INF API AUTHZ bound role %q to connection with id (%s)", r.Name(), conn.UUID())
		}
	}
}

// TODO: this endpoint must be accessible to non-privileged clients.
// secure by having one-time tokens that must be provided as part of
// a request, via a "token" parameter.
func handleCookieReq(conn connection.Connection, handler connection.ConnectionHandler, w http.ResponseWriter, r *http.Request) {
	ns, exists := conn.Namespace()
	if !exists {
		HandleEndpointError(fmt.Errorf("the connection specified has not been bound to a namespace"), w)
		return
	}

	roles := []rbac.Role{}
	roleNames := []string{}
	for _, b := range handler.Authorizer().Bindings() {
		for _, s := range b.Subjects() {
			if s.UUID() == conn.UUID() {
				roles = append(roles, b.Role())
				roleNames = append(roleNames, b.Role().Name())
				break
			}
		}
	}

	cookie, _, err := util.UpdatedAuthCookie(r, ns, roles)
	if err != nil {
		HandleEndpointError(fmt.Errorf("unable to set auth cookie: %v", err), w)
		return
	}

	// set both "Cookie" and "Set-Cookie" headers since
	// client might be making request via XMLHttpRequest
	// and cookie data might not update immediately.
	http.SetCookie(w, cookie)
	r.AddCookie(cookie)
	w.Header().Set("Cookie", fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))

	HandleEndpointSuccess(fmt.Sprintf("successfully saved roles (%v) for id %v", roleNames, conn.UUID()), w)
}

func NewAuthEndpoint() ApiEndpoint {
	return &AuthEndpoint{
		&ApiEndpointSchema{
			path: AUTH_ENDPOINT_PREFIX,
		},
	}
}
