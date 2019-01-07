package util

import (
	"crypto/sha1"
	"encoding/hex"
)

// Hash returns a printable hash of the name
func Hash(name string) string {
	hasher := sha1.New()
	hasher.Write([]byte(name))
	return hex.EncodeToString(hasher.Sum(nil))
}
