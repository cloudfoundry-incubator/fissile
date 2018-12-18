package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// buildPackagesCmd represents the packages command
var buildPackagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Builds BOSH packages in a Docker container.",
	Long: `
This command will compile all required packages in the BOSH releases referenced by
your role manifest. The command will create a compilation container named
` + "`<repository>-cbase-<FISSILE_VERSION>-<RELEASE_NAME>-<RELEASE_VERSION>-pkg-<PACKAGE_NAME>`" + `
for each package (e.g. ` + "`fissile-cbase-1.0.0-cf-217-pkg-nats`" + `).

All containers are removed, whether compilation is successful or not. However, if
the compilation is interrupted during compilation (e.g. sending SIGINT), containers
will most likely be left behind.

Compiled packages are stored in ` + "`<work-dir>/compilation`" + `. Fissile uses the
package's fingerprint as part of the directory structure. This means that if the
same package (with the same version) is used by multiple releases, it will only be
compiled once.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flagBuildPackagesRoles := buildPackagesViper.GetString("roles")
		flagBuildPackagesOnlyReleases := buildPackagesViper.GetString("only-releases")
		flagBuildPackagesWithoutDocker := buildPackagesViper.GetBool("without-docker")
		flagBuildPackagesDockerNetworkMode := buildPackagesViper.GetString("docker-network-mode")
		flagBuildPackagesStemcell := buildPackagesViper.GetString("stemcell")
		flagBuildCompilationCacheConfig := buildPackagesViper.GetString("compilation-cache-config")
		flagBuildPackagesStreamPackages := buildPackagesViper.GetBool("stream-packages")

		err := fissile.GraphBegin(buildViper.GetString("output-graph"))
		if err != nil {
			return err
		}

		err = fissile.LoadManifest()
		if err != nil {
			return err
		}

		return fissile.Compile(
			flagBuildPackagesStemcell,
			fissile.StemcellCompilationDir(flagBuildPackagesStemcell),
			fissile.Options.RoleManifest,
			fissile.Options.Metrics,
			strings.FieldsFunc(flagBuildPackagesRoles, func(r rune) bool { return r == ',' }),
			strings.FieldsFunc(flagBuildPackagesOnlyReleases, func(r rune) bool { return r == ',' }),
			fissile.Options.Workers,
			flagBuildPackagesDockerNetworkMode,
			flagBuildPackagesWithoutDocker,
			fissile.Options.Verbose,
			flagBuildCompilationCacheConfig,
			flagBuildPackagesStreamPackages,
		)
	},
}

var buildPackagesViper = viper.New()

func init() {
	initViper(buildPackagesViper)

	buildCmd.AddCommand(buildPackagesCmd)

	// viper is busted w/ string slice, https://github.com/spf13/viper/issues/200
	buildPackagesCmd.PersistentFlags().StringP(
		"roles",
		"",
		"",
		"Build only packages for the given instance group names; comma separated.",
	)

	buildPackagesCmd.PersistentFlags().StringP(
		"only-releases",
		"",
		"",
		"Build only packages for the given release names; comma separated.",
	)

	buildPackagesCmd.PersistentFlags().BoolP(
		"without-docker",
		"",
		false,
		"Build without docker; this may adversely affect your system.  Only supported on Linux, and requires CAP_SYS_ADMIN.",
	)

	buildPackagesCmd.PersistentFlags().StringP(
		"docker-network-mode",
		"",
		"",
		"Specify network mode to be used when building with docker. e.g. \"--docker-network-mode host\" is equivalent to \"docker run --network=host\"",
	)

	buildPackagesCmd.PersistentFlags().StringP(
		"stemcell",
		"s",
		"",
		"The source stemcell",
	)

	buildPackagesCmd.PersistentFlags().StringP(
		"compilation-cache-config",
		"",
		filepath.Join(os.Getenv("HOME"), ".fissile", "package-cache.yaml"),
		"Points to a file containing configuration for a compiled package cache or contains the configuration as valid yaml",
	)

	buildPackagesCmd.PersistentFlags().BoolP(
		"stream-packages",
		"",
		false,
		"If true, fissile will stream packages to the docker daemon for compilation, instead of mounting volumes",
	)

	buildPackagesViper.BindPFlags(buildPackagesCmd.PersistentFlags())
}
