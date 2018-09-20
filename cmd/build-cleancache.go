package cmd

import (
	"github.com/spf13/cobra"
)

// buildCleanCacheCmd represents the cleancache command
var buildCleanCacheCmd = &cobra.Command{
	Use:   "cleancache",
	Short: "Removes unused BOSH packages from the compilation cache.",
	Long: `
This command will inspect the compilation cache populated by its sibling "packages"
and remove all which are not required anymore.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		err := fissile.LoadManifest(
			flagRoleManifest,
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		return fissile.CleanCache(workPathCompilationDir)
	},
}

func init() {
	buildCmd.AddCommand(buildCleanCacheCmd)
}
