package synk

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/garyburd/redigo/redis"
)

// HandleMessage applies the supplied Message to a given redigo connection.
// It should mutate the database and publish any JSON messages required to
// update clients subscribed to the db.
func HandleMessage(msg interface{}, rConn redis.Conn) error {
	switch msg := msg.(type) {
	case ModObj:
		return redisModObject(msg.Object, rConn)
	case NewObj:
		return redisNewObject(msg.Object, rConn)
	case DelObj:
		return redisDelObject(msg.Object, rConn)
	default:
		txt := fmt.Sprintf("Unknown Message Type: %T", msg)
		return errors.New(txt)
	}
}

// Note that if redisNewObject is passed an unresolved Object, The unresolved
// version will be saved. This should be fine as long as the object gets
// passed to redisModObj later.
func redisNewObject(obj Object, rConn redis.Conn) error {
	subKey := obj.GetSubKey()
	objKey := obj.Key()
	msg := addObjMsg{
		State: obj.State(),
		Key:   objKey,
		SKey:  subKey,
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return errors.New("redisNewObject failed to convert diff to json")
	}

	redisJSON, err := json.Marshal(obj)
	if err != nil {
		return errors.New("redisNewObject failed to convert object to json")
	}

	// The script ensures that we do not accidentally overwrite a redis KEY
	val, err := redis.String(setAndPublishScript.Do(rConn, objKey, subKey, redisJSON, msgJSON))

	if val == "NO" {
		txt := "synk.redisNewObject failed to create object. Redis key '%s' already exists"
		return fmt.Errorf(txt, objKey)
	}

	return err
}

// This is the newer updated Objects mutator.
// Expects an unresolved object - But NOT a ModObj struct.
// Send the diff to the old chunk
// Send the full object to the new Chunk
func redisModObject(m Object, rConn redis.Conn) error {
	// Previous and new Subscription keys
	psk := m.GetPrevSubKey()
	nsk := m.GetSubKey()
	key := m.Key()
	sameSKey := psk == nsk
	// Create the message to send to clients
	msg := modObjMessage{
		Diff: m.Resolve(),
		Key:  key,
		SKey: psk,
	}

	if !sameSKey {
		msg.NSKey = nsk
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return errors.New("redisModObject failed to convert diff to JSON")
	}

	// This is the object we will save in redis
	objJSON, err := json.Marshal(m)
	if err != nil {
		return errors.New("redisModObject failed to convert object to JSON")
	}

	// Is the object in the same chunk as before?
	if sameSKey {
		rConn.Send("MULTI")
		rConn.Send("SADD", nsk, key) // Redundant, but safer
		rConn.Send("SET", key, objJSON)
		rConn.Send("PUBLISH", psk, msgJSON)
		_, err = rConn.Do("EXEC")
		return err
	}

	// The object changed chunks. We will need to update two redis sets, and
	// publish in two places.
	addMsg := addObjMsg{
		State: m.State(),
		Key:   key,
		SKey:  nsk,
		PSKey: psk,
	}
	addJSON, err := json.Marshal(addMsg)
	if err != nil {
		return errors.New("redisModObject failed to convert full state to JSON")
	}

	// This line below is a hack that I am using to tell our sever to contionally
	// send this message to the client. If the client is subscribed to the chunk
	// that this object is moving from, then that client will receive the diff and
	// they do not need or want to receive the addObj message. Our websocket
	// server checks if JSON messages are prefixed with the from string, and only
	// sends the message to the client if it is needed.
	addJSON = []byte("from " + psk + string(addJSON))

	rConn.Send("MULTI")
	rConn.Send("SREM", psk, key)
	rConn.Send("SADD", nsk, key)
	rConn.Send("SET", key, objJSON)
	rConn.Send("PUBLISH", psk, msgJSON)
	rConn.Send("PUBLISH", nsk, addJSON)
	_, err = rConn.Do("EXEC")

	return err
}

func redisDelObject(msg Object, rConn redis.Conn) error {

	// Note that we are using the Previous subscription key. If we are deleting
	// an object that was moving to another subscription, but the move was not yet
	// resolved, the clients will still think the character is in the old subKey.
	remMsg := remObjMsg{
		SKey: msg.GetPrevSubKey(),
		Key:  msg.Key(),
	}

	remJSON, err := json.Marshal(remMsg)
	if err != nil {
		txt := "synk.redisDelObj failed to convert msg to json: " + err.Error()
		return errors.New(txt)
	}

	rConn.Send("MULTI")
	rConn.Send("SREM", remMsg.SKey, msg.Key())
	rConn.Send("DEL", remMsg.Key)
	rConn.Send("PUBLISH", remMsg.SKey, remJSON)
	_, err = rConn.Do("EXEC")

	return err
}
