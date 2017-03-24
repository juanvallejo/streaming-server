package socket

import (
	"log"
	"net/http"

	sockio "github.com/googollee/go-socket.io"
)

type ConnectionHandlerFunc func(sockio.Socket)

var ConnectionHandler ConnectionHandlerFunc

func HandleSocketRequest(handler *sockio.Server, w http.ResponseWriter, r *http.Request) {
	origin := getClientOrigin(r)
	log.Printf("SOCKET handling socket request for ref %q\n", origin)

	// allow specific request origin access with credentials
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	handler.ServeHTTP(w, r)

	handler.On("connection", ConnectionHandler)
}

func defaultConnectionHandler(client sockio.Socket) {
	log.Printf("SOCKET client (%s) has connected with id %q\n", client.Request().RemoteAddr, client.Id())

	// TODO: socket.RegisterClient
}

func init() {
	ConnectionHandler = defaultConnectionHandler
}
