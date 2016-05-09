package cmd

import (
	"github.com/spf13/cobra"
)

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Has subcommands to create documentation for fissile.",
	Long:  ``,
}

func init() {
	RootCmd.AddCommand(docsCmd)
}
