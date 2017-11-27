package synk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/garyburd/redigo/redis"
)

// ObjectLoader is a type that can load objects from redis.
type ObjectLoader interface {
	LoadObject(typeKey string, bytes []byte)
}

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

// RequestObjects tries to create a go object for every item in a slice of
// subscrition keys.
//
// The caller must provide a function for converting typeKey+bytes to objects.
func RequestObjects(conn redis.Conn, subKeys []string, constructor ContainerConstructor) ([]Object, error) {
	keys, vals, err := RequestByteSlices(conn, subKeys)
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
