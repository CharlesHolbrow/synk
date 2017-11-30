package synk

import (
	"fmt"
	"os"
	"strings"
)

// RedisAddr is the address that the synk library uses. You can specify a custom
// address by setting the SYNK_REDIS_ADDR environment variable.
//
// EX:
// SYNK_REDIS_ADDR=127.0.0.2
// SYNK_REDIS_ADDR=127.0.0.3:5555
//
// If no port is specified ":6379" is used
// If no host is specified redis's default (127.0.0.1) is used
var RedisAddr = os.Getenv("SYNK_REDIS_ADDR")

// MongoAddr is the address that the synk library uses by. Defaults to localhost
// Check mgo docs to see how ports are specified
var MongoAddr = os.Getenv("SYNK_MONGO_ADDR")

// Initialize some Defaults
func init() {
	// redigo accepts just a host number, which causes it to bind to 127.0.0.1
	if strings.Index(RedisAddr, ":") == -1 {
		RedisAddr = RedisAddr + ":6379"
	}

	if MongoAddr == "" {
		MongoAddr = "localhost"
	}

	fmt.Println("Redis Address:", RedisAddr)
	fmt.Println("Mongo Address:", MongoAddr)
}

// Config stores all the customization options for running a synk server. While
// it is not a golang interface, it is the main interface for client code
// that runs a synk server.
type Config struct {
	Loader                  Loader
	CustomClientConstructor ClientConstructor
}

// ContainerConstructor creates an Object container for a given type key. This
// allows client code to pass in custom logic for building containers based on
// client types
//
type ContainerConstructor func(typeKey string) Object

// CustomClient provides an interface for creatnig custom behavior when
// a client creates a Connection to the synk server. It also provides an
// interface for writing custom message handlers.
type CustomClient interface {
	OnConnect(client *Client)
	OnMessage(client *Client, method string, data []byte)
	OnSubscribe(client *Client, subKeys []string, objs []Object)
}

// A ClientConstructor must be supplied when implementing custom handlers.
// The supplied function will create the custom message handler when clients
// connect.
//
// If you are writing a sync server, you will probably want to write custom
// handlers for messages received from clients. You will need to implement
// the CustomClient AND a constructor for that Client type.
//
// When a client makes a websocket connection, you constructor will be called,
// and passed a Client object. The constructor function is expected to return
// an instance of your CustomClient that provides the OnConnect and OnMessage
// callbacks.
type ClientConstructor func(client *Client) CustomClient

// There are two ways to modify Objects.
//
// 1. The top level Create/Delete/Modify functions. Use these when you need
//    confirmation
//
// !!!!IMPORTANT!!!!! - as of November 2017 the method below is deprecated
//
// 2. The SynkRedisConnection's Create/Delete/Modify functions. I think these
//    should work fine for most things: if the write fails, we need to re-get
//    the collection we are working on, and re-start the simulation. Note that
//    this is how we handle the client connection too -- If the connection is
//    broken we just re-get the collection and continue where we left off.

// Object is the interface for anything that will be saved in redis with diffs
// that will be pushed to clients. The methods are a sub-set of the Character
// interface methods.
type Object interface {
	State() interface{}
	Resolve() interface{}
	Changed() bool
	Init()
	Copy() Object
	TypeKey() string // We still use this
	GetSubKey() string
	GetPrevSubKey() string
	Version() uint
	TagInit(typeKey string)
	TagGetID() string
	TagSetSub(sKey string)
}

// Mutator represents a type that can get/modify objects.
//
// Typically a Mutator object will be constructed by client code, and
// initialized with a connection to a database and messaging service.
//
// When writing a Mutator it should not keep a connection to a database open.
//
// For example if using:
// - redis/redigo - store a connection pool, get a connection from the pool
// - mongodb/mgo  - store a session that is Copied() from another session
//
// The synk library provides the MongoMutator and RedisMutator types, both of
// which satisfy Mutator. However -- Client code must provide a
// ContainerConstructor so the Loaded Objects can be deserialized correctly.
type Mutator interface {
	Create(obj Object) error
	Delete(obj Object) error
	Modify(obj Object) error
	Load(subKeys []string) ([]Object, error)
	Close() error
}

type Loader interface {
	Load(subKeys []string) ([]Object, error)
	Clone() Loader
	Close() error
}
