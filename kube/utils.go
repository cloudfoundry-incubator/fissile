package kube

import (
	"fmt"
	"strconv"
	"strings"
)

// parsePortRange converts a port range string to a starting and an ending port number
// port ranges can be single integers (e.g. 8080) or they can be ranges (e.g. 10001-10010)
func parsePortRange(portRange, name, description string) (int32, int32, error) {
	// Note that we do ParseInt with bitSize=16 because the max port number is 65535

	idx := strings.Index(portRange, "-")
	if idx < 0 {
		portNum, err := strconv.ParseInt(portRange, 10, 16)
		if err != nil {
			return 0, 0, fmt.Errorf("Port %s has invalid %s port %s: %s", name, description, portRange, err)
		}
		return int32(portNum), int32(portNum), nil
	}

	minPort, err := strconv.ParseInt(portRange[:idx], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("Port %s has invalid %s starting port %s: %s", name, description, portRange[:idx], err)
	}
	maxPort, err := strconv.ParseInt(portRange[idx+1:], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("Port %s has invalid %s ending port %s: %s", name, description, portRange[idx+1:], err)
	}
	if minPort > maxPort {
		return 0, 0, fmt.Errorf("Port %s has invalid %s port range %s", name, description, portRange)
	}
	return int32(minPort), int32(maxPort), nil
}
