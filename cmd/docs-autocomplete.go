package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// docsAutocompleteCmd represents the autocomplete command
var docsAutocompleteCmd = &cobra.Command{
	Use:   "autocomplete",
	Short: "Generates bash auto-complete scripts.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		fmt.Println("autocomplete called")
	},
}

func init() {
	docsCmd.AddCommand(docsAutocompleteCmd)
}
