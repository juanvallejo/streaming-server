package socket

import (
	"fmt"
	"log"
	"strings"

	sockio "github.com/googollee/go-socket.io"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type Handler struct {
	clientHandler   client.SocketClientHandler
	CommandHandler  cmd.SocketCommandHandler
	PlaybackHandler playback.StreamPlaybackHandler
	StreamHandler   stream.StreamHandler
}

const ROOM_URL_SEGMENT = "/v/"
const ROOM_DEFAULT_LOBBY = "lobby"

func (h *Handler) HandleClientConnection(conn sockio.Socket) {
	log.Printf("SOCKET CONN client (%s) has connected with id %q\n", conn.Request().RemoteAddr, conn.Id())

	h.RegisterClient(conn)
	log.Printf("SOCKET currently %v clients registered\n", h.clientHandler.GetClientSize())

	// TODO: remove room's StreamPlayback once last client has left
	conn.On("disconnection", func() {
		log.Printf("SOCKET DCONN client with id %q has disconnected\n", conn.Id())

		if c, err := h.clientHandler.GetClient(conn.Id()); err == nil {
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
			log.Printf("SOCKET CLIENT ERR client %q sent malformed request to update username. Ignoring request.", conn.Id())
			return
		}

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("SOCKET CLIENT ERR %v. Broadcasting as info_clienterror event", err)
			c.BroadcastErrorTo(err)
			return
		}

		err = util.UpdateClientUsername(c, username, h.clientHandler, h.PlaybackHandler)
		if err != nil {
			log.Printf("SOCKET CLIENT ERR %v. Broadcasting as \"info_clienterror\" event", err)
			c.BroadcastErrorTo(err)
		}
	})

	conn.On("request_chatmessage", func(data map[string]interface{}) {
		username, ok := data["user"]
		if ok {
			log.Printf("SOCKET CLIENT INFO client with id %q requested a chat message broadcast with name %q", conn.Id(), username)
		}

		c, err := h.clientHandler.GetClient(conn.Id())
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
			cmdSegments := strings.Split(command, " ")
			cmdArgs := []string{}
			if len(cmdSegments) > 1 {
				cmdArgs = cmdSegments[1:]
			}

			log.Printf("SOCKET CLIENT INFO interpreting chat message as user command %q for client id (%q) with name %q", command, conn.Id(), username)
			result, err := h.CommandHandler.ExecuteCommand(cmdSegments[0], cmdArgs, c, h.clientHandler, h.PlaybackHandler, h.StreamHandler)
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

		fmt.Printf("SOCKET CLIENT INFO chatmessage received %v\n", data)
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
func (h *Handler) ParseCommandMessage(client *client.Client, data map[string]interface{}) (error, string, bool) {
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

	return nil, command[1:], true
}

// RegisterClient receives a socket connection, creates a new client, and assigns the client to a room.
// if client is first to join room, then the room did not exist before; if this is the case, the following steps
// are then taken to "create" a room:
// 1. create a streamPlayback object whose "id" becomes the room's name
// 2.
func (h *Handler) RegisterClient(sockioconn sockio.Socket) {
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

	c := h.clientHandler.CreateClient(sockioconn)
	c.JoinRoom(roomName)

	sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
	if !exists {
		log.Printf("SOCKET CLIENT StreamPlayback did not exist for room with name %q. Creating...", roomName)
		h.PlaybackHandler.NewStreamPlayback(roomName)

		return
	}

	log.Printf("SOCKET CLIENT found StreamPlayback for room with name %q", roomName)

	pStream, exists := sPlayback.GetStream()
	if exists {
		log.Printf("SOCKET CLIENT found stream info (%s) associated with StreamPlayback for room with name %q... Sending \"streamload\" signal to client", pStream.GetStreamURL(), roomName)
		c.BroadcastTo("streamload", &client.Response{
			Id:    c.GetId(),
			Extra: pStream.GetInfo(),
		})
	}
}

func (h *Handler) DeregisterClient(sockioconn sockio.Socket) error {
	err := h.clientHandler.DestroyClient(sockioconn)
	if err != nil {
		return fmt.Errorf("error: unable to de-register client: %v", err)
	}
	return nil
}

func NewHandler(commandHandler cmd.SocketCommandHandler, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) *Handler {
	return &Handler{
		clientHandler:   clientHandler,
		CommandHandler:  commandHandler,
		PlaybackHandler: playbackHandler,
		StreamHandler:   streamHandler,
	}
}
