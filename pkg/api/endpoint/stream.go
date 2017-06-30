package endpoint

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"fmt"

	"os"

	"github.com/juanvallejo/streaming-server/pkg/api/types"
	paths "github.com/juanvallejo/streaming-server/pkg/server/path"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

const STREAM_ENDPOINT_PREFIX = "/stream"

// StreamEndpoint implements ApiEndpoint
type StreamEndpoint struct {
	*ApiEndpointSchema
}

// Handle returns a "discovery" of all local streams in the server data root.
func (e *StreamEndpoint) Handle(segments []string, w http.ResponseWriter, r *http.Request) {
	dir, err := ioutil.ReadDir(paths.StreamDataRootPath)
	if err != nil {
		HandleEndpointError(err, w)
		return
	}

	if len(segments) > 1 {
		if len(segments) == 2 {
			handleStreamMetadata(segments[1], w, r)
			return
		}

		HandleEndpointNotFound(w)
		return
	}

	sList := types.StreamList{
		Kind:  types.API_TYPE_STREAM_LIST,
		Items: []stream.Stream{},
	}

	for _, f := range dir {
		if f.IsDir() {
			continue
		}

		mimeType, err := paths.FileMimeFromFilePath(f.Name())
		if err != nil {
			continue
		}
		if !strings.HasPrefix(mimeType, "video") {
			continue
		}

		s := stream.NewLocalVideoStream(f.Name())
		sList.Items = append(sList.Items, s)
	}

	b, err := sList.Serialize()
	if err != nil {
		HandleEndpointError(err, w)
		return
	}
	w.Write(b)
}

func handleStreamMetadata(streamUrl string, w http.ResponseWriter, r *http.Request) {
	fpath := paths.StreamDataFilePathFromFilename(streamUrl)
	_, err := os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			HandleEndpointError(fmt.Errorf("unable to load %q: video file does not exist.", streamUrl), w)
			return
		}

		HandleEndpointError(fmt.Errorf("unable to load %q: %v", streamUrl, err), w)
		return
	}

	s := stream.NewLocalVideoStream(streamUrl)
	localStream, ok := s.(*stream.LocalVideoStream)
	if !ok {
		HandleEndpointError(fmt.Errorf("invalid local stream object"), w)
		return
	}

	data, err := stream.FetchLocalVideoMetadata(localStream)
	if err != nil {
		log.Printf("HAHAHAH")
		HandleEndpointError(err, w)
		return
	}
	s.SetInfo(data)

	// convert to ApiCodec
	codec, ok := s.(types.ApiCodec)
	if !ok {
		HandleEndpointError(fmt.Errorf("expected local stream object to be ApiCodec"), w)
		return
	}

	b, err := codec.Serialize()
	if err != nil {
		HandleEndpointError(fmt.Errorf("error serializing local stream data: %v", err), w)
		return
	}
	w.Write(b)
}

func NewStreamEndpoint() ApiEndpoint {
	return &StreamEndpoint{
		&ApiEndpointSchema{
			path: STREAM_ENDPOINT_PREFIX,
		},
	}
}
