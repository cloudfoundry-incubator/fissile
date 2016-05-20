package version_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cppforlife/go-semi-semantic/version"
)

func TestSettings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Suite")
}

func MustNewVersionFromString(v string) Version {
	ver, err := NewVersionFromString(v)
	if err != nil {
		panic(fmt.Sprintf("Invalid version '%s': %s", v, err))
	}

	return ver
}

func MustNewVersion(release, preRelease, postRelease VersionSegment) Version {
	ver, err := NewVersion(release, preRelease, postRelease)
	if err != nil {
		panic(fmt.Sprintf("Invalid version '%v, %v, %v': %s", release, preRelease, postRelease, err))
	}

	return ver
}

func MustNewVersionSegmentFromString(v string) VersionSegment {
	verSeg, err := NewVersionSegmentFromString(v)
	if err != nil {
		panic(fmt.Sprintf("Invalid version segment '%s': %s", v, err))
	}

	return verSeg
}

func MustNewVersionSegment(components []VerSegComp) VersionSegment {
	verSeg, err := NewVersionSegment(components)
	if err != nil {
		panic(fmt.Sprintf("Invalid version segment '%v': %s", components, err))
	}

	return verSeg
}
