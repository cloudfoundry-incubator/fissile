package cmd

import (
	"code.cloudfoundry.org/fissile/kube"
	"code.cloudfoundry.org/fissile/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildHelmOutputDir       string
	flagBuildHelmUseMemoryLimits bool
	flagBuildHelmUseCPULimits    bool
	flagBuildHelmTagExtra        string
	flagBuildHelmAuthType        string
)

// buildHelmCmd represents the helm command
var buildHelmCmd = &cobra.Command{
	Use:   "helm",
	Short: "Creates Helm chart.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		flagBuildHelmOutputDir = buildHelmViper.GetString("output-dir")
		flagBuildHelmUseMemoryLimits = buildHelmViper.GetBool("use-memory-limits")
		flagBuildHelmUseCPULimits = buildHelmViper.GetBool("use-cpu-limits")
		flagBuildHelmTagExtra = buildHelmViper.GetString("tag-extra")
		flagBuildHelmAuthType = buildHelmViper.GetString("auth-type")

		err := fissile.GraphBegin(buildViper.GetString("output-graph"))
		if err != nil {
			return err
		}

		err = fissile.LoadManifest()
		if err != nil {
			return err
		}

		opinions, err := model.NewOpinions(
			fissile.Options.LightOpinions,
			fissile.Options.DarkOpinions,
		)
		if err != nil {
			return err
		}

		settings := kube.ExportSettings{
			OutputDir:       flagBuildHelmOutputDir,
			Registry:        fissile.Options.DockerRegistry,
			Username:        fissile.Options.DockerUsername,
			Password:        fissile.Options.DockerPassword,
			Organization:    fissile.Options.DockerOrganization,
			Repository:      fissile.Options.Repository,
			UseMemoryLimits: flagBuildHelmUseMemoryLimits,
			UseCPULimits:    flagBuildHelmUseCPULimits,
			FissileVersion:  fissile.Version,
			Opinions:        opinions,
			CreateHelmChart: true,
			TagExtra:        flagBuildHelmTagExtra,
			AuthType:        flagBuildHelmAuthType,
		}

		return fissile.GenerateKube(settings)
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
