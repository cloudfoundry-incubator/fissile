package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/SUSE/fissile/app"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

// showReleaseCmd represents the release command
var showReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Displays information about BOSH releases.",
	Long: `
Displays a report of all jobs and packages in all referenced releases.
The report contains the name, version, description and counts of jobs and packages.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Show job information

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		outputFormat := app.OutputFormat(flagOutputFormat)

		switch outputFormat {
		case app.OutputFormatHuman:
			if err := fissile.ListJobs(flagVerbose); err != nil {
				return err
			}

			return fissile.ListPackages(flagVerbose)

		case app.OutputFormatJSON, app.OutputFormatYAML:
			releases, err := fissile.SerializeReleases()
			if err != nil {
				return err
			}

			jobs, err := fissile.SerializeJobs()
			if err != nil {
				return err
			}

			pkgs, err := fissile.SerializePackages()
			if err != nil {
				return err
			}

			data := map[string]interface{}{
				"releases": releases,
				"jobs":     jobs,
				"packages": pkgs,
			}

			var buf []byte
			if outputFormat == app.OutputFormatJSON {
				buf, err = json.Marshal(data)
			} else {
				buf, err = yaml.Marshal(data)
			}

			if err != nil {
				return err
			}
			fissile.UI.Printf("%s", buf)
			return nil

		default:
			return fmt.Errorf("Invalid output format '%s', expected one of human, json, or yaml", outputFormat)
		}
	},
}

func init() {
	showCmd.AddCommand(showReleaseCmd)
}
