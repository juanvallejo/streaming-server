package endpoint

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api/config"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

const YOUTUBE_ENDPOINT_PREFIX = "/youtube"

var (
	youtubeMaxResults       = 20
	youtubeEndpointTemplate = "https://www.googleapis.com/youtube/v3/search?part=snippet&q=%v&type=video&maxResults=%v&key=%v"
)

// YoutubeEndpoint implements ApiEndpoint
type YoutubeEndpoint struct {
	*ApiEndpointSchema
}

// Handle returns a "discovery" of all local streams in the server data root.
func (e *YoutubeEndpoint) Handle(connHandler connection.ConnectionHandler, segments []string, w http.ResponseWriter, r *http.Request) {
	if len(segments) < 2 {
		HandleEndpointError(fmt.Errorf("unimplemented endpoint"), w)
		return
	}

	// since we are dealing with a url value, split
	// the unsanitized variant of the request path
	// containing the url encoded value
	segments = strings.Split(r.URL.String(), "/")
	segments = segments[2:]

	switch {
	case segments[1] == "search":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /search/query"), w)
			return
		}

		handleApiSearch(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

func handleApiSearch(searchQuery string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(youtubeEndpointTemplate, searchQuery, youtubeMaxResults, config.YT_API_KEY)
	res, err := http.Get(reqUrl)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(data)
}

func NewYoutubeEndpoint() ApiEndpoint {
	return &YoutubeEndpoint{
		&ApiEndpointSchema{
			path: YOUTUBE_ENDPOINT_PREFIX,
		},
	}
}
