package types

import (
	"encoding/json"

	"github.com/juanvallejo/streaming-server/pkg/stream"
)

const (
	API_TYPE_STREAM_LIST = "streamList"
)

// ApiCodec provides methods of serializing and de-serializing
// object information
type ApiCodec interface {
	Serialize() ([]byte, error)
}

// StreamList composes a slice of Stream
type StreamList struct {
	Kind  string          `json:"kind"`
	Items []stream.Stream `json:"items"`
}

func (s *StreamList) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}
