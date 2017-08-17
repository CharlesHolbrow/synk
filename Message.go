package synk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MethodMessage contains only a .Method string. It is used internally when
// converting a byte slice to an explicit Message struct.
type MethodMessage struct {
	Method string `json:"method"`
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

	switch mm.Method {
	case "updateSubscription":
		var msg UpdateSubscriptionMessage
		err = json.Unmarshal(raw, &msg)
		return msg, err
	default:
		txt := fmt.Sprintf("Unknown Message type: %s\n", mm.Method)
		return mm, errors.New(txt)
	}
}

// UpdateSubscriptionMessage is a request (probably from a client) to change
// the client's map subscription by listing chunks to add and/or remove
type UpdateSubscriptionMessage struct {
	Method string   `json:"method"`
	MapID  string   `json:"mapID"`
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}
