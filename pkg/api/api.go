package api

import (
	"io"
	"log"
	"net"
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

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	log.Printf("API Serving request from %s for endpoint %q\n", ip, r.URL.String())

	apiPath := strings.Split(r.URL.String(), ApiSuffix)[1]
	if len(apiPath) == 0 {
		discovery.ServeDiscoveryEndpoint(w, r)
		return
	}

	io.WriteString(w, "{}")
}

// HandleWatchRequest handles socket.io client requests for different api resources
func HandleWatchRequest(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "socket.io")
}

func handleApiStreams(w http.ResponseWriter, r *http.Request) {

}

func handleApiUsers(w http.ResponseWriter, r *http.Request) {

}

func handleApiMovies(w http.ResponseWriter, r *http.Request) {

}

func RegisterDefaultEndpoints() {
	registeredEndpoints = make(map[string]func(w http.ResponseWriter, r *http.Request))

	registeredEndpoints["/api/streams"] = handleApiStreams
	registeredEndpoints["/api/users"] = handleApiUsers
	registeredEndpoints["/api/movies"] = handleApiMovies
}

func init() {
	RegisterDefaultEndpoints()
}
