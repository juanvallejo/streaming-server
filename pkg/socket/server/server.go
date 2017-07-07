package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/socket/connection"

	"github.com/gorilla/websocket"
)

const (
	MAX_READ_BUF_SIZE  = 1024
	MAX_WRITE_BUF_SIZE = 1024
)

type ServerEventCallback func(connection.Connection)

type SocketServer interface {
	// On receives a string and a ServerEventCallback function and stores
	// the callback in an internal list, mapped to the given string.
	On(string, ServerEventCallback)
	// Emit receives a string and a Socket connection, and calls every ServerEventCallback
	// mapped to that string, passing the Socket connection as its only argument.
	Emit(string, connection.Connection)
}

// Server implements http.Handler and SocketServer
type Server struct {
	// callbacks stores event functions for socket connections
	callbacks map[string][]ServerEventCallback
	//
	handler connection.Handler
}

func (s *Server) On(eventName string, callback ServerEventCallback) {
	_, exists := s.callbacks[eventName]
	if !exists {
		s.callbacks[eventName] = []ServerEventCallback{}
	}

	s.callbacks[eventName] = append(s.callbacks[eventName], callback)
}

func (s *Server) Emit(eventName string, conn connection.Connection) {
	c, exists := s.callbacks[eventName]
	if !exists {
		return
	}

	for _, callback := range c {
		callback(conn)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("INF SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	conn, err := websocket.Upgrade(w, r, w.Header(), MAX_READ_BUF_SIZE, MAX_WRITE_BUF_SIZE)
	if err != nil {
		log.Printf("ERR SOCKET SERVER unable to upgrade connection for %q: %v", r.URL.String(), err)
		return
	}

	socketConn := s.handler.NewConnection(conn, r)
	s.Emit("connection", socketConn)

	s.handler.Handle(socketConn)
}

func NewServer() *Server {
	return &Server{
		callbacks: make(map[string][]ServerEventCallback),
		handler:   connection.NewHandler(),
	}
}

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
