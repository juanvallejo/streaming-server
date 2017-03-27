package socket

import (
	"fmt"
	"log"
	"net/http"

	sockio "github.com/googollee/go-socket.io"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
)

type Handler struct {
	clientsById map[string]*client.Client

	SocketServer *sockio.Server
}

type ConnectionHandlerFunc func(*Handler, sockio.Socket)

var ConnectionHandler ConnectionHandlerFunc

// New creates a socket server handler
func New(server *sockio.Server) *Handler {
	return &Handler{
		clientsById:  make(map[string]*client.Client),
		SocketServer: server,
	}
}

func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	h.SocketServer.ServeHTTP(w, r)
	h.SocketServer.On("connection", func(sockioconn sockio.Socket) {
		ConnectionHandler(h, sockioconn)
	})
}

func (h *Handler) RegisterClient(sockioconn sockio.Socket) {
	log.Printf("SOCKET registering client with id %q\n", sockioconn.Id())

	c := client.New(sockioconn)
	h.clientsById[sockioconn.Id()] = c
}

func (h *Handler) DeregisterClient(sockioconn sockio.Socket) error {
	if _, ok := h.clientsById[sockioconn.Id()]; ok {
		delete(h.clientsById, sockioconn.Id())
		return nil
	}

	return fmt.Errorf("error: unable to deregister client with id %q.", sockioconn.Id())
}

func (h *Handler) GetClientSize() int {
	return len(h.clientsById)
}

func defaultConnectionHandler(handler *Handler, conn sockio.Socket) {
	log.Printf("SOCKET CONN client (%s) has connected with id %q\n", conn.Request().RemoteAddr, conn.Id())

	handler.RegisterClient(conn)
	log.Printf("SOCKET currently %v clients registered\n", handler.GetClientSize())

	conn.On("disconnection", func() {
		log.Printf("SOCKET DCONN client with id %q has disconnected\n", conn.Id())
		err := handler.DeregisterClient(conn)
		if err != nil {
			log.Printf("SOCKET ERR %v", err)
		}
	})

}

func init() {
	ConnectionHandler = defaultConnectionHandler
}
