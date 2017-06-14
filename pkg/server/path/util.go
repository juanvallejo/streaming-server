package path

import (
	"fmt"
	"net/http"
)

func FilePathFromRequest(r *http.Request) string {
	return FileRootPath + r.URL.String()
}

func FilePathFromUrl(url string) string {
	return FileRootPath + url
}

func FilePathFromFilename(fname string) string {
	return fmt.Sprintf("%s%s/%s", FileRootPath, FileRootUrl, fname)
}
