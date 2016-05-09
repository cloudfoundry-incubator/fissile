package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// docsManCmd represents the man command
var docsManCmd = &cobra.Command{
	Use:   "man",
	Short: "Generates man pages for fissile.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		fmt.Println("man called")
	},
}

func init() {
	docsCmd.AddCommand(docsManCmd)
}
