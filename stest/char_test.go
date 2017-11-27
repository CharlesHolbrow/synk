package stest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/CharlesHolbrow/synk"
	"github.com/garyburd/redigo/redis"
	mgo "gopkg.in/mgo.v2"
)

func creator(typeKey string) synk.Object {
	switch typeKey {
	case "c:h":
		return &Human{}
	case "c:o":
		return &Orc{}
	}
	return nil
}

func epanic(message string, err error) {
	if err != nil {
		panic(message + err.Error())
	}
}

func TestHuman_GetSubKey(t *testing.T) {
	session, err := mgo.Dial("localhost")
	epanic("failed to dial mongodb:", err)
	rConn, err := redis.Dial("tcp", ":6379")
	epanic("failed to dial redis:", err)

	ms := synk.MongoSynk{
		Coll:    session.DB("synk").C("objects"),
		Creator: creator,
		RConn:   rConn,
	}

	h := &Human{}
	// h.TagKey = "TestChar3"
	h.SetMapID("000a")
	h.SetX(-1)
	h.SetCX(-1)
	h.SetCI(9)

	ms.Create(h)

	h.SetY(h.GetY() + 1)
	ms.Modify(h)

	var o2 synk.Object

	objects, err := ms.Load([]string{h.GetSubKey()})
	fmt.Println("Length of results:", len(objects))

	found := false

	for _, obj := range objects {
		if obj.TagGetID() == h.TagGetID() {
			fmt.Println("Found Object:", obj)
			fmt.Println("Same as h", h)
			fmt.Printf("%v\n", h.Tag)
			if h2, ok := obj.(*Human); ok {
				fmt.Printf("%v\n", h2.Tag)
			}
			o2 = h
			fmt.Println("Are the two objects deeply equal?", reflect.DeepEqual(o2, obj))
			found = true
			break
		}
	}

	if !found {
		t.Error("coulnd not find object in results with id:", h.TagGetID())
	}

}
