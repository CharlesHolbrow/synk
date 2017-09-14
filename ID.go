package synk

import (
	"math/rand"
	"time"
)

// Untyped string constant. It's a string, but it's not a Go
// value of type string. Ids are used in the pixel aether to
// identify unique entities such as player characters.
// For now, we only choose from characters that are easy to
// differentiate.
const idChars = "23456789ABCDEFGHJKLMNPQRSTWXYZabcdefghijkmnopqrstuvwxyz"

// These 62 characters include all numbers and letters
const idChars2 = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// How long are our IDs
const idLen = 16

// Get properly randomized values
func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// The ID Type used be clients and Objects
type ID [idLen]byte

// NewID Creates n id randomly from the distinct characters
func NewID() ID {
	var id ID
	for i := range id {
		id[i] = idChars2[rand.Intn(len(idChars2))]
	}
	return id
}

func (id ID) String() string {
	return string(id[:])
}
