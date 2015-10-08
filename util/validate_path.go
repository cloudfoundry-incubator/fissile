package util

import (
	"fmt"
	"os"
)

func ValidatePath(path string, shouldBeDir bool, pathDescription string) error {
	pathInfo, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Path %s (%s) does not exist.", path, pathDescription)
		}

		return err
	}

	if pathInfo.IsDir() && !shouldBeDir {
		return fmt.Errorf("Path %s (%s) points to a directory. It should be a a file.", path, pathDescription)
	} else if !pathInfo.IsDir() && shouldBeDir {
		return fmt.Errorf("Path %s (%s) points to a file. It should be a directory.", path, pathDescription)
	}

	return nil
}
