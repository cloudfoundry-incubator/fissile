package util

import (
	"io/ioutil"
	"os"
	"path"
)

// TempDir overrides the default TempDir, since Docker needs this
// to be in your user folder. If this isn't in your user folder, then
// docker cannot attach to the volume, and you get odd errors back
// from docker.
func TempDir(dir, prefix string) (name string, err error) {
	homeDir := os.Getenv("HOME")

	var fullPath string

	if dir != "" {
		fullPath = path.Join(homeDir, "tmp", dir)
	} else {
		fullPath = path.Join(homeDir, "tmp")
	}

	if pathExists, err := exists(fullPath); err != nil || !pathExists {
		err := os.MkdirAll(fullPath, 0777)

		if err != nil {
			return "", err
		}
	}

	return ioutil.TempDir(fullPath, prefix)
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
