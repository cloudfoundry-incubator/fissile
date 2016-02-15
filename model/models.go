package model

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
