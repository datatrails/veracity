package veracity

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	app := &cli.App{
		Usage: "common read only operations on datatrails merklelog verifiable data",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "loglevel", Value: "NOOP"},
			&cli.Int64Flag{Name: "height", Value: 14, Usage: "override the massif height"},
			&cli.StringFlag{
				Name: "account", Aliases: []string{"s"},
				Usage: fmt.Sprintf("the azure storage account. defaults to `%s' and triggers use of emulator url", AzuriteStorageAccount),
			},
			&cli.StringFlag{
				Name: "container", Aliases: []string{"c"},
				Usage: "the azure storage container. this is necessary when using the azurite storage emulator",
				Value: DefaultContainer,
			},

			&cli.StringFlag{
				Name: "url", Aliases: []string{"u"},
			},
			&cli.StringFlag{
				Name: "tenant", Aliases: []string{"t"},
			},
			&cli.StringFlag{
				Name: "bug", Usage: "specify a bug number to enable a work around or special behaviour",
			},
			&cli.BoolFlag{
				Name: "envauth", Usage: "set to enable authorization from the environment (not all commands support this)",
			},
			&cli.StringFlag{
				Name: "logdir", Usage: "local directory containing log files",
			},
		},
	}
	return app
}

func AddCommands(app *cli.App) *cli.App {
	app.Commands = append(app.Commands, NewNodeCmd())
	app.Commands = append(app.Commands, NewEventsVerifyCmd())

	// Note: this command is not implemented
	// app.Commands = append(app.Commands, NewProveCmd())
	app.Commands = append(app.Commands, NewNodeScanCmd())
	app.Commands = append(app.Commands, NewDiagCmd())
	app.Commands = append(app.Commands, NewEventDiagCmd())
	app.Commands = append(app.Commands, NewMassifsCmd())
	app.Commands = append(app.Commands, NewLogWatcherCmd())
	app.Commands = append(app.Commands, NewLogTailCmd())
	return app
}
