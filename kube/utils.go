package kube

import (
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"

	"github.com/SUSE/fissile/helm"
)

const (
	// RoleNameLabel is a thing
	RoleNameLabel = "skiff-role-name"
	// VolumeStorageClassAnnotation is the annotation label for storage/v1beta1/StorageClass
	VolumeStorageClassAnnotation = "volume.beta.kubernetes.io/storage-class"
)

// parsePortRange converts a port range string to a starting and an ending port number
// port ranges can be single integers (e.g. 8080) or they can be ranges (e.g. 10001-10010)
func parsePortRange(portRange, name, description string) (int, int, error) {
	// Note that we do ParseInt with bitSize=16 because the max port number is 65535
	idx := strings.Index(portRange, "-")
	if idx < 0 {
		portNum, err := strconv.Atoi(portRange)
		if err != nil {
			return 0, 0, fmt.Errorf("Port %s has invalid %s port %s: %s", name, description, portRange, err)
		}
		return portNum, portNum, nil
	}
	minPort, err := strconv.Atoi(portRange[:idx])
	if err != nil {
		return 0, 0, fmt.Errorf("Port %s has invalid %s starting port %s: %s", name, description, portRange[:idx], err)
	}
	maxPort, err := strconv.Atoi(portRange[idx+1:])
	if err != nil {
		return 0, 0, fmt.Errorf("Port %s has invalid %s ending port %s: %s", name, description, portRange[idx+1:], err)
	}
	if minPort > maxPort {
		return 0, 0, fmt.Errorf("Port %s has invalid %s port range %s", name, description, portRange)
	}
	return minPort, maxPort, nil
}

type portInfo struct {
	name string
	port int
}

func getPortInfo(name string, minPort, maxPort int) ([]portInfo, error) {
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
	for i := 0; i <= rangeSize; i++ {
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

func newTypeMeta(apiVersion, kind string) *helm.Mapping {
	return helm.NewMapping("apiVersion", apiVersion, "kind", kind)
}

func newObjectMeta(name string) *helm.Mapping {
	meta := helm.NewMapping("name", name)
	meta.Add("labels", helm.NewMapping(RoleNameLabel, name))
	return meta
}

func newSelector(name string) *helm.Mapping {
	meta := helm.NewMapping()
	meta.Add("matchLabels", helm.NewMapping(RoleNameLabel, name))
	return meta
}

// newKubeConfig sets up generic a Kube config structure with minimal metadata
func newKubeConfig(apiVersion, kind string, name string, modifiers ...helm.NodeModifier) *helm.Mapping {
	mapping := newTypeMeta(apiVersion, kind)
	mapping.Set(modifiers...)
	mapping.Add("metadata", newObjectMeta(name))

	return mapping
}

func makeVarName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}
