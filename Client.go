package synk

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// When sending a message to a client via a websocket connection. how long to
	// wait before we give up.
	writeTimeout = 10 * time.Second

	// How long do we wait for a pong (after sending a ping)
	pongTimeout = 60 * time.Second

	// Ping websockets at this interval. As per the websocket chat example,
	// this must be less than pongTimeout
	pingInterval = (pongTimeout * 9) / 10

	// Size of the buffer for outgoing messages to clients
	clientBufferLength = 64
)

// synkClient represents a connected client in a browser. Includes the client's
// websocket and redis connections.
// The client does not know which pools it is a part of.
type synkClient struct {
	Node          *Node
	Loader        Loader
	custom        CustomClient
	wsConn        *websocket.Conn
	fromWebSocket chan interface{}
	toWebSocket   chan []byte // safe to send messages here concurrently
	id            ID
	subscriptions map[string]bool
	closeOnce     sync.Once
	waitGroup     sync.WaitGroup
}

func newClient(node *Node, wsConn *websocket.Conn) (*synkClient, error) {
	var client *synkClient
	log.Println("Creating New Client...")

	client = &synkClient{
		Node:          node,
		Loader:        node.CreateLoader(),
		wsConn:        wsConn,
		fromWebSocket: make(chan interface{}, clientBufferLength),
		toWebSocket:   make(chan []byte, clientBufferLength),
		id:            NewID(),
		subscriptions: make(map[string]bool),
	}
	client.waitGroup.Add(1)

	//
	go func() {
		client.waitGroup.Wait()
		fmt.Println("Graceful shutdown:", client.ID())
		client.Loader.Close()
	}()

	go client.startMainLoop()
	go client.startReadingFromWebSocket()

	client.custom = node.NewClient(client)
	client.custom.OnConnect(client)

	log.Println("synk.newClient created:", client.id)
	return client, nil
}

// ID returns the client's id as a string
func (client *synkClient) ID() string {
	return client.id.String()
}

// Publish a message to the synk system
func (client *synkClient) Publish(key string, msg interface{}) error {
	return client.Loader.Publish(key, msg)
}

// Receive handles byteSlices from redis.
//
// It will be called by pubsub.RedisAgents when we receive a value from redis.
//
// Input: client.Node.redisAgents
// Output: client.toWebSocket channel
//
// While this function is safe for concurrent calls, we may still want to be
// careful with where it gets called from, because the order synk messages
// arrive in is important.
func (client *synkClient) Receive(key string, bytes []byte) (err error) {
	s := string(bytes)
	split := strings.Index(s, "{")
	if split == 0 {
		client.toWebSocket <- bytes
		return
	}
	if split == -1 {
		fmt.Printf("synk.Client.Receive: Error with bytes from redis: %s\n", bytes)
		return
	}

	// This json was sent with a header. This is a proprietary extension that lets
	// us contidionally send this message to the client. It's hacky, but likely
	// sufficient for now.
	bytes = []byte(s[split:])
	header := s[:split]

	switch {
	case strings.HasPrefix(header, "from "):
		// This object is moving into the space 'from' another chunk. If we are
		// already subscribed to that chunk then we do not need to send the message.
		fromWhere := header[5:]
		if _, ok := client.subscriptions[fromWhere]; !ok {
			// We are not subscribed to the chunk this object is entering from.
			client.toWebSocket <- bytes
		}
	default:
		log.Println("handleByteSliceFromRedis: unrecognized header", header)
		client.toWebSocket <- bytes
	}
	return
}

// The main client loop is responsible for writing to wsConn and the client's
// redis connection.
//
// Monitor the Client.writeToWebSocket go channel. Forwards messages to the
// client via the websocket connection. This behavior was in a function called
// writePump in the chat example.
//
// Monitor client.messageChannel - These are parsed messages from the client.
//
// Inputs:
// 		- client.toWebSocket chan
//		- client.fromWebSocket chan
//		- client.quit chan
// Output:
//		- client.wsConn,
// 		- client.rConn/.rSubscription
//
// Any one of the following causes termination:
// - error or timeout sending to websocket
// - ping timeout on websocket
// - message received on .quit channel
func (client *synkClient) startMainLoop() {
	ticker := time.NewTicker(pingInterval)

	defer ticker.Stop()
	defer client.Close()
	defer log.Println("Close startMainLoop", client.id)

	for {
		select {
		case message, ok := <-client.toWebSocket:
			if !ok {
				// the client.writeToWebSocket channel was closed up-stream
				log.Println("Client.writeToWebSocket: go channel was closed")
				return
			}
			// We received a message that is intended for the client. Note that if we
			// return (or break out of the for loop), the message will not be in our
			// buffer AND will never have reached the client
			if err := client.writeToWebSocket(message); err != nil {
				return
			}
		case <-ticker.C:
			client.wsConn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := client.wsConn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		// Above this should be logic for writing to the websocket
		// Below should be other stuff
		case message, ok := <-client.fromWebSocket:
			if !ok {
				// ws connection is broken, but we are still receiving messages
				// from redis.
				return
			}
			// Please note the genius of handling subscription methods here.
			// This allows us to start subscribing before we send the requested
			// chunks. However modObj messages that arrive after we send the
			// chunks to the client, because they will be qued in this goroutine.
			//
			// This gaurantees that the client will not miss modObj messages
			// between the time that it receives it's first setChunk and and
			// when the subscription takes effect.
			err := client.handleMessage(message)
			if err != nil {
				log.Println("Client.writeToWebSocket: Error handling message from client:", err)
				// Should this quit/return?
			}
		}
	}
}

// startReadingFromWebSocket pumps messages from the WebSocket to the
// client.fromWebSocket channel.
//
// Inputs: client.wsConn
// Output: client.fromWebSocket chan
//
// Only this function may send to fromWebSocket, because this function will also
// close fromWebSocket.
//
// Break out of receive loop when wsConn.Readmessage returns an error. This
// happens when:
// - We call wsConn.Close() directly
// - An error reading from the websocket (ex. closed browser)
func (client *synkClient) startReadingFromWebSocket() {
	// Closing the fromWebSocket channel will cause the main loop to exit. The
	// great thing about this defered call is that we can be confident that even
	// it this function panics, the fromWebSocket channel will still be closed.
	// This is important, because the client.Close() function blocks until the
	// client.fromWebSocket channel is closed.
	defer close(client.fromWebSocket)

	// Receive messages from the websocket
	for {
		_, bytes, err := client.wsConn.ReadMessage()
		// Errors from websocket library are expected to be *websocket.CloseError
		if wsErr, ok := err.(*websocket.CloseError); ok {
			if wsErr.Code == websocket.CloseGoingAway {
				log.Println("Client closed tab:", client.id)
			} else {
				log.Println("synk.Client.startReadingFromWebSocket Error:", wsErr)
			}
			break
		} else if err != nil {
			// I don't think this should ever happen, but it can't hurt to double check
			log.Println("synk.Client.startReadingFromWebSocket Unexpected Read Error:", err)
			break
		}

		// We successully received bytes. Try to parse them.
		if message, parseErr := MessageFromBytes(bytes); parseErr != nil {
			// Failed to parse message. Report error, but don't break out of the loop.
			log.Println("synk.Client.startReadingFromWebSocket: error parsing bytes:", parseErr)
		} else {
			client.fromWebSocket <- message
		}
	}
}

func (client *synkClient) String() string {
	return fmt.Sprintf("{Client.ID: %s}", client.id)
}

// not safe for concurrent calls
func (client *synkClient) handleMessage(message interface{}) error {
	switch msg := message.(type) {
	case UpdateSubscriptionMessage:
		client.updateSubscription(msg)
	case CustomMessage:
		if client.custom != nil {
			client.custom.OnMessage(client, msg.Method, msg.Data)
		} else {
			return fmt.Errorf("Client.handleMessages does not handle %T", message)
		}
	}
	return nil
}

// subscribe/unsubscribe to the keys in the diff map. This is NOT safe for
// concurrent calls, and may only be called by a single goroutine
func (client *synkClient) updateSubscription(msg UpdateSubscriptionMessage) error {

	for _, subKey := range msg.Remove {
		delete(client.subscriptions, subKey)
	}
	for _, subKey := range msg.Add {
		client.subscriptions[subKey] = true
	}

	client.Node.redisAgents.Update(client, msg.Add, msg.Remove)

	// Send subscribe request
	if len(msg.Add) > 0 {

		objs, err := client.Loader.Load(msg.Add)

		if err != nil {
			log.Printf("Client.updateSubscription: error geting Objects: %s\n", err)
			return err
		}

		// We are inside the startMainLoop() function in the call stack, so we
		// must be absolutely sure that we do not block. If we send fragment to
		// the toWebSocket channel, we might block, because we might overload
		// the toWebSocket channel buffer.
		//
		// We have already updated our subscription, so immediately send the
		// current state to the web socket.
		if len(objs) > 0 {
			for _, obj := range objs {
				bytes, err := json.Marshal(addMsg{
					State:   obj.State(),
					ID:      obj.TagGetID(),
					SKey:    obj.GetSubKey(),
					Version: obj.Version(),
					Type:    obj.TypeKey(),
				})
				if err != nil {
					log.Printf("Client.updateSubscription failed to marshal %v\n", obj)
				}
				client.writeToWebSocket(bytes)
			}
		}
		client.custom.OnSubscribe(client, msg.Add, objs)
	}
	return nil
}

// Close tears down client resources, and stops client goroutines.
// Safe for concurrent calls.
func (client *synkClient) Close() {
	client.closeOnce.Do(func() {
		// If the wsConn is still open, sub/unsub requests may be incoming. To
		// ensure there are no pending sub/unsub reqeusts, first close the wsConn.
		//
		// Note that it is safe to call wsConn.Close() twice allthough the
		// second time it will return an error.
		//
		// This should terminate the startReadingFromWebSocket goroutine, and
		// close the client.fromWebSocket channel. Closing this channel also
		// causes the main loop to break.
		client.wsConn.Close()

		// Wait for the the main loop to close.
		for range client.fromWebSocket {
		}

		// Once the fromWebSocket channel is closed, we are gauranteed not to
		// get an pub/sub requests from the client.
		client.Node.redisAgents.RemoveAgent(client)

		client.waitGroup.Done()
	})
}

// May only be called from the mainLoop. Not safe for concurrent calls.
func (client *synkClient) writeToWebSocket(message []byte) error {
	client.wsConn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := client.wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Println("Client: Error Writing to websocket:", err)
		return err
	}
	return nil
}

// WriteToWebSocket sends data via websocket. Safe for concurrent calls.
func (client *synkClient) WriteToWebSocket(data []byte) {
	// While this is the same as just sending data to the toWebSocket channel, this
	// method is exported while the channel is not. This way it is harder for client
	// code to accidentally overwrite the channel.
	client.toWebSocket <- data
}
