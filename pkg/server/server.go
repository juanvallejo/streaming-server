package server

import (
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api"
	"github.com/juanvallejo/streaming-server/pkg/socket"
)

const (
	defaultPort string = "8080"
	defaultHost string = "0.0.0.0"

	RootHTMLPath = "pkg/html"

	apiBaseUrl    = "/api"
	staticBaseUrl = "/static"
	socketBaseUrl = "/socket.io"
)

type ServerOptions struct {
	Host string
	Out  io.Writer
	Port string

	Server *http.Server
}

// map of req strings to file names
var fileHandlers map[string]string
var reqHandlers map[string]func(http.ResponseWriter, *http.Request)

// Handler handles http and socket.io requests
type Handler struct {
	socketServer *socket.Server
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("HTTP handling request %q", r.URL.String())

	if h, ok := reqHandlers[r.URL.String()]; ok {
		h(w, r)
		return
	}

	// handle socket.io endpoint requests
	if strings.HasPrefix(r.URL.String(), socketBaseUrl) {
		if h.socketServer != nil {
			h.socketServer.HandleRequest(w, r)
			return
		}
	}

	// handle wildcard urls for static ui files
	reg := regexp.MustCompile(staticBaseUrl)
	if reg.MatchString(r.URL.String()) {
		if h, ok := reqHandlers[staticBaseUrl]; ok {
			h(w, r)
			return
		}
	}

	// handle wildcard urls for api requests
	if strings.HasPrefix(r.URL.String(), apiBaseUrl) {
		if h, ok := reqHandlers[apiBaseUrl]; ok {
			h(w, r)
			return
		}
	}

	handleNotFoundReq(w, r)
}

func addHandlers() {
	reqHandlers = make(map[string]func(http.ResponseWriter, *http.Request))
	reqHandlers["/"] = handleRootReq
	reqHandlers["/static"] = handleStaticReq
	reqHandlers["/api"] = handleApiReq

	fileHandlers = make(map[string]string)
	fileHandlers["/"] = "index.html"
}

// New creates a new server from server options
// and ensures that at least a host and a port
// have been set.
func NewServer(socketServer *socket.Server, opts *ServerOptions) *ServerOptions {
	if len(opts.Port) == 0 {
		opts.Port = defaultPort
	}
	if len(opts.Host) == 0 {
		opts.Host = defaultHost
	}
	if opts.Out == nil {
		panic("No output method defined.")
	}

	opts.Server = &http.Server{
		Addr: opts.getAddr(),
		Handler: &Handler{
			socketServer: socketServer,
		},
	}

	addHandlers()
	return opts
}

// Serve starts an http server using specified settings.
func (s *ServerOptions) Serve() {
	log.Printf("HTTP Serving on %s\n", s.getAddr())

	err := s.Server.ListenAndServe()
	if err != nil {
		panic(err.Error())
	}
}

func (s *ServerOptions) getAddr() string {
	return s.Host + ":" + s.Port
}

func handleRootReq(w http.ResponseWriter, r *http.Request) {
	if fileName, ok := fileHandlers[r.URL.String()]; ok {
		log.Printf("HTTP Serving root request with filename %q\n", fileName)
		http.ServeFile(w, r, RootHTMLPath+"/"+fileName)
		return
	}

	log.Printf("HTTP Attempted to serve root with unknown path %s\n", r.URL.String())
	handleNotFoundReq(w, r)
}

func handleStaticReq(w http.ResponseWriter, r *http.Request) {
	if fileName, ok := fileHandlers[r.URL.String()]; ok {
		log.Printf("Serving static file %q with path %q\n", fileName, RootHTMLPath+fileName)
		http.ServeFile(w, r, RootHTMLPath+fileName)
		return
	}

	if len(r.URL.String()) == 0 {
		log.Printf("HTTP Static file requested, but request was empty\n")
		handleNotFoundReq(w, r)
		return
	}

	log.Printf("HTTP Attempting to serve static file without map %q\n", RootHTMLPath+r.URL.String())
	http.ServeFile(w, r, RootHTMLPath+r.URL.String())
}

func handleApiReq(w http.ResponseWriter, r *http.Request) {
	api.HandleRequest(w, r)
}

func handleNotFoundReq(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, "404: File not found.")
}
