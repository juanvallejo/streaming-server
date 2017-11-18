package server

import (
	"io"
	"log"
	"net/http"
)

const (
	defaultPort string = "8080"
	defaultHost string = "0.0.0.0"
)

type ServerOptions struct {
	Host string
	Out  io.Writer
	Port string

	Server *http.Server
}

// New creates a new server from server options
// and ensures that at least a host and a port
// have been set.
func NewServer(requestHandler *RequestHandler, opts *ServerOptions) *ServerOptions {
	if len(opts.Port) == 0 {
		opts.Port = defaultPort
	}
	if len(opts.Host) == 0 {
		opts.Host = defaultHost
	}
	if opts.Out == nil {
		panic("No output method defined.")
	}

	if requestHandler == nil {
		panic("no request handler defined")
	}

	opts.Server = &http.Server{
		Addr:    opts.getAddr(),
		Handler: requestHandler,
	}
	return opts
}

// Serve starts an http server using specified settings.
func (s *ServerOptions) Serve() {
	log.Printf("INF HTTP Serving on %s\n", s.getAddr())

	err := s.Server.ListenAndServe()
	if err != nil {
		panic(err.Error())
	}
}

func (s *ServerOptions) getAddr() string {
	return s.Host + ":" + s.Port
}
