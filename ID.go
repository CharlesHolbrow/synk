package synk

import "math/rand"

// Untyped string constant. It's a string, but it's not a Go
// value of type string. Ids are used in the pixel aether to
// identify unique entities such as player characters.
// For now, we only choose from characters that are easy to
// differentiate.
const idChars = "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz"

const idLen = 16

// ID Identification data type used by aether
type ID [idLen]byte

// NewID Creates n id randomly from the distinct characters
func NewID() ID {
	var id ID
	for i := range id {
		id[i] = idChars[rand.Intn(len(idChars))]
	}
	return id
}

func (id ID) String() string {
	return string(id[:])
}
