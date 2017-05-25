package main

import (
	"flag"
	"log"
	"os"

	sockio "github.com/googollee/go-socket.io"

	"github.com/juanvallejo/streaming-server/pkg/server"
	"github.com/juanvallejo/streaming-server/pkg/socket"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

func main() {
	flag.Parse()

	socketServer, err := sockio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	socketHandler := socket.New(socketServer, cmd.NewHandler(), client.NewHandler(), playback.NewHandler())

	// init http server with socket.io support
	application := server.New(&server.ServerOptions{
		Port:          "8080",
		Host:          "0.0.0.0",
		Out:           os.Stdout,
		SocketHandler: socketHandler,
	})
	application.Serve()

}
