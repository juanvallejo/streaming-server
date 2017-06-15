package path

import (
	"io"
	"log"
	"net/http"
)

var (
	NotFoundPathUrl = "/404"
	ErrorPathUrl    = "/500"

	ApiRootUrl    = "/api"
	FileRootUrl   = "/src/static"
	SocketRootUrl = "/socket.io"
	RoomRootUrl   = "/room"
	StreamRootUrl = "/stream"

	RoomRootRegex   = "^\\/v\\/.*"
	StreamRootRegex = "^\\/s\\/.*"

	StreamDataRootPath = "data"
	FileRootPath       = "pkg/webclient"
)

// Path is an interface representing an http url handler
type Path interface {
	// GetUrl returns the fully-qualified url of an http url handler
	GetUrl() string
	// Handle receives an http writer and http request and
	// responds to the request with the appropriate resources / response.
	// an error is returned if the request is not able to be handled.
	Handle(string, http.ResponseWriter, *http.Request) error
}

// PathHandler implements Path and provides
// default path-handling values
type PathHandler struct {
	pathUrl string
}

func (h PathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, "404: page not found.")
	return nil
}

func (h PathHandler) GetUrl() string {
	return h.pathUrl
}

func NewPathNotFound() PathHandler {
	return PathHandler{
		pathUrl: NotFoundPathUrl,
	}
}

type ErrorPathHandler struct {
	PathHandler
}

func (h ErrorPathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	log.Printf("WARN HTTP PATH attempt to handle server error")

	w.WriteHeader(http.StatusInternalServerError)
	io.WriteString(w, "500: internal server error.")
	return nil
}

func NewPathError() ErrorPathHandler {
	return ErrorPathHandler{
		PathHandler{
			pathUrl: ErrorPathUrl,
		},
	}
}

func HandleInvalidRange(w http.ResponseWriter, r *http.Request) {
	log.Printf("ERR HTTP PATH could not handle request with invalid range")

	w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	io.WriteString(w, "invalid range")
}