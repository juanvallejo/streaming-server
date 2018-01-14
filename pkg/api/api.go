package api

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api/discovery"
	"github.com/juanvallejo/streaming-server/pkg/api/endpoint"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

const (
	ApiPrefix = "/api"
)

var (
	ErrMalformed = errors.New("malformed api endpoint; should be /api/...")
)

type ApiHandlerFunc func(w http.ResponseWriter, r *http.Request)

// Handler provides api handler methods
// and implements http.Handler
type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// ApiHandler implements Handler
type ApiHandler struct {
	endpoints   map[string]endpoint.ApiEndpoint
	connections connection.ConnectionHandler
}

func (h *ApiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.String(), ApiPrefix) {
		panic("API Request could not be handled for non api-formatted url")
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	log.Printf("INF API Serving request from %s for endpoint %q\n", ip, r.URL.Path)

	h.HandleEndpoint(r.URL, w, r)
}

func (h *ApiHandler) HandleEndpoint(url *url.URL, w http.ResponseWriter, r *http.Request) {
	path := url.Path

	// sanitize api string; if it ends in a "/", remove...
	if string(path[len(path)-1]) == "/" {
		path = path[0 : len(path)-1]
	}

	segs := strings.Split(path, "/")
	if "/"+segs[1] != ApiPrefix {
		endpoint.HandleEndpointError(ErrMalformed, w)
		return
	}

	if len(segs) < 3 || (len(segs) == 3 && len(segs[2]) == 0) {
		// if two, path was /api
		if len(segs) > 1 && ("/"+segs[1]) == ApiPrefix {
			discovery.ServeDiscoveryEndpoint(w, r)
			return
		}

		endpoint.HandleEndpointError(ErrMalformed, w)
		return
	}

	root := "/" + segs[2] // segs[1] should be ApiPrefix

	if e, exists := h.endpoints[ApiPrefix+root]; exists {
		e.Handle(h.connections, segs[2:], w, r)
		return
	}

	log.Printf("ERR API unable to handle missing endpoint %q", path)
	endpoint.HandleEndpointNotFound(w)
}

func (h *ApiHandler) RegisterEndpoint(e endpoint.ApiEndpoint) {
	if e, exists := h.endpoints[ApiPrefix+e.GetPath()]; exists {
		log.Panic("ERR API attempt to register existing endpoint: " + e.GetPath())
	}

	h.endpoints[ApiPrefix+e.GetPath()] = e

}

func NewHandler(connHandler connection.ConnectionHandler) Handler {
	handler := &ApiHandler{
		endpoints:   make(map[string]endpoint.ApiEndpoint),
		connections: connHandler,
	}
	handler.registerDefaultEndpoints()
	return handler
}

func (h *ApiHandler) registerDefaultEndpoints() {
	h.RegisterEndpoint(endpoint.NewStreamEndpoint())
	h.RegisterEndpoint(endpoint.NewYoutubeEndpoint())
	h.RegisterEndpoint(endpoint.NewTwitchEndpoint())
	h.RegisterEndpoint(endpoint.NewAuthEndpoint())
}
