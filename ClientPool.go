package synk

// ClientPool is a collection of client that we can add, remove or broadcast to
// safely from multiple goroutines.
type ClientPool struct {
	// send a client here to create
	add       chan *synkClient
	remove    chan *synkClient
	all       map[ID]*synkClient
	broadcast chan []byte
}

func newClientPool() *ClientPool {
	pool := ClientPool{
		add:       make(chan *synkClient),
		remove:    make(chan *synkClient),
		all:       make(map[ID]*synkClient),
		broadcast: make(chan []byte),
	}
	return &pool
}

func (pool *ClientPool) run() {
	for {
		select {
		case client := <-pool.add:
			pool.all[client.id] = client
		case client := <-pool.remove:
			if _, ok := pool.all[client.id]; ok {
				close(client.toWebSocket)
				delete(pool.all, client.id)
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
