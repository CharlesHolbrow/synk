package synk

// If you are writing a sync server, you will probably want to write custom
// handlers for messages received from clients. You will need to implement
// the CustomClient AND a constructor for that Client type.
//
// When a client makes a websocket connection, you constructor will be called,
// and passed a Client object. The constructor function is expected to return
// an instance of your CustomClient that provides the OnConnect and OnMessage
// callbacks.

// CustomClient provides an interface for creatnig custom behavior when
// a client creates a Connection to the synk server. It also provides an
// interface for writing custom message handlers.
type CustomClient interface {
	OnConnect(client *Client)
	OnMessage(client *Client, method string, data []byte)
	OnSubscribe(client *Client, subKeys []string, objs []Object)
}

// A CustomClientConstructor must be supplied when implementing custom handlers.
// The supplied function will create the custom message handler when clients connect
type CustomClientConstructor func(client *Client) CustomClient
