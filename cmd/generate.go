package cmd

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	gqlcon "github.com/99designs/gqlgen/codegen/config"
	"github.com/frankie-seb/sinatra/config"
	"github.com/frankie-seb/sinatra/plugins/helpers"
	"github.com/frankie-seb/sinatra/plugins/resolvers"
	"github.com/frankie-seb/sinatra/plugins/schema"
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
			var gqlcfg *gqlcon.Config
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

			// Generate the schema
			h := &schema.HooksConfig{}
			if err := schema.SchemaWrite(cfg, h); err != nil {
				fmt.Println("error while trying to generate schema")
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(3)
			}

			// Generate the gqlgen config
			gqlcfg, err = config.LoadGqlgenConfig(cfg)

			if err != nil {
				fmt.Println("error while trying to generate the config")
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(3)
			}

			// Run generator
			if err = api.Generate(gqlcfg,
				api.AddPlugin(helpers.NewHelperPlugin(
					cfg,
				)),
				api.AddPlugin(resolvers.NewResolverPlugin(
					cfg,
				)),
			); err != nil {
				fmt.Println("error while trying run sinatra")
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(3)
			}

			return nil
		},
	}
)
