package synk

import (
	"encoding/json"
	"fmt"
	"testing"

	"reflect"

	"github.com/rafaeljusto/redigomock"
)

type Dog struct {
	ID   string
	Age  int
	Name string
}

type TestObjLoader struct {
	t *testing.T
}

var globalBytes, _ = json.Marshal(Dog{Age: 18, Name: "Buster", ID: "ab"})

func (tol TestObjLoader) LoadObject(key string, bytes []byte) {
	fmt.Printf("typeKey: %s bytes: %s\n", key, bytes)
	expectedKey := "c:dog"
	if key != expectedKey {
		tol.t.Errorf("Got '%s' for key. Expected '%s'", key, expectedKey)
	}

	// Bad test  -- because we are comparing the exact same object
	if !reflect.DeepEqual(bytes, globalBytes) {
		tol.t.Errorf("Got:  %s\nWant: %s\n", bytes, globalBytes)
	}
}

func TestRequestObjects(t *testing.T) {

	conn := redigomock.NewConn()
	conn.Script([]byte(getKeysObjectsScript), 1, "objs:000a:0|0").ExpectSlice([]interface{}{"c:dog:ab", globalBytes})

	tol := TestObjLoader{t}

	LoadObjects(tol, conn, []string{"objs:000a:0|0"})
}
