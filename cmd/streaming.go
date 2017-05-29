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
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

func main() {
	flag.Parse()

	//socketServer, err := socket.NewServer(nil)
	//if err != nil {
	//	log.Fatal(err)
	//}

	socketHandler := socket.NewHandler(cmd.NewHandler(), client.NewHandler(), playback.NewHandler(), stream.NewHandler())
	socketServer, err := socket.NewServer(nil, socketHandler)
	if err != nil {
		log.Fatal(err)
	}

	// init http server with socket.io support
	application := server.NewServer(socketServer, &server.ServerOptions{
		Port: "8080",
		Host: "0.0.0.0",
		Out:  os.Stdout,
	})
	application.Serve()

}
