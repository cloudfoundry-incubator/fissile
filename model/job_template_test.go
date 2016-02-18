package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobTemplatesContentOk(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.Nil(err)

	assert.Equal(1, len(release.Jobs))

	assert.Equal(2, len(release.Jobs[0].Templates))

	for _, template := range release.Jobs[0].Templates {
		assert.NotEmpty(template)
	}
}
