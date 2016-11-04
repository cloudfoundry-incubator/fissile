package cmd

import (
	"github.com/spf13/cobra"
)

// buildCleancacheCmd represents the cleancache command
var buildCleancacheCmd = &cobra.Command{
	Use:   "cleancache",
	Short: "Removes unused BOSH packages from the compilation cache.",
	Long: `
This command will inspect the compilation cache populated by its sibling "packages"
and remove all which are not required anymore.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		); err != nil {
			return err
		}

		return fissile.CleanCache(workPathCompilationDir)
	},
}

func init() {
	buildCmd.AddCommand(buildCleancacheCmd)
}
