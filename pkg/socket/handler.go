package socket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

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
	log.Printf("INF SOCKET CONN client (%s) has connected with id %q\n", conn.Request().RemoteAddr, conn.Id())

	h.RegisterClient(conn)
	log.Printf("INF SOCKET currently %v clients registered\n", h.clientHandler.GetClientSize())

	// TODO: remove room's StreamPlayback once last client has left
	conn.On("disconnection", func(data connection.MessageDataCodec) {
		log.Printf("INF DCONN SOCKET client with id %q has disconnected\n", conn.Id())

		if c, err := h.clientHandler.GetClient(conn.Id()); err == nil {
			userName, exists := c.GetUsername()
			if exists {
				c.BroadcastFrom("info_clientleft", &client.Response{
					Id:   conn.Id(),
					From: userName,
				})
			}

			room, exists := c.GetRoom()
			if exists {
				sPlayback, sPlaybackExists := h.PlaybackHandler.GetStreamPlayback(room)
				if sPlaybackExists {
					// if client has joined a room, and room still exists
					// check if at least one other client is in that room. If not,
					// mark room as reapable.
					shouldReap := true
					for _, x := range h.clientHandler.GetClients() {
						if c.GetId() == x.GetId() {
							continue
						}
						r, e := x.GetRoom()
						if !e {
							continue
						}
						if r == room {
							shouldReap = false
							break
						}
					}

					if shouldReap {
						// update room's last updated time to give buffer
						// between last client leaving and room reaping.
						sPlayback.SetLastUpdated(time.Now())
						sPlayback.Reapable = true
					}
				}
			}
		}

		err := h.DeregisterClient(conn)
		if err != nil {
			log.Printf("ERR SOCKET %v", err)
		}
	})

	// this event is received when a client is requesting a username update
	conn.On("request_updateusername", func(data connection.MessageDataCodec) {
		messageData, ok := data.(connection.MessageData)
		if !ok {
			log.Printf("ERR SOCKET CLIENT socket connection event handler for event %q received data of wrong type. Expecting connection.MessageData", "request_chatmessage")
			return
		}

		rawUsername, ok := messageData.Key("user")
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
	conn.On("request_chatmessage", func(data connection.MessageDataCodec) {
		messageData, ok := data.(connection.MessageData)
		if !ok {
			log.Printf("ERR SOCKET CLIENT socket connection event handler for event %q received data of wrong type. Expecting connection.MessageData", "request_chatmessage")
			return
		}

		username, ok := messageData.Key("user")
		if ok {
			log.Printf("INF SOCKET CLIENT client with id %q requested a chat message broadcast with name %q", conn.Id(), username)
		}

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT could not retrieve client. Ignoring request_chatmessage request: %v", err)
			return
		}

		command, isCommand, err := h.ParseCommandMessage(c, messageData)
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

			log.Printf("INF SOCKET CLIENT interpreting chat message as user command %q for client id (%q) with name %q", command, conn.Id(), username)
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

		images, err := h.ParseMessageMedia(messageData)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to parse client chat message media: %v", err)
			return
		}

		res := &client.Response{
			Id:    c.GetId(),
			From:  "system",
			Extra: make(map[string]interface{}),
		}

		// if images could be extracted from message, add to response
		if len(images) > 0 {
			res.Extra["images"] = images
		}

		b, err := data.Serialize()
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to serialize client chat message data: %v", err)
			return
		}

		err = json.Unmarshal(b, res)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to de-serialize client chat message into client response: %v", err)
			return
		}

		c.BroadcastAll("chatmessage", res)
		fmt.Printf("INF SOCKET CLIENT chatmessage received %v\n", data)
	})

	// this event is received when a client is requesting the current queue state
	conn.On("request_queuesync", func(data connection.MessageDataCodec) {
		log.Printf("INF SOCKET CLIENT client with id %q requested a queue-sync", conn.Id())

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve client from connection id. Ignoring request_streamsync request: %v", err)
			return
		}

		sPlayback, err := h.getPlaybackFromClient(c)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT %v", err)
			c.BroadcastErrorTo(err)
			return
		}

		res := &client.Response{
			Id:   c.GetId(),
			From: "system",
		}

		b, err := sPlayback.GetQueue().Serialize()
		if err != nil {
			return
		}

		err = json.Unmarshal(b, &res.Extra)
		if err != nil {
			return
		}

		c.BroadcastTo("queuesync", res)
	})

	// this event is received when a client is requesting the current queue state for a specific Queue stack
	conn.On("request_stacksync", func(data connection.MessageDataCodec) {
		log.Printf("INF SOCKET CLIENT client with id %q requested a queue-stack-sync", conn.Id())

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve client from connection id. Ignoring request_streamsync request: %v", err)
			return
		}

		sPlayback, err := h.getPlaybackFromClient(c)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT %v", err)
			c.BroadcastErrorTo(err)
			return
		}

		res := &client.Response{
			Id:   c.GetId(),
			From: "system",
		}

		userQueue, exists, err := util.GetUserQueue(c, sPlayback.GetQueue())
		if err != nil {
			return
		}
		if !exists {
			userQueue = playback.NewAggregatableQueue(c.GetId())
		}

		b, err := userQueue.Serialize()
		if err != nil {
			return
		}

		err = json.Unmarshal(b, &res.Extra)
		if err != nil {
			return
		}

		c.BroadcastTo("stacksync", res)
	})

	// this event is received when a client is requesting current stream state information
	conn.On("request_streamsync", func(data connection.MessageDataCodec) {
		log.Printf("INF SOCKET CLIENT client with id %q requested a streamsync", conn.Id())

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve client from connection id. Ignoring request_streamsync request: %v", err)
			return
		}

		sPlayback, err := h.getPlaybackFromClient(c)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT %v", err)
			c.BroadcastErrorTo(err)
			return
		}

		res := &client.Response{
			Id: c.GetId(),
		}

		err = util.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to serialize playback status: %v", err)
			return
		}

		c.BroadcastTo("streamsync", res)
	})

	// this event is received when a client is requesting current stream user information
	conn.On("request_userlist", func(data connection.MessageDataCodec) {
		log.Printf("INF SOCKET CLIENT client with id %q requested a userlist", conn.Id())

		c, err := h.clientHandler.GetClient(conn.Id())
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to retrieve user info for connection id %q. No such user associated with id.", conn.Id())
			return
		}

		room, exists := c.GetRoom()
		if !exists {
			log.Printf("ERR SOCKET CLIENT client with id %q requested a user list for room, but client is not currently in a room. Broadcasting error...", conn.Id())
			c.BroadcastErrorTo(fmt.Errorf("error: unable to get user list - you are not currently in a room"))
			return
		}

		userList := &client.SerializableClientList{}
		for _, user := range h.clientHandler.GetClients() {
			uRoom, uRoomExists := user.GetRoom()
			if !uRoomExists || uRoom != room {
				continue
			}

			username, _ := user.GetUsername()
			room, _ := user.GetRoom()

			userList.Clients = append(userList.Clients, client.SerializableClient{
				Username: username,
				Id:       user.GetId(),
				Room:     room,
			})
		}

		c.BroadcastTo("userlist", userList)
	})

	// this event is received when a client is requesting to update stream state information in the server
	conn.On("streamdata", func(data connection.MessageDataCodec) {
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

		jsonData, err := data.Serialize()
		if err != nil {
			log.Printf("ERR SOCKET CLIENT unable to convert received data map into json string: %v", err)
		}

		log.Printf("INF SOCKET CLIENT received streaminfo from client with id (%q). Updating stream information...", c.GetId())
		err = s.SetInfo(jsonData)
		if err != nil {
			log.Printf("ERR SOCKET CLIENT error updating stream data: %v", err)
			return
		}
	})
}

// ParseMessageMedia receives connection.MessageData and parses
// image urls in the "message" key, removing urls from the
// text message, and returning them as a slice of strings
func (h *Handler) ParseMessageMedia(data connection.MessageData) ([]string, error) {
	message, ok := data.Key("message")
	if !ok {
		return []string{}, fmt.Errorf("error: invalid client message format; message field empty")
	}

	rawText, ok := message.(string)
	if !ok {
		return []string{}, fmt.Errorf("error: client message media parse error; unable to cast message to string")
	}

	re := regexp.MustCompile("(http(s)?://[^ ]+\\.(jpg|png|gif|jpeg))( )?")
	urls := re.FindAllString(rawText, -1)
	if urls == nil || len(urls) == 0 {
		return []string{}, nil
	}

	newText := re.ReplaceAllString(rawText, "")
	data.Set("message", newText)

	return urls, nil
}

// ParseCommandMessage receives a client pointer and a data map sent by a client
// and determines whether the "message" field in the client data map contains a
// valid client command. An error is returned if there are any errors while parsing
// the message field. A boolean (true) is returned if the message field contains a
// valid client command, along with a string ("command") containing a StreamCommand id
//
// A valid client command will always begin with a "/" and never contain more than
// one "/" character.
func (h *Handler) ParseCommandMessage(client *client.Client, data connection.MessageData) (string, bool, error) {
	message, ok := data.Key("message")
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
	log.Printf("INF SOCKET CLIENT registering client with id %q\n", conn.Id())

	roomName, err := util.GetRoomNameFromRequest(conn.Request())
	if err != nil {
		log.Printf("WRN SOCKET CLIENT websocket connection initiated outside of a valid room. Assigning default lobby room %q.", ROOM_DEFAULT_LOBBY)
		roomName = ROOM_DEFAULT_LOBBY
	}

	log.Printf("INF SOCKET CLIENT assigning client to room with name %q", roomName)

	c := h.clientHandler.CreateClient(conn)
	c.JoinRoom(roomName)

	c.BroadcastFrom("info_clientjoined", &client.Response{
		Id: c.GetId(),
	})

	sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
	if !exists {
		log.Printf("INF SOCKET CLIENT StreamPlayback did not exist for room with name %q. Creating...", roomName)
		sPlayback = h.PlaybackHandler.NewStreamPlayback(roomName)
		sPlayback.OnTick(func(currentTime int) {
			currPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
			if !exists {
				log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT attempted to send streamsync event to client, but stream playback does not exist.")
				return
			}

			if currentTime%2 == 0 {
				currStream, exists := currPlayback.GetStream()
				if exists {
					// if stream exists and playback timer >= playback stream duration, stop stream
					// or queue the next item in the playback queue (if queue not empty)
					if currStream.GetDuration() > 0 && float64(currPlayback.GetTime()) >= currStream.GetDuration() {
						queue := currPlayback.GetQueue()
						queueItem, err := queue.Next()
						if err == nil {
							log.Printf("INF CALLBACK-PLAYBACK SOCKET CLIENT detected end of stream. Auto-queuing next stream...")

							nextStream, ok := queueItem.(stream.Stream)
							if !ok {
								log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT expected next queue item to implement stream.Stream... Unable to advance the queue.")
								return
							}

							currPlayback.SetStream(nextStream)
							currPlayback.Reset()

							res := &client.Response{
								Id:   c.GetId(),
								From: "system",
							}

							err = util.SerializeIntoResponse(currPlayback.GetStatus(), &res.Extra)
							if err != nil {
								log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT unable to serialize nextStream codec: %v", err)
								return
							}

							c.BroadcastAll("streamload", res)
						} else {
							log.Printf("INF CALLBACK-PLAYBACK SOCKET CLIENT detected end of stream and no queue items. Stopping stream...")
							currPlayback.Stop()
						}

						// emit updated playback state to client if stream has ended
						log.Printf("INF CALLBACK-PLAYBACK SOCKET CLIENT stream has ended after %v seconds.", currentTime)
						res := &client.Response{
							Id: c.GetId(),
						}

						err = util.SerializeIntoResponse(currPlayback.GetStatus(), &res.Extra)
						if err != nil {
							log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT unable to serialize playback status: %v", err)
							return
						}

						c.BroadcastAll("streamsync", res)
					}
				}
			}

			// if stream timer has not reached its duration, wait until next ROOM_DEFAULT_STREAMSYNC_RATE tick
			// before updating client with playback information
			if currentTime%ROOM_DEFAULT_STREAMSYNC_RATE != 0 {
				return
			}

			log.Printf("INF CALLBACK-PLAYBACK SOCKET CLIENT streamsync event sent after %v seconds", currentTime)

			res := &client.Response{
				Id: c.GetId(),
			}

			err = util.SerializeIntoResponse(currPlayback.GetStatus(), &res.Extra)
			if err != nil {
				log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT unable to serialize playback status: %v", err)
				return
			}

			c.BroadcastAll("streamsync", res)
		})

		return
	}

	// mark playback object as unreapable
	sPlayback.Reapable = false
	sPlayback.SetLastUpdated(time.Now())

	log.Printf("INF SOCKET CLIENT found StreamPlayback for room with name %q", roomName)

	pStream, exists := sPlayback.GetStream()
	if exists {
		log.Printf("INF SOCKET CLIENT found stream info (%s) associated with StreamPlayback for room with name %q... Sending \"streamload\" signal to client", pStream.GetStreamURL(), roomName)
		res := &client.Response{
			Id: c.GetId(),
		}

		err = util.SerializeIntoResponse(sPlayback.GetStatus(), &res.Extra)
		if err != nil {
			log.Printf("ERR CALLBACK-PLAYBACK SOCKET CLIENT unable to serialize playback status: %v", err)
			return
		}

		c.BroadcastTo("streamload", res)
	}
}

func (h *Handler) DeregisterClient(conn connection.Connection) error {
	err := h.clientHandler.DestroyClient(conn)
	if err != nil {
		return fmt.Errorf("error: unable to de-register client: %v", err)
	}
	return nil
}

func (h *Handler) getPlaybackFromClient(c *client.Client) (*playback.StreamPlayback, error) {
	roomName, exists := c.GetRoom()
	if !exists {
		return nil, fmt.Errorf("client with id (%q) has no room association. Ignoring streamsync request.", c.GetId())
	}

	sPlayback, exists := h.PlaybackHandler.GetStreamPlayback(roomName)
	if !exists {
		return nil, fmt.Errorf("Warning: could not update stream playback. No room could be detected.")
	}

	return sPlayback, nil
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
