package stream

import "github.com/juanvallejo/streaming-server/pkg/stream/playback"

type Stream struct {
	playback *playback.Playback
}

func New() *Stream {
	return &Stream{}
}
