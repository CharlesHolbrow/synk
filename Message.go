package synk

import (
	"encoding/json"
	"errors"
)

// MethodMessage contains only a .Method string. It is used internally when
// converting a byte slice to an explicit Message struct.
type MethodMessage struct {
	Method string `json:"method"`
}

// CustomMessage passes raw bytes to client code so that that code can
// deserialize it into the appropriate type.
type CustomMessage struct {
	Method string `json:"method"`
	Data   []byte `json:"-"`
}

// UpdateSubscriptionMessage is a request (probably from a client) to change
// the client's map subscription by listing chunks to add and/or remove
type UpdateSubscriptionMessage struct {
	Method string   `json:"method"`
	MapID  string   `json:"mapID"`
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// MessageFromBytes creates a Message struct from raw json stored in a
// []byte slice.
func MessageFromBytes(raw []byte) (interface{}, error) {
	var mm MethodMessage
	var err error

	err = json.Unmarshal(raw, &mm)
	if err != nil {
		return mm, err
	}

	if mm.Method == "" {
		return mm, errors.New("MessageFromBytes received a message with no 'method' json")
	}

	switch mm.Method {
	case "updateSubscription":
		var msg UpdateSubscriptionMessage
		err = json.Unmarshal(raw, &msg)
		return msg, err
	default:
		return CustomMessage{Method: mm.Method, Data: raw}, nil
	}
}
