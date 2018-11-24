package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Has subcommands to build all images and necessary artifacts.",
	Long: `
This command has various subcommands to build artifacts.

The ` + "`--output-graph`" + ` flag is used to generate a graphviz-style DOT
language file for troubleshooting purposes.
	`,
}
var buildViper = viper.New()

func init() {
	initViper(buildViper)

	RootCmd.AddCommand(buildCmd)

	buildCmd.PersistentFlags().StringP(
		"output-graph",
		"",
		"",
		"Output a graphviz graph to the given file name",
	)

	buildViper.BindPFlags(buildCmd.PersistentFlags())
}
