package model

// ReleaseLicense represents the license of a BOSH release
type ReleaseLicense struct {
	SHA1     string
	Contents []byte
	Release  *Release
}

// JobProperty is a generic key-value property referenced by a job
type JobProperty struct {
	Name        string
	Description string
	Default     interface{}
	Job         *Job
}
