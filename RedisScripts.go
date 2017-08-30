package synk

import "github.com/garyburd/redigo/redis"

// Get two parallel arrays. One with Keys, the other with Byte Slices
var getKeysObjectsScript = `
local ids = redis.call("SUNION", unpack(KEYS))
if #ids == 0 then
	return {{}, {}}
end
local objs = redis.call("MGET", unpack(ids))
return {ids, objs}
`

// GetKeysObjects is a redis script that retrieves all objects in redis from a
// collection of object keys. It needs to be called with the following argument
// signature:
// GetFlatObjects.Do(c redis.Conn, kCount int, k1, k2...)
var GetKeysObjects = redis.NewScript(-1, getKeysObjectsScript)

var setAndPublishText = `
local res = redis.call("SET", KEYS[1], ARGV[1], "NX")
if res then
    redis.call("SADD", KEYS[2], KEYS[1])
	redis.call("PUBLISH", KEYS[2], ARGV[2])
	return "OK"
end
return "NO"
`

// SetAndPublish sets a synk Object, and publishes a message to clients. If an
// object with the specified key already exists, it is not set, and the update
// message is not published. The script is quite 'dumb', and you have to be
// careful to call it with the correct four arguments.
//
// Two keys
// 1. object key (including it's type key and ID)
// 2. subscription key - note that this is the same as the name of the channel we will publish on
// Two args
// 3. object JSON
// 4. publish JSON
//
// Returns "OK" if SET completed. "NO" If no operation occurred
var setAndPublishScript = redis.NewScript(2, setAndPublishText)
