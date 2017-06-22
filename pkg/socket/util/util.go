package util

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/validation"
)

const ROOM_URL_SEGMENT = "/v/"

// TODO: make this function concurrency-safe
func UpdateClientUsername(c *client.Client, username string, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler) error {
	err := validation.ValidateClientUsername(username)
	if err != nil {
		return err
	}

	prevName, hasPrevName := c.GetUsername()

	log.Printf("SOCKET CLIENT INFO client with id %q requested a username update (%q -> %q)", c.GetId(), prevName, username)

	if hasPrevName && prevName == username {
		return fmt.Errorf("error: you already have that username")
	}

	for _, otherUser := range clientHandler.GetClients() {
		otherUserName, hasName := otherUser.GetUsername()
		if !hasName {
			continue
		}
		if username == otherUserName {
			return fmt.Errorf("error: the username %q is taken", username)
		}
	}

	if err := c.UpdateUsername(username); err != nil {
		oldName := "[none]"
		if hasPrevName {
			oldName = prevName
		}

		log.Printf("SOCKET CLIENT ERR failed to update username (%q -> %q) for client with id %q", oldName, username, c.GetId())
		return err
	}

	log.Printf("SOCKET CLIENT sending \"updateusername\" event to client with id %q (%s)\n", c.GetId(), username)
	c.BroadcastTo("updateusername", &client.Response{
		From: username,
	})

	isNewUser := ""
	if !hasPrevName {
		isNewUser = "true"
	}

	c.BroadcastFrom("info_updateusername", &client.Response{
		Id:   c.GetId(),
		From: username,
		Extra: map[string]interface{}{
			"oldUser":   prevName,
			"isNewUser": isNewUser,
		},
		IsSystem: true,
	})

	return nil
}

// GetRoomNameFromRequest receives a socket connection request and returns
// a fully-qualified room name from the request's referer information
func GetRoomNameFromRequest(req *http.Request) (string, error) {
	segs := strings.Split(req.Referer(), ROOM_URL_SEGMENT)
	if len(segs) > 1 {
		return segs[1], nil
	}

	return "", fmt.Errorf("http request referer field (%s) had an unsupported ROOM_URL_SEGMENT(%q) format", req.Referer(), ROOM_URL_SEGMENT)
}

func GetCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(fmt.Sprintf("unable to get filepath: %v", err))
	}

	return dir
}
