package synk

// ClientPool is a collection of client that we can add, remove or broadcast to
// safely from multiple goroutines.
type ClientPool struct {
	// send a client here to create
	add       chan *Client
	remove    chan *Client
	all       map[ID]*Client
	broadcast chan []byte
}

func newClientPool() *ClientPool {
	pool := ClientPool{
		add:       make(chan *Client),
		remove:    make(chan *Client),
		all:       make(map[ID]*Client),
		broadcast: make(chan []byte),
	}
	return &pool
}

func (pool *ClientPool) run() {
	for {
		select {
		case client := <-pool.add:
			pool.all[client.ID] = client
		case client := <-pool.remove:
			if _, ok := pool.all[client.ID]; ok {
				close(client.toWebSocket)
				delete(pool.all, client.ID)
			}
		case message := <-pool.broadcast:
			for _, client := range pool.all {
				select {
				case client.toWebSocket <- message:
				default:
					// buffer overrun
				}
			}
		}
	}
}
