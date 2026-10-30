package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"

	"github.com/wuhan005/sayrud/internal/appconst"
	"github.com/wuhan005/sayrud/internal/cmd"
)

func main() {
	app := &cli.Command{
		Name:           "Sayrud",
		Description:    "Yet another collaborative spreadsheet",
		Version:        appconst.BuildVersion,
		DefaultCommand: cmd.Web.Name,
		Commands: []*cli.Command{
			cmd.Web,
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		logrus.WithError(err).Fatal("Failed to run")
	}
}
