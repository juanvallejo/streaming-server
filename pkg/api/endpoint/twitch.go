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
	TWITCH_RESULT_KIND_CLIP  = "twitch#clip"
)

var (
	twitchMaxResults             = 20
	twitchStreamEndpointTemplate = "https://api.twitch.tv/kraken/videos/%s"
	twitchClipEndpointTemplate   = "https://api.twitch.tv/kraken/clips/%s"
)

// TwitchEndpoint implements ApiEndpoint
type TwitchEndpoint struct {
	*ApiEndpointSchema
}

type TwitchEndpointResponse struct {
	Items interface{} `json:"items"`
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

type TwitchClipItem struct {
	*EndpointResponseItem

	VideoId     string  `json:"slug"`
	Language    string  `json:"language"`
	Views       int     `json:"views"`
	PublishedAt string  `json:"created_at"`
	Duration    float64 `json:"duration"`
	Length      int     `json:"length"`
	Preview     string  `json:"animated_preview_url"`
	Game        string  `json:"game"`

	Thumbnails TwitchClipItemThumbnail `json:"thumbnails"`

	Vod TwitchClipItemVod `json:"vod"`
}

func (t *TwitchItem) Decode(b []byte) error {
	return json.Unmarshal(b, &t)
}

func (t *TwitchClipItem) Decode(b []byte) error {
	return json.Unmarshal(b, &t)
}

type TwitchClipItemVod struct {
	Id     string  `json:"id"`
	Offset float64 `json:"offset"`
	Url    string  `json:"url"`
}

type TwitchClipItemThumbnail struct {
	Medium string `json:"medium"`
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
	// the un-sanitized variant of the request path
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
	case segments[1] == "clip":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /clip/slug"), w)
			return
		}

		handleTwitchApiClip(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

func handleTwitchApiStream(streamId string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(twitchStreamEndpointTemplate, streamId)
	handleTwitchApiRequest(reqUrl, nil, encodeTwitchVideoItem, w)
}

func handleTwitchApiClip(clipSlug string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(twitchClipEndpointTemplate, clipSlug)
	handleTwitchApiRequest(reqUrl, map[string]string{
		"Accept": "application/vnd.twitchtv.v5+json",
	}, encodeTwitchClipItem, w)
}

// TwitchItemCodec receives bytes and returns an encoded TwitchItem
type TwitchItemCodec func([]byte) ([]byte, error)

func encodeTwitchVideoItem(b []byte) ([]byte, error) {
	item := &TwitchItem{
		EndpointResponseItem: &EndpointResponseItem{},
	}
	err := item.Decode(b)
	if err != nil {
		return nil, err
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

	return json.Marshal(resp)
}

func encodeTwitchClipItem(b []byte) ([]byte, error) {
	item := &TwitchClipItem{
		EndpointResponseItem: &EndpointResponseItem{},
	}
	err := item.Decode(b)
	if err != nil {
		return nil, err
	}

	// set default item api fields
	item.Kind = TWITCH_RESULT_KIND_CLIP

	item.Thumb = item.Thumbnails.Medium
	item.Id = item.Vod.Id
	item.Length = int(item.Duration)

	if len(item.Vod.Url) == 0 {
		return nil, fmt.Errorf("original VOD for specified clip is no longer available")
	}

	// sanitize vod url
	item.Url = strings.Split(item.Vod.Url, "?")[0]

	// in the case of a video clip, we return an item
	// with the original, full video url, and the clip's
	// slug as a query parameter.
	item.Url = item.Url + "?clip=" + item.VideoId

	resp := &TwitchEndpointResponse{
		Items: []*TwitchClipItem{item},
	}

	return json.Marshal(resp)
}

func handleTwitchApiRequest(url string, extraHeaders map[string]string, codec TwitchItemCodec, w http.ResponseWriter) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	req.Header.Set("Client-ID", config.TWITCH_API_KEY)
	if extraHeaders != nil {
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
	}

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

	encodedResponse, err := codec(data)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(encodedResponse)
}

func NewTwitchEndpoint() ApiEndpoint {
	return &TwitchEndpoint{
		&ApiEndpointSchema{
			path: TWITCH_ENDPOINT_PREFIX,
		},
	}
}
