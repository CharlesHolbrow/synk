package stest

import (
	"fmt"

	"github.com/CharlesHolbrow/synk"
)

// Human is a test create for Pagen
//@PA:c:h
type Human struct {
	synk.Tag `json:"-" bson:",inline"`
	// ID
	ID string `json:"id"`
	// X, Y, and some others are mandatory for Character
	X     int    `json:"x"`
	Y     int    `json:"y"`
	CX    int    `json:"cx"`
	CY    int    `json:"cy"`
	CI    int    `json:"ci"`
	MapID string `json:"mapID"`
	diff  humanDiff
}

// These methods are required to Satisfy the Object and Character interfaces

// GetSubKey gets the most recent subscription key.
func (c *Human) GetSubKey() string {
	return fmt.Sprintf("%v:%v|%v", "000a", 8, -7)
}

// GetPrevSubKey gets the previous subscription key.
func (c *Human) GetPrevSubKey() string {
	return fmt.Sprintf("%v:%v|%v", "000a", 8, -7)
	// return ObjsKey(c.GetPrevMapID(), c.GetPrevCX(), c.GetPrevCY())
}
