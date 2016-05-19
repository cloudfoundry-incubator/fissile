package cmd

import (
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Has subcommands that display information about build artifacts.",
}

func init() {
	RootCmd.AddCommand(showCmd)
}
