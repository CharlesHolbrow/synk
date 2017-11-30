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
//
// Important: The Collection must be a unique session.
type MongoSynk struct {
	Coll      *mgo.Collection
	Creator   ContainerConstructor
	RedisPool *redis.Pool
}

// Clone this MongySynk.
func (ms *MongoSynk) Clone() Loader {
	return &MongoSynk{
		Coll:      ms.Coll.Database.Session.Copy().DB(ms.Coll.Database.Name).C(ms.Coll.Name),
		Creator:   ms.Creator,
		RedisPool: ms.RedisPool,
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

// Load retrieves all objects from MongoDB that are in a given slice of
// subscription Keys
func (ms *MongoSynk) Load(sKeys []string) ([]Object, error) {
	var rawResults []bson.Raw
	var results []Object
	var err error

	err = ms.Coll.Find(bson.M{"sub": bson.M{"$in": sKeys}}).All(&rawResults)
	epanic("MongoSynk.GetObjects: error with .All mongo query", err)

	results = make([]Object, 0, len(rawResults))
	for _, raw := range rawResults {

		temp := typeOnly{}
		err = raw.Unmarshal(&temp)
		if err != nil {
			log.Println("MongoSynk.GetObjects failed to get ID from raw mongo object:", err.Error())
			continue
		}
		container := ms.Creator(temp.Type)
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
	return results, nil
}

////////////////////////////////////////////////////////////////
//
// Three Main Mutation Methods: Create, Modify, Delete
//
////////////////////////////////////////////////////////////////

// Create an object, and send an add message
func (ms *MongoSynk) Create(obj Object) error {
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

	err := ms.Coll.Insert(obj)
	epanic("Failed to create new object in Mongodb", err)

	err = ms.send(msg)
	epanic("Failed to send addMsg", err)
	return nil
}

// Modify a MongoObject, publishing a mod message once the mutation is complete.
func (ms *MongoSynk) Modify(obj Object) error {
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

	if !simple {
		msg.NSKey = nsk
	}

	if simple {
		err = ms.Coll.UpdateId(id, obj)
		epanic("Mongosynk.Modify failed to insert object", err)
		err = ms.sendMod(msg)
		epanic("MongoSynk.Modify failed to send message", err)
		return nil
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
		Type:    obj.TypeKey(),
	}

	obj.TagSetSub(nsk)

	err = ms.Coll.UpdateId(id, obj)
	epanic("MongoSynk.Modify failed to insert on a non-simple Modify", err)
	err = ms.sendMod(msg)
	epanic("MongoSynk.Modify failed to send mod message on a non-simple Modify", err)
	err = ms.sendAddFrom(amsg, psk)
	epanic("MongoSynk.Modify failed to send add message on a non-simple Modify", err)

	return nil
}

// Delete an object from the db, publishing a rem message on completion.
func (ms *MongoSynk) Delete(obj Object) error {

	// Note that we are using the Previous subscription key. If we are deleting
	// an object that was moving to another subscription, but the move was not yet
	// resolved, the clients will still think the character is in the old subKey.
	msg := remMsg{
		SKey: obj.GetPrevSubKey(),
		ID:   obj.TagGetID(),
		Type: obj.TypeKey(),
	}

	err := ms.Coll.RemoveId(obj.TagGetID())
	epanic("MongoSynk.Delete failed to Remove an object", err)
	err = ms.sendRem(msg)
	epanic("MongoSynk.Delete failed to send rem message", err)

	return nil
}

// Close returns connection resources to their pools
func (ms *MongoSynk) Close() error {
	var returnError error
	if ms.Coll != nil {
		ms.Coll.Database.Session.Close()
	}
	return returnError
}

// Publish a message. If the message is a []byte, publish it directly. Otherwise
// Marshal it to JSON.
func (ms *MongoSynk) Publish(channel string, msg interface{}) error {
	conn := ms.RedisPool.Get()
	defer conn.Close()

	var bytes []byte
	var err error

	if msg, ok := msg.([]byte); ok {
		bytes = msg
	} else {
		bytes, err = json.Marshal(msg)
		if err != nil {
			return err
		}
	}

	_, err = conn.Do("PUBLISH", channel, bytes)
	return err
}

////////////////////////////////////////////////////////////////
//
// Stubs for sending messages to clients
//
////////////////////////////////////////////////////////////////

func (ms *MongoSynk) send(msg addMsg) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	rConn := ms.RedisPool.Get()
	defer rConn.Close()
	_, err = rConn.Do("PUBLISH", msg.SKey, bytes)
	return err
}

func (ms *MongoSynk) sendMod(msg modMsg) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	rConn := ms.RedisPool.Get()
	defer rConn.Close()

	_, err = rConn.Do("PUBLISH", msg.SKey, bytes)
	return err
}

func (ms *MongoSynk) sendAddFrom(msg addMsg, from string) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	rConn := ms.RedisPool.Get()
	defer rConn.Close()

	_, err = rConn.Do("PUBLISH", msg.SKey, []byte("from "+msg.PSKey+string(bytes)))
	return err
}

func (ms *MongoSynk) sendRem(msg remMsg) error {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	rConn := ms.RedisPool.Get()
	defer rConn.Close()

	_, err = rConn.Do("PUBLISH", msg.SKey, bytes)
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
