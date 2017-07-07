package model

// JobTemplate represents a BOSH job template
type JobTemplate struct {
	SourcePath      string
	DestinationPath string
	Job             *Job
	Content         string
}

// Marshal implements the util.Marshaler interface
func (t *JobTemplate) Marshal() (interface{}, error) {
	var jobFingerprint string
	if t.Job != nil {
		jobFingerprint = t.Job.Fingerprint
	}

	return map[string]interface{}{
		"sourcePath":      t.SourcePath,
		"destinationPath": t.DestinationPath,
		"job":             jobFingerprint,
		"content":         t.Content,
	}, nil
}
