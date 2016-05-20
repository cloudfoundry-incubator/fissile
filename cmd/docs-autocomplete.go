package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var flagDocsAutocompleteOutputFile string

// docsAutocompleteCmd represents the autocomplete command
var docsAutocompleteCmd = &cobra.Command{
	Use:   "autocomplete",
	Short: "Generates a bash auto-complete script.",
	Long: `
You can source the script to provide tab completion in bash.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		flagDocsAutocompleteOutputFile = viper.GetString("output-file")

		if flagDocsAutocompleteOutputFile, err = absolutePath(
			flagDocsAutocompleteOutputFile,
		); err != nil {
			return err
		}

		return RootCmd.GenBashCompletionFile(flagDocsAutocompleteOutputFile)
	},
}

func init() {
	docsCmd.AddCommand(docsAutocompleteCmd)

	docsAutocompleteCmd.PersistentFlags().StringP(
		"output-file",
		"O",
		"./fissile-autocomplete.sh",
		"Specifies a file location where a bash autocomplete script will be generated.",
	)

	viper.BindPFlags(docsAutocompleteCmd.PersistentFlags())
}
