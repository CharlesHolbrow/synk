package synk

import (
	"encoding/json"
	"log"

	"github.com/garyburd/redigo/redis"
	"gopkg.in/mgo.v2/bson"

	mgo "gopkg.in/mgo.v2"
)

func epanic(reason string, err error) {
	if err == nil {
		return
	}
	log.Panicln(reason, err.Error())
}

// MongoSynk interfaces with the mongodb database
type MongoSynk struct {
	session *mgo.Session
	db      *mgo.Database
	RConn   redis.Conn
}

// NewMongoSynk returns a new MongoSynk instance. Panic if dialing mongo fails.
// BUG(charles): you are responsible for adding the Creator and RConn manually.
func NewMongoSynk() *MongoSynk {
	session, err := mgo.Dial("localhost")
	epanic("Error connecting to mongo", err)

	return &MongoSynk{
		session: session,
		db:      session.DB("synk"),
	}
}

////////////////////////////////////////////////////////////////
//
// Methods for extracting objects from MonogoDB
//
////////////////////////////////////////////////////////////////

type typeOnly struct {
	Type string `bson:"t"`
}

// GetObjects retrieves all objects from MongoDB that are in a given slice of
// subscription Keys
func GetObjects(coll *mgo.Collection, sKeys []string, creator ContainerConstructor) []MongoObject {
	var rawResults []bson.Raw
	var results []MongoObject
	var err error

	err = coll.Find(bson.M{"sub": bson.M{"$in": sKeys}}).All(&rawResults)
	epanic("MongoSynk.GetObjects: error with .All mongo query", err)

	results = make([]MongoObject, 0, len(rawResults))
	for _, raw := range rawResults {

		temp := typeOnly{}
		err = raw.Unmarshal(&temp)
		if err != nil {
			log.Println("MongoSynk.GetObjects failed to get ID from raw mongo object:", err.Error())
			continue
		}
		container := creator(temp.Type)
		if container == nil {
			log.Println("MongoSynk.GetObjects found no container for type:", temp.Type)
			continue
		}
		err = raw.Unmarshal(container)
		if err != nil {
			log.Println("MongoSynk.GetObjects faild to marshal object into container", temp.Type)
			continue
		}
		results = append(results, container)
	}
	return results
}

////////////////////////////////////////////////////////////////
//
// Three Main Mutation Methods: Create, Modify, Delete
//
////////////////////////////////////////////////////////////////

// Create an object, and send an add message
func (ms *MongoSynk) Create(obj MongoObject) {
	typeKey := obj.TypeKey()

	// This will set the object's ID and Type, so that the correct value will
	// be stored in mongodb.
	obj.TagInit(typeKey)

	// We may have used setters when building the object (this is recommended).
	// Resolve the object to apply any pending changes.
	obj.Resolve()

	// Make sure to add the subscription to the Tag so that it will be correct
	// in mongodb.
	subscription := obj.GetSubKey()
	obj.TagSetSub(subscription)

	msg := addMsg{
		State:   obj.State(),
		ID:      obj.TagGetID(), // full ID
		SKey:    subscription,
		Version: obj.Version(),
		Type:    typeKey,
	}

	err := ms.db.C("objects").Insert(obj)
	epanic("Failed to create new character in Mongodb", err)

	err = ms.send(msg)
	epanic("Trying to send addObjMsg", err)
}

// Modify a MongoObject, publishing a mod message once the mutation is complete.
func (ms *MongoSynk) Modify(obj MongoObject) {
	var err error

	nsk := obj.GetSubKey()
	psk := obj.GetPrevSubKey()
	id := obj.TagGetID()

	// The Modify() operation is considered simple iff the object's subscription
	// is unchanged.
	simple := nsk == psk

	// Create the message to send to clients
	msg := modMsg{
		Diff:    obj.Resolve(),
		ID:      id,
		SKey:    psk,
		Version: obj.Version(),
	}

	if simple {
		msg.SKey = nsk
	}

	if simple {
		err = ms.db.C("objects").UpdateId(id, obj)
		epanic("Mongosynk.Modify failed to insert object", err)
		err = ms.sendMod(msg)
		epanic("MongoSynk.Modify failed to send message", err)
		return
	}

	// The object changed chunks. We will need to update two redis sets, and
	// publish in two places.

	// Update the subscription key used by mongodb
	obj.TagSetSub(nsk)
	// This add message includes a psk
	amsg := addMsg{
		State:   obj.State(),
		ID:      id,
		SKey:    nsk,
		PSKey:   psk,
		Version: obj.Version(),
	}

	obj.TagSetSub(nsk)

	err = ms.db.C("objects").Insert(obj)
	epanic("MongoSynk.Modify failed to insert on a non-simple Modify", err)
	err = ms.sendMod(msg)
	epanic("MongoSynk.Modify failed to send mod message on a non-simple Modify", err)
	err = ms.sendAddFrom(amsg, psk)
	epanic("MongoSynk.Modify failed to send add message on a non-simple Modify", err)
}

// Delete an object from the db, publishing a rem message on completion.
func (ms *MongoSynk) Delete(obj MongoObject) {
	msg := remMsg{
		SKey: obj.GetPrevSubKey(),
		ID:   obj.TagGetID(),
		Type: obj.TypeKey(),
	}

	err := ms.db.C("objects").RemoveId(obj.TagGetID())
	epanic("MongoSynk.Delete failed to Remove an object", err)
	err = ms.sendRem(msg)
	epanic("MongoSynk.Delete failed to send rem message", err)
}

////////////////////////////////////////////////////////////////
//
// Stubs for sending messages to clients
//
////////////////////////////////////////////////////////////////

func (ms *MongoSynk) send(msg addMsg) error {
	log.Printf("[STUB] - Sending addMsg for %s to %s with type %s", msg.ID, msg.SKey, msg.Type)
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ms.RConn.Do("PUBLISH", msg.SKey, bytes)
	return err
}
func (ms *MongoSynk) sendMod(msg modMsg) error {
	log.Printf("[STUB] - Sending modMsg for %s to %s", msg.ID, msg.SKey)
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ms.RConn.Do("PUBLISH", msg.SKey, bytes)
	return err
}
func (ms *MongoSynk) sendAddFrom(msg addMsg, from string) error {
	log.Printf("[STUB] - Sending addMsg for %s to %s for object moving from %s", msg.ID, msg.SKey, from)
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ms.RConn.Do("PUBLISH", msg.SKey, []byte("from "+msg.PSKey+string(bytes)))
	return err
}
func (ms *MongoSynk) sendRem(msg remMsg) error {
	log.Printf("[STUB] - Sending remMsg for %s to %s", msg.ID, msg.SKey)
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ms.RConn.Do("PUBLISH", msg.SKey, bytes)
	return err
}

////////////////////////////////////////////////////////////////
//
// New style message to send to Clients
//
////////////////////////////////////////////////////////////////

// addObjMsg is sent to the client to tell that client to create a new object.
// This would happen when an object moves into the client's subscription, OR
// when an object is newly created.
type addMsg struct {
	Method addMethod   `json:"method"`
	State  interface{} `json:"state"`
	ID     string      `json:"id"`
	// SKey is where we add this object to
	SKey string `json:"sKey"`
	// If the object is moving from another chunk, include psKey
	PSKey   string `json:"psKey,omitempty"`
	Version uint   `json:"v"`
	Type    string `json:"t"`
}

type addMethod struct{}

func (m addMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"add\""), nil
}

// modObjMessage represents relative changes made to an object.
//
// This is also the message that the client receives when the object is moving
// from one chunk to another.
type modMsg struct {
	Method  modMethod   `json:"method"`
	Diff    interface{} `json:"diff"`
	ID      string      `json:"id"`
	Version uint        `json:"v"`
	// SKey is the subscription key where the object is prior to movement.
	SKey string `json:"sKey"`
	// NSKey is the subscription key that the object is moving to. Only present if
	// the object is changing chunks.
	NSKey string `json:"nsKey,omitempty"`
}

type modMethod struct{}

func (m modMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"mod\""), nil
}

// remObjMsg tells clients to remove and teardown an object. This is NOT the
// message that a client gets when an object is moving from one chunk to
// another, even if the client is not subscribed to the destination.
type remMsg struct {
	Method remMethod `json:"method"`
	SKey   string    `json:"sKey"`
	Type   string    `json:"t"`
	ID     string    `json:"id"`
}

type remMethod struct{}

func (m remMethod) MarshalJSON() ([]byte, error) {
	return []byte("\"rem\""), nil
}
