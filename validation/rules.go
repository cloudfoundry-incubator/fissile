package validation

import "fmt"

const (
	// UDP protocol
	UDP = `UDP`
	// TCP protocol
	TCP = `TCP`
)

// IsValidPortNum tests that the argument is a valid, non-zero port number.
func IsValidPortNum(port int) error {
	if 1 <= port && port <= 65535 {
		return nil
	}
	return fmt.Errorf(`must be between %d and %d, inclusive`, 1, 65535)
}

// IsValidProtocol tests that the argument is TCP or UDP.
func IsValidProtocol(protocol string) error {
	if protocol != TCP && protocol != UDP {
		return fmt.Errorf(`must be TCP or UDP`)
	}
	return nil
}
