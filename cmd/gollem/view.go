package main

import (
	"context"
	"fmt"
	"strings"

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
				Name:    "gs",
				Sources: cli.EnvVars("GOLLEM_VIEW_GS"),
				Usage:   "Google Cloud Storage URI (e.g. gs://bucket/prefix/)",
			},
			&cli.BoolFlag{
				Name:    "no-browser",
				Sources: cli.EnvVars("GOLLEM_VIEW_NO_BROWSER"),
				Usage:   "Do not open browser automatically",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir := cmd.String("dir")
			gs := cmd.String("gs")

			if dir == "" && gs == "" {
				return fmt.Errorf("either --dir or --gs must be specified")
			}
			if dir != "" && gs != "" {
				return fmt.Errorf("--dir and --gs are mutually exclusive")
			}

			var src traceSource
			if dir != "" {
				src = newLocalSource(dir)
			} else {
				bucket, prefix, err := parseGSURI(gs)
				if err != nil {
					return err
				}
				src, err = newCSSource(ctx, bucket, prefix)
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

// parseGSURI parses a gs:// URI into bucket and prefix.
// e.g. "gs://my-bucket/path/to/traces/" -> ("my-bucket", "path/to/traces/")
func parseGSURI(uri string) (bucket, prefix string, err error) {
	if !strings.HasPrefix(uri, "gs://") {
		return "", "", fmt.Errorf("invalid GCS URI %q: must start with gs://", uri)
	}
	path := strings.TrimPrefix(uri, "gs://")
	if path == "" {
		return "", "", fmt.Errorf("invalid GCS URI %q: bucket name is required", uri)
	}

	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]
	if bucket == "" {
		return "", "", fmt.Errorf("invalid GCS URI %q: bucket name is empty", uri)
	}
	if len(parts) > 1 {
		prefix = parts[1]
	}
	return bucket, prefix, nil
}
