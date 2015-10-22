package app

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
)

func TestFindAbsolutePaths(t *testing.T) {
	assert := assert.New(t)

	pwd, err := os.Getwd()
	assert.Nil(err)
	set := flag.NewFlagSet("test", 0)
	set.String("flag1", "path to first arg", "help for flag1")
	set.String("flag2", "path to second arg", "help for flag2")
	set.Set("flag1", "filename01.txt")
	set.Set("flag2", "filename02")

	c := cli.NewContext(nil, set, nil)

	paths, err := absolutePathsForFlags(c, "flag1", "flag2", "flag3")
	assert.Nil(err)

	assert.Equal(paths["flag1"], filepath.Join(pwd, c.String("flag1")))
	assert.Equal(paths["flag2"], filepath.Join(pwd, "filename02"))
	_, ok := paths["flag3"]

	assert.False(ok)

	// To get an error object out of app.AbsolutePathsForFlags we would need to delete
	// the current directory while the test is running.
}

func TestFindAbsolutePathsForArray(t *testing.T) {
	assert := assert.New(t)

	pwd, err := os.Getwd()
	assert.Nil(err)

	paths, err := absolutePathsForArray([]string{"filename01.txt", "filename02"})
	assert.Nil(err)

	assert.Equal(paths[0], filepath.Join(pwd, "filename01.txt"))
	assert.Equal(paths[1], filepath.Join(pwd, "filename02"))

	// To get an error object out of app.AbsolutePathsForFlags we would need to delete
	// the current directory while the test is running.
}
