package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/fissile/builder"
	"code.cloudfoundry.org/fissile/model"
	"code.cloudfoundry.org/fissile/model/releaseresolver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// buildImagesCmd represents the images command.
var buildReleaseImagesCmd = &cobra.Command{
	Use:   "release-images",
	Short: "Builds Docker images from your BOSH releases.",
	Long: `
This command goes through builds a Docker image for each specified release.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		imgBuilder := &builder.ReleasesImageBuilder{
			CompilationCacheConfig: buildReleaseImagesViper.GetString("compilation-cache-config"),
			DockerNetworkMode:      buildPackagesViper.GetString("docker-network-mode"),
			DockerOrganization:     fissile.Options.DockerOrganization,
			DockerRegistry:         fissile.Options.DockerRegistry,
			FissileVersion:         fissile.Version,
			Force:                  buildReleaseImagesViper.GetBool("force"),
			Grapher:                fissile,
			MetricsPath:            fissile.Options.Metrics,
			NoBuild:                buildReleaseImagesViper.GetBool("no-build"),
			OutputDirectory:        buildReleaseImagesViper.GetString("output-directory"),
			RepositoryPrefix:       fissile.Options.RepositoryPrefix,
			StemcellName:           buildReleaseImagesViper.GetString("stemcell"),
			StreamPackages:         buildPackagesViper.GetBool("stream-packages"),
			UI:                     fissile.UI,
			Verbose:                fissile.Options.Verbose,
			WithoutDocker:          buildPackagesViper.GetBool("without-docker"),
			WorkerCount:            fissile.Options.Workers,
		}
		imgBuilder.CompilationDir = fissile.StemcellCompilationDir(imgBuilder.StemcellName)

		if len(imgBuilder.StemcellName) == 0 {
			return fmt.Errorf("--stemcell is a required parameter")
		}

		if imgBuilder.OutputDirectory != "" && !imgBuilder.Force {
			fissile.UI.Println("--force required when --output-directory is set")
			imgBuilder.Force = true
		}

		names := strings.FieldsFunc(buildReleaseImagesViper.GetString("name"), func(r rune) bool { return r == ',' })
		sha1s := strings.FieldsFunc(buildReleaseImagesViper.GetString("sha1"), func(r rune) bool { return r == ',' })
		urls := strings.FieldsFunc(buildReleaseImagesViper.GetString("url"), func(r rune) bool { return r == ',' })
		versions := strings.FieldsFunc(buildReleaseImagesViper.GetString("version"), func(r rune) bool { return r == ',' })

		if len(names) == 0 {
			return fmt.Errorf("Must specify at least a single release for release-images build command")
		}
		if len(names) != len(urls) {
			return fmt.Errorf("Must specify same number of release names as release URLs")
		}
		if len(sha1s) != len(urls) {
			return fmt.Errorf("Must specify same number of release SHA1s as release URLs")
		}
		if len(versions) != len(urls) {
			return fmt.Errorf("Must specify same number of release versions as release URLs")
		}

		releaseRefs := make([]*model.ReleaseRef, len(names))
		for i := range names {
			releaseRefs[i] = &model.ReleaseRef{
				Name:    names[i],
				Version: versions[i],
				URL:     urls[i],
				SHA1:    sha1s[i],
			}
		}
		resolver := releaseresolver.NewReleaseResolver(fissile.Options.FinalReleasesDir)
		releases, err := resolver.Load(model.ReleaseOptions{}, releaseRefs)
		if err != nil {
			return fmt.Errorf("Error loading release information: %v", err)
		}

		err = fissile.GraphBegin(buildViper.GetString("output-graph"))
		if err != nil {
			return err
		}

		return imgBuilder.Build(releases)

	},
}

var buildReleaseImagesViper = viper.New()

func init() {
	initViper(buildReleaseImagesViper)

	buildCmd.AddCommand(buildReleaseImagesCmd)

	buildReleaseImagesCmd.PersistentFlags().BoolP(
		"no-build",
		"N",
		false,
		"If specified, the Dockerfile and assets will be created, but the image won't be built.",
	)

	buildReleaseImagesCmd.PersistentFlags().BoolP(
		"force",
		"F",
		false,
		"If specified, image creation will proceed even when images already exist.",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"output-directory",
		"O",
		"",
		"Output the result as tar files in the given directory rather than building with docker",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"stemcell",
		"s",
		"",
		"The source stemcell",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"compilation-cache-config",
		"",
		filepath.Join(os.Getenv("HOME"), ".fissile", "package-cache.yaml"),
		"Points to a file containing configuration for a compiled package cache or contains the configuration as valid yaml",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"name",
		"",
		"",
		"The release name",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"sha1",
		"",
		"",
		"The release SHA1",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"url",
		"",
		"",
		"The release URL",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"version",
		"",
		"",
		"The release version",
	)

	buildReleaseImagesCmd.PersistentFlags().BoolP(
		"without-docker",
		"",
		false,
		"Build without docker; this may adversely affect your system.  Only supported on Linux, and requires CAP_SYS_ADMIN.",
	)

	buildReleaseImagesCmd.PersistentFlags().StringP(
		"docker-network-mode",
		"",
		"",
		"Specify network mode to be used when building with docker. e.g. \"--docker-network-mode host\" is equivalent to \"docker run --network=host\"",
	)

	buildReleaseImagesCmd.PersistentFlags().BoolP(
		"stream-packages",
		"",
		false,
		"If true, fissile will stream packages to the docker daemon for compilation, instead of mounting volumes",
	)

	buildReleaseImagesViper.BindPFlags(buildReleaseImagesCmd.PersistentFlags())
}
