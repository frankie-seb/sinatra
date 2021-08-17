package cmd

import (
	"fmt"
	"os"

	"github.com/frankie-seb/sinatra/config"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

var (
	generateCmd = &cli.Command{
		Name:  "generate",
		Usage: "Generate the Sinatra ORM",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose, v", Usage: "show logs"},
			&cli.StringFlag{Name: "config, c", Usage: "the config filename"},
			&cli.StringFlag{Name: "server", Usage: "where to write the server stub to", Value: "server.go"},
			&cli.StringFlag{Name: "schema", Usage: "where to write the schema stub to", Value: "graph/schema.graphqls"},
		},
		Action: func(ctx *cli.Context) error {
			var cfg *config.Config
			var err error
			if configFilename := ctx.String("config"); configFilename != "" {
				cfg, err = config.LoadConfig(configFilename)
				if err != nil {
					return err
				}
			} else {
				cfg, err = config.LoadConfigFromDefaultLocations()
				if os.IsNotExist(errors.Cause(err)) {
					cfg, err = config.LoadDefaultConfig()
				}

				if err != nil {
					return err
				}
			}

			fmt.Println("CGF", cfg)
			// Run sqlboiler
			// Run generator

			// if err = api.Generate(cfg); err != nil {
			// 	return err
			// }
			return nil
		},
	}
)
