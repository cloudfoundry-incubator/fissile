package main

import (
	"log"
	"os"
	"runtime"

	"github.com/hpcloud/fissile/app"

	"github.com/codegangsta/cli"
	"github.com/fatih/color"
)

func main() {
	if runtime.GOOS == "windows" {
		log.SetOutput(color.Output)
	}

	fissile := app.NewFissileApp()

	app := cli.NewApp()
	app.Name = "fissile"
	app.Usage = "Use fissile to break apart a BOSH release."

	app.Commands = []cli.Command{
		{
			Name:    "download-release",
			Aliases: []string{"dr"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "URL for a bosh release.",
				},
				cli.StringFlag{
					Name:  "path, p",
					Usage: "Target path for extracting the release",
				},
			},
			Usage: "Download a BOSH release",
			Action: func(c *cli.Context) {
			},
		},
		{
			Name:    "list-jobs",
			Aliases: []string{"lj"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
					Value: ".",
				},
			},
			Usage: "List all jobs in a BOSH release",
			Action: func(c *cli.Context) {
				fissile.ListJobs(c.String("release"))
			},
		},
		{
			Name:    "list-packages",
			Aliases: []string{"lp"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
				},
			},
			Usage: "List all packages in a BOSH release",
			Action: func(c *cli.Context) {
				fissile.ListPackages(c.String("release"))
			},
		},
		{
			Name:    "list-configs",
			Aliases: []string{"lc"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
				},
			},
			Usage: "List all configurations for all jobs in a release",
			Action: func(c *cli.Context) {
				fissile.ListFullConfiguration(c.String("release"))
			},
		},
		{
			Name:    "print-templates",
			Aliases: []string{"pt"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
				},
			},
			Usage: "Print all template for all jobs in a release",
			Action: func(c *cli.Context) {
				fissile.PrintAllTemplateContents(c.String("release"))
			},
		},
		{
			Name:    "show-baseimage",
			Aliases: []string{"sb"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "docker-endpoint, d",
					Value: "unix:///var/run/docker.sock",
					Usage: "Docker endpoint.",
				},
				cli.StringFlag{
					Name:  "base-image, b",
					Value: "ubuntu:14.04",
					Usage: "Base image.",
				},
			},
			Usage: "Show information about a base docker image",
			Action: func(c *cli.Context) {
				fissile.ShowBaseImage(c.String("docker-endpoint"), c.String("base-image"))
			},
		},
		{
			Name:    "prepare-compilationbase",
			Aliases: []string{"pc"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "docker-endpoint, d",
					Value: "unix:///var/run/docker.sock",
					Usage: "Docker endpoint.",
				},
				cli.StringFlag{
					Name:  "base-image, b",
					Value: "ubuntu:14.04",
					Usage: "Base image.",
				},
				cli.StringFlag{
					Name:  "repository, p",
					Value: "fissile",
					Usage: "Repository name.",
				},
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
				},
			},
			Usage:       "Prepare a compilation base image",
			Description: "The name of the created image will be <REPOSITORY>:<RELEASE_NAME>-<RELEASE_VERSION>-cbase. If the image already exists, this command does nothing.",
			Action: func(c *cli.Context) {
				fissile.CreateBaseCompilationImage(
					c.String("docker-endpoint"),
					c.String("base-image"),
					c.String("release"),
					c.String("repository"),
				)
			},
		},
		{
			Name:    "compile-packages",
			Aliases: []string{"cp"},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "docker-endpoint, d",
					Value: "unix:///var/run/docker.sock",
					Usage: "Docker endpoint.",
				},
				cli.StringFlag{
					Name:  "base-image, b",
					Value: "ubuntu:14.04",
					Usage: "Base image.",
				},
				cli.StringFlag{
					Name:  "repository, p",
					Value: "fissile",
					Usage: "Repository name.",
				},
				cli.StringFlag{
					Name:  "release, r",
					Usage: "Path to a BOSH release.",
				},
				cli.StringFlag{
					Name:  "target, t",
					Usage: "Path to the location of the compiled packages.",
				},
				cli.IntFlag{
					Name:  "workers, w",
					Value: 4,
					Usage: "Number of compiler workers to use.",
				},
			},
			Usage:       "Compile packages",
			Description: "Compiles packages from the release using parallel workers",
			Action: func(c *cli.Context) {
				fissile.Compile(
					c.String("docker-endpoint"),
					c.String("base-image"),
					c.String("release"),
					c.String("repository"),
					c.String("target"),
					c.Int("workers"),
				)
			},
		},
	}

	app.Run(os.Args)
}
