package main

import (
	"testing"

	"github.com/knakk/specs"
)

func TestDummy(t *testing.T) {
	s := specs.New(t)
	s.Expect(1, 1, "1 is not 1 today.")
}
