package synk

import (
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"gopkg.in/mgo.v2"
)

// Node is the main interface for interacting with a synk server.
//
// It is designed to be intialized passively without a constructor. The methods
// will panic if the recieving struct is not populated with the required public
// members.
type Node struct {
	mongoSession *mgo.Session
	redisPool    *redis.Pool
	NewContainer ContainerConstructor
	NewClient    ClientConstructor
	mutex        sync.RWMutex
}

// Create network connections. Panic if any connection fails.
func (node *Node) dial() {
	node.mutex.RLock()
	if node.mongoSession != nil && node.redisPool != nil {
		node.mutex.RUnlock()
		return
	}
	node.mutex.RUnlock()
	node.mutex.Lock()
	if node.mongoSession == nil {
		node.mongoSession = DialMongo()
	}
	if node.redisPool == nil {
		node.redisPool = DialRedisPool()
	}
	node.mutex.Unlock()
}

// CreateMutator returns a ready to use Mutator. The Mutator must be .Closed()
// when it is no longer needed.
//
// Panic if NewContainer is not initialized.
//
// BUG(charles): Should Mutators have a dedicated redis Connection?
func (node *Node) CreateMutator() Mutator {
	if node.NewContainer == nil {
		panic("Tried to get a mutator, but not NewContainer is not set")
	}
	node.dial()

	return &MongoSynk{
		Creator:   node.NewContainer,
		Coll:      node.mongoSession.Clone().DB("synk").C("objects"),
		RedisPool: node.redisPool,
	}
}

// CreateLoader returns a ready to use Loader. The Loader must be .Closed()
// when it is no longer needed.
//
// Panic if NewContainer is not initialized.
func (node *Node) CreateLoader() Loader {
	if node.NewContainer == nil {
		panic("Tried to get a Loader, but not NewContainer is not set")
	}
	node.dial()

	return &MongoSynk{
		Creator:   node.NewContainer,
		Coll:      node.mongoSession.Clone().DB("synk").C("objects"),
		RedisPool: node.redisPool,
	}
}

// Helpers

// DialRedisPool creates a redigo connection pool with the default synk
// configuration. The synk package level RedisAddr which is adapted from the
// SYNK_REDIS_ADDR environment variable is used to connect
func DialRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", RedisAddr, redis.DialConnectTimeout(8*time.Second))
			if err != nil {
				panic("Failed to connect to redis: " + err.Error())
			}
			return conn, err
		},
	}
}

// DialRedis gets a redis connection with the default synk configuration. The
// synk package level RedisAddr which is adapted from the SYNK_REDIS_ADDR
// environment variable is used to connect.
//
// Panic on connection error
func DialRedis() redis.Conn {
	conn, err := redis.Dial("tcp", RedisAddr, redis.DialConnectTimeout(8*time.Second))
	if err != nil {
		panic("Failed to connect to redis: " + err.Error())
	}
	return conn
}

// DialMongo creates the first MongoSession. Further sessions should be created
// with session.Copy()
func DialMongo() *mgo.Session {
	session, err := mgo.Dial(MongoAddr)
	if err != nil {
		panic("Error Dialing mongodb: " + err.Error())
	}
	return session
}
