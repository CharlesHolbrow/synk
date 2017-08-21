package synk

import (
	"log"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Handler upgrades http requests to websockets. Each new request will have a
// websocket connection. Made to be used with the http.Handle function.
type Handler struct {
	redisAddr         string
	clientPool        *ClientPool
	redisPool         *redis.Pool
	builder           BuildObject
	clientConstructor CustomClientConstructor
}

// NewHandler creates a WsHandler for use with http.Handle
func NewHandler(redisAddr string, builder BuildObject, constructor CustomClientConstructor, synkConn *RedisConnection) *Handler {

	clientPool := newClientPool()
	go clientPool.run()

	h := &Handler{redisAddr, clientPool, &synkConn.Pool, builder, constructor}

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
	client, err := newClient(h.redisAddr, wsConn, h.builder, h.redisPool)
	if err != nil {
		log.Println("Failed to create Client:", err)
		wsConn.Close()
		return
	}

	client.custom = h.clientConstructor(client)
	client.custom.OnConnect(client)

	// Now that the client was created successfully, It is the client's
	// responsibility to close the wsConn

	h.clientPool.add <- client
	client.waitGroup.Wait()
	h.clientPool.remove <- client
}
