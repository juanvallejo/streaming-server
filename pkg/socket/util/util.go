package util

import (
	"fmt"
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
	"github.com/juanvallejo/streaming-server/pkg/validation"
)

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

	// update startedBy information before updating username
	if refreshed := playbackHandler.RefreshInfoFromClient(c); refreshed {
		log.Printf("SOCKET CLIENT detected client with id %q to have begun playback. Updating playback info to match client's updated username.", c.GetId())
	}

	log.Printf("SOCKET CLIENT sending \"updateusername\" (%q) event to client with id %q\n", username, c.GetId())
	c.BroadcastTo("updateusername", &client.Response{
		From: username,
	})

	// if client has no previous name, client is joining the chat for the first time
	if !hasPrevName {
		msg := fmt.Sprintf("%s has joined the chat", username)
		c.BroadcastSystemMessageFrom(msg)
	} else {
		msg := fmt.Sprintf("%s is now known as %s", prevName, username)
		c.BroadcastSystemMessageFrom(msg)
	}

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
