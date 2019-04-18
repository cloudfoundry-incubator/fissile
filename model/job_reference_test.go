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

	job := &Job{
		Name: "silly job",
		Properties: []*JobProperty{
			&JobProperty{
				Name:    "prop",
				Default: "bar",
			},
		},
		AvailableProviders: map[string]JobProvidesInfo{
			"<not used>": JobProvidesInfo{
				JobLinkInfo: JobLinkInfo{
					Name: "<not used>",
				},
				Properties: []string{"exported-prop"},
			},
		},
		DesiredConsumers: []JobConsumesInfo{
			JobConsumesInfo{
				JobLinkInfo: JobLinkInfo{
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
				Job:  job,
				Name: "silly job",
				ResolvedConsumes: map[string]JobConsumesInfo{
					"serious": JobConsumesInfo{
						JobLinkInfo: JobLinkInfo{
							Name:     "serious",
							Type:     "serious-type",
							RoleName: "dummy role",
							JobName:  job.Name,
						},
					},
				},
				ResolvedConsumedBy: map[string][]JobLinkInfo{
					"consumed-by": []JobLinkInfo{
						{
							Name:     "consumed-by",
							Type:     "consumed-by-type",
							RoleName: "another role",
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

	// `service_name` is empty because we never resolved links
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
				"job": "silly job",
				"service_name": ""
			}
		},
		"consumed_by": {
			"consumed-by": [{
				"role": "another role",
				"job": "silly job",
				"service_name": ""
			}]
		},
		"exported_properties": [
			"exported-prop"
		]
	}`, string(json))
}
