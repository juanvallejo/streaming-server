package server

import (
	"log"

	"github.com/juanvallejo/streaming-server/pkg/server/path"
)

type RequestRouter struct {
	routes map[string]string
}

// Route receives a request url string and returns
// the mapped value (if one exists), or the url string
func (r *RequestRouter) Route(url string) string {
	if routed, exists := r.routes[url]; exists {
		log.Printf("INF HTTP ROUTER routing request (%s -> %s)", url, routed)
		return routed
	}

	return url
}

func (r *RequestRouter) AddRoute(from, to string) {
	r.routes[from] = to
}

func NewRequestRouter() *RequestRouter {
	r := &RequestRouter{
		routes: make(map[string]string),
	}
	addDefaultRoutes(r)
	return r
}

func addDefaultRoutes(r *RequestRouter) {
	r.AddRoute("/favicon.ico", path.FileRootUrl+"/favicon.ico")
}
