package synk

import "fmt"

// Tag includes fields that are required for a synk Objects. It is
// intended to be included in a type object as an anonymous member.
//
// These fields are used by the synk pipeline to make sure that data sent to
// mongodb has an ID and Subscription. The data in Tag members may be
// unreliable. For example, the client object is responsible for including the
// GetSubKey() and GetPrevSubKey() methods.
//
// IMPORTANT: pagen does not create set/getters for keys that begin with "Tag"
type Tag struct {
	// TagID is the full identification of the object in the the format:
	// typeKey[:typeKey]:Key
	TagID string `json:"_id" bson:"_id"`

	// TagKey is the unique/random portion of an object's ID. This will be set
	// by synk when the object is created. You can optionally set it before
	// creating the object with synk.
	TagKey string `json:"-" bson:"-"`

	// TagSub is the object's subscription key. This is only used by the Synk
	// Library. The object is still expected to have GetSubKey and GetPrevSubKey
	// methods.
	TagSub string `json:"sub" bson:"sub"`

	// V is the Object's version. Clients will use this to verify the correct
	// update is being applied to the correct object
	V uint `json:"v" bson:"v"`
}

// TagInit - Accept a typeKey, and return full ID. Only mutate unset fields.
func (t *Tag) TagInit(typeKey string) string {
	// Set the TypeKey (but only if it's not set)
	if t.TagKey == "" {
		t.TagKey = NewID().String()
	}

	// Set the full ID (but only if it not )
	if t.TagID == "" {
		t.TagID = typeKey + ":" + t.TagKey
	}

	// Return the full ID
	return t.TagID
}

// TagSetSub sets the mongo 'sub' field. This is how we tell mongodb which
// subscription field the object is in. Note that the Object is still expected
// to have GetPrevSubKey and GetSubKey methods.
func (t *Tag) TagSetSub(sKey string) {
	t.TagSub = sKey
}

// TagSetID updates the mongo _id field. This will be called automatically
// by the Tag Library when the object is created.
func (t *Tag) TagSetID(id string) error {
	if t.TagID == "" {
		t.TagID = id
		return nil
	}
	if t.TagID == id {
		return nil
	}

	return fmt.Errorf("Tried to change synk Object's ID from %s to %s", t.TagID, id)
}

// Version returns the object's current .V version
func (t *Tag) Version() uint {
	return t.V
}
