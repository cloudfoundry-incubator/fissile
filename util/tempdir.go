// +build windows unix !darwin

package util

import "io/ioutil"

// TempDir overrides the default TempDir, since Docker needs this
// to be in your user folder.
func TempDir(dir, prefix string) (name string, err error) {
	return ioutil.TempDir(dir, prefix)
}
