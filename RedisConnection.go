package synk

import (
	"log"
	"time"

	"github.com/garyburd/redigo/redis"
)

// RedisConnection wraps a redigo connection Pool
type RedisConnection struct {
	addr       string
	Pool       redis.Pool
	objMsgConn redis.Conn  // will be passed to our mutator functions
	ToRedis    chan Object // Should be one of NewObj, DelObj, or ModObj
}

// NewConnection builds a new AetherRedisConnection
func NewConnection(redisAddr string) *RedisConnection {
	arc := &RedisConnection{
		addr: redisAddr,
		Pool: redis.Pool{
			MaxIdle:     100,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				conn, err := redis.Dial("tcp", redisAddr, redis.DialConnectTimeout(8*time.Second))
				if err != nil {
					log.Println("Failed to connect to redis:", err.Error())
				}
				return conn, err
			},
		},
	}
	arc.objMsgConn = arc.Pool.Get()
	arc.ToRedis = make(chan Object, 128)

	go func() {
		for msg := range arc.ToRedis {
			err := HandleMessage(msg, arc.objMsgConn)
			if err != nil {
				log.Printf("NewConnection: Error sending message to redis:\n"+
					"\tError: %s\n"+
					"\tMessage: %s\n", err, msg)
			}
		}
	}()

	return arc
}

// Create an object in redis. Safe for concurrent calls. Note that this mutates
// the object:
// - If the object has no ID, add a random ID
// - call the objects Resolve() method, which clears the objects's diff
//
// Note that creating an object in this way prevents us from knowing if the
// object creation succeeded. If we want that, it might be worth getting a
func (synkConn *RedisConnection) Create(obj Object) {
	if obj.GetID() == "" {
		obj.SetID(NewID().String())
	}

	// While we were creating this object struct, we may have used setters to
	// set initial values. Resolve them here so the newly created object is
	// up-to-date.
	obj.Resolve()

	// Remember, we can't write to a redigo connection concurrently. We must
	// either copy the object, and send it to a channel dedicated to a single
	// connection, OR we must get a channel from the pool that will be dedicated
	// to a given goroutine. This Method is intended to be called concurrently,
	// so here we must copy the object
	objCopy := obj.Copy()

	// The main purpose of init is to ensure that all the fields will be sent
	// to clients.
	objCopy.Init()

	synkConn.ToRedis <- NewObj{objCopy}
}

// Delete an object from the DB and from clients
//
// IMPORTANT: The object must be unresolved, so that client side copies will
// still think that the character is in the previous subscription key (in the
// event that it changed subscription keys before being passed here). Only synk
// code should ever call an objects .Resolve() method.
func (synkConn *RedisConnection) Delete(obj Object) {
	// I don't think we need to copy the object, because it should have already
	// deleted from other places. This is not thoroughly tested, so I'm going to
	// do it anyway for now.
	// unresolved -- which means that the client side copies will still think
	// that the character is in the previous subscription key.
	synkConn.ToRedis <- DelObj{
		Object: obj.Copy(),
	}
}

// Modify an object in redis.
func (synkConn *RedisConnection) Modify(obj Object) {
	if obj.Changed() {
		synkConn.ToRedis <- ModObj{Object: obj.Copy()}
		// BUG(charles): see notes in Create about Resolving() immediately
		obj.Resolve()
	}
}
