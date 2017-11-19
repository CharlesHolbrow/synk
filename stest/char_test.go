package stest

import (
	"testing"

	"github.com/CharlesHolbrow/synk"
)

func TestHuman_GetSubKey(t *testing.T) {
	ms := synk.NewMongoSynk()

	h := &Human{}
	h.SetMapID("000a")
	h.SetX(-1)
	h.SetCX(-1)
	h.SetCI(9)

	ms.Create(h)
}
