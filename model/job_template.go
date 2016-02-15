package model

import ()

// JobTemplate represents a BOSH job template
type JobTemplate struct {
	SourcePath      string
	DestinationPath string
	Job             *Job
	Content         string
}
