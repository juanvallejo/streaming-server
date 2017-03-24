package socket

import (
	"net/http"
	"strings"
)

// retrieve a client's origin consisting of
// protocol://hostname:port for a given request.
// if a given request had no easily disernable
// origin path, a wildcard origin is returned.
func getClientOrigin(r *http.Request) string {
	origin := "*"
	clientPath := r.Referer()

	clientProto := strings.Split(clientPath, "://")
	if len(clientProto) > 1 {
		clientHost := strings.Split(clientProto[1], "/")
		origin = clientProto[0] + "://" + clientHost[0]
	}

	return origin
}
