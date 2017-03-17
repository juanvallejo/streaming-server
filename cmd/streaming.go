package main

import (
	"flag"
	"os"

	"github.com/juanvallejo/streaming-server/pkg/server"
)

func main() {
	flag.Parse()

	application := server.New(&server.ServerOptions{
		Port: "8080",
		Host: "0.0.0.0",
		Out:  os.Stdout,
	})
	application.Serve()
}
