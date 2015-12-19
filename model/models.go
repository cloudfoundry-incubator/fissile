package model

// ReleaseLicense represents the license of a BOSH release
type ReleaseLicense struct {
	// ActualSHA1 is the SHA1 of the archive we actually have
	ActualSHA1 string
	// SHA1 is the SHA1 of the archive as specified in the manifest
	SHA1 string
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
