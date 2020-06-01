package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

// TODO: save subtitles state in playback state sent to the client
// also allow for subtitles to be specified from a URL
type SubtitlesCmd struct {
	*Command
}

const (
	SUBTITLES_NAME        = "subtitles"
	SUBTITLES_DESCRIPTION = "controls stream subtitles for every client"
	SUBTITLES_USAGE       = "Usage: /" + SUBTITLES_NAME + " &lt;(off|path/to/subtitles.srt)&gt;"

	SUBTITLES_FILE_ROOT = "/webclient/src/static/subtitles/"
)

var (
	subtitles_aliases = []string{"subs"}
)

func (h *SubtitlesCmd) Execute(cmdHandler SocketCommandHandler, args []string, user *client.Client, clientHandler client.SocketClientHandler, playbackHandler playback.PlaybackHandler, streamHandler stream.StreamHandler) (string, error) {
	username, hasUsername := user.GetUsername()
	if !hasUsername {
		username = user.UUID()
	}

	userRoom, hasRoom := user.Namespace()
	if !hasRoom {
		log.Printf("SOCKET CLIENT ERR client with id %q attempted to control stream playback with no room assigned", user.UUID())
		return "", fmt.Errorf("error: you must be in a stream to control stream playback")
	}

	currentDir := util.GetCurrentDirectory()
	subtitlesRootDir := path.Join(currentDir, "/../../", SUBTITLES_FILE_ROOT)

	subtitlesFilepath := ""
	if len(args) == 0 {
		subtitlesFilepath = findSubtitlesFilepathGivenCurrentNamespace(playbackHandler, userRoom, subtitlesRootDir)
		if len(subtitlesFilepath) == 0 {
			return "", fmt.Errorf("error: no subtitles filepath specified")
		}
	} else if args[0] == "off" {
		user.BroadcastAll("info_subtitles", &client.Response{
			Id:   user.UUID(),
			From: username,
			Extra: map[string]interface{}{
				"on": false,
			},
		})

		user.BroadcastSystemMessageAll(fmt.Sprintf("%q has requested to remove subtitles from the stream", username))
		return "attempting to remove subtitles from the stream...", nil
	} else {
		subtitlesFilepath = path.Join(subtitlesRootDir, args[0])
	}

	_, err := os.Stat(subtitlesFilepath)
	if err != nil {
		log.Printf("SOCKET CLIENT ERR unable to load subtitle file for stream %q: %v", userRoom, err)
		return "", fmt.Errorf("error: missing subtitles file for current stream")
	}

	log.Printf("SOCKET CLIENT INFO attempting to load subtitles file %q\n", subtitlesFilepath)

	clientRelativeSubtitlesFilepath := strings.Split(subtitlesFilepath, "/webclient/")
	if len(clientRelativeSubtitlesFilepath) < 2 {
		return "", fmt.Errorf("error: unable to parse client-relative subtitles URL")
	}

	user.BroadcastAll("info_subtitles", &client.Response{
		Id:   user.UUID(),
		From: username,
		Extra: map[string]interface{}{
			"path": path.Join("/", clientRelativeSubtitlesFilepath[1]),
			"on":   true,
		},
	})

	user.BroadcastSystemMessageAll(fmt.Sprintf("%q has requested to add subtitles to the stream", username))
	return "attempting to add subtitles to the stream...", nil
}

// findSubtitlesFilepathGivenCurrentNamespace lists all valid subtitle files in the SUBTITLES_FILE_ROOT
// and attempts to find the first file whose name matches the current stream's URL minus its extension
func findSubtitlesFilepathGivenCurrentNamespace(playbackHandler playback.PlaybackHandler, userRoom connection.Namespace, subtitlesRootDir string) string {
	validSubtitleExts := map[string]bool{
		".vtt": true,
	}

	dir, err := os.Stat(subtitlesRootDir)
	if err != nil || !dir.IsDir() {
		return ""
	}

	files, err := ioutil.ReadDir(subtitlesRootDir)
	if err != nil {
		return ""
	}

	if len(files) == 0 {
		return ""
	}

	validFilenames := []string{}
	for _, file := range files {
		if !validSubtitleExts[path.Ext(file.Name())] {
			continue
		}

		validFilenames = append(validFilenames, file.Name())
	}

	if len(validFilenames) == 0 {
		return ""
	}

	nsPlayback, exists := playbackHandler.PlaybackByNamespace(userRoom)
	if !exists {
		return ""
	}

	currentStream, exists := nsPlayback.GetStream()
	if !exists {
		return ""
	}

	streamURLWithNoExt := strings.TrimSuffix(currentStream.GetStreamURL(),
		path.Ext(currentStream.GetStreamURL()))

	for _, validSubFilename := range validFilenames {
		subFilenameWithNoExt := strings.TrimSuffix(validSubFilename,
			path.Ext(validSubFilename))

		if subFilenameWithNoExt != streamURLWithNoExt {
			continue
		}

		return path.Join(subtitlesRootDir, validSubFilename)
	}

	return ""
}

func NewCmdSubtitles() SocketCommand {
	return &SubtitlesCmd{
		&Command{
			name:        SUBTITLES_NAME,
			description: SUBTITLES_DESCRIPTION,
			usage:       SUBTITLES_USAGE,

			aliases: subtitles_aliases,
		},
	}
}
