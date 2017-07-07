package types

const (
	API_TYPE_STREAM_LIST = "streamList"
)

// ApiCodec provides methods of serializing and de-serializing
// object information
type ApiCodec interface {
	Serialize() ([]byte, error)
}
