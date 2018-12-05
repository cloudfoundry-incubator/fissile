package releaseresolver

import (
	"archive/tar"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/util"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/mholt/archiver"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
	"gopkg.in/yaml.v2"
)

// loadReleases loads information about BOSH releases, whether locally or from the internet
func loadReleases(releaseRefs []*model.ReleaseRef, cacheDir string) ([]*model.Release, error) {
	releases := []*model.Release{}

	// Download releases as needed
	var allErrs error
	var wg sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wg))

	for _, releaseRef := range releaseRefs {
		if isReleaseRefRemote(releaseRef) {
			// this is a final release that we need to download
			wg.Add(1)

			go func(releaseRef *model.ReleaseRef) {
				err := downloadReleaseArchive(releaseRef, cacheDir, progress)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
				}
				wg.Done()
			}(releaseRef)
		} else {
			// This is a local release; check for archives
			pathInfo, err := os.Stat(releaseRef.URL)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
			} else if !pathInfo.IsDir() {
				wg.Add(1)
				go func() {
					// This is a file, unpack it
					if releaseRef.SHA1 == "" {
						// Calculate the hash
					}
					err := unpackReleaseArchive(releaseRef, releaseRef.URL, cacheDir)
					if err != nil {
						allErrs = multierror.Append(allErrs, err)
					}
					wg.Done()
				}()
			}
		}
	}

	wg.Wait()

	// Now that all releases have been downloaded and unpacked,
	// add them to the collection
	for _, releaseRef := range releaseRefs {
		var release *model.Release
		var err error
		if isReleaseRefRemote(releaseRef) {
			finalReleaseUnpackedPath := getFinalReleaseUnpackedPath(releaseRef, cacheDir)

			// create a release object and add it to the collection
			release, err = model.NewFinalRelease(finalReleaseUnpackedPath)

			if err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		} else {
			err = fixupLocalRelease(releaseRef, cacheDir)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
				continue
			}
			if err = checkFinalReleasePath(releaseRef.URL); err == nil {
				// For final releases, only can use release name and version defined in release.MF, cannot specify them through flags.
				release, err = model.NewFinalRelease(releaseRef.URL)
				if err != nil {
					return nil, fmt.Errorf("Error loading final release information: %s", err.Error())
				}
			} else {
				// For dev releases, use the information given
				release, err = model.NewDevRelease(
					releaseRef.URL,
					releaseRef.Name,
					releaseRef.Version,
					cacheDir)
				if err != nil {
					return nil, fmt.Errorf("Error loading dev release information: %s", err.Error())
				}
			}
		}
		releases = append(releases, release)
	}

	return releases, allErrs
}

// isReleaseRefRemote returns true if the given ReleaseRef looks like a remote URL
func isReleaseRefRemote(releaseRef *model.ReleaseRef) bool {
	if releaseRef.URL == "" {
		return false
	}
	u, err := url.ParseRequestURI(releaseRef.URL)
	return err == nil && u.IsAbs()
}

// checkFinalReleasePath checks to see if a given path is an unpacked final release; returns nil on success
func checkFinalReleasePath(releasePath string) error {
	if err := util.ValidatePath(releasePath, true, "release directory"); err != nil {
		return err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "release.MF"), false, "release 'release.MF' file"); err != nil {
		return err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "dev_releases"), true, "release 'dev_releases' file"); err == nil {
		return err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "jobs"), true, "release 'jobs' directory"); err != nil {
		return err
	}
	if err := util.ValidatePath(filepath.Join(releasePath, "packages"), true, "release 'packages' directory"); err != nil {
		return err
	}

	err := error(nil)
	return err
}

func getFinalReleaseUnpackedPath(releaseRef *model.ReleaseRef, cacheDir string) string {
	return filepath.Join(
		cacheDir,
		".final_releases",
		fmt.Sprintf("%s-%s-%s", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))
}

// downloadReleaseArchive downloads and unpacks a release reference
func downloadReleaseArchive(releaseRef *model.ReleaseRef, cacheDir string, progress *mpb.Progress) error {
	finalReleasesWorkDir := filepath.Join(cacheDir, ".final_releases")
	finalReleaseTarballPath := filepath.Join(
		finalReleasesWorkDir,
		fmt.Sprintf("%s-%s-%s.tgz", releaseRef.Name, releaseRef.Version, releaseRef.SHA1))

	usingTempFile := false
	var finalReleaseUnpackedPath string
	if releaseRef.Name == "" || releaseRef.Version == "" || releaseRef.SHA1 == "" {
		usingTempFile = true
		tempFile, err := ioutil.TempFile(finalReleasesWorkDir, "download-temp-*.tgz")
		if err != nil {
			return err
		}
		finalReleaseTarballPath = tempFile.Name()
		_ = tempFile.Close()
		defer os.Remove(finalReleaseTarballPath)

		finalReleaseUnpackedPath, err = ioutil.TempDir(finalReleasesWorkDir, "unpack-temp-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(finalReleaseUnpackedPath)
	} else {
		finalReleaseUnpackedPath = getFinalReleaseUnpackedPath(releaseRef, cacheDir)

		manifestPath := filepath.Join(finalReleaseUnpackedPath, "release.MF")
		if _, err := os.Stat(manifestPath); err == nil || !os.IsNotExist(err) {
			// Already unpacked successfully
			return err
		}

		err := os.MkdirAll(finalReleaseUnpackedPath, 0700)
		if err != nil {
			return err
		}
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

	// download the release in a subdirectory of the cacheDir
	err := util.DownloadFile(finalReleaseTarballPath, releaseRef.URL, func(percentage int) {
		bar.IncrBy(percentage - lastPercentage)
		lastPercentage = percentage
	})
	if err != nil {
		return err
	}
	defer func() {
		os.Remove(finalReleaseTarballPath)
	}()

	// Fix up release info if it's missing
	if usingTempFile {
		releaseRef.URL = finalReleaseTarballPath
		err = fixupLocalRelease(releaseRef, cacheDir)
		if err != nil {
			return err
		}
	}

	// unpack
	err = unpackReleaseArchive(releaseRef, finalReleaseTarballPath, cacheDir)
	if err != nil {
		return err
	}

	return nil
}

// unpackReleaseArchive unpacks a final release tarball
func unpackReleaseArchive(releaseRef *model.ReleaseRef, tarballPath, cacheDir string) error {
	finalReleaseUnpackedPath := getFinalReleaseUnpackedPath(releaseRef, cacheDir)
	manifestPath := filepath.Join(finalReleaseUnpackedPath, "release.MF")

	err := archiver.TarGz.Open(tarballPath, finalReleaseUnpackedPath)
	if err != nil {
		// Delete the manifest so we retry downloading next time; ignore any errors
		_ = os.Remove(manifestPath)
		return err
	}
	return nil
}

// fixupLocalRelease will unpack local releases as necessary
func fixupLocalRelease(releaseRef *model.ReleaseRef, cacheDir string) error {
	if isReleaseRefRemote(releaseRef) {
		panic(fmt.Sprintf("Release %+v is remote", releaseRef))
	}
	pathInfo, err := os.Stat(releaseRef.URL)
	if err != nil {
		return err
	}
	if pathInfo.IsDir() {
		// Already unpacked (possibly dev) release
		return nil
	}

	// Read the release information from the archive
	reader, err := os.Open(releaseRef.URL)
	if err != nil {
		return err
	}
	util.TargzIterate(releaseRef.URL, reader, func(reader *tar.Reader, header *tar.Header) error {
		if path.Clean(header.Name) != "release.MF" {
			return nil
		}
		err := yaml.NewDecoder(reader).Decode(releaseRef)
		if err != nil {
			return err
		}
		return nil
	})
	_, err = reader.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}
	hasher := sha1.New()
	_, err = io.Copy(hasher, reader)
	if err != nil {
		return err
	}
	releaseRef.SHA1 = fmt.Sprintf("%x", hasher.Sum(nil))

	err = unpackReleaseArchive(releaseRef, releaseRef.URL, cacheDir)
	if err != nil {
		return err
	}
	releaseRef.URL = getFinalReleaseUnpackedPath(releaseRef, cacheDir)
	return checkFinalReleasePath(releaseRef.URL)
}
