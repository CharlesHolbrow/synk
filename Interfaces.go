package synk

// ObjectLoader is a type that can load objects from redis.
type ObjectLoader interface {
	LoadObject(typeKey string, bytes []byte)
}

// ContainerConstructor creates an Object containers for a given type key. This
// allows client code to pass in custom logic for building containers based on
// client types
type ContainerConstructor func(typeKey string) MongoObject

// There are two ways to modify Objects.
// 1. The top level Create/Delete/Modify functions. Use these when you need
//    confirmation
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
	Key() string     // This used to the the ID
	TypeKey() string // We still use this
	GetSubKey() string
	GetPrevSubKey() string
	GetID() string       // Remove this
	SetID(string) string // Remove this
	Version() uint
}

// MongoObject represents any object that can be saved in MongoDB
type MongoObject interface {
	Object
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
	Create(obj MongoObject) error
	Delete(obj MongoObject) error
	Modify(obj MongoObject) error
	Load(subKeys []string) ([]MongoObject, error)
	Close() error
}
