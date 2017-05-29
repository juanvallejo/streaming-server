package socket

import (
	"log"
	"net/http"

	sockio "github.com/googollee/go-socket.io"
)

type Server struct {
	*sockio.Server
}

func NewServer(transportNames []string, handler *Handler) (*Server, error) {
	socketServer, err := sockio.NewServer(transportNames)
	if err != nil {
		return nil, err
	}

	s := &Server{
		socketServer,
	}
	s.addServerHandlers(handler)

	return s, nil
}

func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	s.ServeHTTP(w, r)
}

func (s *Server) addServerHandlers(handler *Handler) {
	s.On("connection", func(sockioconn sockio.Socket) {
		handler.HandleClientConnection(sockioconn)
	})
}
