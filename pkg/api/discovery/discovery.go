package discovery

import (
	"io"
	"net/http"
)

func ServeDiscoveryEndpoint(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "{discovery}")
}
