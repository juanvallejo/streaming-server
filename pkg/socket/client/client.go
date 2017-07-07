package client

import (
	"encoding/json"
	"fmt"
	"strings"

	api "github.com/juanvallejo/streaming-server/pkg/api/types"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
)

const (
	MAX_USERNAME_HIST = 2 // max number of usernames per client to store
	USER_SYSTEM       = "system"
)

var RESERVED_USERNAMES = map[string]bool{
	"system": true,
}

type Client struct {
	connection connection.Connection
	usernames  []string // stores MAX_USERNAME_HIST usernames; tail represents current username
	room       string
}

// Response is a serializable schema representing
// a response to be sent to the client
type Response struct {
	Id         string                 `json:"id"`
	IsSystem   bool                   `json:"system"`
	From       string                 `json:"user"`
	Message    string                 `json:"message"`
	ErrMessage string                 `json:"error"`
	Extra      map[string]interface{} `json:"extra"`
}

func (r *Response) Serialize() ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

// New receives a socket.io client connection and creates
// a new socket client, containing information about a
// unique socket client connection.
func NewClient(conn connection.Connection) *Client {
	return &Client{
		connection: conn,
		usernames:  make([]string, 0, MAX_USERNAME_HIST),
	}
}

// GetId returns the connection id for the socket client
func (c *Client) GetId() string {
	return c.connection.Id()
}

func (c *Client) UpdateUsername(username string) error {
	if _, ok := RESERVED_USERNAMES[strings.ToLower(username)]; ok {
		return fmt.Errorf("You may not use that username")
	}

	if len(c.usernames) < 2 {
		c.usernames = append(c.usernames, username)
		return nil
	}

	// shift elements left by one
	for i := 1; i < len(c.usernames); i++ {
		c.usernames[i-1] = c.usernames[i]
	}

	c.usernames[len(c.usernames)-1] = username

	return nil
}

// GetUsername returns the currently active username for a client
// or a bool (false) if client has no username history
func (c *Client) GetUsername() (string, bool) {
	if len(c.usernames) == 0 {
		return "", false
	}

	return c.usernames[len(c.usernames)-1], true
}

// GetPreviousUsername returns the last active username for a client
// or a bool (false) if 0 or 1 total usernames have been recorded so far
func (c *Client) GetPreviousUsername() (string, bool) {
	if len(c.usernames) < 2 {
		return "", false
	}

	return c.usernames[len(c.usernames)-2], true
}

// BroadcastErrorTo broadcasts an error message event to the current client
func (c *Client) BroadcastErrorTo(err error) {
	c.BroadcastTo("info_clienterror", &Response{
		ErrMessage: err.Error(),
		IsSystem:   true,
	})
}

func (c *Client) BroadcastAll(evt string, data api.ApiCodec) {
	room, inRoom := c.GetRoom()
	if !inRoom {
		panic("broadcast attempt from client without room")
	}

	m := getBroadcastMessage(evt, data)
	c.connection.Broadcast(room, evt, m)
}

func (c *Client) BroadcastTo(evt string, data api.ApiCodec) {
	m := getBroadcastMessage(evt, data)
	c.connection.Send(m)
}

func (c *Client) BroadcastFrom(evt string, data api.ApiCodec) {
	room, inRoom := c.GetRoom()
	if !inRoom {
		panic("broadcast attempt from client without room")
	}

	m := getBroadcastMessage(evt, data)
	c.connection.BroadcastFrom(room, evt, m)
}

// BroadcastSystemMessageFrom emits a system-level message from the current
// client to the rest of its channel
func (c *Client) BroadcastSystemMessageFrom(msg string) {
	c.BroadcastFrom("chatmessage", &Response{
		From:     USER_SYSTEM,
		Message:  msg,
		IsSystem: true,
	})
}

// BroadcastSystemMessageTo emits a system-level message to the current
// client only
func (c *Client) BroadcastSystemMessageTo(msg string) {
	c.BroadcastTo("chatmessage", &Response{
		From:     USER_SYSTEM,
		Message:  msg,
		IsSystem: true,
	})
}

func (c *Client) BroadcastChatActionTo(methodName string, args []interface{}) {
	if args == nil {
		args = []interface{}{}
	}

	c.BroadcastTo("chatmethodaction", &Response{
		From: USER_SYSTEM,
		Extra: map[string]interface{}{
			"methodname": methodName,
			"args":       args,
		},
	})
}

func (c *Client) BroadcastChatActionFrom(methodName string, args []interface{}) {
	if args == nil {
		args = []interface{}{}
	}

	c.BroadcastFrom("chatmethodaction", &Response{
		From: USER_SYSTEM,
		Extra: map[string]interface{}{
			"methodname": methodName,
			"args":       args,
		},
	})
}

func (c *Client) BroadcastChatActionAll(methodName string, args []interface{}) {
	c.BroadcastChatActionTo(methodName, args)
	c.BroadcastChatActionFrom(methodName, args)
}

// UsernameEquals implements a quick comparison between two clients.
// ClientA is only equal to ClientB if and only if their
// currently active username strings match.
func (c *Client) UsernameEquals(c2 *Client) bool {
	cUser, hasUser := c.GetUsername()
	if !hasUser {
		return false
	}

	c2User, hasUser := c2.GetUsername()
	if !hasUser {
		return false
	}

	if cUser == c2User {
		return true
	}

	return false
}

// UsernameEqualsPrevious implements a quick comparison between two clients
// ClientA is only equal to ClientB if and only if ClientA's current username
// matches ClientB's previously active username.
func (c *Client) UsernameEqualsPrevious(c2 *Client) bool {
	currentUser, hasUser := c.GetUsername()
	if !hasUser {
		return false
	}

	oldUser, hasUser := c2.GetPreviousUsername()
	if !hasUser {
		return false
	}

	if currentUser == oldUser {
		return true
	}

	return false
}

// JoinRoom assigns a client to a channel named after their current
// fully qualified room name.
func (c *Client) JoinRoom(room string) {
	c.connection.Join(room)
	c.room = room
}

// GetRoom returns the name of the channel / room
// the socket is currently in, or boolean false if
// the socket has not been assigned to a room yet
func (c *Client) GetRoom() (string, bool) {
	if len(c.room) == 0 {
		return c.room, false
	}

	return c.room, true
}

// GetConnection returns the socket connection for the current client
func (c *Client) GetConnection() connection.Connection {
	return c.connection
}

func getBroadcastMessage(evt string, codec api.ApiCodec) []byte {
	message := &connection.Message{
		Event: evt,
	}

	switch data := codec.(type) {
	case *connection.MessageData:
		message.Data = data
	case *Response:
		b, err := codec.Serialize()
		if err != nil {
			panic("unable to serialize message *Response from codec")
		}

		err = json.Unmarshal(b, &message.Data)
		if err != nil {
			panic("unable to de-serialize *Response from codec")
		}
	default:
		panic("unknown broadcast message data type received from codec")
	}

	m, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}

	return m
}
