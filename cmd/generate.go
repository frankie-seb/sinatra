package cmd

import (
	"os"

	"github.com/frankie-seb/sinatra/config"
	"github.com/frankie-seb/sinatra/plugins/sqlboiler"
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
			&cli.StringFlag{Name: "skip-db, sdb", Usage: "where to write the server stub to"},
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

			// Run db models generation
			if skipDB := ctx.String("skip-db"); skipDB == "" {
				if err = sqlboiler.Run(cfg); err != nil {
					return err

				}
			}
			// Run generator

			// if err = api.Generate(cfg); err != nil {
			// 	return err
			// }
			return nil
		},
	}
)
