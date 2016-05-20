package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
)

var flagDocsManOutputDir string

// docsManCmd represents the man command
var docsManCmd = &cobra.Command{
	Use:   "man",
	Short: "Generates man pages for fissile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		flagDocsManOutputDir = viper.GetString("man-output-dir")

		if flagDocsManOutputDir, err = absolutePath(
			flagDocsManOutputDir,
		); err != nil {
			return err
		}

		header := doc.GenManHeader{
			Section: "1",
			Manual:  "Fissile Manual",
			Source:  fmt.Sprintf("Fissile %s", version),
		}

		return doc.GenManTree(RootCmd, &header, flagDocsManOutputDir)
	},
}

func init() {
	docsCmd.AddCommand(docsManCmd)

	docsManCmd.PersistentFlags().StringP(
		"man-output-dir",
		"O",
		"./man",
		"Specifies a location where man pages will be generated.",
	)

	viper.BindPFlags(docsManCmd.PersistentFlags())
}
