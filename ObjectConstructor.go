package synk

// ObjectConstructor is any function that can create an object from a type key and a
// byte slice. If error is nil, the Object should have been created successfully.
type ObjectConstructor func(typeKey string, bytes []byte) (Object, error)
