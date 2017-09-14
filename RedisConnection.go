package synk

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
)

// RedisConnection wraps a redigo connection Pool
type RedisConnection struct {
	addr string
	Pool redis.Pool
}

// NewConnection builds a new AetherRedisConnection
func NewConnection(redisAddr string) *RedisConnection {
	arc := &RedisConnection{
		addr: redisAddr,
		Pool: redis.Pool{
			MaxIdle:     100,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				conn, err := redis.Dial("tcp", redisAddr, redis.DialConnectTimeout(8*time.Second))
				if err != nil {
					log.Println("Failed to connect to redis:", err.Error())
				}
				return conn, err
			},
		},
	}
	return arc
}

// GetID returns an ID string from redis. Each call incrememnts a counter,
// meaning that this will never return the same value for any prefix twice. It
// is intended to be used with typeKeys to generate character IDs.
func (arc *RedisConnection) GetID(counterKey string) (string, error) {
	conn := arc.Pool.Get()
	defer conn.Close()
	return getID(counterKey, conn)
}

// NewObjectID gets an ID from redis, and sets that on the new object.
// We may eventully want to replace this with randomly generated IDs
func (arc *RedisConnection) NewObjectID(o Object) error {
	conn := arc.Pool.Get()
	defer conn.Close()
	if o.GetID() == "" {
		if id, err := getID(o.TypeKey(), conn); err == nil {
			o.SetID(id)
		} else {
			return fmt.Errorf("arc.NewObjectID failed to get ID: %s", err)
		}
	}
	return nil
}

// GetID is a helper for retrieving unique ID for objects. The Typical use
func getID(counterKey string, conn redis.Conn) (string, error) {
	r, err := redis.Int(conn.Do("INCR", "count:"+counterKey))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(r), 36), nil
}
