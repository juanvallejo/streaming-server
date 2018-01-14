package endpoint

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"encoding/json"

	"github.com/juanvallejo/streaming-server/pkg/api/config"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

const (
	TWITCH_ENDPOINT_PREFIX   = "/twitch"
	TWITCH_RESULT_KIND_VIDEO = "twitch#video"
)

var (
	twitchMaxResults             = 20
	twitchStreamEndpointTemplate = "https://api.twitch.tv/kraken/videos/%s"
)

// TwitchEndpoint implements ApiEndpoint
type TwitchEndpoint struct {
	*ApiEndpointSchema
}

type TwitchEndpointResponse struct {
	Items []*TwitchItem `json:"items"`
}

type TwitchItem struct {
	*EndpointResponseItem

	Status      string                `json:"status"`
	Language    string                `json:"language"`
	Views       int                   `json:"views"`
	PublishedAt string                `json:"published_at"`
	Length      int                   `json:"length"`
	Preview     string                `json:"animated_preview_url"`
	Thumbnails  []TwitchItemThumbnail `json:"thumbnails"`
	Channel     TwitchItemChannel     `json:"channel"`
	VideoId     string                `json:"_id"`
	Game        string                `json:"game"`
}

func (t *TwitchItem) Decode(b []byte) error {
	return json.Unmarshal(b, &t)
}

type TwitchItemThumbnail struct {
	Url string `json:"url"`
}

type TwitchItemChannel struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Handle returns a "discovery" of all local streams in the server data root.
func (e *TwitchEndpoint) Handle(connHandler connection.ConnectionHandler, segments []string, w http.ResponseWriter, r *http.Request) {
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
	case segments[1] == "stream":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /stream/stream_id"), w)
			return
		}

		handleTwitchApiStream(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

func handleTwitchApiStream(streamId string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(twitchStreamEndpointTemplate, streamId)
	handleTwitchApiRequest(reqUrl, w)
}

func handleTwitchApiRequest(url string, w http.ResponseWriter) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	req.Header.Set("Client-ID", config.TWITCH_API_KEY)

	res, err := client.Do(req)
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

	// turn response data into json array
	item := &TwitchItem{
		EndpointResponseItem: &EndpointResponseItem{},
	}
	err = item.Decode(data)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	// set default item api fields
	item.Kind = TWITCH_RESULT_KIND_VIDEO
	item.Id = item.VideoId

	if len(item.Thumbnails) > 0 {
		item.Thumb = item.Thumbnails[0].Url
	}

	resp := &TwitchEndpointResponse{
		Items: []*TwitchItem{item},
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(respBytes)
}

func NewTwitchEndpoint() ApiEndpoint {
	return &TwitchEndpoint{
		&ApiEndpointSchema{
			path: TWITCH_ENDPOINT_PREFIX,
		},
	}
}
