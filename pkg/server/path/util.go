package path

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
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

func FileMimeFromFilePath(fpath string) (string, error) {
	mimeType := mime.TypeByExtension(FileExtensionFromFilePath(fpath))
	if len(mimeType) == 0 {
		return "", fmt.Errorf("unable to calculate mimetype for file %q", fpath)
	}
	return mimeType, nil
}

func FileExtensionFromFilePath(fpath string) string {
	segs := strings.Split(fpath, ".")
	return "." + segs[len(segs)-1]
}

func StreamDataFilePathFromFilename(fname string) string {
	return StreamDataRootPath + "/" + fname
}

func StreamDataFilePathFromUrl(url string) string {
	return StreamDataRootPath + "/" + StreamDataFilenameFromUrl(url)
}

// StreamDataFilenameFromUrl receives a stream-formatted request url and
// returns its last segment, or the original url if the path is malformed.
func StreamDataFilenameFromUrl(url string) string {
	segs := strings.Split(url, "/")
	if len(segs) == 0 {
		return url
	}

	return segs[len(segs)-1]
}
