package socket

import (
	"log"
	"net/http"

	sockio "github.com/googollee/go-socket.io"
)

type Server struct {
	*sockio.Server
}

func NewServer(transportNames []string) (*Server, error) {
	socketServer, err := sockio.NewServer(transportNames)
	if err != nil {
		return nil, err
	}

	s := &Server{
		socketServer,
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	s.Server.ServeHTTP(w, r)
}
