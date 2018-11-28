package model

// ReleaseOptions for releases
type ReleaseOptions struct {
	ReleasePaths    []string
	ReleaseNames    []string
	ReleaseVersions []string
	BOSHCacheDir    string
}

// ReleaseResolver loads job specs from releases and acts as a registry for
// release structs containing those  job specs
type ReleaseResolver interface {
	Load(ReleaseOptions, []*ReleaseRef) (Releases, error)
	CanValidate() bool
	MapReleases(Releases) error
	FindRelease(string) (*Release, bool)
}
