package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var flagDocsMarkdownOutputDir string

// docsMarkdownCmd represents the markdown command
var docsMarkdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Generates markdown documentation for fissile.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

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

	docsMarkdownCmd.PersistentFlags().StringVarP(
		&flagDocsMarkdownOutputDir,
		"output-dir",
		"O",
		"./docs",
		"Specifies a location where markdown documentation will be generated.",
	)
}
