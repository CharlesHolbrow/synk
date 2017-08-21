package synk

import (
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

func getID(counterKey string, conn redis.Conn) (string, error) {
	r, err := redis.Int(conn.Do("INCR", "count:"+counterKey))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(int64(r), 36), nil
}
