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
	YOUTUBE_ENDPOINT_PREFIX = "/youtube"

	YoutubePlaylistItem = "youtube#playlistItem"
	YoutubeSearchResult = "youtube#searchResult"
	YoutubeVideoKind    = "youtube#video"
)

var (
	youtubeMaxResults           = 20
	youtubeMaxPlaylistResults   = 15
	youtubeEndpointTemplate     = "https://www.googleapis.com/youtube/v3/search?part=snippet&q=%v&type=video&maxResults=%v&key=%v"
	youtubeEndpointListTemplate = "https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&playlistId=%v&maxResults=%v&key=%v"
)

// YoutubeEndpoint implements ApiEndpoint
type YoutubeEndpoint struct {
	*ApiEndpointSchema
}

type YoutubeItem struct {
	*EndpointResponseItem

	Info    YoutubeItemInfo    `json:"info"`
	Snippet YoutubeItemSnippet `json:"snippet"`
}

// holds an "id" struct for youtube videos
type YoutubeItemInfo struct {
	Kind    string `json:"kind"`
	VideoId string `json:"videoId"`
}

type YoutubeItemThumbnails struct {
	Default  YoutubeItemThumbnail `json:"default"`
	Medium   YoutubeItemThumbnail `json:"medium"`
	High     YoutubeItemThumbnail `json:"high"`
	Standard YoutubeItemThumbnail `json:"standard"`
}

type YoutubeItemThumbnail struct {
	Url    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type YoutubeItemSnippet struct {
	PublishedAt string                `json:"publishedAt"`
	ChannelId   string                `json:"channelId"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	PlaylistId  string                `json:"playlistId"`
	Thumbnails  YoutubeItemThumbnails `json:"thumbnails"`

	ResourceId YoutubeItemSnippetResource `json:"resourceId"`
}

type YoutubeItemSnippetResource struct {
	Kind    string `json:"kind"`
	VideoId string `json:"videoId"`
}

type YoutubeEndpointResponse struct {
	Items []*YoutubeItem `json:"items"`
}

func (r *YoutubeEndpointResponse) Encode() ([]byte, error) {
	return json.Marshal(r)
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
	case segments[1] == "list":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /list/query"), w)
			return
		}

		handleApiList(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

func handleApiSearch(searchQuery string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(youtubeEndpointTemplate, searchQuery, youtubeMaxResults, config.YT_API_KEY)
	handleApiRequest(YoutubeSearchResult, reqUrl, w)
}

func handleApiList(listId string, w http.ResponseWriter) {
	reqUrl := fmt.Sprintf(youtubeEndpointListTemplate, listId, youtubeMaxPlaylistResults, config.YT_API_KEY)
	handleApiRequest(YoutubePlaylistItem, reqUrl, w)
}

func handleApiRequest(kind string, url string, w http.ResponseWriter) {
	res, err := http.Get(url)
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

	// modify standard youube api search result items
	// in order to conform to the standard api response
	// for this server - replace certain field types
	// with appropriate values
	if kind == YoutubeSearchResult {
		data = []byte(strings.Replace(string(data), "\"id\":", "\"info\":", -1))
	}

	resp := &YoutubeEndpointResponse{}
	err = json.Unmarshal(data, resp)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	// default required spec fields for an api response item
	for _, respItem := range resp.Items {
		originalKind := respItem.Kind

		if originalKind == YoutubePlaylistItem {
			respItem.Id = respItem.Snippet.ResourceId.VideoId
		} else if originalKind == YoutubeSearchResult {
			respItem.Kind = respItem.Info.Kind
			respItem.Id = respItem.Info.VideoId
		}

		respItem.Thumb = "https://img.youtube.com/vi/" + respItem.Id + "/default.jpg"
		respItem.Url = "https://www.youtube.com/watch?v=" + respItem.Id
		respItem.Title = respItem.Snippet.Title
	}

	respBytes, err := resp.Encode()
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(respBytes)
}

func NewYoutubeEndpoint() ApiEndpoint {
	return &YoutubeEndpoint{
		&ApiEndpointSchema{
			path: YOUTUBE_ENDPOINT_PREFIX,
		},
	}
}
