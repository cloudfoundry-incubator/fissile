package cmd

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/fissile/app"
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
		err := fissile.LoadManifest()
		if err != nil {
			return err
		}

		switch fissile.Options.OutputFormat {
		case app.OutputFormatHuman:
			err := fissile.ListJobs()
			if err != nil {
				return err
			}
			return fissile.ListPackages()

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
			if fissile.Options.OutputFormat == app.OutputFormatJSON {
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
			return fmt.Errorf("Invalid output format '%s', expected one of human, json, or yaml", fissile.Options.OutputFormat)
		}
	},
}

func init() {
	showCmd.AddCommand(showReleaseCmd)
}
