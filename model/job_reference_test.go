package model

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteConfigs(t *testing.T) {
	assert := assert.New(t)

	job := &ReleaseJob{
		Name: "silly job",
		SpecProperties: []*JobSpecProperty{
			&JobSpecProperty{
				Name:    "prop",
				Default: "bar",
			},
		},
		AvailableProviders: map[string]jobProvidesInfo{
			"<not used>": jobProvidesInfo{
				jobLinkInfo: jobLinkInfo{
					Name: "<not used>",
				},
				Properties: []string{"exported-prop"},
			},
		},
		DesiredConsumers: []jobConsumesInfo{
			jobConsumesInfo{
				jobLinkInfo: jobLinkInfo{
					Name: "serious",
					Type: "serious-type",
				},
			},
		},
	}

	role := &InstanceGroup{
		Name: "dummy role",
		JobReferences: JobReferences{
			{
				ReleaseJob: job,
				Name:       "silly job",
				ResolvedConsumers: map[string]jobConsumesInfo{
					"serious": jobConsumesInfo{
						jobLinkInfo: jobLinkInfo{
							Name:     "serious",
							Type:     "serious-type",
							RoleName: "dummy role",
							JobName:  job.Name,
						},
					},
				},
			},
		},
	}

	tempFile, err := ioutil.TempFile("", "fissile-job-test")
	assert.NoError(err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(strings.Replace(`---
	properties:
		foo: 3
	`, "\t", "    ", -1))
	assert.NoError(err)
	assert.NoError(tempFile.Close())

	json, err := role.JobReferences[0].WriteConfigs(role, tempFile.Name(), tempFile.Name())
	assert.NoError(err)

	assert.JSONEq(`
	{
		"job": {
			"name": "dummy role"
		},
		"parameters": {},
		"properties": {
			"prop": "bar"
		},
		"networks": {
			"default": {}
		},
		"exported_properties": [
			"prop"
		],
		"consumes": {
			"serious": {
				"role": "dummy role",
				"job": "silly job"
			}
		},
		"exported_properties": [
			"exported-prop"
		]
	}`, string(json))
}
