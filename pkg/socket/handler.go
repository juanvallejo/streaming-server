package socket

import (
	"fmt"
	"log"
	"strings"

	sockio "github.com/googollee/go-socket.io"

	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/stream/playback"
)

type connHandler struct {
	clientsById    map[string]*client.Client
	CommandHandler cmd.SocketCommandHandler
	StreamPlayback *playback.Playback
}

const ROOM_URL_SEGMENT = "/v/"
const ROOM_DEFAULT_LOBBY = "lobby"

func (h *connHandler) Handle(conn sockio.Socket) {
	log.Printf("SOCKET CONN client (%s) has connected with id %q\n", conn.Request().RemoteAddr, conn.Id())

	h.RegisterClient(conn)
	log.Printf("SOCKET currently %v clients registered\n", h.GetClientSize())

	conn.On("disconnection", func() {
		log.Printf("SOCKET DCONN client with id %q has disconnected\n", conn.Id())

		if c, err := h.GetClientById(conn.Id()); err == nil {
			userName, exists := c.GetUsername()
			if exists {
				c.BroadcastFrom("info_clientleft", &client.Response{
					Id:   conn.Id(),
					From: userName,
				})
			}
		}

		err := h.DeregisterClient(conn)
		if err != nil {
			log.Printf("SOCKET ERR %v", err)
		}
	})

	conn.On("request_updateusername", func(data map[string]string) {
		username, ok := data["user"]
		if !ok {
			log.Printf("SOCKET CLIENT ERR client %q sent malformed request to update username. Ignoring action.", conn.Id())
			return
		}

		c, err := h.GetClientById(conn.Id())
		if err != nil {
			log.Printf("SOCKET CLIENT ERR %v", err)
			return
		}

		prevName, hasPrevName := c.GetUsername()

		if err := c.UpdateUsername(username); err != nil {
			oldName := "[none]"
			if hasPrevName {
				oldName = prevName
			}

			log.Printf("SOCKET CLIENT ERR failed to update username (%q -> %q) for client with id %q", oldName, username, c.GetId())
			c.BroadcastErrorTo(err)
			return
		}

		log.Printf("SOCKET CLIENT INFO client with id %q requested a username update (%q -> %q)", conn.Id(), prevName, username)

		// update startedBy information before updating username
		if updated := h.StreamPlayback.UpdateStartedBy(c); updated {
			log.Printf("SOCKET CLIENT detected client with id %q to have begun playback. Updating playback info to match client's updated username.", c.GetId())
		}

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

		c.BroadcastTo("updateusername", &client.Response{
			From: username,
		})
		c.BroadcastFrom("info_updateusername", &client.Response{
			Id:   c.GetId(),
			From: username,
			Extra: map[string]interface{}{
				"oldUser":   prevName,
				"isNewUser": isNewUser,
			},
			IsSystem: true,
		})
	})

	conn.On("request_chatmessage", func(data map[string]interface{}) {
		username, ok := data["user"]
		if ok {
			log.Printf("SOCKET CLIENT INFO client with id %q requested a chat message broadcast with name %q", conn.Id(), username)
		}

		c, err := h.GetClientById(conn.Id())
		if err != nil {
			log.Printf("SOCKET CLIENT ERR %v", err)
		}

		err, command, isCommand := h.ParseCommandMessage(c, data)
		if err != nil {
			log.Printf("SOCKET CLIENT ERR unable to parse client chat message as command: %v", err)
			c.BroadcastSystemMessageTo(err.Error())
			return
		}

		if isCommand {
			log.Printf("SOCKET CLIENT INFO interpreting chat message as user command for client id (%q) with name %q", conn.Id(), username)
			result, err := h.CommandHandler.ExecuteCommand(command)
			if err != nil {
				log.Printf("SOCKET CLIENT ERR unable to execute command with id %q: %v", command, err)
				c.BroadcastSystemMessageTo(err.Error())
				return
			}

			c.BroadcastSystemMessageTo(result)
			return
		}

		// if err := h.ReplaceMessageImageURL(data); err != nil {
		// 	log.Printf("SOCKET CLIENT WARN ")
		// }

		res := client.ResponseFromClientData(data)
		c.BroadcastAll("chatmessage", &res)

		fmt.Printf("chatmessage received %v\n", data)
	})
}

// ParseCommandMessage receives a client pointer and a data map sent by a client
// and determines whether the "message" field in the client data map contains a
// valid client command. An error is returned if there are any errors while parsing
// the message field. A boolean (true) is returned if the message field contains a
// valid client command, along with a string ("command") containing a StreamCommand id
//
// A valid client command will always begin with a "/" and never contain more than
// one "/" character.
func (h *connHandler) ParseCommandMessage(client *client.Client, data map[string]interface{}) (error, string, bool) {
	message, ok := data["message"]
	if !ok {
		return fmt.Errorf("error: invalid client command format; message field empty"), "", false
	}

	command, ok := message.(string)
	if !ok {
		return fmt.Errorf("error: client command parse error; unable to cast message to string"), "", false
	}

	if string(command[0]) != "/" {
		return nil, "", false
	}

	if segments := strings.Split(command, "/"); len(segments) > 2 {
		return fmt.Errorf("error: commands should only contain a single \"/\" character"), "", false
	}

	return nil, command[1:], true
}

func (h *connHandler) RegisterClient(sockioconn sockio.Socket) {
	req := sockioconn.Request()
	segs := strings.Split(req.Referer(), ROOM_URL_SEGMENT)
	roomName := ROOM_DEFAULT_LOBBY

	// if more than one url segment in url string with valid ROOM_URL_SEGMENT,
	// user can be safely assumed to be in a valid "streaming room"
	if len(segs) > 1 {
		roomName = segs[1]
		log.Printf("SOCKET CLIENT assigning client to room with name %q", roomName)
	} else {
		log.Printf("SOCKET CLIENT WARN websocket connection initiated outside of a valid room. Assigning default lobby room %q.", roomName)
	}

	log.Printf("SOCKET CLIENT registering client with id %q\n", sockioconn.Id())

	c := client.New(sockioconn)
	h.clientsById[sockioconn.Id()] = c

	c.JoinRoom(roomName)
}

func (h *connHandler) DeregisterClient(sockioconn sockio.Socket) error {
	if c, ok := h.clientsById[sockioconn.Id()]; ok {
		if room, inRoom := c.GetRoom(); inRoom {
			sockioconn.Leave(room)
		}

		delete(h.clientsById, sockioconn.Id())
		return nil
	}

	return fmt.Errorf("error: unable to deregister client with id %q.", sockioconn.Id())
}

func (h *connHandler) GetClientById(id string) (*client.Client, error) {
	if c, ok := h.clientsById[id]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("client with id %q not found", id)
}

func (h *connHandler) GetClientSize() int {
	return len(h.clientsById)
}
