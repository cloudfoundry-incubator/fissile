package cmd

import (
	"code.cloudfoundry.org/fissile/kube"
	"code.cloudfoundry.org/fissile/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBuildKubeOutputDir       string
	flagBuildKubeUseMemoryLimits bool
	flagBuildKubeUseCPULimits    bool
	flagBuildKubeTagExtra        string
)

// buildKubeCmd represents the kube command
var buildKubeCmd = &cobra.Command{
	Use:   "kube",
	Short: "Creates Kubernetes configuration files.",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		flagBuildKubeOutputDir = buildKubeViper.GetString("output-dir")
		flagBuildKubeUseMemoryLimits = buildKubeViper.GetBool("use-memory-limits")
		flagBuildKubeUseCPULimits = buildKubeViper.GetBool("use-cpu-limits")
		flagBuildKubeTagExtra = buildKubeViper.GetString("tag-extra")

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
			OutputDir:       flagBuildKubeOutputDir,
			Registry:        fissile.Options.DockerRegistry,
			Username:        fissile.Options.DockerUsername,
			Password:        fissile.Options.DockerPassword,
			Organization:    fissile.Options.DockerOrganization,
			Repository:      fissile.Options.RepositoryPrefix,
			UseMemoryLimits: flagBuildKubeUseMemoryLimits,
			UseCPULimits:    flagBuildKubeUseCPULimits,
			FissileVersion:  fissile.Version,
			Opinions:        opinions,
			CreateHelmChart: false,
			TagExtra:        flagBuildKubeTagExtra,
		}

		return fissile.GenerateKube(settings)
	},
}
var buildKubeViper = viper.New()

func init() {
	initViper(buildKubeViper)

	buildCmd.AddCommand(buildKubeCmd)

	buildKubeCmd.PersistentFlags().StringP(
		"output-dir",
		"",
		".",
		"Kubernetes configuration files will be written to this directory",
	)

	buildKubeCmd.PersistentFlags().BoolP(
		"use-memory-limits",
		"",
		true,
		"Include memory limits when generating kube configurations",
	)

	buildKubeCmd.PersistentFlags().BoolP(
		"use-cpu-limits",
		"",
		true,
		"Include cpu limits when generating helm chart",
	)

	buildKubeCmd.PersistentFlags().StringP(
		"tag-extra",
		"",
		"",
		"Additional information to use in computing the image tags",
	)

	buildKubeViper.BindPFlags(buildKubeCmd.PersistentFlags())
}
