package kube

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/helm"
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

type portInfo struct {
	name string
	port int32
}

func getPortInfo(name string, minPort, maxPort int32) ([]portInfo, error) {

	// We may need to fixup the port name.  It must:
	// - not be empty
	// - be no more than 15 characters long
	// - consist only of letters, digits, or hyphen
	// - start and end with a letter or a digit
	// - there can not be consecutive hyphens
	nameChars := make([]rune, 0, len(name))
	for _, ch := range name {
		switch {
		case ch >= 'A' && ch <= 'Z':
			nameChars = append(nameChars, ch)
		case ch >= 'a' && ch <= 'z':
			nameChars = append(nameChars, ch)
		case ch >= '0' && ch <= '9':
			nameChars = append(nameChars, ch)
		case ch == '-':
			if len(nameChars) == 0 {
				// Skip leading hyphens
				continue
			}
			if nameChars[len(nameChars)-1] == '-' {
				// Skip consecutive hyphens
				continue
			}
			nameChars = append(nameChars, ch)
		}
	}
	// Strip trailing hyphens
	for len(nameChars) > 0 && nameChars[len(nameChars)-1] == '-' {
		nameChars = nameChars[:len(nameChars)-1]
	}
	fixedName := string(nameChars)
	if fixedName == "" {
		return nil, fmt.Errorf("Port name %s does not contain any letters or digits", name)
	}

	rangeSize := maxPort - minPort
	suffixLength := 0
	if rangeSize > 0 {
		suffixLength = len(fmt.Sprintf("-%d", rangeSize))
	}
	if len(fixedName)+suffixLength > 15 {
		// Kubernetes doesn't like names that long
		availableLength := 7 - suffixLength
		fixedName = fmt.Sprintf("%s%x", fixedName[:availableLength], crc32.ChecksumIEEE([]byte(fixedName)))
	}

	results := make([]portInfo, 0, maxPort-minPort+1)
	for i := int32(0); i <= rangeSize; i++ {
		singleName := fixedName
		if suffixLength > 0 {
			singleName = fmt.Sprintf("%s-%d", fixedName, i)
		}
		results = append(results, portInfo{
			name: singleName,
			port: minPort + i,
		})
	}

	return results, nil
}

// newKubeConfig sets up generic a Kube config structure with minimal metadata
func newKubeConfig(kind string, name string, modifiers ...helm.NodeModifier) *helm.Object {
	object := helm.NewObject(modifiers...)
	object.Add("apiVersion", helm.NewScalar("v1"))
	object.Add("kind", helm.NewScalar(kind))

	meta := helm.NewObject()
	meta.Add("name", helm.NewScalar(name))
	object.Add("metadata", meta)

	return object
}
