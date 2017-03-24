package main

import (
	"flag"
	"log"
	"os"

	sockio "github.com/googollee/go-socket.io"
	"github.com/juanvallejo/streaming-server/pkg/server"
)

func main() {
	flag.Parse()

	sockiosrv, err := sockio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}

	// init http server with socket.io support
	application := server.New(&server.ServerOptions{
		Port:         "8080",
		Host:         "0.0.0.0",
		Out:          os.Stdout,
		SocketServer: sockiosrv,
	})
	application.Serve()

}
