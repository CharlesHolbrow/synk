package synk

// ContainerConstructor creates an Object containers for a given type key. This
// allows client code to pass in custom logic for building containers based on
// client types
type ContainerConstructor func(typeKey string) Object

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

// Mutator represents a type that can get and modify objects.
//
// Typically a Mutator object will be constructed by client code, and
// initialized with a connection to a database and messaging service.
//
// When writing a Mutator it should not keep a connection to a database open.
//
// For example if using:
// - redis/redigo - store a connection pool, get a connection from the pool
// - mongodb/mgo  - store a session that is Copied() from another session
type Mutator interface {
	Create(obj Object) error
	Delete(obj Object) error
	Modify(obj Object) error
	Load(subKeys []string) ([]Object, error)
	Close() error
}

// If you are writing a sync server, you will probably want to write custom
// handlers for messages received from clients. You will need to implement
// the CustomClient AND a constructor for that Client type.
//
// When a client makes a websocket connection, you constructor will be called,
// and passed a Client object. The constructor function is expected to return
// an instance of your CustomClient that provides the OnConnect and OnMessage
// callbacks.

// CustomClient provides an interface for creatnig custom behavior when
// a client creates a Connection to the synk server. It also provides an
// interface for writing custom message handlers.
type CustomClient interface {
	OnConnect(client *Client)
	OnMessage(client *Client, method string, data []byte)
	OnSubscribe(client *Client, subKeys []string, objs []Object)
}

// A CustomClientConstructor must be supplied when implementing custom handlers.
// The supplied function will create the custom message handler when clients connect
type CustomClientConstructor func(client *Client) CustomClient
