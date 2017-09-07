package cmd

import (
	"github.com/SUSE/fissile/model"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildHelmOutputDir       string
	flagBuildHelmDefaultEnvFiles []string
	flagBuildHelmUseMemoryLimits bool
)

// buildHelmCmd represents the helm command
var buildHelmCmd = &cobra.Command{
	Use:   "helm",
	Short: "Creates Helm chart.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {

		flagBuildHelmOutputDir = buildHelmViper.GetString("output-dir")
		flagBuildHelmDefaultEnvFiles = splitNonEmpty(buildHelmViper.GetString("defaults-file"), ",")
		flagBuildHelmUseMemoryLimits = buildHelmViper.GetBool("use-memory-limits")

		err := fissile.LoadReleases(
			flagRelease,
			flagReleaseName,
			flagReleaseVersion,
			flagCacheDir,
		)
		if err != nil {
			return err
		}

		opinions, err := model.NewOpinions(
			flagLightOpinions,
			flagDarkOpinions,
		)
		if err != nil {
			return err
		}

		return fissile.GenerateKube(
			flagRoleManifest,
			flagBuildHelmOutputDir,
			flagRepository,
			flagDockerRegistry,
			flagDockerOrganization,
			fissile.Version,
			flagBuildHelmDefaultEnvFiles,
			flagBuildHelmUseMemoryLimits,
			true,
			opinions,
		)
	},
}
var buildHelmViper = viper.New()

func init() {
	initViper(buildHelmViper)

	buildCmd.AddCommand(buildHelmCmd)

	buildHelmCmd.PersistentFlags().StringP(
		"output-dir",
		"",
		".",
		"Helm chart files will be written to this directory",
	)

	buildHelmCmd.PersistentFlags().StringP(
		"defaults-file",
		"D",
		"",
		"Env files that contain defaults for the configuration variables",
	)

	buildHelmCmd.PersistentFlags().BoolP(
		"use-memory-limits",
		"",
		true,
		"Include memory limits when generating helm chart",
	)

	buildHelmViper.BindPFlags(buildHelmCmd.PersistentFlags())
}
