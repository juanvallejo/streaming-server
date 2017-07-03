package socket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/juanvallejo/streaming-server/pkg/playback"
	"github.com/juanvallejo/streaming-server/pkg/socket/client"
	"github.com/juanvallejo/streaming-server/pkg/socket/cmd"
	"github.com/juanvallejo/streaming-server/pkg/socket/connection"
	socketserver "github.com/juanvallejo/streaming-server/pkg/socket/server"
	"github.com/juanvallejo/streaming-server/pkg/socket/util"
	"github.com/juanvallejo/streaming-server/pkg/stream"
)

type Handler struct {
	clientHandler   client.SocketClientHandler
	CommandHandler  cmd.SocketCommandHandler
	PlaybackHandler playback.StreamPlaybackHandler
	StreamHandler   stream.StreamHandler

	server *socketserver.Server
}

const (
	ROOM_DEFAULT_LOBBY           = "lobby"
	ROOM_DEFAULT_STREAMSYNC_RATE = 15 // seconds to wait before emitting streamsync to clients
)

func (h *Handler) HandleClientConnection(conn connection.Connection) {
	log.Printf("INFO SOCKET CONN client (%s) has connected with id %q\n", conn.Request().RemoteAddr, conn.Id())

	h.RegisterClient(conn)
	log.Printf("INFO SOCKET currently %v clients registered\n", h.clientHandler.GetClientSize())

	// TODO: remove room's StreamPlayback once last client has left
	conn.On("disconnection", func(data *connection.Message) {
		log.Printf("INFO DCONN SOCKET client with id %q has disconnected\n", conn.Id())

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
			log.Printf("ERR SOCKET %v", err)
		}
	})

	// this event is received when a client is requesting a username update
	conn.On("request_updateusername", func(data *connection.Message) {
		rawUsername, ok := data.Data["user"]
		if !ok {
			log.Printf("ERR SOCKET CLIENT client %q sent malformed request to update username. Ignoring request.", conn.Id())
			return
		}

		username, ok := rawUsername.(string)
		if !ok {
			log.Printf("ERR SOCKET CLIENT client %q sent a non-string value for the field %q", conn.Id(), "username")
			return
		}

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT %v. Broadcasting as info_clienterror event", err)
			c.BroadcastErrorTo(err)
			return
		}

		err = util.UpdateClientUsername(c, username, h.clientHandler, h.PlaybackHandler)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT %v. Broadcasting as \"info_clienterror\" event", err)
			c.BroadcastErrorTo(err)
			return
		}
	})

	// this event is received when a client is requesting to broadcast a chat message
	conn.On("request_chatmessage", func(data *connection.Message) {
		username, ok := data.Data["user"]
		if ok {
			log.Printf("INFO SOCKET CLIENT client with id %q requested a chat message broadcast with name %q", conn.Id(), username)
		}

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT could not retrieve client. Ignoring request_chatmessage request: %v", err)
			return
		}

		command, isCommand, err := h.ParseCommandMessage(c, data.Data)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to parse client chat message as command: %v", err)
			c.BroadcastSystemMessageTo(err.Error())
			return
		}

		if isCommand {
			cmdSegments := strings.Split(command, " ")
			cmdArgs := []string{}
			if len(cmdSegments) > 1 {
				cmdArgs = cmdSegments[1:]
			}

			log.Printf("INFO SOCKET CLIENT interpreting chat message as user command %q for client id (%q) with name %q", command, conn.Id(), username)
			result, err := h.CommandHandler.ExecuteCommand(cmdSegments[0], cmdArgs, c, h.clientHandler, h.PlaybackHandler, h.StreamHandler)
			if err != nil {
				log.Printf("ERR SOCKET CLIENT unable to execute command with id %q: %v", command, err)
				c.BroadcastSystemMessageTo(err.Error())
				return
			}

			if len(result) > 0 {
				c.BroadcastSystemMessageTo(result)
			}
			return
		}

		// TODO: parse message multimedia
		// if err := h.ReplaceMessageImageURL(data); err != nil {
		// 	log.Printf("SOCKET CLIENT WARN ")
		// }

		res := client.ResponseFromClientData(data.Data)
		c.BroadcastAll("chatmessage", &res)

		fmt.Printf("INFO SOCKET CLIENT chatmessage received %v\n", data)
	})

	// this event is received when a client is requesting current stream state information
	conn.On("request_streamsync", func(data *connection.Message) {
		log.Printf("INFO SOCKET CLIENT client with id %q requested a streamsync", conn.Id())

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve client from connection id. Ignoring request_streamsync request: %v", err)
			return
		}

		roomName, exists := c.GetRoom()
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id (%q) has no room association. Ignoring streamsync request.", c.GetId())
			return
		}

		sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id (%q) requested a streamsync but no StreamPlayback could be found associated with that client.", c.GetId())
			c.BroadcastErrorTo(fmt.Errorf("Warning: could not update stream playback. No room could be detected."))
			return
		}

		c.BroadcastTo("streamsync", &client.Response{
			Id:    c.GetId(),
			Extra: sPlayback.GetStatus(),
		})
	})

	// this event is received when a client is requesting to update stream state information in the server
	conn.On("streamdata", func(data *connection.Message) {
		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve client from connection id. Ignoring request_streamsync request: %v", err)
			return
		}

		roomName, exists := c.GetRoom()
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id (%q) has no room association. Ignoring streamsync request.", c.GetId())
			return
		}

		sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id (%q) requested a streamsync but no StreamPlayback could be found associated with that client.", c.GetId())
			c.BroadcastErrorTo(fmt.Errorf("Warning: could not update stream playback. No room could be detected."))
			return
		}

		s, exists := sPlayback.GetStream()
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id (%q) sent updated streamdata but no stream could be found associated with the current playback.", c.GetId())
			return
		}

		jsonData, err := json.Marshal(data.Data)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to convert received data map into json string: %v", err)
		}

		log.Printf("INFO SOCKET CLIENT received streaminfo from client with id (%q). Updating stream information...", c.GetId())
		err = s.SetInfo(jsonData)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT error updating stream data: %v", err)
			return
		}
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
func (h *Handler) ParseCommandMessage(client *client.Client, data map[string]interface{}) (string, bool, error) {
	message, ok := data["message"]
	if !ok {
		return "", false, fmt.Errorf("error: invalid client command format; message field empty")
	}

	command, ok := message.(string)
	if !ok {
		return "", false, fmt.Errorf("error: client command parse error; unable to cast message to string")
	}

	if string(command[0]) != "/" {
		return "", false, nil
	}

	return command[1:], true, nil
}

// RegisterClient receives a socket connection, creates a new client, and assigns the client to a room.
// if client is first to join room, then the room did not exist before; if this is the case, a new
// streamPlayback object is created to represent the "room" in memory. The streamPlayback's id becomes
// the client's room name.
// If a streamPlayback already exists for the current "room" and the streamPlayback has a reference to a
// stream.Stream, a "streamload" event is sent to the client with the current stream.Stream information.
// This method is not concurrency-safe.
func (h *Handler) RegisterClient(conn connection.Connection) {
	log.Printf("INFO SOCKET CLIENT registering client with id %q\n", conn.Id())

	roomName, err := util.GetRoomNameFromRequest(conn.Request())
	if err != nil {
		log.Printf("WARN SOCKET CLIENT websocket connection initiated outside of a valid room. Assigning default lobby room %q.", ROOM_DEFAULT_LOBBY)
		roomName = ROOM_DEFAULT_LOBBY
	}

	log.Printf("INFO SOCKET CLIENT assigning client to room with name %q", roomName)

	c := h.clientHandler.CreateClient(conn)
	c.JoinRoom(roomName)

	c.BroadcastFrom("info_clientjoined", &client.Response{
		Id: c.GetId(),
	})

	sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
	if !exists {
		log.Printf("INFO SOCKET CLIENT StreamPlayback did not exist for room with name %q. Creating...", roomName)
		sPlayback = h.PlaybackHandler.NewStreamPlayback(roomName)
		sPlayback.OnTick(func(currentTime int) {
			currPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
			if !exists {
				log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT attempted to send streamsync event to client, but stream playback does not exist.")
				return
			}

			currStream, exists := currPlayback.GetStream()
			if exists {
				// if stream exists and playback timer >= playback stream duration, stop stream
				// or queue the next item in the playback queue (if queue not empty)
				if currStream.GetDuration() > 0 && float64(currPlayback.GetTime()) >= currStream.GetDuration() {
					queue := currPlayback.GetQueue()
					nextStream, err := queue.Pop()
					if err == nil {
						log.Printf("INFO CALLBACK-PLAYBACK SOCKET CLIENT detected end of stream. Auto-queuing next stream...")

						currPlayback.SetStream(nextStream)
						currPlayback.Reset()
						c.BroadcastAll("streamload", &client.Response{
							Id:    c.GetId(),
							From:  "system",
							Extra: nextStream.GetInfo(),
						})
					} else {
						log.Printf("INFO CALLBACK-PLAYBACK SOCKET CLIENT detected end of stream and no queue items. Stopping stream...")
						currPlayback.Stop()
					}

					// emit updated playback state to client if stream has ended
					log.Printf("INFO CALLBACK-PLAYBACK SOCKET CLIENT stream has ended after %v seconds.", currentTime)
					c.BroadcastAll("streamsync", &client.Response{
						Id:    c.GetId(),
						Extra: currPlayback.GetStatus(),
					})
				}
			}

			// if stream timer has not reached its duration, wait until next ROOM_DEFAULT_STREAMSYNC_RATE tick
			// before updating client with playback information
			if currentTime%ROOM_DEFAULT_STREAMSYNC_RATE != 0 {
				return
			}

			log.Printf("INFO CALLBACK-PLAYBACK SOCKET CLIENT streamsync event sent after %v seconds", currentTime)
			c.BroadcastAll("streamsync", &client.Response{
				Id:    c.GetId(),
				Extra: currPlayback.GetStatus(),
			})
		})

		return
	}

	log.Printf("INFO SOCKET CLIENT found StreamPlayback for room with name %q", roomName)

	pStream, exists := sPlayback.GetStream()
	if exists {
		log.Printf("INFO SOCKET CLIENT found stream info (%s) associated with StreamPlayback for room with name %q... Sending \"streamload\" signal to client", pStream.GetStreamURL(), roomName)
		c.BroadcastTo("streamload", &client.Response{
			Id:    c.GetId(),
			Extra: pStream.GetInfo(),
		})
	}
}

func (h *Handler) DeregisterClient(conn connection.Connection) error {
	err := h.clientHandler.DestroyClient(conn)
	if err != nil {
		return fmt.Errorf("error: unable to de-register client: %v", err)
	}
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.server.ServeHTTP(w, r)
}

func NewHandler(commandHandler cmd.SocketCommandHandler, clientHandler client.SocketClientHandler, playbackHandler playback.StreamPlaybackHandler, streamHandler stream.StreamHandler) *Handler {
	handler := &Handler{
		clientHandler:   clientHandler,
		CommandHandler:  commandHandler,
		PlaybackHandler: playbackHandler,
		StreamHandler:   streamHandler,

		server: socketserver.NewServer(),
	}

	handler.addRequestHandlers()
	return handler
}

func (h *Handler) addRequestHandlers() {
	h.server.On("connection", func(conn connection.Connection) {
		h.HandleClientConnection(conn)
	})
}
