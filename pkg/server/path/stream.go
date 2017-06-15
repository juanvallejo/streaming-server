package path

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var maxChunkSize int64 = 20000000

// RoomPathHandler implements Path
// and handles all room url requests
type StreamPathHandler struct {
	PathHandler
}

func (h StreamPathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	fpath := StreamDataFilePathFromUrl(r.URL.String())

	// determine if requested file exists
	fileStat, err := os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			p404 := NewPathNotFound()
			return p404.Handle(fpath, w, r)
		}

		return err
	}

	contentRange := r.Header.Get("Range")
	if len(contentRange) == 0 {
		contentRange = "bytes=0-"
	}

	contentRange = strings.Replace(contentRange, "bytes=", "", -1)
	positions := strings.Split(contentRange, "-")

	startPos, err := strconv.ParseInt(positions[0], 10, 32)
	if err != nil {
		return err
	}
	endPos, err := strconv.ParseInt(positions[1], 10, 32)
	if err != nil {
		endPos = fileStat.Size() - 1
	}

	if startPos > endPos {
		HandleInvalidRange(w, r)
		return nil
	}

	if endPos-startPos > maxChunkSize {
		endPos = startPos + maxChunkSize
	}

	chunkSize := endPos - startPos + 1
	mimeType := mime.TypeByExtension(fileExtensionFromUrl(fpath))
	if len(mimeType) == 0 {
		log.Printf("ERR HTTP PATH unable to calculate mimetype for file %q", fpath)
		return fmt.Errorf("unable to calculate mimetype for file %q", fpath)
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %v-%v/%v", startPos, endPos, fileStat.Size()))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusPartialContent)

	file, err := os.Open(fpath)
	if err != nil {
		log.Printf("ERR HTTP PATH unable to open requested stream file %q: %v", fileStat.Name(), err)
		return err
	}
	defer file.Close()

	log.Printf("INFO HTTP PATH serving requested file (%s) with a chunk size of %v bytes", fileStat.Name(), chunkSize)

	// if chunkSize is less than maxChunkSize, read byte range
	// from file, and stream byte range to client
	contents := make([]byte, chunkSize)
	n, err := file.ReadAt(contents, startPos)
	if err != nil {
		if err != io.EOF && n == 0 {
			return err
		}
	}

	w.Write(contents)
	return nil
}

func NewPathStream() StreamPathHandler {
	return StreamPathHandler{
		PathHandler{
			pathUrl: StreamRootUrl,
		},
	}
}

func fileExtensionFromUrl(url string) string {
	segs := strings.Split(url, ".")
	return "." + segs[len(segs)-1]
}
