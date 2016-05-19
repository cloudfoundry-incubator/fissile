package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
)

var (
	flagDocsMarkdownOutputDir string
)

// docsMarkdownCmd represents the markdown command
var docsMarkdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Generates markdown documentation for fissile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		flagDocsMarkdownOutputDir = viper.GetString("output-dir")

		if flagDocsMarkdownOutputDir, err = absolutePath(
			flagDocsMarkdownOutputDir,
		); err != nil {
			return err
		}

		return doc.GenMarkdownTree(RootCmd, flagDocsMarkdownOutputDir)
	},
}

func init() {
	docsCmd.AddCommand(docsMarkdownCmd)

	docsMarkdownCmd.PersistentFlags().StringP(
		"output-dir",
		"O",
		"./docs",
		"Specifies a location where markdown documentation will be generated.",
	)

	viper.BindPFlags(docsMarkdownCmd.PersistentFlags())
}
