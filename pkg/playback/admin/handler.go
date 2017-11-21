package admin

import (
	"log"
	"time"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

//var SelectionTimePeriod = 3 * time.Minute
var SelectionTimePeriod = 3 * time.Minute

type AdminPicker interface {
	// Cancel cancels all pending admin selections
	Cancel() bool
	// Candidate returns the currently picked candidate or nil
	Candidate() connection.Connection
	// Clear clears the currently chosen candidate (if any)
	Clear() bool
	// Pick receives a list of connections and an authorizer
	// and selects a connection to bind to the rbac "admin" role
	// after a pre-determined period of time.
	Pick([]connection.Connection, rbac.Authorizer, client.SocketClientHandler)
}

// LeastRecentAdminHandler implements AdminHandler
// and selects the connection with the most recent
// timestamp to bind to the admin rbac role.
type LeastRecentAdminPicker struct {
	candidate  connection.Connection
	cancelChan chan bool
}

func (p *LeastRecentAdminPicker) Candidate() connection.Connection {
	return p.candidate
}

func (p *LeastRecentAdminPicker) Clear() bool {
	if p.candidate == nil {
		return false
	}

	p.candidate = nil
	return true
}

func (p *LeastRecentAdminPicker) Pick(conns []connection.Connection, authorizer rbac.Authorizer, handler client.SocketClientHandler) {
	p.Cancel()

	if len(conns) == 0 {
		return
	}

	pick := conns[0]

	// select connection with most recent timestamp
	for _, c := range conns {
		if c.Metadata().CreationTimestamp().Sub(pick.Metadata().CreationTimestamp()) < 0 {
			pick = c
		}
	}

	log.Printf("INF PLAYBACK ADMIN-PICKER elected connection with id (%q) as admin candidate...", pick.UUID())

	p.candidate = pick
	go pickAdmin(p, authorizer, handler, p.cancelChan)
}

func (p *LeastRecentAdminPicker) Cancel() bool {
	if p.candidate != nil {
		p.cancelChan <- true
		p.candidate = nil
		return true
	}
	return false
}

func NewLeastRecentAdminPicker() AdminPicker {
	return &LeastRecentAdminPicker{
		cancelChan: make(chan bool, 2),
	}
}

func pickAdmin(picker AdminPicker, authorizer rbac.Authorizer, handler client.SocketClientHandler, cancel chan bool) {
	for {
		shouldBreak := false

		select {
		case <-cancel:
			log.Printf("INF PLAYBACK ADMIN-PICKER cancel signal received - cancelling...\n")
			return
		case <-time.After(SelectionTimePeriod):
			log.Printf("INF PLAYBACK ADMIN-PICKER selecting new admin...\n")
			shouldBreak = true
			break
		}

		if shouldBreak {
			break
		}
	}

	defer picker.Clear()
	subject := picker.Candidate()
	if subject == nil {
		log.Printf("ERR PLAYBACK ADMIN-PICKER unable to select admin candidate - no (<nil>) candidate has been set")
		return
	}

	adminRole, exists := authorizer.Role(rbac.ADMIN_ROLE)
	if exists {
		if authorizer.Bind(adminRole, subject) {
			log.Printf("INF PLAYBACK ADMIN-PICKER bound connection with id (%s) to rbac role %q\n", subject.UUID(), "admin")

			// broadcast info to client
			if c, err := handler.GetClient(subject.UUID()); err == nil {
				c.BroadcastSystemMessageTo("You have been selected as the new admin for this room.")
				c.BroadcastAll("info_userlistupdated", &client.Response{
					Id: c.UUID(),
				})
			}

			return
		}

		log.Printf("INF PLAYBACK ADMIN-PICKER unable to bind connection with id (%s) to rbac role %q\n", subject.UUID(), "admin")
		return
	}

	log.Printf("ERR PLAYBACK ADMIN-PICKER unable to bind connection with id (%s) to rbac role %q - role not found in provided authorizer\n", subject.UUID(), "admin")
}
