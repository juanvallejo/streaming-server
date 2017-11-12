package main

import (
	"flag"
	"log"
	"os"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/server"
	"github.com/juanvallejo/streaming-server/pkg/socket"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

func main() {
	port := flag.String("port", "8080", "default port to listen on")
	authz := flag.Bool("rbac", false, "enable role-based access control for request commands.")
	flag.Parse()

	connHandler := connection.NewHandler()
	cmdHandler := cmd.NewHandler()

	if *authz {
		log.Printf("INF AUTHZ rbac authorization enabled.\n")

		authorizer := rbac.NewAuthorizer()
		cmd.AddDefaultRoles(authorizer)

		connHandler = connection.NewHandlerWithRBAC(authorizer)
		cmdHandler = cmd.NewHandlerWithRBAC(authorizer)

	}

	socketHandler := socket.NewHandler(
		connHandler,
		cmdHandler,
		client.NewHandler(),
		playback.NewGarbageCollectedHandler(),
		stream.NewGarbageCollectedHandler(),
	)

	requestHandler := server.NewRequestHandler(socketHandler)

	// init http server with socket.io support
	application := server.NewServer(requestHandler, &server.ServerOptions{
		Port: *port,
		Host: "0.0.0.0",
		Out:  os.Stdout,
	})
	application.Serve()
}
