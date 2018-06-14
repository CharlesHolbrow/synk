package synk

import (
	"fmt"
	"time"

	"github.com/CharlesHolbrow/pubsub"
	"github.com/garyburd/redigo/redis"
	"gopkg.in/mgo.v2"
)

// Node is the main interface for interacting with a synk server. Every golang
// application in a sync project should create one Node instance.
//
// Exported methods must be safe for concurrent calls.
type Node struct {
	mongoSession *mgo.Session
	redisPool    *redis.Pool
	redisAgents  *pubsub.RedisAgents
	newContainer ContainerConstructor
	newClient    ClientConstructor
}

// NewNode creates new a *Node with the default connections.
//
// Objects created with NewNode are not ready for use. The NewContainer and
// NewClient members must be set or the CreateMutator/CreateLoader methods will
// fail.
func NewNode() *Node {
	return &Node{
		mongoSession: DialMongo(),
		redisPool:    DialRedisPool(),
		redisAgents:  pubsub.NewRedisAgents(DialRedis()),
	}
}

// CreateMutator returns a ready to use Mutator. The Mutator must be .Closed()
// when it is no longer needed.
//
// Panic if NewContainer is not initialized.
//
// BUG(charles): Should Mutators have a dedicated redis Connection?
func (node *Node) CreateMutator() Mutator {
	if node.newContainer == nil {
		panic("Tried to get a mutator, but not NewContainer is not set")
	}

	return &MongoSynk{
		Creator:   node.NewContainer,
		Coll:      node.mongoSession.Clone().DB(MongoDBName).C("objects"),
		RedisPool: node.redisPool,
	}
}

// CreateLoader returns a ready to use Loader. The Loader must be .Closed()
// when it is no longer needed.
//
// Panic if NewContainer is not initialized.
func (node *Node) CreateLoader() Loader {
	if node.newContainer == nil {
		panic("Tried to get a Loader, but not NewContainer is not set")
	}

	return &MongoSynk{
		Creator:   node.newContainer,
		Coll:      node.mongoSession.Clone().DB(MongoDBName).C("objects"),
		RedisPool: node.redisPool,
	}
}

// RegisterClientConstructor sets function that will be called to create a
// custom client. Consumer code must register a constructor to provide custom
// handlers for messages from WebSocket clients.
func (node *Node) RegisterClientConstructor(constructor ClientConstructor) {
	if node.newClient != nil {
		panic("synk.Node cannot register an additional ClientConstructor")
	}
	node.newClient = constructor
}

// RegisterContainerConstructor sets the function that will be called to create
// containers for synk objects. It is the responsibility of client code to
// register a constructor that handles objects based on their type key.
func (node *Node) RegisterContainerConstructor(constructor ContainerConstructor) {
	if node.newContainer != nil {
		panic("synk.Node cannot register an additional ContainerConstructor")
	}
	node.newContainer = constructor
}

// NewContainer returns a synk object based on the consuming code's registered
// ContainerConstructor.
func (node *Node) NewContainer(typeKey string) Object {
	return node.newContainer(typeKey)
}

// NewClient returns a custom client based on the consuming code's registered
// ClientConstructor.
func (node *Node) NewClient(c Client) CustomClient {
	return node.newClient(c)
}

// Helpers

// DialRedisPool creates a redigo connection pool with the default synk
// configuration. The synk package level RedisAddr which is adapted from the
// SYNK_REDIS_ADDR environment variable is used to connect.
//
// Panic on connection error
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
// Panic on connection error.
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

	if mongoLoginRequired {
		err = session.Login(&mgo.Credential{
			Username:  mongoUser,
			Password:  mongoPass,
			Source:    MongoDBName,
			Mechanism: "SCRAM-SHA-1",
		})
		if err != nil {
			panic("Error Authorizing mongodb: " + err.Error())
		} else {
			fmt.Println("Successfully authenticated:", mongoUser)
		}
	}

	return session
}
