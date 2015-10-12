package app

import (
	"flag"
	"os"
	"path"
	"testing"

	"github.com/codegangsta/cli"
	"github.com/stretchr/testify/assert"
)

func TestFindAbsolutePaths(t *testing.T) {
	assert := assert.New(t)

	pwd, _ := os.Getwd()
	set := flag.NewFlagSet("test", 0)
	set.String("flag1", "path to first arg", "help for flag1")
	set.String("flag2", "path to second arg", "help for flag2")
	set.Set("flag1", "filename01.txt")
	set.Set("flag2", "filename02")

	c := cli.NewContext(nil, set, nil)

	paths, err := absolutePathsForFlags(c, []string{"flag1", "flag2", "flag3"})
	assert.Nil(err)

	assert.Equal(paths["flag1"], path.Join(pwd, c.String("flag1")))
	assert.Equal(paths["flag2"], path.Join(pwd, "filename02"))
	_, ok := paths["flag3"]
	
	assert.False(ok)

	// To get an error object out of app.AbsolutePathsForFlags we would need to delete
	// the current directory while the test is running.
}

