package synk

import (
	"log"
	"time"

	"github.com/garyburd/redigo/redis"
	mgo "gopkg.in/mgo.v2"
)

// Helper struct used by toRedisChan
type toRedis struct {
	commandName string
	args        []interface{}
}

// Synk wraps a redigo connection Pool
type Synk struct {
	addr string
	Pool redis.Pool

	// Objects sent to this channel should be one of NewObj, DelObj, or ModObj
	// The associated redis connection will be passed to our mutator functions.
	// Redis Connections are not safe for concurrent use.
	MutateRedisChan chan Object
	objMsgConn      redis.Conn

	// Messages sent to this channel will be be forwarded to redis via the
	// 'toRedisConn' channel. This channel is exposed publically with Synk
	// methods like RedisConnection.Publish
	toRedisChan chan toRedis
	toRedisConn redis.Conn

	// I am migrating from Redis to MongoDB
	MongoSynk *MongoSynk //BUG(charles): Monogo refactor - Is this even needed?
	Mongo     *mgo.Session
}

// NewConnection builds a new AetherRedisConnection
func NewConnection(redisAddr string) *Synk {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic("Error Dialing mongodb: " + err.Error())
	}

	arc := &Synk{
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
		MongoSynk: NewMongoSynk(), //BUG(charles): Monogo refactor - Is this even needed?
		Mongo:     session,
	}

	arc.MongoSynk.RConn = arc.Pool.Get()

	// This channel/connection combination is for mutating objects
	arc.MutateRedisChan = make(chan Object, 128)
	arc.objMsgConn = arc.Pool.Get()

	go func() {
		for msg := range arc.MutateRedisChan {
			err := HandleMessage(msg, arc.objMsgConn)
			if err != nil {
				log.Printf("NewConnection: Error sending message to redis:\n"+
					"\tError: %s\n"+
					"\tMessage: %s\n", err, msg)
			}
		}
	}()

	// Continuously pump messages from toRedisChan to redis. Panic if there is
	// an error sending to redis.
	arc.toRedisChan = make(chan toRedis, 1000) // 1000 is arbitrary
	arc.toRedisConn = arc.Pool.Get()

	go func() {
		for val := range arc.toRedisChan {
			if _, err := arc.toRedisConn.Do(val.commandName, val.args...); err != nil {
				log.Printf("synk.Handler encountered an error trying to send %v: %s", val, err)
			}
		}
		log.Panicln("synk: NewHandler: handler.toRedisChan closed!")
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
func (synkConn *Synk) Create(obj Object) {
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

	synkConn.MutateRedisChan <- NewObj{objCopy}
}

// Delete an object from the DB and from clients
//
// IMPORTANT: The object must be unresolved, so that client side copies will
// still think that the character is in the previous subscription key (in the
// event that it changed subscription keys before being passed here). Only synk
// code should ever call an objects .Resolve() method.
func (synkConn *Synk) Delete(obj Object) {
	// I don't think we need to copy the object, because it should have already
	// deleted from other places. This is not thoroughly tested, so I'm going to
	// do it anyway for now.
	// unresolved -- which means that the client side copies will still think
	// that the character is in the previous subscription key.
	synkConn.MutateRedisChan <- DelObj{
		Object: obj.Copy(),
	}
}

// Modify an object in redis.
func (synkConn *Synk) Modify(obj Object) {
	if obj.Changed() {
		synkConn.MutateRedisChan <- ModObj{Object: obj.Copy()}
		// BUG(charles): see notes in Create about Resolving() immediately
		obj.Resolve()
	}
}

// Publish updates sends a message to be processed by redis.
func (synkConn *Synk) Publish(args ...interface{}) {
	synkConn.toRedisChan <- toRedis{commandName: "PUBLISH", args: args}
}
