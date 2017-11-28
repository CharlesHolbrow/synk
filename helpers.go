package synk

import (
	"time"

	"github.com/garyburd/redigo/redis"
	mgo "gopkg.in/mgo.v2"
)

func DialRedis(redisAddr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", redisAddr, redis.DialConnectTimeout(8*time.Second))
			if err != nil {
				panic("Failed to connect to redis: " + err.Error())
			}
			return conn, err
		},
	}
}

// DialMongo creates the first MongoSession. Further sessions should be created
// with session.Copy()
func DialMongo() *mgo.Session {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic("Error Dialing mongodb: " + err.Error())
	}
	return session
}
