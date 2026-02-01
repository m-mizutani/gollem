package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "gollem",
		Usage: "gollem CLI tools",
		Commands: []*cli.Command{
			viewCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", slog.Any("error", err))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
