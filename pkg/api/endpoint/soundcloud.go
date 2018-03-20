package endpoint

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api/config"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

const (
	SC_ENDPOINT_PREFIX = "/soundcloud"
)

var (
	soundCloudEndpointTemplate = "http://api.soundcloud.com/tracks?q=%s&client_id=%v"
)

type SoundCloudItem struct {
	*EndpointResponseItem

	Id   int                `json:"id"`
	Uri  string             `json:"uri"`
	User SoundCloudUserInfo `json:"user"`
}

type SoundCloudEndpointResponse struct {
	Items []*SoundCloudItem `json:"items"`
}

type SoundCloudUserInfo struct {
	Thumb string `json:"avatar_url"`
}

// SoundCloudEndpoint implements ApiEndpoint
type SoundCloudEndpoint struct {
	*ApiEndpointSchema
}

// Handle returns a "discovery" of all local streams in the server data root.
func (e *SoundCloudEndpoint) Handle(connHandler connection.ConnectionHandler, segments []string, w http.ResponseWriter, r *http.Request) {
	if len(segments) < 2 {
		HandleEndpointError(fmt.Errorf("unimplemented endpoint"), w)
		return
	}

	// since we are dealing with a url value, split
	// the un-sanitized variant of the request path
	// containing the url encoded value
	segments = strings.Split(r.URL.String(), "/")
	segments = segments[2:]

	switch {
	case segments[1] == "search":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /seatch/term"), w)
			return
		}

		handleSoundCloudApiStream(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

func handleSoundCloudApiStream(permalink string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(soundCloudEndpointTemplate, permalink, config.SC_API_KEY)
	handleSoundCloudApiRequest(reqUrl, w)
}

func handleSoundCloudApiRequest(reqUrl string, w http.ResponseWriter) {
	res, err := http.Get(reqUrl)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	resp := &SoundCloudEndpointResponse{}
	items := []*SoundCloudItem{}
	err = json.Unmarshal(data, &items)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	// default required spec fields for an api response item
	for _, item := range items {
		item.Thumb = item.User.Thumb
		item.Url = item.Uri

		resp.Items = append(resp.Items, item)
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(respBytes)
}

func NewSoundCloudEndpoint() ApiEndpoint {
	return &SoundCloudEndpoint{
		&ApiEndpointSchema{
			path: SC_ENDPOINT_PREFIX,
		},
	}
}
