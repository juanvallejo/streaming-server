package util

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
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

	log.Printf("INF SOCKET CLIENT client with id %q requested a username update (%q -> %q)", c.GetId(), prevName, username)

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

		log.Printf("ERR SOCKET CLIENT failed to update username (%q -> %q) for client with id %q", oldName, username, c.GetId())
		return err
	}

	log.Printf("INF SOCKET CLIENT sending \"updateusername\" event to client with id %q (%s)\n", c.GetId(), username)
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
	segs := strings.Split(req.URL.String(), ROOM_URL_SEGMENT)
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

// serializeIntoResponse receives an api.ApiCodec and
// serializes it into a given structure pointer.
func SerializeIntoResponse(codec api.ApiCodec, dest interface{}) error {
	b, err := codec.Serialize()
	if err != nil {
		return err
	}

	return json.Unmarshal(b, dest)
}

// GetUserQueue receives a playback.RoundRobinQueue and a
// client.Client and attempts to find an aggregated queue
// matching the client's id.
// Returns an error if a queue is found but is not aggregatable.
func GetUserQueue(user *client.Client, queue playback.RoundRobinQueue) (playback.AggregatableQueue, bool, error) {
	for _, q := range queue.List() {
		if q.UUID() == user.GetId() {
			userQueue, ok := q.(playback.AggregatableQueue)
			if !ok {
				return nil, false, fmt.Errorf("expected user queue to implement playback.AggregatableQueue")
			}

			return userQueue, true, nil
		}
	}

	// queue not found, return empty one
	return nil, false, nil
}
