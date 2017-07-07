package model

import "github.com/SUSE/fissile/util"

// ReleaseLicense represents the license of a BOSH release
type ReleaseLicense struct {
	// Files is a mapping of license file names to contents
	Files map[string][]byte
	// Release this license belongs to
	Release *Release
}

// JobProperty is a generic key-value property referenced by a job
type JobProperty struct {
	Name        string
	Description string
	Default     interface{}
	Job         *Job
}

// MarshalJSON implements the encoding/json.Marshaler interface
func (p *JobProperty) MarshalJSON() ([]byte, error) {
	data, err := p.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return util.JSONMarshal(data)
}

// MarshalYAML implements the yaml.Marshaler interface
func (p *JobProperty) MarshalYAML() (interface{}, error) {
	var jobName string
	if p.Job != nil {
		jobName = p.Job.Name
	}

	return map[string]interface{}{
		"name":        p.Name,
		"description": p.Description,
		"default":     p.Default,
		"job":         jobName,
	}, nil
}
