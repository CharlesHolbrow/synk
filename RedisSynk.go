package synk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/garyburd/redigo/redis"
)

// RedisSynk interfaces with Redis for fast storage.
//
// Note that because redis is a KeyValue store it works a little differently
// than mongodb.
//
// Redis stores a "set" for each SubScription key. This set saves a redisKey
// for all the objects in that subscription set. A redisKey is in the format:
//
// typeKey:objectID
//
// So an object with id abc123 of type c:d would have the redisKey:
//
// c:d:abc123
//
// For this reason, Object IDs must bever have a ':' character.
type RedisSynk struct {
	Pool        *redis.Pool
	Constructor ContainerConstructor
}

// Clone this struct. Helps to satisfy synk.Mutator
func (rs *RedisSynk) Clone() Mutator {
	newRs := rs
	return newRs
}

// Create an Object, and store it in Redis
func (rs *RedisSynk) Create(obj Object) error {
	conn := rs.Pool.Get()
	defer conn.Close()
	return redisNewObject(obj, conn)
}

// Delete an Object stored in Redis
func (rs *RedisSynk) Delete(obj Object) error {
	conn := rs.Pool.Get()
	defer conn.Close()
	return redisDelObject(obj, conn)

}

// Modify an Object stored in Redis
func (rs *RedisSynk) Modify(obj Object) error {
	conn := rs.Pool.Get()
	defer conn.Close()
	return redisModObject(obj, conn)
}

// Close any open connections
func (rs *RedisSynk) Close() error {
	return nil
}

// Load Retrieves Objects from Redis
func (rs *RedisSynk) Load(subKeys []string) ([]Object, error) {
	conn := rs.Pool.Get()
	defer conn.Close()

	return RedisRequestObjects(conn, subKeys, rs.Constructor)
}

/***************************************************************
Below are functions that implementing most of the
functionality for RedisSynk.

Note that these are not RedisSynk methods
***************************************************************/

// RedisRequestByteSlices from redis. Given a slice of subscription keys, get
// byte slices for all keys. Results are returned as two parallel slices. If
// there are no results, the slices will be of length zero.
//
// The two slices are gauranteed to be of equal length.
func RedisRequestByteSlices(conn redis.Conn, subKeys []string) ([]string, [][]byte, error) {
	// The script requires the first argument to be the number of keys. We have to
	// make it one element longer than the points array.
	size := len(subKeys)
	args := make([]interface{}, size+1)
	args[0] = size
	for i, k := range subKeys {
		args[i+1] = k
	}

	// redis.Values will return []interface{}
	keysAndObjects, err := redis.Values(GetKeysObjects.Do(conn, args...))
	if err != nil {
		log.Println("RequestByteSlices - get values fail: " + err.Error())
		return nil, nil, err
	}

	if len(keysAndObjects) == 0 {
		return make([]string, 0), make([][]byte, 0), nil
	}

	keys, keyOk := redis.Strings(keysAndObjects[0], nil)
	vals, valOk := redis.ByteSlices(keysAndObjects[1], nil)
	if keyOk != nil || valOk != nil || len(keys) != len(vals) {
		txt := "RequestByteSlices got mismatched or invalid response from redis\n"
		txt = txt + fmt.Sprintf("RequestByteSlices keys: %s\n", keys)
		txt = txt + fmt.Sprintf("RequestByteSlices vals: %s\n", vals)
		txt = txt + fmt.Sprintf("RequestByteSlices len(keys): %v\n", len(keys))
		log.Println(txt)
		return nil, nil, errors.New(txt)
	}

	return keys, vals, nil
}

// RedisRequestObjects tries to create a go object for every item in a slice of
// subscrition keys.
//
// The caller must provide a function for converting typeKey+bytes to objects.
func RedisRequestObjects(conn redis.Conn, subKeys []string, constructor ContainerConstructor) ([]Object, error) {
	keys, vals, err := RedisRequestByteSlices(conn, subKeys)
	if err != nil {
		return nil, err
	}

	results := make([]Object, 0, len(keys))

	for i, key := range keys {

		// Pass the typeKey in to the ObjectLoader. Note that we are not passing
		// in the ID. It is the Loader's responsibility to reconstruct the ID
		// from the serialized data. This may cause bugs iff the ID in the
		// object is not consistent with the Object's key. If that happens, we
		// have larger bugs to worry about.
		index := strings.LastIndex(key, ":")
		// If there is no ':' character in the key, pass in the raw key.
		if index != -1 {
			key = key[:index]
		}

		container := constructor(key)
		if container == nil {
			return nil, errors.New("No container for type: " + key)
		}

		err = json.Unmarshal(vals[i], container)

		if err == nil {
			results = append(results, container)
		} else {
			//BUG(charles): error is handled twice
			log.Printf("Failed RequestObjects failed to create object: %s\n", err)
		}
	}
	return results, err
}

// redisKey creates a suitable Key for storing an object in redis
func redisKey(obj Object) string {
	return obj.TypeKey() + ":" + obj.TagGetID()
}

// id will be empty iff key ends with ':'
func redisTypeAndID(redisKey string) (string, string) {
	index := strings.LastIndex(redisKey, ":")
	if index == -1 {
		return redisKey, redisKey
	}
	end := index + 1
	return redisKey[:index], redisKey[end:]
}

// Note that if redisNewObject is passed an unresolved Object, The unresolved
// version will be saved. This should be fine as long as the object gets
// passed to redisModObj later.
func redisNewObject(obj Object, rConn redis.Conn) error {
	subKey := obj.GetSubKey()
	typeKey := obj.TypeKey()
	redisKey := redisKey(obj)

	msg := addMsg{
		State:   obj.State(),
		ID:      obj.TagGetID(),
		Type:    typeKey,
		SKey:    subKey,
		Version: obj.Version(),
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
	val, err := redis.String(setAndPublishScript.Do(rConn, redisKey, subKey, redisJSON, msgJSON))

	if val == "NO" {
		txt := "synk.redisNewObject failed to create object. Redis key '%s' already exists"
		return fmt.Errorf(txt, redisKey)
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
	id := m.TagGetID()
	redisKey := redisKey(m)
	simple := psk == nsk
	diff := m.Resolve()

	// Create the message to send to clients
	msg := modMsg{
		Diff:    diff,
		ID:      id,
		SKey:    psk,
		Version: m.Version(),
	}

	if !simple {
		msg.NSKey = nsk
	}

	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return errors.New("redisModObject failed to convert modMsg to JSON")
	}

	// This is the object we will save in redis
	objJSON, err := json.Marshal(m)
	if err != nil {
		return errors.New("redisModObject failed to convert object to JSON")
	}

	// Is the object in the same chunk as before?
	if simple {
		rConn.Send("MULTI")
		rConn.Send("SADD", nsk, redisKey) // Redundant, but safer
		rConn.Send("SET", redisKey, objJSON)
		rConn.Send("PUBLISH", psk, msgJSON)
		_, err = rConn.Do("EXEC")
		return err
	}

	// The object changed chunks. We will need to update two redis sets, and
	// publish in two places.
	addMsg := addMsg{
		State:   m.State(),
		ID:      id,
		SKey:    nsk,
		PSKey:   psk,
		Version: m.Version(),
		Type:    m.TypeKey(),
	}
	addJSON, err := json.Marshal(addMsg)
	if err != nil {
		return errors.New("redisModObject failed to convert full state to JSON")
	}

	// This line below is a hack that I am using to tell our sever to condionally
	// send this message to the client. If the client is subscribed to the chunk
	// that this object is moving from, then that client will receive the diff and
	// they do not need or want to receive the addObj message. Our websocket
	// server checks if JSON messages are prefixed with the from string, and only
	// sends the message to the client if it is needed.
	addJSON = []byte("from " + psk + string(addJSON))

	rConn.Send("MULTI")
	rConn.Send("SREM", psk, redisKey)
	rConn.Send("SADD", nsk, redisKey)
	rConn.Send("SET", redisKey, objJSON)
	rConn.Send("PUBLISH", psk, msgJSON)
	rConn.Send("PUBLISH", nsk, addJSON)
	_, err = rConn.Do("EXEC")

	return err
}

func redisDelObject(obj Object, rConn redis.Conn) error {

	// Note that we are using the Previous subscription key. If we are deleting
	// an object that was moving to another subscription, but the move was not yet
	// resolved, the clients will still think the character is in the old subKey.
	remMsg := remMsg{
		SKey: obj.GetPrevSubKey(),
		ID:   obj.TagGetID(),
		Type: obj.TypeKey(),
	}

	remJSON, err := json.Marshal(remMsg)
	if err != nil {
		txt := "synk.redisDelObj failed to convert msg to json: " + err.Error()
		return errors.New(txt)
	}

	redisKey := redisKey(obj)

	rConn.Send("MULTI")
	rConn.Send("SREM", remMsg.SKey, redisKey)
	rConn.Send("DEL", redisKey)
	rConn.Send("PUBLISH", remMsg.SKey, remJSON)
	_, err = rConn.Do("EXEC")

	return err
}

/******************************************************************************
The methods below are part of a functionality for parallelizing object mutation.

As of November 2017 these are unused, but I will certainly need this working at
some point in the future. Currently we have to wait for the DB after every
mutation.
*******************************************************************************/

// HandleMessage applies the supplied Message to a given redigo connection.
// It should mutate the database and publish any JSON messages required to
// update clients subscribed to the db.
func redisHandleMessage(msg interface{}, rConn redis.Conn) error {
	switch msg := msg.(type) {
	case modObj:
		return redisModObject(msg.Object, rConn)
	case newObj:
		return redisNewObject(msg.Object, rConn)
	case delObj:
		return redisDelObject(msg.Object, rConn)
	default:
		txt := fmt.Sprintf("Unknown Message Type: %T", msg)
		return errors.New(txt)
	}
}

type newObj struct {
	Object
}

type modObj struct {
	Object
}

type delObj struct {
	Object
}
