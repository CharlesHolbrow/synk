package synk

// ObjectConstructor is any function that can create an object from a type key
// and a byte slice. If error is nil, the Object should have been created
// successfully.
//
// You will need to write an ObjectConstructor for your synk server, so that the
// server knows how to deserialize your objects based on their type keys
type ObjectConstructor func(typeKey string, bytes []byte) (Object, error)
