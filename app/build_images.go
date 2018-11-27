package app

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/fissile/builder"
	"code.cloudfoundry.org/fissile/docker"
	"code.cloudfoundry.org/fissile/model"
	"github.com/SUSE/stampy"
	"github.com/fatih/color"
)

// BuildImagesOptions contains all option values for the `fissile build images` command.
type BuildImagesOptions struct {
	NoBuild                  bool
	Force                    bool
	Roles                    []string
	PatchPropertiesDirective string
	OutputDirectory          string
	Stemcell                 string
	StemcellID               string
	TagExtra                 string
	Labels                   map[string]string
}

// GenerateRoleImages generates all role images using releases
func (f *Fissile) GenerateRoleImages(opt BuildImagesOptions) error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	if f.Options.Metrics != "" {
		stampy.Stamp(f.Options.Metrics, "fissile", "create-images", "start")
		defer stampy.Stamp(f.Options.Metrics, "fissile", "create-images", "done")
	}

	if errs := f.Validate(); len(errs) != 0 {
		return fmt.Errorf(errs.Errors())
	}

	if opt.OutputDirectory != "" {
		err := os.MkdirAll(opt.OutputDirectory, 0755)
		if err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("Output directory %s exists and is not a directory", opt.OutputDirectory)
			}
			if err != nil {
				return fmt.Errorf("Error creating directory %s: %s", opt.OutputDirectory, err)
			}
		}
	}

	packagesImageBuilder, err := builder.NewPackagesImageBuilder(
		f.Options.RepositoryPrefix,
		opt.Stemcell,
		opt.StemcellID,
		f.CompilationDir(),
		f.DockerDir(),
		f.Version,
		f.UI,
	)
	if err != nil {
		return err
	}

	instanceGroups, err := f.Manifest.SelectInstanceGroups(opt.Roles)
	if err != nil {
		return err
	}

	if opt.OutputDirectory == "" {
		err = f.GeneratePackagesRoleImage(opt, instanceGroups, packagesImageBuilder)
	} else {
		err = f.GeneratePackagesRoleTarball(opt, instanceGroups, packagesImageBuilder)
	}
	if err != nil {
		return err
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(f.Manifest, instanceGroups, f)
	if err != nil {
		return err
	}

	roleBuilder, err := builder.NewRoleImageBuilder(
		opt.Stemcell,
		f.CompilationDir(),
		f.DockerDir(),
		f.Manifest.ManifestFilePath,
		f.Options.LightOpinions,
		f.Options.DarkOpinions,
		f.Options.Metrics,
		opt.TagExtra,
		f.Version,
		f.UI,
		f,
	)
	if err != nil {
		return err
	}

	return roleBuilder.BuildRoleImages(instanceGroups, f.Options.DockerRegistry, f.Options.DockerOrganization, f.Options.RepositoryPrefix, packagesLayerImageName, opt.OutputDirectory, opt.Force, opt.NoBuild, f.Options.Workers)
}

// GeneratePackagesRoleImage builds the docker image for the packages layer
// where all packages are included
func (f *Fissile) GeneratePackagesRoleImage(opt BuildImagesOptions, instanceGroups model.InstanceGroups, packagesImageBuilder *builder.PackagesImageBuilder) error {

	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	dockerManager, err := docker.NewImageManager()
	if err != nil {
		return fmt.Errorf("Error connecting to docker: %s", err.Error())
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(f.Manifest, instanceGroups, f)
	if err != nil {
		return fmt.Errorf("Error finding instance group's package name: %s", err.Error())
	}
	if !opt.Force {
		if hasImage, err := dockerManager.HasImage(packagesLayerImageName); err == nil && hasImage {
			f.UI.Printf("Packages layer %s already exists. Skipping ...\n", color.YellowString(packagesLayerImageName))
			return nil
		}
	}

	if hasImage, err := dockerManager.HasImage(opt.Stemcell); err != nil {
		return fmt.Errorf("Error looking up stemcell image: %s", err)
	} else if !hasImage {
		return fmt.Errorf("Failed to find stemcell image %s. Did you pull it?", opt.Stemcell)
	}

	if opt.NoBuild {
		f.UI.Println("Skipping packages layer docker image build because of --no-build flag.")
		return nil
	}

	f.UI.Printf("Building packages layer docker image %s ...\n",
		color.YellowString(packagesLayerImageName))
	log := new(bytes.Buffer)
	stdoutWriter := docker.NewFormattingWriter(
		log,
		docker.ColoredBuildStringFunc(packagesLayerImageName),
	)

	tarPopulator := packagesImageBuilder.NewDockerPopulator(instanceGroups, opt.Labels, opt.Force)
	err = dockerManager.BuildImageFromCallback(packagesLayerImageName, stdoutWriter, tarPopulator)
	if err != nil {
		log.WriteTo(f.UI)
		return fmt.Errorf("Error building packages layer docker image: %s", err.Error())
	}
	f.UI.Println(color.GreenString("Done."))

	return nil
}

// GeneratePackagesRoleTarball builds a tarball snapshot of the build context
// for the docker image for the packages layer where all packages are included
func (f *Fissile) GeneratePackagesRoleTarball(opt BuildImagesOptions, instanceGroups model.InstanceGroups, packagesImageBuilder *builder.PackagesImageBuilder) error {
	if f.Manifest == nil || len(f.Manifest.LoadedReleases) == 0 {
		return fmt.Errorf("Releases not loaded")
	}

	packagesLayerImageName, err := packagesImageBuilder.GetPackagesLayerImageName(f.Manifest, instanceGroups, f)
	if err != nil {
		return fmt.Errorf("Error finding instance group's package name: %v", err)
	}
	outputPath := filepath.Join(opt.OutputDirectory, fmt.Sprintf("%s.tar", packagesLayerImageName))

	if !opt.Force {
		info, err := os.Stat(outputPath)
		if err == nil && !info.IsDir() {
			f.UI.Printf("Packages layer %s already exists. Skipping ...\n", color.YellowString(outputPath))
			return nil
		}
	}

	if opt.NoBuild {
		f.UI.Println("Skipping packages layer tarball build because of --no-build flag.")
		return nil
	}

	f.UI.Printf("Building packages layer tarball %s ...\n", color.YellowString(outputPath))

	tarFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("Failed to create tar file %s: %s", outputPath, err)
	}
	tarWriter := tar.NewWriter(tarFile)

	// We always force build all packages here to avoid needing to talk to the
	// docker daemon to figure out what we can keep
	tarPopulator := packagesImageBuilder.NewDockerPopulator(instanceGroups, opt.Labels, true)
	err = tarPopulator(tarWriter)
	if err != nil {
		return fmt.Errorf("Error writing tar file: %s", err)
	}
	err = tarWriter.Close()
	if err != nil {
		return fmt.Errorf("Error closing tar file: %s", err)
	}
	f.UI.Println(color.GreenString("Done."))

	return nil
}
