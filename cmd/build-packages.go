package cmd

import (
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

		flagBuildPackagesRoles := viper.GetString("roles")

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.Compile(
			flagRepository,
			workPathCompilationDir,
			flagRoleManifest,
			flagMetrics,
			strings.Split(flagBuildPackagesRoles, ","),
			flagWorkers,
		)
	},
}

func init() {
	buildCmd.AddCommand(buildPackagesCmd)

	// viper is busted w/ string slice, https://github.com/spf13/viper/issues/200
	buildPackagesCmd.PersistentFlags().StringP(
		"roles",
		"",
		"",
		"Build only packages for the given role names; comma separated.",
	)

	viper.BindPFlags(buildPackagesCmd.PersistentFlags())
}
