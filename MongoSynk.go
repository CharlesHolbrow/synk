package synk

import (
	"fmt"
	"log"

	mgo "gopkg.in/mgo.v2"
)

func epanic(reason string, err error) {
	if err == nil {
		return
	}
	log.Panicln(reason, err.Error())
}

type MongoSynk struct {
	session *mgo.Session
	db      *mgo.Database
}

type MongoObject interface {
	Object
	TagInit(typeKey string) string
	TagSetSub(sKey string)
	TagSetID(id string) error
}

// NewMongoSynk returns a new MongoSynk instance. Panic if dialing mongo fails.
func NewMongoSynk() *MongoSynk {
	session, err := mgo.Dial("localhost")
	epanic("Error connecting to mongo", err)

	return &MongoSynk{
		session: session,
		db:      session.DB("synk"),
	}
}

func (ms *MongoSynk) send(msg addMsg) error {
	fmt.Printf("[STUB] - Sending addMsg: %v", msg)
	return nil
}

func (ms *MongoSynk) Create(obj MongoObject) {
	// We may have used setters when building the object (this is recommended).
	// Resolve the object to apply any pending changes.
	obj.Resolve()

	// Set the object's ID, so that the correct value will be stored in mongodb.
	id := obj.TagInit(obj.TypeKey())
	subKey := obj.GetSubKey()

	// Make sure to add the subscription to the Tag so that it will be correct
	// in mongodb.
	obj.TagSetSub(subKey)

	msg := addMsg{
		State:   obj.State(),
		ID:      id,
		SKey:    subKey,
		Version: obj.Version(),
	}

	err := ms.db.C("objects").Insert(obj)
	epanic("Failed to create new character in Mongodb", err)

	err = ms.send(msg)
	epanic("Trying to send addObjMsg", err)
}

// New style message to send to Clients
// addObjMsg is sent to the client to tell that client to create a new object.
// This would happen when an object moves into the client's subscription, OR
// when an object is newly created.
type addMsg struct {
	Method addObjMethod `json:"method"`
	State  interface{}  `json:"state"`
	ID     string       `json:"id"`
	// SKey is where we add this object to
	SKey string `json:"sKey"`
	// If the object is moving from another chunk, include psKey
	PSKey   string `json:"psKey,omitempty"`
	Version uint   `json:"v"`
}

type addMethod struct{}

func (m addMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"add\""), nil
}
