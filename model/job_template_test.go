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
	assert.NoError(err)

	ntpReleasePath := filepath.Join(workDir, "../test-assets/ntp-release")
	ntpReleasePathBoshCache := filepath.Join(ntpReleasePath, "bosh-cache")
	release, err := NewDevRelease(ntpReleasePath, "", "", ntpReleasePathBoshCache)
	assert.NoError(err)

	assert.Len(release.Jobs, 1)

	assert.Len(release.Jobs[0].Templates, 2)

	for _, template := range release.Jobs[0].Templates {
		assert.NotEmpty(template)
	}
}

func TestJobTemplateMarshal(t *testing.T) {
	testCases := []struct {
		name     string
		template *JobTemplate
		expected map[string]interface{}
	}{
		{
			name: "simple",
			template: &JobTemplate{
				SourcePath:      "/source",
				DestinationPath: "/dest",
				Job:             &Job{Fingerprint: "asdf"},
				Content:         "<content>",
			},
			expected: map[string]interface{}{
				"sourcePath":      "/source",
				"destinationPath": "/dest",
				"job":             "asdf",
				"content":         "<content>",
			},
		},
		{
			name: "jobless",
			template: &JobTemplate{
				SourcePath:      "/source",
				DestinationPath: "/dest",
				Job:             nil,
				Content:         "<content>",
			},
			expected: map[string]interface{}{
				"sourcePath":      "/source",
				"destinationPath": "/dest",
				"job":             "",
				"content":         "<content>",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert := assert.New(t)

			actual, err := testCase.template.Marshal()
			if assert.NoError(err) {
				assert.Equal(testCase.expected, actual)
			}
		})
	}
}
