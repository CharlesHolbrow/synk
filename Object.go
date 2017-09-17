package synk

import (
	"github.com/garyburd/redigo/redis"
)

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
	Key() string
	TypeKey() string
	GetSubKey() string
	GetPrevSubKey() string
	GetID() string
	SetID(string) string
}

// Create an object in redis. Wait for redis to respond.
// Invokes object's Resolve() method
func Create(obj Object, conn redis.Conn) error {
	if obj.GetID() == "" {
		obj.SetID(NewID().String())
	}
	obj.Resolve()
	obj.Init()
	return redisNewObject(obj, conn)
}

// Delete an object from Redis. Wait for redis to respond
func Delete(obj Object, conn redis.Conn) error {
	return redisDelObject(obj, conn)
}

// Modify an object. Wait for redis to respond.
// Invokes object's Resolve() method
func Modify(obj Object, conn redis.Conn) (err error) {
	if obj.Changed() {
		err = redisModObject(obj, conn)
		obj.Resolve()
	}
	return
}

// Messages that are sent TO HandleMessages

// NewObj message is emitted by Fragment when an object is created
type NewObj struct {
	Object
}

// DelObj message is emitted by Fragment when an object is removed from the map altogether
type DelObj struct {
	Object
}

// ModObj message is emitted by a Fragment when an object is changed
type ModObj struct {
	Object
}

// Messages that are sent FROM redis mutators to the client. These should
// - Have a the 'method' field that JSONifies to a method name
// - Have an sKey string field that indicates a subscription field
// - potentially have a new SKey 'nsKey'

// modObjMessage represents relative changes made to an object.
//
// This is also the message that the client receives when the object is moving
// from one chunk to another.
type modObjMessage struct {
	Method modObjMethod `json:"method"`
	Diff   interface{}  `json:"diff"`
	Key    string       `json:"key"`
	// SKey is the subscription key where the object is prior to movement.
	SKey string `json:"sKey"`
	// NSKey is the subscription key that the object is moving to. Only present if
	// the object is changing chunks.
	NSKey string `json:"nsKey,omitempty"`
}

type modObjMethod struct{}

func (m modObjMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"modObj\""), nil
}

// addObjMsg is sent to the client to tell that client to create a new object.
// This would happen when an object moves into the client's subscription, OR
// when an object is newly created.
type addObjMsg struct {
	Method addObjMethod `json:"method"`
	State  interface{}  `json:"state"`
	Key    string       `json:"key"`
	// SKey is where we add this object to
	SKey string `json:"sKey"`
	// If the object is moving from another chunk, include psKey
	PSKey string `json:"psKey,omitempty"`
}

type addObjMethod struct{}

func (m addObjMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"addObj\""), nil
}

// remObjMsg tells clients to remove and teardown an object. This is NOT the
// message that a client gets when an object is moving from one chunk to
// another, even if the client is not subscribed to the destination.
type remObjMsg struct {
	Method remObjMethod `json:"method"`
	SKey   string       `json:"sKey"`
	Key    string       `json:"key"`
}

type remObjMethod struct{}

func (m remObjMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"remObj\""), nil
}
