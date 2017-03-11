package main

import "github.com/juanvallejo/streaming-server/pkg/server"

func main() {
	application := server.New(&server.ServerOptions{
		Port: "8000",
		Host: "0.0.0.0",
	})
	application.Serve()
}
