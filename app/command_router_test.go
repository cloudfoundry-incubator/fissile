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

func TestDevReleaseArgumentsJustPathsOK(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{"/foo", "/bar"})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name"}.Apply(set)
	cli.StringSliceFlag{Name: "release-version"}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.Nil(err)
}

func TestDevReleaseArgumentsPathsAndNamesOK(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{"/foo", "/bar"})
	releaseName := cli.StringSlice([]string{"foo-name", "bar-name"})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name", Value: &releaseName}.Apply(set)
	cli.StringSliceFlag{Name: "release-version"}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.Nil(err)
}

func TestDevReleaseArgumentsPathsNamesAndVersionsOK(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{"/foo", "/bar"})
	releaseName := cli.StringSlice([]string{"foo-name", "bar-name"})
	releaseValue := cli.StringSlice([]string{"0+dev.1", "0+dev.2"})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name", Value: &releaseName}.Apply(set)
	cli.StringSliceFlag{Name: "release-version", Value: &releaseValue}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.Nil(err)
}

func TestDevReleaseArgumentsNotOK(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name"}.Apply(set)
	cli.StringSliceFlag{Name: "release-version"}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.NotNil(err)
	assert.Contains(err.Error(), "Please specify at least one release path")
}

func TestDevReleaseArgumentsMissingName(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{"/foo", "/bar"})
	releaseName := cli.StringSlice([]string{"foo-name"})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name", Value: &releaseName}.Apply(set)
	cli.StringSliceFlag{Name: "release-version"}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.NotNil(err)
	assert.Contains(err.Error(), "If you specify custom release names, you need to do it for all of them")
}

func TestDevReleaseArgumentsMissingVersion(t *testing.T) {
	assert := assert.New(t)

	set := flag.NewFlagSet("test", 0)

	release := cli.StringSlice([]string{"/foo", "/bar"})
	releaseValue := cli.StringSlice([]string{"0+dev.3"})

	cli.StringSliceFlag{Name: "release", Value: &release}.Apply(set)
	cli.StringSliceFlag{Name: "release-name"}.Apply(set)
	cli.StringSliceFlag{Name: "release-version", Value: &releaseValue}.Apply(set)

	c := cli.NewContext(nil, set, nil)

	err := validateDevReleaseArgs(c)
	assert.NotNil(err)
	assert.Contains(err.Error(), "If you specify custom release versions, you need to do it for all of them")
}
