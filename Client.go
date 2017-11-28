package synk

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
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

// Client represents a connected client in a browser. Includes the client's
// websocket and redis connections.
// The client does not know which pools it is a part of.
type Client struct {
	custom        CustomClient
	Synk          *Synk
	Mutator       Mutator
	creator       ContainerConstructor // How objects from mongo will be created
	wsConn        *websocket.Conn
	rConn         redis.Conn       // This is the connection used by rSubscription
	rSubscription redis.PubSubConn // This uses rConn as the underlying conn
	fromWebSocket chan interface{}
	toWebSocket   chan []byte // safe to send messages here concurrently
	ID            ID
	quit          chan bool
	subscriptions map[string]bool
	closeOnce     sync.Once
	waitGroup     sync.WaitGroup
}

func newClient(config *Config, synkConn *Synk, wsConn *websocket.Conn) (*Client, error) {
	var client *Client
	log.Println("Creating New Client...")

	// get a redis connection
	rConn, err := redis.Dial("tcp", config.RedisAddr)
	if err != nil {
		log.Println("error connecting to redis:", err)
		return client, err
	}

	client = &Client{
		Synk:          synkConn,
		Mutator:       config.Mutator.Clone(),
		wsConn:        wsConn,
		rConn:         rConn,
		rSubscription: redis.PubSubConn{Conn: rConn},
		fromWebSocket: make(chan interface{}, clientBufferLength),
		toWebSocket:   make(chan []byte, clientBufferLength),
		ID:            NewID(),
		// quit is buffered, because it should never block. Be careful when
		// quiting from outside the Client methods. We want to be sure to remove
		// all references to the client when it terminates so we can be
		// confident that it will be garbage collected.
		quit:          make(chan bool, 8),
		subscriptions: make(map[string]bool),
	}
	client.waitGroup.Add(1)

	//
	go func() {
		client.waitGroup.Wait()
		client.Mutator.Close()
	}()

	go client.startMainLoop()
	go client.startReadingFromRedis()
	go client.startReadingFromWebSocket()

	log.Println("synk.newClient created:", client.ID)
	return client, nil
}

// Input: client.rSubscription
// Output: client.writeToWebSocket chan
//
// This is not the only goroutine that will send to writeToWebSocket, so it
// must not close writeToWebSocket when it encounters an error.
//
// Any one of the following events causes termination:
// - rPubSub (or rConn) is closed (Handler may close the rConn)
// - There is an error reading from rPubSub (triggers send on .quit channel)
func (client *Client) startReadingFromRedis() {
	defer client.Close()
	defer log.Println("Close startReadingFromRedis", client.ID)
	for {
		switch v := client.rSubscription.Receive().(type) {
		case redis.Message:
			client.handleByteSliceFromRedis(v.Data)
		case redis.PMessage:
			client.toWebSocket <- v.Data // v.Channel, v.Pattern, v.Data
		case redis.Subscription:
			// Redis is confirming our subscription v.Channel, v.Kind, v.Count
		case error:
			log.Println("Client.startReadingFromRedis: Subscription receive error:", v)
			// It Appears that the only way to exit out of this thread from code
			// is to .Close() the PubSubConn. However, I believe this case will
			// also execute if there is an error with with the Connection.
			// We do not know which one of these two things happened, but either
			// way we want to signal that we want to close this client.
			//
			// The main client goroutine (not this one) will Close the
			// PubSubConn when it exits.
			return
		}
	}
}

func (client *Client) handleByteSliceFromRedis(bytes []byte) {
	s := string(bytes)
	split := strings.Index(s, "{")
	if split == 0 {
		client.toWebSocket <- bytes
		return
	}
	if split == -1 {
		fmt.Printf("synk.Client: Error with bytes from redis: %s\n", bytes)
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
func (client *Client) startMainLoop() {
	ticker := time.NewTicker(pingInterval)

	defer ticker.Stop()
	defer client.Close()
	defer log.Println("Close startWritingToWebSocket", client.ID)

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
		case message := <-client.fromWebSocket:
			// Please note the genius of handling subscription methods here.
			// This allows us to start subscribing before we send the requested
			// chunks. However setTile methods that arrive after we send the
			// chunks to the client, because they will be qued in this goroutine.
			//
			// This gaurantees that the client will not miss setTile methods
			// between the time that it receives it's first setChunk and and
			// when the subscription takes effect.
			err := client.handleMessage(message)
			if err != nil {
				log.Println("Client.writeToWebSocket: Error handling message from client:", err)
				// Should this quit/return?
			}
		case <-client.quit:
			// Someone (possibly the redis goroutine) asked us to quit.
			// Acquiesce.
			return
		}
	}
}

// startReadingFromWebSocket pumps messages from the ws to the
// Client.messageChannel.
//
// Inputs: client.wsConn
// Output: client.messageChannel chan
//
// Terminate when wsConn.Readmessage returns an error. This happens when:
// - We call wsConn.Close() directly
// - An error reading from the websocket (ex. closed browser)
func (client *Client) startReadingFromWebSocket() {
	defer client.Close()
	defer log.Println("startReadingFromWebSocket returned", client.ID)

	for {
		_, bytes, err := client.wsConn.ReadMessage()
		if err != nil {
			log.Println("Client.startReadingFromWebSocket: ws read error:", err)
			break
		}
		// parse our bytes into a message
		message, parseErr := MessageFromBytes(bytes)
		if parseErr != nil {
			// Failed to parse message. Report error, but don't break out of the loop.
			log.Println("Client.startReadingFromWebSocket: error parsing bytes:", parseErr)
		} else {
			client.fromWebSocket <- message
		}
	}
}

func (client *Client) String() string {
	return fmt.Sprintf("{Client.ID: %s}", client.ID)
}

// not safe for concurrent calls
func (client *Client) handleMessage(message interface{}) error {
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
func (client *Client) updateSubscription(msg UpdateSubscriptionMessage) error {
	var err error

	add := make([]interface{}, len(msg.Add))
	remove := make([]interface{}, len(msg.Remove))

	for i, subKey := range msg.Remove {
		remove[i] = subKey
		delete(client.subscriptions, subKey)
	}
	for i, subKey := range msg.Add {
		add[i] = subKey
		client.subscriptions[subKey] = true
	}

	// Send the unsubscribe request
	if len(remove) > 0 {
		err = client.rSubscription.Unsubscribe(remove...)
	}
	if err != nil {
		log.Println("Client.updateSubscription: error with Unsubscribe", err)
		return err
	}

	// Send subscribe request
	if len(add) > 0 {
		err = client.rSubscription.Subscribe(add...)

		if err != nil {
			log.Println("Client.updateSubscription: error with Subscribe", err)
			return err
		}

		objs, err := client.Mutator.Load(msg.Add)

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
func (client *Client) Close() {
	client.closeOnce.Do(func() {
		//client.rSubscription.Unsubscribe() not needed, we are closing the connection
		client.rConn.Close()
		client.wsConn.Close()
		client.waitGroup.Done()
	})
	client.quit <- true
}

// May only be called from the mainLoop. Not safe for concurrent calls.
func (client *Client) writeToWebSocket(message []byte) error {
	client.wsConn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := client.wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
		log.Println("Client: Error Writing to websocket:", err)
		return err
	}
	return nil
}

// WriteToWebSocket sends data via websocket. Safe for concurrent calls.
//
// While this is the same as just sending data to the toWebSocket channel, this
// method is exported while the channel is not. This way it is harder for client
// code to accidentally overwrite the channel.
func (client *Client) WriteToWebSocket(data []byte) {
	client.toWebSocket <- data
}
