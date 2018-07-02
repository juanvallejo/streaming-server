package endpoint

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/api/config"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"

	"github.com/rylio/ytdl"
)

const (
	YOUTUBE_ENDPOINT_PREFIX = "/youtube"

	YoutubePlaylistItem = "youtube#playlistItem"
	YoutubeSearchResult = "youtube#searchResult"
	YoutubeVideoKind    = "youtube#video"
	YoutubeAssetKind    = "youtube#asset"
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
			HandleEndpointError(fmt.Errorf("not enough arguments: /search/term"), w)
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
	case segments[1] == "transform":
		if len(segments) < 3 {
			HandleEndpointError(fmt.Errorf("not enough arguments: /transform/url"), w)
			return
		}

		handleUrlTransform(segments[2], w)
		return
	}

	HandleEndpointError(fmt.Errorf("unimplemented parameter"), w)
}

// handleUrlTransform receives a standard YouTube url (e.g. https://youtube.com/watch?v=VIDEO_ID)
// and returns the actual resource url for the corresponding video.
func handleUrlTransform(publicUrl string, w http.ResponseWriter) {
	unescaped, err := url.PathUnescape(publicUrl)
	if err != nil {
		HandleEndpointError(fmt.Errorf("error parsing url fragment: %v", err), w)
		return
	}

	vidUrl, err := url.Parse(unescaped)
	if err != nil {
		HandleEndpointError(fmt.Errorf("error parsing into url obj: %v", err), w)
		return
	}

	info, err := ytdl.GetVideoInfo(vidUrl)
	if err != nil {
		HandleEndpointError(fmt.Errorf("unable to fetch youtube video info: %v", err), w)
		return
	}

	format := info.Formats[0]

	// find an mp4 format with an okay quality
	for _, f := range info.Formats {
		if f.Extension != "mp4" {
			continue
		}

		format = f
		break
	}

	fmt.Printf("INF HTTP PATH YOUTUBE found video format for youtube url %q: %s (%s)\n", vidUrl, format.Extension, format.Resolution)

	u, err := info.GetDownloadURL(format)
	if err != nil {
		HandleEndpointError(fmt.Errorf("unable to fetch download url: %v", err), w)
		return
	}

	// add original youtube video id
	respItem := &YoutubeItem{
		EndpointResponseItem: &EndpointResponseItem{
			Title: info.Title,
			Url:   fmt.Sprintf("%s&youtubeid=%s", u.String(), vidUrl.Query().Get("v")),
			Kind:  YoutubeAssetKind,
		},

		Info:    YoutubeItemInfo{},
		Snippet: YoutubeItemSnippet{},
	}

	resp := &YoutubeEndpointResponse{
		Items: []*YoutubeItem{respItem},
	}

	respBytes, err := resp.Encode()
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	w.Write(respBytes)
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
