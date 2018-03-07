package cmd

import (
	"github.com/SUSE/fissile/kube"
	"github.com/SUSE/fissile/model"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildHelmOutputDir           string
	flagBuildHelmDefaultEnvFiles     []string
	flagBuildHelmUseMemoryLimits     bool
	flagBuildHelmUseCPULimits        bool
	flagBuildHelmTagExtra            string
	flagBuildHelmUseSecretsGenerator bool
	flagBuildHelmAuthType            string
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
		flagBuildHelmUseCPULimits = buildHelmViper.GetBool("use-cpu-limits")
		flagBuildHelmTagExtra = buildHelmViper.GetString("tag-extra")
		flagBuildHelmUseSecretsGenerator = buildHelmViper.GetBool("use-secrets-generator")
		flagBuildOutputGraph = buildViper.GetString("output-graph")
		flagBuildHelmAuthType = buildHelmViper.GetString("auth-type")

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

		settings := kube.ExportSettings{
			OutputDir:           flagBuildHelmOutputDir,
			Registry:            flagDockerRegistry,
			Username:            flagDockerUsername,
			Password:            flagDockerPassword,
			Organization:        flagDockerOrganization,
			Repository:          flagRepository,
			UseMemoryLimits:     flagBuildHelmUseMemoryLimits,
			UseCPULimits:        flagBuildHelmUseCPULimits,
			FissileVersion:      fissile.Version,
			Opinions:            opinions,
			CreateHelmChart:     true,
			UseSecretsGenerator: flagBuildHelmUseSecretsGenerator,
			TagExtra:            flagBuildHelmTagExtra,
			AuthType:            flagBuildHelmAuthType,
		}

		if flagBuildOutputGraph != "" {
			err = fissile.GraphBegin(flagBuildOutputGraph)
			if err != nil {
				return err
			}
			defer func() {
				fissile.GraphEnd()
			}()
		}

		return fissile.GenerateKube(flagRoleManifest, flagBuildHelmDefaultEnvFiles, settings)
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

	buildHelmCmd.PersistentFlags().BoolP(
		"use-cpu-limits",
		"",
		true,
		"Include cpu limits when generating helm chart",
	)

	buildHelmCmd.PersistentFlags().StringP(
		"tag-extra",
		"",
		"",
		"Additional information to use in computing the image tags",
	)

	buildHelmCmd.PersistentFlags().BoolP(
		"use-secrets-generator",
		"",
		false,
		"Passwords will not be set by helm templates, but all secrets with a generator will be set/updated at runtime via a generator job like https://github.com/SUSE/scf-seret-generator",
	)

	buildHelmCmd.PersistentFlags().StringP(
		"auth-type",
		"",
		"",
		"Sets the Kubernetes auth type",
	)

	buildHelmViper.BindPFlags(buildHelmCmd.PersistentFlags())
}
