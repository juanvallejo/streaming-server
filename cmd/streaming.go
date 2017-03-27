package main

import (
	"flag"
	"log"
	"os"

	sockio "github.com/googollee/go-socket.io"
	"github.com/juanvallejo/streaming-server/pkg/server"
	"github.com/juanvallejo/streaming-server/pkg/socket"
)

func main() {
	flag.Parse()

	sockiosrv, err := sockio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	sockHandler := socket.New(sockiosrv)

	// init http server with socket.io support
	application := server.New(&server.ServerOptions{
		Port:          "8080",
		Host:          "0.0.0.0",
		Out:           os.Stdout,
		SocketHandler: sockHandler,
	})
	application.Serve()

}
