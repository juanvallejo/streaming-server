package endpoint

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// ApiEndpoint provides a REST handler for an api request
type ApiEndpoint interface {
	// GetPath returns the full api request path for the current endpoint handler
	GetPath() string
	// Handle receives an array of url "segments", and an http writer and request.
	// "segments" are defined as a string slice consisting of each piece of the
	// api request path, minus the api request root ("/api/"). Hence for a request
	// path "/api/stream/verb/noun", the "segments" received would be:
	//   ["stream", "verb", "noun"].
	Handle([]string, http.ResponseWriter, *http.Request)
}

type ApiEndpointSchema struct {
	path string
}

type ApiErrorResponse struct {
	Error    string `json:"error"`
	HTTPCode int    `json:"httpCode"`
}

func (e *ApiEndpointSchema) GetPath() string {
	return e.path
}

func HandleEndpointError(err error, w http.ResponseWriter) {
	message := fmt.Sprintf("error: %v", err)

	res := ApiErrorResponse{
		Error:    message,
		HTTPCode: http.StatusInternalServerError,
	}

	b, err := json.Marshal(res)
	if err != nil {
		log.Panic("unable to marshal api error response")
	}

	w.Write(b)
}

func HandleEndpointNotFound(w http.ResponseWriter) {
	res := ApiErrorResponse{
		Error:    "endpoint not found",
		HTTPCode: http.StatusNotFound,
	}

	b, err := json.Marshal(res)
	if err != nil {
		log.Panic("unable to marshal api error response")
	}

	w.Write(b)
}
