package synk

import (
	"log"
	"strings"

	"github.com/garyburd/redigo/redis"
)

// ObjectLoader is a type that can load objects from redis.
type ObjectLoader interface {
	LoadObject(typeKey string, bytes []byte)
}

var getFlatObjectsScript = `
local objs = {}

for _, key in ipairs(KEYS) do
	local ids = redis.call("SMEMBERS", key)
	for _, id in ipairs(ids) do
		objs[#objs+1] = {id, redis.call("GET", id)}
	end
end

return objs
`

// GetFlatObjects is a redis script that retrieves all objects in redis from a
// collection of object keys. It needs to be called with the following argument
// signature:
// GetFlatObjects.Do(c redis.Conn, kCount int, k1, k2...)
var GetFlatObjects = redis.NewScript(-1, getFlatObjectsScript)

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

	// Values will be []interface{}
	luaObjects, err := redis.Values(GetFlatObjects.Do(conn, args...))
	if err != nil {
		log.Println("Loader.RequestObjects - get values fail: " + err.Error())
		return err
	}

	// luaObjects is an []interface{}. We want to step through each interface{}
	// and extract its key and bytes
	for _, intrfce := range luaObjects {
		// Convert each interface{} to its actual value - []interface{} - Note that
		// the object returned in the lua script is a table of tables. Each inner
		// table is a lua array with two elements - the key, and the serialized
		// payload. Note that tables returned by lua must be indexed with
		// consecutive integers (as per the redis lua EVAL specification). A value
		// returned by lua may not be indexed with strings.
		slice, err := redis.Values(intrfce, nil)
		if err != nil || len(slice) < 2 {
			continue
		}
		rKey, err := redis.String(slice[0], nil) // redis key
		if err != nil {
			continue
		}
		rVal, err := redis.Bytes(slice[1], nil)
		if err != nil {
			continue
		}

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
