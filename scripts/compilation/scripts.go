package compilation

import (
	"fmt"
	"io/ioutil"
)

const (
	UbuntuBase = "ubuntu"
	FakeBase   = "fake"

	CompilationScript    = "compile"
	PreprequisitesScript = "prerequisites"
)

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

func GetScript(baseType, scriptType string) ([]byte, error) {
	assetPath := fmt.Sprintf("scripts/compilation/%s-%s.sh", baseType, scriptType)

	script, err := Asset(assetPath)
	if err != nil {
		return nil, fmt.Errorf("Error loading script asset. This is probably a bug: %s", err.Error())
	}

	return script, nil
}
