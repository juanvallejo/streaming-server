package server

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api"
	"github.com/juanvallejo/streaming-server/pkg/server/path"
	"github.com/juanvallejo/streaming-server/pkg/socket"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

// RequestHandler implements http.Handler and provides additional websocket request handling.
// It also stores generic path handlers for request not matching a specific set of criteria.
type RequestHandler struct {
	router         *RequestRouter
	paths          map[string]path.Path
	sockReqHandler *socket.Handler
	apiHandler     api.Handler
}

// ServeHTTP handles incoming http requests. A request is handled based on its
// url string. The following steps are taken when evaluating a request's url:
//   1. If a url's prefix or pattern matches a socket.io request pattern ("/socker.io/...")
//      then the socketRequestHandler is relayed the request altogether.
//   2. If a socketRequestHandler has not been defined, or the url matches a file-root
//      location pattern ("/src/static/...") then it is served as a static file.
//   3. If a url matches a room request regex pattern ("/v/..."), then the room index file
//      is served back to the client.
//   4. If a url begins with an api request prefix ("/api/..."), then the api handler
//      is relayed the request entirely.
//	 5. If a url does not match any of the above patterns, it is then treated as a generic
// 		"path", which requires a path-handler for that specific url to have been registered.
func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := h.router.Route(r.URL.String())
	segs := strings.Split(url, "/")

	// handle websocket requests
	if "/"+segs[1] == path.SocketRootUrl {
		if h.sockReqHandler != nil {
			h.sockReqHandler.ServeHTTP(w, r)
			return
		}

		log.Printf("WRN HTTP received websocket request but no websocket handler defined. Ignoring...")
	}

	// handle urls for static files
	if strings.HasPrefix(url, path.FileRootUrl) {
		h.HandleFile(url, w, r)
		return
	}

	// handle wildcard urls for rooms
	reg := regexp.MustCompile(path.RoomRootRegex)
	if reg.MatchString(url) {
		h.HandleRoom(url, w, r)
		return
	}

	// handle wildcard urls for streams
	reg = regexp.MustCompile(path.StreamRootRegex)
	if reg.MatchString(url) {
		h.HandleStream(url, w, r)
		return
	}

	// handle urls for api requests
	if "/"+segs[1] == path.ApiRootUrl {
		h.apiHandler.ServeHTTP(w, r)
		return
	}

	h.HandlePath(url, w, r)
}

// Handle performs a path-url lookup. If a path by a given url
// has not been registered, a boolean false is returned.
// If an error occurs while handling a path, a boolean true is
// returned, as the path exists, and the error is returned.
func (h *RequestHandler) HandlePath(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("INF HTTP PATH handling path with url %q", r.URL.String())

	p, exists := h.paths[url]
	if !exists {
		path.HandleNotFound(url, w, r)
		return
	}

	err := p.Handle(url, w, r)
	if err != nil {
		log.Printf("ERR HTTP PATH error handling request (%s): %v", r.URL.String(), err)
		path.HandleServerError(url, w, r)
		return
	}
}

func (h *RequestHandler) HandleFile(url string, w http.ResponseWriter, r *http.Request) {
	if len(url) == 0 {
		log.Printf("WRN HTTP Static file requested, but request was empty\n")
		path.HandleNotFound(url, w, r)
		return
	}

	log.Printf("INF HTTP PATH Attempting to serve static file %q\n", path.FilePathFromUrl(url))
	http.ServeFile(w, r, path.FilePathFromUrl(url))
}

func (h *RequestHandler) HandleRoom(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("INF HTTP PATH handler for path with url %q matched room name pattern", url)

	// determine if a room name was given after the RoomRootUrl
	segs := strings.Split(url, path.RoomRootPrefix)
	key := segs[len(segs) - 1]
	if len(key) == 0 {
		path.RedirectHome(url, w, r)
		return
	}

	h.HandlePath(path.RoomRootUrl, w, r)
}

func (h *RequestHandler) HandleStream(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("INF HTTP PATH handler for path with url %q matched stream name pattern", url)
	h.HandlePath(path.StreamRootUrl, w, r)
}

func (h *RequestHandler) RegisterPath(p path.Path) {
	h.paths[p.GetUrl()] = p
}

func NewRequestHandler(socketRequestHandler *socket.Handler, connHandler connection.ConnectionHandler) *RequestHandler {
	handler := &RequestHandler{
		router:         NewRequestRouter(),
		paths:          make(map[string]path.Path),
		sockReqHandler: socketRequestHandler,
		apiHandler:     api.NewHandler(connHandler),
	}
	addRequestHandlers(handler)
	return handler
}

func addRequestHandlers(handler *RequestHandler) {
	handler.RegisterPath(path.NewPathRoot())
	handler.RegisterPath(path.NewPathRoom())
	handler.RegisterPath(path.NewPathStream())
}
