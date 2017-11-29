package synk

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Handler upgrades http requests to websockets. Each new request will have a
// websocket connection. Made to be used with the http.Handle function.
type Handler struct {
	clientPool *ClientPool
	SynkConn   *Synk
	config     *Config
}

// NewHandler creates a WsHandler for use with http.Handle
func NewHandler(config *Config) *Handler {

	h := &Handler{
		config:     config,
		SynkConn:   NewSynk(config),
		clientPool: newClientPool(),
	}

	go h.clientPool.run()

	return h
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Get a pointer to a websocket connection
	wsConn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println("ws upgrade error:", err)
		// July 18, 2017: calling wsConn.Close() panics
		return
	}

	// create a new Client object
	client, err := newClient(h.config, h.SynkConn, wsConn)

	if err != nil {
		log.Println("Failed to create Client:", err)
		wsConn.Close()
		return
	}

	client.custom = h.config.CustomClientConstructor(client)
	client.custom.OnConnect(client)

	// Now that the client was created successfully, It is the client's
	// responsibility to close the wsConn

	h.clientPool.add <- client
	client.waitGroup.Wait()
	h.clientPool.remove <- client
}
