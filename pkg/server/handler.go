package server

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api"
	"github.com/juanvallejo/streaming-server/pkg/server/path"
	"github.com/juanvallejo/streaming-server/pkg/socket"
)

// RequestHandler implements http.Handler and provides additional websocket request handling
type RequestHandler struct {
	router         *RequestRouter
	paths          map[string]path.Path
	sockReqHandler *socket.Handler
}

func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := h.router.Route(r.URL.String())

	// handle websocket requests
	if strings.HasPrefix(url, path.SocketRootUrl) {
		if h.sockReqHandler != nil {
			h.sockReqHandler.ServeHTTP(w, r)
			return
		}

		log.Printf("WARN HTTP received websocket request but no websocket handler defined. Ignoring...")
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

	// handle wildcard urls for api requests
	if strings.HasPrefix(url, path.ApiRootUrl) {
		api.ServeHTTP(w, r)
		return
	}

	h.HandlePath(url, w, r)
}

// Handle performs a path-url lookup. If a path by a given url
// has not been registered, a boolean false is returned.
// If an error occurs while handling a path, a boolean true is
// returned, as the path exists, and the error is returned.
func (h *RequestHandler) HandlePath(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("INFO HTTP PATH handling path with url %q", url)

	p, exists := h.paths[url]
	if !exists {
		h.HandleNotFound(url, w, r)
		return
	}

	err := p.Handle(url, w, r)
	if err != nil {
		h.HandleError(url, w, r)
		return
	}
}

func (h *RequestHandler) HandleError(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("ERR HTTP PATH handling error; ocurred handling url %q", r.URL.String())

	if p500, exists := h.paths[path.ErrorPathUrl]; exists {
		p500.Handle(url, w, r)
		return
	}

	log.Fatal("request handler has no path error handler")
}

func (h *RequestHandler) HandleNotFound(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("WARN HTTP PATH handler for path with url %q was not found", url)

	if p404, exists := h.paths[path.NotFoundPathUrl]; exists {
		p404.Handle(url, w, r)
		return
	}

	log.Fatal("request handler has no path-not-found handler")
}

func (h *RequestHandler) HandleFile(url string, w http.ResponseWriter, r *http.Request) {
	if len(url) == 0 {
		log.Printf("WARN HTTP Static file requested, but request was empty\n")
		h.HandleNotFound(url, w, r)
		return
	}

	log.Printf("INFO HTTP PATH Attempting to serve static file %q\n", path.FilePathFromUrl(url))
	http.ServeFile(w, r, path.FilePathFromUrl(url))
}

func (h *RequestHandler) HandleRoom(url string, w http.ResponseWriter, r *http.Request) {
	log.Printf("INFO HTTP PATH handler for path with url %q matched room name pattern", url)

	if p, exists := h.paths[path.RoomRootUrl]; exists {
		p.Handle(url, w, r)
		return
	}

	log.Printf("WARN HTTP PATH handler for room path request (%q) not found", url)
}

func (h *RequestHandler) RegisterPath(p path.Path) {
	h.paths[p.GetUrl()] = p
}

func NewRequestHandler(socketRequestHandler *socket.Handler) *RequestHandler {
	handler := &RequestHandler{
		router:         NewRequestRouter(),
		paths:          make(map[string]path.Path),
		sockReqHandler: socketRequestHandler,
	}
	addRequestHandlers(handler)
	return handler
}

func addRequestHandlers(handler *RequestHandler) {
	handler.RegisterPath(path.NewPathNotFound())
	handler.RegisterPath(path.NewPathError())
	handler.RegisterPath(path.NewPathRoot())
	handler.RegisterPath(path.NewPathRoom())
}
