package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/frankie-seb/sinatra/config"
	in "github.com/frankie-seb/sinatra/internal"
	"github.com/urfave/cli/v2"
)

var (
	initCmd = &cli.Command{
		Name:    "init",
		Aliases: []string{"initialize", "initialise", "create"},
		Usage:   "Initialize a Sinatra Project",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose, v", Usage: "show logs"},
			&cli.StringFlag{Name: "config, c", Usage: "the config filename"},
			&cli.StringFlag{Name: "server", Usage: "where to write the server stub to", Value: "server.go"},
			&cli.StringFlag{Name: "schema", Usage: "where to write the schema stub to", Value: "graph/schema.graphqls"},
		},
		Action: func(ctx *cli.Context) error {
			err := initializeProject(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("Your Sinatra ORM is ready")
			return nil
		},
	}
)

func initializeProject(ctx *cli.Context) error {
	configFilename := ctx.String("config")

	pkgName := in.ImportPathForDir(".")
	if pkgName == "" {
		return fmt.Errorf("unable to determine import path for current directory, you probably need to run go mod init first")
	}

	if !configExists(configFilename) {
		if err := initConfig(configFilename, pkgName); err != nil {
			return err
		}
	}

	return nil
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

func initConfig(configFilename string, pkgName string) error {
	if configFilename == "" {
		configFilename = "config.yml"
	}

	if err := os.MkdirAll(filepath.Dir(configFilename), 0755); err != nil {
		return fmt.Errorf("unable to create config dir: " + err.Error())
	}

	c, err := in.GetTemplateContent("config")
	if err != nil {
		return fmt.Errorf("could not load template: %v", err)
	}

	tpl, err := template.New("").Parse(c)
	if err != nil {
		return fmt.Errorf("parse: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, pkgName); err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(configFilename, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("unable to write cfg file: " + err.Error())
	}

	return nil
}
