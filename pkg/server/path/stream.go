package path

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var maxByteRange int64 = 20000000
var maxChunkSize int = 4096

// RoomPathHandler implements Path
// and handles all room url requests
type StreamPathHandler struct {
	*PathHandler
}

func (h *StreamPathHandler) Handle(url string, w http.ResponseWriter, r *http.Request) error {
	fpath := StreamDataFilePathFromUrl(r.URL.String())

	// determine if requested file exists
	fileStat, err := os.Stat(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			HandleNotFound(url, w, r)
			return nil
		}

		return err
	}

	contentRange := r.Header.Get("Range")
	if len(contentRange) == 0 {
		contentRange = "bytes=0-"
	}

	contentRange = strings.Replace(contentRange, "bytes=", "", -1)
	positions := strings.Split(contentRange, "-")

	startPos, err := strconv.ParseInt(positions[0], 10, 64)
	if err != nil {
		HandleInvalidRange(fmt.Sprintf("range value too large: %v", err), w, r)
		return nil
	}
	endPos, err := strconv.ParseInt(positions[1], 10, 64)
	if err != nil {
		endPos = fileStat.Size() - 1
	}

	if startPos > endPos {
		HandleInvalidRange("range start position is greater than ending position.", w, r)
		return nil
	}

	if endPos-startPos > maxByteRange {
		endPos = startPos + maxByteRange
	}

	byteRangeSize := endPos - startPos + 1
	mimeType, err := FileMimeFromFilePath(fpath)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %v-%v/%v", startPos, endPos, fileStat.Size()))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusPartialContent)

	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	log.Printf("INF HTTP PATH serving requested file (%s) with a byte-range size of %v bytes", fileStat.Name(), byteRangeSize)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("expected http.ResponseWriter to implement http.Flusher")
	}

	buff := make([]byte, maxChunkSize)
	totalRead := int64(0)
	for totalRead < byteRangeSize {
		n, err := file.ReadAt(buff, totalRead+startPos)
		if err != nil {
			if err != io.EOF && n == 0 {
				return err
			}
		}

		totalRead += int64(n)
		w.Write(buff[0:n])

		flusher.Flush()
	}

	return nil
}

func NewPathStream() Path {
	return &StreamPathHandler{
		&PathHandler{
			pathUrl: StreamRootUrl,
		},
	}
}
