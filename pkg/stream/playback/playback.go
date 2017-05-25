package playback

import (
	"log"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
)

type StreamPlaybackHandler interface {
	// UpdateStartedBy receives a client and updates the
	// startedBy field with the client's current username
	UpdateStartedBy(*client.Client)
	// RefreshInfoFromClient receives a client and updates altered
	// client details used as part of playback info.
	// Returns a bool (true) if the client received contains
	// old info matching the one stored by the playback handler,
	// and such info has been since updated in the client.
	RefreshInfoFromClient(*client.Client) bool
}

type Handler struct {
	startedBy string
}

func (h *Handler) UpdateStartedBy(c *client.Client) {
	if name, hasName := c.GetUsername(); hasName {
		h.startedBy = name
		return
	}

	log.Printf("SOCKET CLIENT PLAYBACK attempted to update `startedBy` information, but the current client with id %q has no registered username.", c.GetId())
}

func (h *Handler) RefreshInfoFromClient(c *client.Client) bool {
	cOldUser, hasOldUser := c.GetPreviousUsername()
	if !hasOldUser {
		return false
	}

	if len(h.startedBy) > 0 && h.startedBy == cOldUser {
		cUser, hasUser := c.GetUsername()
		if !hasUser {
			// should never happen, if a client has an "old username"
			// they MUST have a currently active username
			panic("client had previous username without an active / current username")
		}
		h.startedBy = cUser
		return true
	}

	return false
}

func NewHandler() *Handler {
	return &Handler{}
}
