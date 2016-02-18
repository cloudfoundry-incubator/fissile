package configstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpinionsLoad(t *testing.T) {
	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	confOpinions, err := newOpinions(opinionsFile, opinionsFileDark)
	assert.Nil(err)
	assert.NotNil(confOpinions)
}

func TestGetOpinionForKey(t *testing.T) {

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	confOpinions, err := newOpinions(opinionsFile, opinionsFileDark)
	assert.Nil(err)
	assert.NotNil(confOpinions)

	masked, value := confOpinions.GetOpinionForKey([]string{"tor", "int_opinion"})
	assert.False(masked)
	assert.Equal(31, value)
}

func TestGetOpinionWithDarkKey(t *testing.T) {

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	confOpinions, err := newOpinions(opinionsFile, opinionsFileDark)
	assert.Nil(err)
	assert.NotNil(confOpinions)

	masked, value := confOpinions.GetOpinionForKey([]string{"tor", "masked_opinion"})
	assert.True(masked)
	assert.Nil(value)
}

func TestGetOpinionForKeyInvalid(t *testing.T) {

	assert := assert.New(t)

	workDir, err := os.Getwd()
	assert.Nil(err)

	opinionsFile := filepath.Join(workDir, "../test-assets/test-opinions/opinions.yml")
	opinionsFileDark := filepath.Join(workDir, "../test-assets/test-opinions/dark-opinions.yml")

	confOpinions, err := newOpinions(opinionsFile, opinionsFileDark)
	assert.Nil(err)
	assert.NotNil(confOpinions)

	masked, value := confOpinions.GetOpinionForKey([]string{"cc", "app_events", "cutoff_age_in_days", "foo"})
	assert.False(masked)
	assert.Nil(value)
}
