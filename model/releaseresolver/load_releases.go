package releaseresolver

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/mholt/archiver"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

//LoadReleasesFromDisk loads information about BOSH releases
func LoadReleasesFromDisk(options model.ReleaseOptions) ([]*model.Release, error) {
	releases := make([]*model.Release, len(options.ReleasePaths))
	for idx, releasePath := range options.ReleasePaths {
		var releaseName, releaseVersion string
		if len(options.ReleaseNames) != 0 {
			releaseName = options.ReleaseNames[idx]
		}
		if len(options.ReleaseVersions) != 0 {
			releaseVersion = options.ReleaseVersions[idx]
		}
		var release *model.Release
		var err error
		if _, err = isFinalReleasePath(releasePath); err == nil {
			// For final releases, only can use release name and version defined in release.MF, cannot specify them through flags.
			release, err = model.NewFinalRelease(releasePath)
			if err != nil {
				return nil, fmt.Errorf("Error loading final release information: %s", err.Error())
			}
		} else {
			release, err = model.NewDevRelease(releasePath, releaseName, releaseVersion, options.BOSHCacheDir)
			if err != nil {
				return nil, fmt.Errorf("Error loading dev release information: %s", err.Error())
			}
		}
		releases[idx] = release
	}
	return releases, nil
}

func isFinalReleasePath(releasePath string) (bool, error) {
	if err := util.ValidatePath(releasePath, true, "release directory"); err != nil {
		return false, err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "release.MF"), false, "release 'release.MF' file"); err != nil {
		return false, err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "dev_releases"), true, "release 'dev_releases' file"); err == nil {
		return false, err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "jobs"), true, "release 'jobs' directory"); err != nil {
		return false, err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "packages"), true, "release 'packages' directory"); err != nil {
		return false, err
	}
	return true, nil
}

// downloadReleaseReferences downloads/builds and loads releases referenced in the
// manifest
func downloadReleaseReferences(releaseRefs []*model.ReleaseRef, finalReleasesDir string) ([]*model.Release, error) {
	releases := []*model.Release{}

	var allErrs error
	var wg sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wg))

	// go through each referenced release
	for _, releaseRef := range releaseRefs {
		wg.Add(1)

		go func(releaseRef *model.ReleaseRef) {
			defer wg.Done()
			_, err := url.ParseRequestURI(releaseRef.URL)
			if err != nil {
				// this is a local release that we need to build/load
				// TODO: support this
				allErrs = multierror.Append(allErrs, fmt.Errorf("Dev release %s is not supported as manifest references", releaseRef.Name))
				return
			}
			// this is a final release that we need to download
			finalReleaseTarballPath := filepath.Join(
				finalReleasesDir,
				fmt.Sprintf("%s-%s-%s.tgz", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))
			finalReleaseUnpackedPath := filepath.Join(
				finalReleasesDir,
				fmt.Sprintf("%s-%s-%s", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))

			if _, err := os.Stat(filepath.Join(finalReleaseUnpackedPath, "release.MF")); err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(finalReleaseUnpackedPath, 0700)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}

				// Show download progress
				bar := progress.AddBar(
					100,
					mpb.BarRemoveOnComplete(),
					mpb.PrependDecorators(
						decor.Name(releaseRef.Name, decor.WCSyncSpaceR),
						decor.Percentage(decor.WCSyncWidth),
					))
				lastPercentage := 0

				// download the release in a directory next to the role manifest
				err = util.DownloadFile(finalReleaseTarballPath, releaseRef.URL, func(percentage int) {
					bar.IncrBy(percentage - lastPercentage)
					lastPercentage = percentage
				})
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}
				defer func() {
					os.Remove(finalReleaseTarballPath)
				}()

				// unpack
				err = archiver.TarGz.Open(finalReleaseTarballPath, finalReleaseUnpackedPath)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
					return
				}
			}
		}(releaseRef)
	}

	wg.Wait()

	// Now that all releases have been downloaded and unpacked,
	// add them to the collection
	for _, releaseRef := range releaseRefs {
		finalReleaseUnpackedPath := filepath.Join(
			finalReleasesDir,
			fmt.Sprintf("%s-%s-%s", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))

		// create a release object and add it to the collection
		release, err := model.NewFinalRelease(finalReleaseUnpackedPath)

		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}
		releases = append(releases, release)
	}

	return releases, allErrs
}
