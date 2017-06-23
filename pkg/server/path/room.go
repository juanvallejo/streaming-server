package path

import "net/http"

var (
	RoomPathIndexFile = "video.html"
)

// RoomPathHandler implements Path
// and handles all room url requests
type RoomPathHandler struct {
	*PathHandler
}

func (h *RoomPathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	http.ServeFile(w, r, FilePathFromFilename(RoomPathIndexFile))
	return nil
}

func NewPathRoom() Path {
	return &RoomPathHandler{
		&PathHandler{
			pathUrl: RoomRootUrl,
		},
	}
}
