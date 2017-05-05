package playback

import "github.com/juanvallejo/streaming-server/pkg/socket/client"

type Playback struct {
	startedBy string
}

// UpdateStartedBy receives a client and updates `startedBy` with the
// client's current username if the client's last active username
// matches the currently active of startedBy
func (p *Playback) UpdateStartedBy(c *client.Client) bool {
	cOldUser, hasOldUser := c.GetPreviousUsername()
	if !hasOldUser {
		return false
	}

	if len(p.startedBy) > 0 && p.startedBy == cOldUser {
		cUser, hasUser := c.GetUsername()
		if !hasUser {
			// should never happen, if a client has an "old username"
			// they MUST have a currently active username
			panic("client had previous username without an active / current username")
		}
		p.startedBy = cUser
		return true
	}

	return false
}
