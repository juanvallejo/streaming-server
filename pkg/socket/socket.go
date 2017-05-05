package socket

import (
	"log"
	"net/http"

	sockio "github.com/googollee/go-socket.io"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type Socket struct {
	ConnectionHandler *connHandler
	SocketServer      *sockio.Server
	StreamPlayback    *playback.Playback
}

// New creates a socket server connection handler
func New(server *sockio.Server, playback *playback.Playback) *Socket {
	return &Socket{
		ConnectionHandler: &connHandler{
			clientsById: make(map[string]*client.Client),

			CommandHandler: cmd.NewHandler(),
			StreamPlayback: playback,
		},
		SocketServer: server,
	}
}

func (s *Socket) HandleRequest(w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	s.SocketServer.ServeHTTP(w, r)
	s.SocketServer.On("connection", func(sockioconn sockio.Socket) {
		s.ConnectionHandler.Handle(sockioconn)
	})
}
