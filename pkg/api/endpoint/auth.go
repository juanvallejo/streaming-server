package endpoint

import (
	"fmt"
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

	switch {
	case segments[1] == "cookie":
		handleCookieReq(connHandler, w, r)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented endpoint"), w)
}

// TODO: this endpoint must be accessible to non-privileged clients.
// secure by having one-time tokens that must be provided as part of
// a request, via a "token" parameter.
func handleCookieReq(handler connection.ConnectionHandler, w http.ResponseWriter, r *http.Request) {
	// no-op if authorizer does not exist
	authorizer := handler.Authorizer()
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

	conn, exists := handler.Connection(connId)
	if !exists {
		HandleEndpointError(fmt.Errorf("unable to find connection by id %v", connId), w)
		return
	}

	ns, exists := conn.Namespace()
	if !exists {
		HandleEndpointError(fmt.Errorf("the connection specified has not been bound to a namespace"), w)
		return
	}

	roles := []rbac.Role{}
	roleNames := []string{}
	for _, b := range authorizer.Bindings() {
		for _, s := range b.Subjects() {
			if s.UUID() == conn.UUID() {
				roles = append(roles, b.Role())
				roleNames = append(roleNames, b.Role().Name())
				break
			}
		}
	}

	createdCookie, err := util.SetAuthCookie(w, r, ns, roles)
	if err != nil {
		HandleEndpointError(fmt.Errorf("unable to save auth cookie with roles for conneciton with id %q: %v", connId, err), w)
		return
	}

	msg := fmt.Sprintf("successfully saved roles (%v) for id %v", roleNames, connId)
	if createdCookie {
		msg = fmt.Sprintf("successfully saved new roles (%v) for id %v", roleNames, connId)
	}

	HandleEndpointSuccess(msg, w)
}

func NewAuthEndpoint() ApiEndpoint {
	return &AuthEndpoint{
		&ApiEndpointSchema{
			path: AUTH_ENDPOINT_PREFIX,
		},
	}
}
