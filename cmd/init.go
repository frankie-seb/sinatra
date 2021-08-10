package cmd

import (
	"fmt"
	"os"
	"path"

	"github.com/FrankieHealth/be-base/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pkgName string

	initCmd = &cobra.Command{
		Use:     "init [name]",
		Aliases: []string{"initialize", "initialise", "create"},
		Short:   "Initialize a Sinatra Project",
		Long:    "Initialize a Sinatra Project",
		Run: func(_ *cobra.Command, args []string) {
			projectPath, err := initializeProject(args)
			cobra.CheckErr(err)
			fmt.Printf("Your Sinatra ORM is ready at\n%s\n", projectPath)
		},
	}
)

func init() {
	initCmd.Flags().BoolP("verbose", "d", false, "Debug mode prints stack traces on error")
}

func initializeProject(args []string) (string, error) {
	pkgName := code.ImportPathForDir(".")
	if pkgName == "" {
		return fmt.Errorf("unable to determine import path for current directory, you probably need to run go mod init first")
	}

	if !configExists(configFilename) {
		if err := initConfig(configFilename, pkgName); err != nil {
			return err
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if len(args) > 0 {
		if args[0] != "." {
			wd = fmt.Sprintf("%s/%s", wd, args[0])
		}
	}

	project := &Project{
		AbsolutePath: wd,
		PkgName:      pkgName,
		Viper:        viper.GetBool("useViper"),
		AppName:      path.Base(pkgName),
	}

	if err := project.Create(); err != nil {
		return "", err
	}

	return project.AbsolutePath, nil
}

func configExists(configFilename string) bool {
	var cfg *config.Config

	if configFilename != "" {
		cfg, _ = config.LoadConfig(configFilename)
	} else {
		cfg, _ = config.LoadConfigFromDefaultLocations()
	}
	return cfg != nil
}
