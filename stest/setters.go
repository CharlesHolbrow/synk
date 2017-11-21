package stest

import "github.com/CharlesHolbrow/synk"

// humanDiff diff type for synk.Object
type humanDiff struct {
  CY *int `json:"cy,omitempty"`
  CI *int `json:"ci,omitempty"`
  MapID *string `json:"mapID,omitempty"`
  X *int `json:"x,omitempty"`
  Y *int `json:"y,omitempty"`
  CX *int `json:"cx,omitempty"`
}

// State returns a fully populated diff of the unresolved state
func (o *Human) State() interface{} {
	d := humanDiff{
    CY: &o.CY,
    CI: &o.CI,
    MapID: &o.MapID,
    X: &o.X,
    Y: &o.Y,
    CX: &o.CX,
  }
  return d
}

// Resolve applies the current diff, then returns it
func (o *Human) Resolve() interface{} {
  if o.diff.CY != nil {o.CY = *o.diff.CY}
  if o.diff.CI != nil {o.CI = *o.diff.CI}
  if o.diff.MapID != nil {o.MapID = *o.diff.MapID}
  if o.diff.X != nil {o.X = *o.diff.X}
  if o.diff.Y != nil {o.Y = *o.diff.Y}
  if o.diff.CX != nil {o.CX = *o.diff.CX}
  o.V++
  diff := o.diff
  o.diff = humanDiff{}
  return diff
}

// Changed checks if struct has been changed since the last .Resolve()
func (o *Human) Changed() bool {
  return o.diff.CY != nil ||
		o.diff.CI != nil ||
		o.diff.MapID != nil ||
		o.diff.X != nil ||
		o.diff.Y != nil ||
		o.diff.CX != nil
}

// TypeKey getter for main and diff structs
func (o *Human) TypeKey() string { return "c:h" }
// Key getter for main object
func (o *Human) Key() string { return "c:h:"+o.ID }
// TypeKey getter for main and diff structs
func (o humanDiff) TypeKey() string { return "c:h" }
// Diff getter
func (o *Human) Diff() interface{} { return o.diff }
// Copy duplicates this object and returns an interface to it.
// The object's diff will be copied too, with the exception of the diffMap for
// array members. A diffMap is created automatically when we use array Element
// setters (ex SetDataElement). Copy() will create shallow copies of unresolved
// diffMaps. Usually we Resolve() after Copy() which means that our shallow copy
// will be safe to send over a channel.
func (o *Human) Copy() synk.Object {
	n := *o
	return &n
}
// Init (ialize) all diff fields to the current values. The next call to
// Resolve() will return a diff with all the fields initialized.
func (o *Human) Init() {
	o.diff = o.State().(humanDiff)
}
// SetCY on diff
func (o *Human) SetCY(v int) {
  if v != o.CY {
    o.diff.CY = &v
  } else {
    o.diff.CY = nil
  }
}
// GetPrevCY Gets the previous value. Ignores diff.
func (o *Human) GetPrevCY() int { return o.CY }
// GetCY from diff. Fall back to current value if no diff
func (o *Human) GetCY() int {
	if o.diff.CY != nil {
		return *o.diff.CY
	}
	return o.CY
}
// GetCY. Diff method
func (o humanDiff) GetCY() *int { return o.CY }
// SetCI on diff
func (o *Human) SetCI(v int) {
  if v != o.CI {
    o.diff.CI = &v
  } else {
    o.diff.CI = nil
  }
}
// GetPrevCI Gets the previous value. Ignores diff.
func (o *Human) GetPrevCI() int { return o.CI }
// GetCI from diff. Fall back to current value if no diff
func (o *Human) GetCI() int {
	if o.diff.CI != nil {
		return *o.diff.CI
	}
	return o.CI
}
// GetCI. Diff method
func (o humanDiff) GetCI() *int { return o.CI }
// SetMapID on diff
func (o *Human) SetMapID(v string) {
  if v != o.MapID {
    o.diff.MapID = &v
  } else {
    o.diff.MapID = nil
  }
}
// GetPrevMapID Gets the previous value. Ignores diff.
func (o *Human) GetPrevMapID() string { return o.MapID }
// GetMapID from diff. Fall back to current value if no diff
func (o *Human) GetMapID() string {
	if o.diff.MapID != nil {
		return *o.diff.MapID
	}
	return o.MapID
}
// GetMapID. Diff method
func (o humanDiff) GetMapID() *string { return o.MapID }
// GetID returns the ID
func (o *Human) GetID() string { return o.ID }
// SetID -- but only if it has not been set. This helps us avoid accidentally
// setting it twice. Return the item's ID either way.
func (o *Human) SetID(id string) string {
	if o.ID == "" {
		o.ID = id
	}
	return o.ID
}
// SetX on diff
func (o *Human) SetX(v int) {
  if v != o.X {
    o.diff.X = &v
  } else {
    o.diff.X = nil
  }
}
// GetPrevX Gets the previous value. Ignores diff.
func (o *Human) GetPrevX() int { return o.X }
// GetX from diff. Fall back to current value if no diff
func (o *Human) GetX() int {
	if o.diff.X != nil {
		return *o.diff.X
	}
	return o.X
}
// GetX. Diff method
func (o humanDiff) GetX() *int { return o.X }
// SetY on diff
func (o *Human) SetY(v int) {
  if v != o.Y {
    o.diff.Y = &v
  } else {
    o.diff.Y = nil
  }
}
// GetPrevY Gets the previous value. Ignores diff.
func (o *Human) GetPrevY() int { return o.Y }
// GetY from diff. Fall back to current value if no diff
func (o *Human) GetY() int {
	if o.diff.Y != nil {
		return *o.diff.Y
	}
	return o.Y
}
// GetY. Diff method
func (o humanDiff) GetY() *int { return o.Y }
// SetCX on diff
func (o *Human) SetCX(v int) {
  if v != o.CX {
    o.diff.CX = &v
  } else {
    o.diff.CX = nil
  }
}
// GetPrevCX Gets the previous value. Ignores diff.
func (o *Human) GetPrevCX() int { return o.CX }
// GetCX from diff. Fall back to current value if no diff
func (o *Human) GetCX() int {
	if o.diff.CX != nil {
		return *o.diff.CX
	}
	return o.CX
}
// GetCX. Diff method
func (o humanDiff) GetCX() *int { return o.CX }
// Version Gets V. Ignores diff.
func (o *Human) Version() uint { return o.V }
// orcDiff diff type for synk.Object
type orcDiff struct {
  SubKey *string `json:"subKey,omitempty"`
  Name *string `json:"name,omitempty"`
}

// State returns a fully populated diff of the unresolved state
func (o *Orc) State() interface{} {
	d := orcDiff{
    SubKey: &o.SubKey,
    Name: &o.Name,
  }
  return d
}

// Resolve applies the current diff, then returns it
func (o *Orc) Resolve() interface{} {
  if o.diff.SubKey != nil {o.SubKey = *o.diff.SubKey}
  if o.diff.Name != nil {o.Name = *o.diff.Name}
  o.V++
  diff := o.diff
  o.diff = orcDiff{}
  return diff
}

// Changed checks if struct has been changed since the last .Resolve()
func (o *Orc) Changed() bool {
  return o.diff.SubKey != nil ||
		o.diff.Name != nil
}

// TypeKey getter for main and diff structs
func (o *Orc) TypeKey() string { return "c:o" }
// Key getter for main object
func (o *Orc) Key() string { return "c:o:"+o.ID }
// TypeKey getter for main and diff structs
func (o orcDiff) TypeKey() string { return "c:o" }
// Diff getter
func (o *Orc) Diff() interface{} { return o.diff }
// Copy duplicates this object and returns an interface to it.
// The object's diff will be copied too, with the exception of the diffMap for
// array members. A diffMap is created automatically when we use array Element
// setters (ex SetDataElement). Copy() will create shallow copies of unresolved
// diffMaps. Usually we Resolve() after Copy() which means that our shallow copy
// will be safe to send over a channel.
func (o *Orc) Copy() synk.Object {
	n := *o
	return &n
}
// Init (ialize) all diff fields to the current values. The next call to
// Resolve() will return a diff with all the fields initialized.
func (o *Orc) Init() {
	o.diff = o.State().(orcDiff)
}
// SetSubKey on diff
func (o *Orc) SetSubKey(v string) {
  if v != o.SubKey {
    o.diff.SubKey = &v
  } else {
    o.diff.SubKey = nil
  }
}
// GetPrevSubKey Gets the previous value. Ignores diff.
func (o *Orc) GetPrevSubKey() string { return o.SubKey }
// GetSubKey from diff. Fall back to current value if no diff
func (o *Orc) GetSubKey() string {
	if o.diff.SubKey != nil {
		return *o.diff.SubKey
	}
	return o.SubKey
}
// GetSubKey. Diff method
func (o orcDiff) GetSubKey() *string { return o.SubKey }
// SetName on diff
func (o *Orc) SetName(v string) {
  if v != o.Name {
    o.diff.Name = &v
  } else {
    o.diff.Name = nil
  }
}
// GetPrevName Gets the previous value. Ignores diff.
func (o *Orc) GetPrevName() string { return o.Name }
// GetName from diff. Fall back to current value if no diff
func (o *Orc) GetName() string {
	if o.diff.Name != nil {
		return *o.diff.Name
	}
	return o.Name
}
// GetName. Diff method
func (o orcDiff) GetName() *string { return o.Name }
// Version Gets V. Ignores diff.
func (o *Orc) Version() uint { return o.V }
// GetID returns the ID
func (o *Orc) GetID() string { return o.ID }
// SetID -- but only if it has not been set. This helps us avoid accidentally
// setting it twice. Return the item's ID either way.
func (o *Orc) SetID(id string) string {
	if o.ID == "" {
		o.ID = id
	}
	return o.ID
}
