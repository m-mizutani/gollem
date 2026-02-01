package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func viewCommand() *cli.Command {
	return &cli.Command{
		Name:  "view",
		Usage: "Start trace viewer web server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Value:   ":18900",
				Sources: cli.EnvVars("GOLLEM_VIEW_ADDR"),
				Usage:   "Server listen address",
			},
			&cli.StringFlag{
				Name:    "dir",
				Sources: cli.EnvVars("GOLLEM_VIEW_DIR"),
				Usage:   "Local directory containing trace JSON files",
			},
			&cli.StringFlag{
				Name:    "bucket",
				Sources: cli.EnvVars("GOLLEM_VIEW_BUCKET"),
				Usage:   "Google Cloud Storage bucket name",
			},
			&cli.StringFlag{
				Name:    "prefix",
				Sources: cli.EnvVars("GOLLEM_VIEW_PREFIX"),
				Usage:   "Google Cloud Storage object prefix",
			},
			&cli.BoolFlag{
				Name:    "no-browser",
				Sources: cli.EnvVars("GOLLEM_VIEW_NO_BROWSER"),
				Usage:   "Do not open browser automatically",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir := cmd.String("dir")
			bucket := cmd.String("bucket")

			if dir == "" && bucket == "" {
				return fmt.Errorf("either --dir or --bucket must be specified")
			}
			if dir != "" && bucket != "" {
				return fmt.Errorf("--dir and --bucket are mutually exclusive")
			}

			var src traceSource
			if dir != "" {
				src = newLocalSource(dir)
			} else {
				var err error
				src, err = newCSSource(ctx, bucket, cmd.String("prefix"))
				if err != nil {
					return fmt.Errorf("failed to create Cloud Storage source: %w", err)
				}
			}

			opts := []serverOption{
				withAddr(cmd.String("addr")),
				withSource(src),
			}
			if cmd.Bool("no-browser") {
				opts = append(opts, withNoBrowser())
			}

			s := newServer(opts...)
			return s.start(ctx)
		},
	}
}
