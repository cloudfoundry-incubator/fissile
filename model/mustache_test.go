package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	retCode := m.Run()

	os.Exit(retCode)
}

func TestParsing(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	template := "((FISSILE_IDENTITY_SCHEME))://((#FISSILE_IDENTITY_EXTERNAL_HOST))((FISSILE_INSTANCE_ID)).((FISSILE_IDENTITY_EXTERNAL_HOST)):((FISSILE_IDENTITY_EXTERNAL_PORT))((/FISSILE_IDENTITY_EXTERNAL_HOST))((^FISSILE_IDENTITY_EXTERNAL_HOST))scf.uaa-int.uaa.svc.((FISSILE_CLUSTER_DOMAIN)):8443((/FISSILE_IDENTITY_EXTERNAL_HOST))"

	// Act
	pieces, err := ParseTemplate(template)

	// Assert
	assert.NoError(err)
	assert.Contains(pieces, "FISSILE_INSTANCE_ID")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_HOST")
	assert.Contains(pieces, "FISSILE_IDENTITY_EXTERNAL_PORT")
	assert.Contains(pieces, "FISSILE_CLUSTER_DOMAIN")
	assert.NotContains(pieces, "FOO")
}
