package path

import "net/http"

var (
	RootPathUrl       = "/"
	RootPathIndexFile = "index.html"
)

// RootPathHandler implements Path
// and handles all root url requests
type RootPathHandler struct {
	PathHandler
}

func (h RootPathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	http.ServeFile(w, r, FilePathFromFilename(RootPathIndexFile))
	return nil
}

func NewPathRoot() RootPathHandler {
	return RootPathHandler{
		PathHandler{
			pathUrl: RootPathUrl,
		},
	}
}
