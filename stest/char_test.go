package stest

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/CharlesHolbrow/synk"
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
	session := synk.DialMongo()
	pool := synk.DialRedisPool()

	ms := synk.MongoSynk{
		Coll:      session.DB(synk.MongoDBName).C("objects"),
		Creator:   creator,
		RedisPool: pool,
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
	epanic("Error loading synk Objects:", err)

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
