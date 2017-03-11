package server

import (
	"io"
	"net/http"
)

const (
	defaultPort string = "8000"
	defaultHost string = "0.0.0.0"
)

type ServerOptions struct {
	Port string
	Host string

	server *http.Server
}

var handlers map[string]func(http.ResponseWriter, *http.Request)

type Handler struct{}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h, ok := handlers[r.URL.String()]; ok {
		h(w, r)
		return
	}

	handleAnyReq(w, r)
}

func addHandlers() {
	handlers = make(map[string]func(http.ResponseWriter, *http.Request))
	handlers["/"] = handleRootReq
}

// New creates a new server from server options
// and ensures that at least a host and a port
// have been set.
func New(opts *ServerOptions) *ServerOptions {
	if len(opts.Port) == 0 {
		opts.Port = defaultPort
	}
	if len(opts.Host) == 0 {
		opts.Host = defaultHost
	}
	if opts.server == nil {
		opts.server = &http.Server{
			Addr:    opts.getAddr(),
			Handler: &Handler{},
		}
	}

	addHandlers()
	return opts
}

// Serve starts an http server
// using specified settings.
func (s *ServerOptions) Serve() {
	s.server.ListenAndServe()
}

func (s *ServerOptions) getAddr() string {
	return s.Host + ":" + s.Port
}

func handleRootReq(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello World!")
}

func handleAnyReq(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, "404: File not found.")
}
