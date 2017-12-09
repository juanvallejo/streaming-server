package playback

import (
	"fmt"
	"log"
	"time"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd/rbac"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

var SelectionTimePeriod = 3 * time.Minute

// TimeGate receives a time and returns a boolean indicating
// whether or not the time received was "valid" based on a given
// time period.
type TimeGate func(time.Time, time.Duration) bool

type AdminPicker interface {
	// Stop cancels all pending admin selections
	Stop() bool
	// Clear clears the currently chosen candidate (if any)
	// Returns the selected candidate for "admin" from the given connections.
	Pick([]connection.Connection) (connection.Connection, bool)
	// Init initializes the AdminPicker loop
	Init(connection.Namespace, rbac.Authorizer, client.SocketClientHandler, PlaybackHandler) error
}

// LeastRecentAdminHandler implements AdminHandler
// and selects the connection with the most recent
// timestamp to bind to the admin rbac role.
type LeastRecentAdminPicker struct {
	cancellable bool
	cancelChan  chan bool
}

func (p *LeastRecentAdminPicker) Init(ns connection.Namespace, authorizer rbac.Authorizer, clientHandler client.SocketClientHandler, playbackHandler PlaybackHandler) error {
	if authorizer == nil {
		return fmt.Errorf("no authorizer provided")
	}
	p.cancellable = true

	go pickAdmin(p, authorizer, ns, clientHandler, playbackHandler, p.cancelChan)
	return nil
}

func (p *LeastRecentAdminPicker) Pick(conns []connection.Connection) (connection.Connection, bool) {
	if len(conns) == 0 {
		return nil, false
	}

	pick := conns[0]

	// select connection with most recent timestamp
	for _, c := range conns {
		if c.Metadata().CreationTimestamp().Sub(pick.Metadata().CreationTimestamp()) < 0 {
			pick = c
		}
	}

	return pick, true
}

func (p *LeastRecentAdminPicker) Stop() bool {
	if p.cancellable {
		p.cancelChan <- true
		p.cancellable = false
		return true
	}
	return false
}

func NewLeastRecentAdminPicker() AdminPicker {
	return &LeastRecentAdminPicker{
		cancelChan: make(chan bool, 2),
	}
}

func pickAdmin(picker AdminPicker, authorizer rbac.Authorizer, ns connection.Namespace, clientHandler client.SocketClientHandler, playbackHandler PlaybackHandler, stop chan bool) {
	loopPeriod := 1 * time.Minute

	for {
		time.Sleep(loopPeriod)

		select {
		case <-stop:
			log.Printf("INF PLAYBACK ADMIN-PICKER terminated for room %q.\n", ns.Name())
			return
		default:
		}

		p, pExists := playbackHandler.PlaybackByNamespace(ns)
		if !pExists {
			log.Printf("INF PLAYBACK ADMIN-PICKER unable to find playback for namespace with id %v; terminating admin picker...\n", ns.UUID())
			return
		}

		// define adminBindings before the rest of the admin-picking process
		// to ensure we work with the same data throughout
		adminBindings := []rbac.RoleBinding{}

		for _, b := range authorizer.Bindings() {
			if b.Role().Name() == rbac.ADMIN_ROLE {
				adminBindings = append(adminBindings, b)
			}
		}

		// give a buffer of at least SelectionTimerPeriod after the last admin leaves
		// before attempting to select the next admin
		if !p.LastAdminDepartureTime().Equal(time.Time{}) && time.Now().Sub(p.LastAdminDepartureTime()) < SelectionTimePeriod {
			continue
		}

		candidate, exists := picker.Pick(ns.Connections())
		if !exists {
			continue
		}

		// determine if at least one admin in namespace
		if len(adminBindings) > 0 {
			hasAdmin := false
			for _, admins := range adminBindings {
				for _, admin := range admins.Subjects() {
					if findAdmin(ns.Connections(), admin) {
						hasAdmin = true
						break
					}
				}
				if hasAdmin {
					break
				}
			}

			if hasAdmin {
				continue
			}
		}

		after := time.Now().Sub(p.LastAdminDepartureTime())
		if p.LastAdminDepartureTime().Equal(time.Time{}) {
			after = loopPeriod
		}

		log.Printf("INF PLAYBACK ADMIN-PICKER elected connection with id (%q) as admin candidate after %v...\n", candidate.UUID(), after)

		adminRole, exists := authorizer.Role(rbac.ADMIN_ROLE)
		if !exists {
			log.Printf("WRN PLAYBACK ADMIN-PICKER admin role did not exist - creating empty role\n")
			adminRole = rbac.NewRole(rbac.ADMIN_ROLE, []rbac.Rule{})
			authorizer.AddRole(adminRole)
		}

		if authorizer.Bind(adminRole, candidate) {
			log.Printf("INF PLAYBACK ADMIN-PICKER bound connection with id (%s) to rbac role %q\n", candidate.UUID(), "admin")

			// broadcast info to client
			if c, err := clientHandler.GetClient(candidate.UUID()); err == nil {
				c.BroadcastAuthRequestTo("cookie")
				c.BroadcastSystemMessageTo("You have been selected as the new admin for this room.")
				c.BroadcastAll("info_userlistupdated", &client.Response{
					Id: c.UUID(),
				})
			} else {
				log.Printf("ERR PLAYBACK ADMIN-PICKER unable to broadcast admin-picker events to client - no client found wih id %q\n", candidate.UUID())
			}
		}
	}
}

// findAdmin determines if a given subject exists within a given set of connections
func findAdmin(subjects []connection.Connection, subject rbac.Subject) bool {
	for _, c := range subjects {
		if c.UUID() == subject.UUID() {
			return true
		}
	}

	return false
}
