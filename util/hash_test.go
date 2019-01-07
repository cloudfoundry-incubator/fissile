package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	assert.New(t).Equal("4d51b43d077ed5a7b7ae4fb200aeb216b7736a96", Hash("ubuntu:14.04"))
}
