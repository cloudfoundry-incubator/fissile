package releaseresolver

import (
	"fmt"

	"code.cloudfoundry.org/fissile/model"
)

// ReleaseResolver state
type ReleaseResolver struct {
	releaseByName releaseByName
	manifestPath  string
}

type releaseByName map[string]*model.Release

// NewReleaseResolver returns a new ReleaseResolver
func NewReleaseResolver(path string) *ReleaseResolver {
	return &ReleaseResolver{manifestPath: path}
}

// Load loads all releases from either disk or URL
func (r *ReleaseResolver) Load(options model.ReleaseOptions, releaseRefs []*model.ReleaseRef) (model.Releases, error) {
	releases, err := LoadReleasesFromDisk(options)
	if err != nil {
		return nil, err
	}

	embeddedReleases, err := downloadReleaseReferences(releaseRefs, options.FinalReleasesDir)
	if err != nil {
		return nil, err
	}

	return append(releases, embeddedReleases...), nil
}

// CanValidate returns false because this resolver produces validatable results
func (r *ReleaseResolver) CanValidate() bool {
	return false
}

// FindRelease returns the release with the given name
func (r *ReleaseResolver) FindRelease(name string) (*model.Release, bool) {
	release, ok := r.releaseByName[name]
	return release, ok
}

// MapReleases needs to be called before FindRelease
func (r *ReleaseResolver) MapReleases(releases model.Releases) error {
	r.releaseByName = releaseByName{}

	for _, release := range releases {
		_, ok := r.releaseByName[release.Name]

		if ok {
			return fmt.Errorf("Error - release %s has been loaded more than once", release.Name)
		}

		r.releaseByName[release.Name] = release
	}
	return nil
}
