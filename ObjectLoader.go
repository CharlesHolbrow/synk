package synk

import (
	"errors"
	"log"
	"strings"

	"github.com/garyburd/redigo/redis"
)

// ObjectLoader is a type that can load objects from redis.
type ObjectLoader interface {
	LoadObject(typeKey string, bytes []byte)
}

// Get two parallel arrays. One with Keys, the other with Byte Slices
var getKeysObjectsScript = `
local ids = redis.call("SUNION", unpack(KEYS))
if #ids == 0 then
	return {}
end
local objs = redis.call("MGET", unpack(ids))
return {ids, objs}
`

// GetKeysObjects is a redis script that retrieves all objects in redis from a
// collection of object keys. It needs to be called with the following argument
// signature:
// GetFlatObjects.Do(c redis.Conn, kCount int, k1, k2...)
var GetKeysObjects = redis.NewScript(-1, getKeysObjectsScript)

// RequestByteSlices from redis. Given a slice of subscription keys, get byte
// slices for all keys. Results are returned as two parallel slices. If there
// are no results, the slices will be of length zero.
//
// The two slices are gauranteed to be of equal length.
func RequestByteSlices(conn redis.Conn, subKeys []string) ([]string, [][]byte, error) {
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
		log.Println("RequestObjects - get values fail: " + err.Error())
		return nil, nil, err
	}

	if len(keysAndObjects) == 0 {
		return make([]string, 0), make([][]byte, 0), nil
	}

	keys, keyOk := redis.Strings(keysAndObjects[0], nil)
	vals, valOk := redis.ByteSlices(keysAndObjects[1], nil)
	if keyOk != nil || valOk != nil || len(keys) != len(vals) {
		txt := "RequestObjects got mismatched or invalid response from redis"
		log.Println(txt)
		return nil, nil, errors.New(txt)
	}

	return keys, vals, nil
}

// RequestObjects tries to create an go object for every item in a slice of
// subscrition keys.
//
// The caller must provide a function for converting typeKey+bytes to objects.
func RequestObjects(conn redis.Conn, subKeys []string, buildObj ObjectConstructor) ([]Object, error) {
	keys, vals, err := RequestByteSlices(conn, subKeys)
	if err != nil {
		return nil, err
	}

	results := make([]Object, len(keys))
	j := 0
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

		obj, err := buildObj(key, vals[i])
		if err == nil {
			results[j] = obj
			j++
		} else {
			log.Printf("Failed RequestObjects failed to create object: %s\n", err)
		}
	}
	return results[:j], err
}

// LoadObjects calls the LoadObject(typeKey, bytes) method of the supplied
// ObjectLoader for each object in objKeys
func LoadObjects(l ObjectLoader, conn redis.Conn, objKeys []string) error {
	keys, vals, err := RequestByteSlices(conn, objKeys)
	if err != nil {
		return err
	}

	for i, rKey := range keys {
		rVal := vals[i]

		// Pass the typeKey in to the ObjectLoader. Note that we are not passing
		// in the ID. It is the Loader's responsibility to reconstruct the ID
		// from the serialized data. This may cause bugs iff the ID in the
		// object is not consistent with the Object's key. If that happens, we
		// have larger bugs to worry about.
		index := strings.LastIndex(rKey, ":")
		if index == -1 {
			// Note that if there is no ':' character in the key, we just pass
			// they raw key.
			l.LoadObject(rKey, rVal)
		} else {
			// take all but that last part
			l.LoadObject(rKey[:index], rVal)
		}
	}
	return nil
}
