package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/juanvallejo/streaming-server/tools/analytics/embed"
)

const (
	defaultEmbedCodeSourceFilename = "scripts/analytics_embed_code.txt"
)

func main() {
	flag.Parse()

	if err := embed.Embed("pkg/webclient/src/static/index.html", defaultEmbedCodeSourceFilename, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
		return
	}
}
