package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// showLayerCmd represents the layer command
var showLayerCmd = &cobra.Command{
	Use:   "layer",
	Short: "Displays information about all the docker layers used by fissile.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fissile.ShowBaseImage(flagRepository)
	},
}

func init() {
	showCmd.AddCommand(showLayerCmd)
	viper.BindPFlags(showLayerCmd.PersistentFlags())
}
