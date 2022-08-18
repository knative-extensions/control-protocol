package cgo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCgo(t *testing.T) {
	cgo := os.Getenv("CGO_ENABLED")
	assert.Equal(t, "", cgo)
}
