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

// RequestObjects calls the LoadObject(typeKey, bytes) method of the supplied
// ObjectLoader for each object in objKeys
func RequestObjects(l ObjectLoader, conn redis.Conn, objKeys []string) error {
	// The script requires the first argument to be the number of keys. We have to
	// make it one element longer than the points array.
	size := len(objKeys)
	args := make([]interface{}, size+1)
	args[0] = size
	for i, k := range objKeys {
		args[i+1] = k
	}

	// redis.Values will return []interface{}
	keysAndObjects, err := redis.Values(GetKeysObjects.Do(conn, args...))
	if err != nil {
		log.Println("RequestObjects - get values fail: " + err.Error())
		return err
	}
	if len(keysAndObjects) == 0 {
		return nil
	}
	keys, keyOk := redis.Strings(keysAndObjects[0], nil)
	vals, valOk := redis.ByteSlices(keysAndObjects[1], nil)
	if keyOk != nil || valOk != nil || len(keys) != len(vals) {
		txt := "RequestObjects got mismatched or invalid response from redis"
		log.Println(txt)
		return errors.New(txt)
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
			// Note that is there is no ':' character in the key, we just pass
			// they raw key.
			l.LoadObject(rKey, rVal)
		} else {
			// take all but that last part
			l.LoadObject(rKey[:index], rVal)
		}
	}
	return nil
}
