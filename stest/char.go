package stest

import (
	"fmt"

	"github.com/CharlesHolbrow/synk"
)

// Human is a test create for Pagen
//@PA:c:h
type Human struct {
	synk.Tag `json:"-" bson:",inline"`
	// X, Y, and some others are mandatory for Character
	X     int    `json:"x"`
	Y     int    `json:"y"`
	CX    int    `json:"cx"`
	CY    int    `json:"cy"`
	CI    int    `json:"ci"`
	MapID string `json:"mapID"`
	diff  humanDiff
}

// These methods are required to satisfy the Object and Character interfaces

// TypeKey identifies the object type
func (o *Human) TypeKey() string {
	return "c:h"
}

// GetSubKey gets the most recent subscription key.
func (o *Human) GetSubKey() string {
	return fmt.Sprintf("%v:%v|%v", o.GetMapID(), o.GetCX(), o.GetCY())
}

// GetPrevSubKey gets the previous subscription key.
func (o *Human) GetPrevSubKey() string {
	return fmt.Sprintf("%v:%v|%v", o.GetPrevMapID(), o.GetPrevCX(), o.GetPrevCY())
}

func (o *Human) String() string {
	return fmt.Sprintf("Human (%s) at (%d, %d) on %s", o.TagGetID(), o.GetX(), o.GetY(), o.GetMapID())
}

// Orc is another test creature on the map
//@PA:c:o
type Orc struct {
	synk.Tag `bson:",inline"`
	ID       string `json:"id"`
	SubKey   string
	Name     string
	diff     orcDiff
}

func (c *Orc) String() string {
	return fmt.Sprintf("Orc (%s) named %s", c.TagGetID(), c.Name)
}

// TypeKey identifies the object type
func (c *Orc) TypeKey() string {
	return "c:o"
}
