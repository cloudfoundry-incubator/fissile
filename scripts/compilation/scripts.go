package compilation

import (
	"fmt"
	"io/ioutil"
)

const (
	// OpenSUSE ist the name of the openSUSE base image
	OpenSUSE = "opensuse"
	// UbuntuBase is the name of the Ubuntu base image
	UbuntuBase = "ubuntu"
	// FakeBase is the name of the fake base image
	FakeBase = "fake"
	// FailBase is used to force package compile to fail when testing.
	FailBase = "fail"

	// CompilationScript is the compilation script
	CompilationScript = "compile"
	// PrerequisitesScript is the script that installs prerequisites
	PrerequisitesScript = "prerequisites"
)

// SaveScript will write a script to the disk
func SaveScript(baseType, scriptType, path string) error {
	script, err := GetScript(baseType, scriptType)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(path, script, 0700); err != nil {
		return fmt.Errorf("Error saving script asset: %s", err.Error())
	}

	return nil
}

// GetScript will lookup a script
func GetScript(baseType, scriptType string) ([]byte, error) {
	assetPath := fmt.Sprintf("scripts/compilation/%s-%s.sh", baseType, scriptType)

	script, err := Asset(assetPath)
	if err != nil {
		return nil, fmt.Errorf("Error loading script asset. This is probably a bug: %s", err.Error())
	}

	return script, nil
}
