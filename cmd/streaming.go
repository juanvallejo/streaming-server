package main

import (
	"flag"
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
	flag.Parse()

	authorizer := rbac.NewAuthorizer()
	cmd.AddDefaultRoles(authorizer)

	socketHandler := socket.NewHandler(
		connection.NewHandlerWithRBAC(authorizer),
		cmd.NewHandlerWithRBAC(authorizer),
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
