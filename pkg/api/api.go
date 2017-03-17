package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api/discovery"
)

const ApiSuffix = "/api"

var registeredEndpoints map[string]func(w http.ResponseWriter, r *http.Request)

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.String(), ApiSuffix) {
		panic("API Request could not be handled for non api-formatted url")
	}

	apiPath := strings.Split(r.URL.String(), ApiSuffix)[1]
	if len(apiPath) == 0 {
		discovery.ServeDiscoveryEndpoint(w, r)
		return
	}

	io.WriteString(w, "{}")
}

func RegisterDefaultEndpoints() {
	// registeredEndpoints["/api/stream"] =
}

func init() {
	RegisterDefaultEndpoints()
}
