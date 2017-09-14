package synk

// Object is the interface for anything that will be saved in redis with diffs
// that will be pushed to clients. The methods are a sub-set of the Character
// interface methods.
type Object interface {
	State() interface{}
	Resolve() interface{}
	Init()
	Copy() Object
	Key() string
	TypeKey() string
	GetSubKey() string
	GetPrevSubKey() string
	GetID() string
	SetID(string) string
}

// Messages that are sent TO redis mutators methods

// NewObj message is emitted by Fragment when an object is created
type NewObj struct {
	Object
}

// DelObj message is emitted by Fragment when an object is removed from the map altogether
type DelObj struct {
	Object
}

// ModObj message is emitted by a Fragment when an object is changed
type ModObj struct {
	Object
}

// Messages that are sent FROM redis mutators to the client. These should
// - Have a the 'method' field that JSONifies to a method name
// - Have an sKey string field that indicates a subscription field
// - potentially have a new SKey 'nsKey'

// MsgModObj represents relative changes made to an object. It is sent to the
// client when an object is created for the first time.
//
// This is the message that the client receives when the object is moving from
// one chunk to another.
type MsgModObj struct {
	Method msgModObj   `json:"method"`
	Diff   interface{} `json:"diff"`
	Key    string      `json:"key"`
	// SKey is the subscription key where the object is prior to movement.
	SKey string `json:"sKey"`
	// NSKey is the subscription key that the object is moving to. Only present if
	// the object is changing chunks.
	NSKey string `json:"nsKey,omitempty"`
}

type msgModObj struct{}

func (m msgModObj) MarshalJSON() ([]byte, error) {
	return []byte("\"modObj\""), nil
}

// MsgAddObj is sent to the client to tell that client to create a new object.
// This would happen when an object moves into the client's subscription, OR
// when an object is newly created.
type MsgAddObj struct {
	Method msgAddObj   `json:"method"`
	State  interface{} `json:"state"`
	Key    string      `json:"key"`
	// SKey is where we add this object to
	SKey string `json:"sKey"`
	// If the object is moving from another chunk, include psKey
	PSKey string `json:"psKey,omitempty"`
}

type msgAddObj struct{}

func (m msgAddObj) MarshalJSON() ([]byte, error) {
	return []byte("\"addObj\""), nil
}

// MsgRemObj tells clients to remove and teardown an object. This is NOT the
// message that a client gets when an object is moving from one chunk to
// another, even if the client is not subscribed to the destination.
type MsgRemObj struct {
	Method msgRemObj `json:"method"`
	SKey   string    `json:"sKey"`
	Key    string    `json:"key"`
}

type msgRemObj struct{}

func (m msgRemObj) MarshalJSON() ([]byte, error) {
	return []byte("\"remObj\""), nil
}
