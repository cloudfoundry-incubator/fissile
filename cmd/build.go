package cmd

import (
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Has subcommands to build all images and necessary artifacts.",
	Long:  ``,
}

func init() {
	RootCmd.AddCommand(buildCmd)
}
